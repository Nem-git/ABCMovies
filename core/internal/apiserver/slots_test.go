package apiserver_test

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/slotstore"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// slotServer builds a server carrying a slot store and a user roster: "op"
// is an operator (owner of operator slots), "alice" a regular user.
func slotServer(t *testing.T) (*apiserver.Server, *slotstore.Store) {
	t.Helper()
	authenticator, session := testAuth(t)
	stores := testStores(t)
	srv := apiserver.NewServer(apiserver.Settings{
		Bus:           apiserver.NewInMemoryBus(),
		Stores:        stores,
		Authenticator: authenticator,
		Session:       session,
	})
	st, err := slotstore.New(store.NewInMemory())
	if err != nil {
		t.Fatalf("slotstore.New: %v", err)
	}
	srv.SetSlotStore(st)
	for name, role := range map[string]auth.Role{"op": auth.RoleOperator, "alice": auth.RoleUser} {
		if err := auth.NewStoreUserStore(stores.Users).PutUser(name, &auth.UserData{Role: role}); err != nil {
			t.Fatalf("PutUser %s: %v", name, err)
		}
	}
	return srv, st
}

func operatorSlotCtx(t *testing.T) context.Context {
	t.Helper()
	_, session := testAuth(t)
	return ctxAs(session, "user:op")
}

func userSlotCtx(t *testing.T) context.Context {
	t.Helper()
	_, session := testAuth(t)
	return ctxAs(session, "user:alice")
}

func providerSlotReq(id, owner string) *apiv1.CreateSlotRequest {
	return &apiv1.CreateSlotRequest{Slot: &corev1.Slot{
		Id:          id,
		Kind:        corev1.SlotKind_SLOT_KIND_PROVIDER,
		OwnerUserId: owner,
		Config: &corev1.Slot_Provider{Provider: &corev1.ProviderConfig{
			Adapter: "jellyfin", Server: "https://jf.example",
		}},
	}}
}

func sinkSlotReq(id, owner string) *apiv1.CreateSlotRequest {
	return &apiv1.CreateSlotRequest{Slot: &corev1.Slot{
		Id:          id,
		Kind:        corev1.SlotKind_SLOT_KIND_SINK,
		OwnerUserId: owner,
		Config:      &corev1.Slot_Sink{Sink: &corev1.SinkConfig{Adapter: "device"}},
	}}
}

// TestCreateSlot_Authz pins the ownership rules: an operator creates
// operator-owned (ownerless) slots; a regular user may only create a slot
// they own; nobody may create a slot owned by somebody else.
func TestCreateSlot_Authz(t *testing.T) {
	srv, _ := slotServer(t)

	// Operator creates an operator-owned slot.
	if _, err := srv.CreateSlot(operatorSlotCtx(t), providerSlotReq("jf", "")); err != nil {
		t.Fatalf("operator create operator slot: %v", err)
	}

	// A regular user cannot create an operator-owned slot.
	_, err := srv.CreateSlot(userSlotCtx(t), sinkSlotReq("device", ""))
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("user create operator slot = %v, want PermissionDenied", err)
	}

	// A regular user may create their own slot.
	if _, err := srv.CreateSlot(userSlotCtx(t), sinkSlotReq("device", "user:alice")); err != nil {
		t.Fatalf("user create own slot: %v", err)
	}

	// Nobody may create a slot owned by another user.
	_, err = srv.CreateSlot(operatorSlotCtx(t), sinkSlotReq("device", "user:alice"))
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("create slot owned by another = %v, want PermissionDenied", err)
	}
}

func TestCreateSlot_DuplicateIDAlreadyExists(t *testing.T) {
	srv, _ := slotServer(t)
	ctx := operatorSlotCtx(t)
	if _, err := srv.CreateSlot(ctx, providerSlotReq("jf", "")); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := srv.CreateSlot(ctx, providerSlotReq("jf", ""))
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("duplicate create = %v, want AlreadyExists", err)
	}
}

// TestGetSlot_HidesUnreachable pins that a slot outside the caller's reach is
// indistinguishable from one that does not exist (PLAN.md §8.1).
func TestGetSlot_HidesUnreachable(t *testing.T) {
	srv, _ := slotServer(t)
	seedOperatorSlot(t, srv, "jf")
	if _, err := srv.CreateSlot(userSlotCtx(t), providerSlotReq("personal", "user:alice")); err != nil {
		t.Fatalf("seed alice slot: %v", err)
	}

	ref := func(id, owner string) *apiv1.GetSlotRequest {
		return &apiv1.GetSlotRequest{Ref: &apiv1.SlotRef{
			Kind: corev1.SlotKind_SLOT_KIND_PROVIDER, OwnerUserId: owner, Id: id,
		}}
	}

	// A regular user cannot see an operator-owned slot.
	_, err := srv.GetSlot(userSlotCtx(t), ref("jf", ""))
	if status.Code(err) != codes.NotFound {
		t.Fatalf("user get operator slot = %v, want NotFound", err)
	}
	// Alice can read her own.
	if _, err := srv.GetSlot(userSlotCtx(t), ref("personal", "user:alice")); err != nil {
		t.Fatalf("alice get own slot: %v", err)
	}
	// The operator may read a user's slot.
	if _, err := srv.GetSlot(operatorSlotCtx(t), ref("personal", "user:alice")); err != nil {
		t.Fatalf("operator get user slot: %v", err)
	}

	// A missing slot is NotFound for everyone.
	_, err = srv.GetSlot(operatorSlotCtx(t), ref("nope", ""))
	if status.Code(err) != codes.NotFound {
		t.Fatalf("missing slot = %v, want NotFound", err)
	}
}

// TestSlotCRUDRoundTrip walks create → get → list → update → delete and
// asserts the store reflects each step.
func TestSlotCRUDRoundTrip(t *testing.T) {
	srv, _ := slotServer(t)
	ctx := operatorSlotCtx(t)

	created, err := srv.CreateSlot(ctx, providerSlotReq("jf", ""))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.GetSlot().GetId() != "jf" {
		t.Fatalf("created slot id = %q, want jf", created.GetSlot().GetId())
	}

	got, err := srv.GetSlot(ctx, &apiv1.GetSlotRequest{Ref: &apiv1.SlotRef{
		Kind: corev1.SlotKind_SLOT_KIND_PROVIDER, Id: "jf",
	}})
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.GetSlot().GetProvider().GetServer() != "https://jf.example" {
		t.Fatalf("round-trip server = %q", got.GetSlot().GetProvider().GetServer())
	}

	listed, err := srv.ListSlots(ctx, &apiv1.ListSlotsRequest{Kind: corev1.SlotKind_SLOT_KIND_PROVIDER})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(listed.GetSlots()) != 1 {
		t.Fatalf("listed = %d, want 1", len(listed.GetSlots()))
	}

	// Update changes the server.
	upd := providerSlotReq("jf", "")
	upd.GetSlot().GetProvider().Server = "https://jf-updated.example"
	if _, err := srv.UpdateSlot(ctx, &apiv1.UpdateSlotRequest{Slot: upd.GetSlot()}); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err = srv.GetSlot(ctx, &apiv1.GetSlotRequest{Ref: &apiv1.SlotRef{
		Kind: corev1.SlotKind_SLOT_KIND_PROVIDER, Id: "jf",
	}})
	if err != nil {
		t.Fatalf("get after update: %v", err)
	}
	if got.GetSlot().GetProvider().GetServer() != "https://jf-updated.example" {
		t.Fatalf("server = %q after update, want https://jf-updated.example", got.GetSlot().GetProvider().GetServer())
	}

	if _, err := srv.DeleteSlot(ctx, &apiv1.DeleteSlotRequest{Ref: &apiv1.SlotRef{
		Kind: corev1.SlotKind_SLOT_KIND_PROVIDER, Id: "jf",
	}}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := srv.GetSlot(ctx, &apiv1.GetSlotRequest{Ref: &apiv1.SlotRef{
		Kind: corev1.SlotKind_SLOT_KIND_PROVIDER, Id: "jf",
	}}); status.Code(err) != codes.NotFound {
		t.Fatalf("get after delete = %v, want NotFound", err)
	}
}

// TestListSlots_UserSeesOnlyOwn pins the list scoping: a regular user only
// sees their own slots even when asking for every owner.
func TestListSlots_UserSeesOnlyOwn(t *testing.T) {
	srv, _ := slotServer(t)
	seedOperatorSlot(t, srv, "op-sink")
	if _, err := srv.CreateSlot(userSlotCtx(t), sinkSlotReq("alice-sink", "user:alice")); err != nil {
		t.Fatalf("seed alice sink: %v", err)
	}

	// A regular user asking for every kind sees only their own.
	all, err := srv.ListSlots(userSlotCtx(t), &apiv1.ListSlotsRequest{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, s := range all.GetSlots() {
		if s.GetOwnerUserId() != "user:alice" {
			t.Fatalf("user saw slot owned by %q", s.GetOwnerUserId())
		}
	}
	if len(all.GetSlots()) != 1 {
		t.Fatalf("user listed %d slots, want 1", len(all.GetSlots()))
	}

	// The operator sees both.
	opAll, err := srv.ListSlots(operatorSlotCtx(t), &apiv1.ListSlotsRequest{})
	if err != nil {
		t.Fatalf("operator list: %v", err)
	}
	if len(opAll.GetSlots()) != 2 {
		t.Fatalf("operator listed %d slots, want 2", len(opAll.GetSlots()))
	}
}

// TestCreateSlot_AppliesLive pins the runtime pipeline (D8): the composed set
// is applied to the running stack before the store is written, and a wiring
// rejection refuses the create without storing the slot.
func TestCreateSlot_AppliesLive(t *testing.T) {
	srv, st := slotServer(t)
	ctx := operatorSlotCtx(t)

	var applied []*corev1.Slot
	srv.SetSlotReloader(func(_ context.Context, slots []*corev1.Slot) error {
		applied = append([]*corev1.Slot(nil), slots...)
		return nil
	})

	if _, err := srv.CreateSlot(ctx, providerSlotReq("jf", "")); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(applied) != 1 || applied[0].GetId() != "jf" {
		t.Fatalf("applied set = %+v, want the one created slot", applied)
	}
	if got, err := st.Get(ctx, corev1.SlotKind_SLOT_KIND_PROVIDER, "", "jf"); err != nil || got == nil {
		t.Fatalf("slot not persisted after live apply: got=%v err=%v", got, err)
	}
}

func TestCreateSlot_WiringRejectionIsNotStored(t *testing.T) {
	srv, st := slotServer(t)
	ctx := operatorSlotCtx(t)
	srv.SetSlotReloader(func(context.Context, []*corev1.Slot) error {
		return errors.New("wiring refused")
	})

	_, err := srv.CreateSlot(ctx, providerSlotReq("jf", ""))
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("create with refusing wiring = %v, want FailedPrecondition", err)
	}
	if got, _ := st.Get(ctx, corev1.SlotKind_SLOT_KIND_PROVIDER, "", "jf"); got != nil {
		t.Fatal("slot must not be stored when the wiring refuses it")
	}
}

// TestSlotAdmin_RequiresAuthAndStore pins the two preconditions every CRUD
// entry point shares: an authenticated caller and an armed slot store.
func TestSlotAdmin_RequiresAuthAndStore(t *testing.T) {
	authenticator, session := testAuth(t)
	srv := apiserver.NewServer(apiserver.Settings{
		Bus:           apiserver.NewInMemoryBus(),
		Stores:        testStores(t),
		Authenticator: authenticator,
		Session:       session,
	})
	// No store armed yet.
	if _, err := srv.ListSlots(ctxAs(session, "user:op"), &apiv1.ListSlotsRequest{}); status.Code(err) != codes.Unavailable {
		t.Fatalf("no store = %v, want Unavailable", err)
	}

	// Arm the store but present no authenticated caller.
	st, err := slotstore.New(store.NewInMemory())
	if err != nil {
		t.Fatalf("slotstore.New: %v", err)
	}
	srv.SetSlotStore(st)
	if _, err := srv.CreateSlot(context.Background(), providerSlotReq("jf", "")); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("no caller = %v, want Unauthenticated", err)
	}
}

// TestCreateSlot_InvalidRejected pins that slot-shape rules run at submit
// time, not only at config load.
func TestCreateSlot_InvalidRejected(t *testing.T) {
	srv, _ := slotServer(t)
	_, err := srv.CreateSlot(operatorSlotCtx(t), &apiv1.CreateSlotRequest{Slot: &corev1.Slot{
		Id:   "jf",
		Kind: corev1.SlotKind_SLOT_KIND_PROVIDER,
		Config: &corev1.Slot_Provider{Provider: &corev1.ProviderConfig{
			Adapter: "jellyfin", // server missing
		}},
	}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid slot = %v, want InvalidArgument", err)
	}
}

// TestCreateSlot_GloballyUniqueID pins that the slot id is the identity
// namespace (PLAN.md §1.25): it is globally unique across every kind and
// owner. A second slot claiming a taken id is refused with AlreadyExists —
// including across owners and across kinds — and is never stored.
func TestCreateSlot_GloballyUniqueID(t *testing.T) {
	srv, _ := slotServer(t)
	ctx := operatorSlotCtx(t)

	// The operator takes id "jf" as a provider.
	seedOperatorSlot(t, srv, "jf")

	// Another owner cannot reuse the same id.
	_, err := srv.CreateSlot(userSlotCtx(t), providerSlotReq("jf", "user:alice"))
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("cross-owner duplicate = %v, want AlreadyExists", err)
	}

	// A different kind cannot reuse it either.
	_, err = srv.CreateSlot(ctx, &apiv1.CreateSlotRequest{Slot: &corev1.Slot{
		Id: "jf", Kind: corev1.SlotKind_SLOT_KIND_CATALOGUE,
		Config: &corev1.Slot_Catalogue{Catalogue: &corev1.CatalogueConfig{
			Adapter: "tmdb", TokenEnv: "TMDB_TOKEN",
		}},
	}})
	if status.Code(err) != codes.AlreadyExists {
		t.Fatalf("cross-kind duplicate = %v, want AlreadyExists", err)
	}

	// Neither collision was stored.
	if got, _ := srv.GetSlot(ctx, &apiv1.GetSlotRequest{Ref: &apiv1.SlotRef{
		Kind: corev1.SlotKind_SLOT_KIND_PROVIDER, Id: "jf",
	}}); got == nil {
		t.Fatal("seeded operator slot vanished")
	}
}

// seedOperatorSlot creates an operator-owned provider slot through the
// operator.
func seedOperatorSlot(t *testing.T, srv *apiserver.Server, id string) {
	t.Helper()
	if _, err := srv.CreateSlot(operatorSlotCtx(t), providerSlotReq(id, "")); err != nil {
		t.Fatalf("seed operator slot %s: %v", id, err)
	}
}
