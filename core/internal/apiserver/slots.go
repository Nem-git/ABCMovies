package apiserver

import (
	"context"
	"strings"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/schema"
	"github.com/nem-git/abcmovies/core/internal/slotvalidation"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// slotKinds is every managed slot kind, in the order a list presents them.
var slotKinds = []corev1.SlotKind{
	corev1.SlotKind_SLOT_KIND_PROVIDER,
	corev1.SlotKind_SLOT_KIND_SINK,
	corev1.SlotKind_SLOT_KIND_CATALOGUE,
	corev1.SlotKind_SLOT_KIND_SUBTITLE_SOURCE,
	corev1.SlotKind_SLOT_KIND_DRM,
}

// CreateSlot stores a managed slot and applies it live (D8). Operator or the
// slot's owner. The slot is validated before it is stored; a duplicate id
// within the same kind and owner is rejected. The change is applied to the
// running stack before it is persisted: a slot the wiring rejects is never
// stored, so the store only ever holds slots that are live.
func (s *Server) CreateSlot(ctx context.Context, req *apiv1.CreateSlotRequest) (*apiv1.CreateSlotResponse, error) {
	if err := schema.ValidateCreateSlotRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if s.slotStore == nil {
		return nil, status.Error(codes.Unavailable, "slot store not configured")
	}
	caller, ok := UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	slot := req.GetSlot()
	if err := slotvalidation.ValidateSlot(slot); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	// The caller must be an operator to create an operator-owned slot, and
	// may only own a slot they create.
	if slot.GetOwnerUserId() == "" {
		if err := s.requireOperator(ctx); err != nil {
			return nil, err
		}
	} else if slot.GetOwnerUserId() != caller {
		return nil, status.Error(codes.PermissionDenied, "cannot create a slot owned by another user")
	}
	// Reject a duplicate. The id is the identity namespace, so it is globally
	// unique across every kind and owner — any existing holder of it is a
	// collision, not just a same-owner repeat.
	if existing, err := s.slotStore.GetByID(ctx, slot.GetId()); err != nil {
		return nil, status.Error(codes.Internal, "failed to check slot existence")
	} else if existing != nil {
		return nil, status.Error(codes.AlreadyExists, "slot id already exists")
	}
	current, err := s.listAllSlots(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read slots")
	}
	if err := s.applyLive(ctx, upsertSlot(current, slot)); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "failed to apply slot: "+err.Error())
	}
	if err := s.slotStore.Put(ctx, slot); err != nil {
		// The runtime is already live with the new set; put it back the way
		// it was so the store stays the single source of truth.
		_ = s.applyLive(ctx, current)
		return nil, status.Error(codes.Internal, "failed to store slot")
	}
	return &apiv1.CreateSlotResponse{Slot: slot}, nil
}

// GetSlot reads one managed slot. Operator or the slot's owner. A slot
// outside the caller's reach answers NotFound, indistinguishable from one
// that does not exist (PLAN.md §8.1).
func (s *Server) GetSlot(ctx context.Context, req *apiv1.GetSlotRequest) (*apiv1.GetSlotResponse, error) {
	if err := schema.ValidateGetSlotRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if s.slotStore == nil {
		return nil, status.Error(codes.Unavailable, "slot store not configured")
	}
	caller, ok := UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	ref := req.GetRef()
	slot, err := s.slotStore.Get(ctx, ref.GetKind(), ref.GetOwnerUserId(), ref.GetId())
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read slot")
	}
	if slot == nil {
		return nil, status.Error(codes.NotFound, "slot not found")
	}
	if !s.canManageSlot(ctx, slot, caller) {
		return nil, status.Error(codes.NotFound, "slot not found")
	}
	return &apiv1.GetSlotResponse{Slot: slot}, nil
}

// UpdateSlot replaces an existing managed slot and applies it live (D8).
// Operator or the slot's owner. The slot must already exist. Applied live
// before it is persisted, so a rejected update leaves the store and the live
// stack unchanged.
func (s *Server) UpdateSlot(ctx context.Context, req *apiv1.UpdateSlotRequest) (*apiv1.UpdateSlotResponse, error) {
	if err := schema.ValidateUpdateSlotRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if s.slotStore == nil {
		return nil, status.Error(codes.Unavailable, "slot store not configured")
	}
	caller, ok := UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	slot := req.GetSlot()
	if err := slotvalidation.ValidateSlot(slot); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	// The caller must be an operator to update an operator-owned slot, and may
	// only update a slot they own.
	if slot.GetOwnerUserId() == "" {
		if err := s.requireOperator(ctx); err != nil {
			return nil, err
		}
	} else if slot.GetOwnerUserId() != caller {
		return nil, status.Error(codes.PermissionDenied, "cannot update a slot owned by another user")
	}
	if existing, err := s.slotStore.Get(ctx, slot.GetKind(), slot.GetOwnerUserId(), slot.GetId()); err != nil {
		return nil, status.Error(codes.Internal, "failed to check slot existence")
	} else if existing == nil {
		return nil, status.Error(codes.NotFound, "slot not found")
	}
	current, err := s.listAllSlots(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read slots")
	}
	if err := s.applyLive(ctx, upsertSlot(current, slot)); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "failed to apply slot: "+err.Error())
	}
	if err := s.slotStore.Put(ctx, slot); err != nil {
		_ = s.applyLive(ctx, current)
		return nil, status.Error(codes.Internal, "failed to store slot")
	}
	return &apiv1.UpdateSlotResponse{Slot: slot}, nil
}

// DeleteSlot removes a managed slot (D8). Operator or the slot's owner. A
// slot outside the caller's reach answers NotFound. Applied live before it is
// persisted, so a rejected removal leaves the store and the live stack
// unchanged.
func (s *Server) DeleteSlot(ctx context.Context, req *apiv1.DeleteSlotRequest) (*apiv1.DeleteSlotResponse, error) {
	if err := schema.ValidateDeleteSlotRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if s.slotStore == nil {
		return nil, status.Error(codes.Unavailable, "slot store not configured")
	}
	caller, ok := UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	ref := req.GetRef()
	slot, err := s.slotStore.Get(ctx, ref.GetKind(), ref.GetOwnerUserId(), ref.GetId())
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read slot")
	}
	if slot == nil {
		return nil, status.Error(codes.NotFound, "slot not found")
	}
	if !s.canManageSlot(ctx, slot, caller) {
		return nil, status.Error(codes.NotFound, "slot not found")
	}
	current, err := s.listAllSlots(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read slots")
	}
	prospective := removeSlot(current, slot.GetKind(), slot.GetOwnerUserId(), slot.GetId())
	if err := s.applyLive(ctx, prospective); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "failed to apply slot removal: "+err.Error())
	}
	if err := s.slotStore.Delete(ctx, slot.GetKind(), slot.GetOwnerUserId(), slot.GetId()); err != nil {
		_ = s.applyLive(ctx, current)
		return nil, status.Error(codes.Internal, "failed to delete slot")
	}
	return &apiv1.DeleteSlotResponse{Slot: slot}, nil
}

// ListSlots lists managed slots. Operators see all; users see their own
// (D8). Optional kind and owner filters narrow the list.
func (s *Server) ListSlots(ctx context.Context, req *apiv1.ListSlotsRequest) (*apiv1.ListSlotsResponse, error) {
	if err := schema.ValidateListSlotsRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if s.slotStore == nil {
		return nil, status.Error(codes.Unavailable, "slot store not configured")
	}
	caller, ok := UserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	// Determine the owner filter: operators may list any owner; a regular
	// user is restricted to their own slots.
	ownerFilter := req.GetOwnerUserId()
	if !s.isOperator(ctx) {
		ownerFilter = caller
	}
	kind := req.GetKind()
	if kind == corev1.SlotKind_SLOT_KIND_UNSPECIFIED {
		// List every kind.
		var all []*corev1.Slot
		for _, k := range slotKinds {
			slots, err := s.slotStore.ListByKind(ctx, k, ownerFilter)
			if err != nil {
				return nil, status.Error(codes.Internal, "failed to list slots")
			}
			all = append(all, slots...)
		}
		return &apiv1.ListSlotsResponse{Slots: all}, nil
	}
	slots, err := s.slotStore.ListByKind(ctx, kind, ownerFilter)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list slots")
	}
	return &apiv1.ListSlotsResponse{Slots: slots}, nil
}

// listAllSlots reads every stored slot across kinds. The admin pipeline
// composes the prospective set from this, applies it live, and only then
// persists the single change, so a set the wiring rejects is never stored.
func (s *Server) listAllSlots(ctx context.Context) ([]*corev1.Slot, error) {
	var all []*corev1.Slot
	for _, k := range slotKinds {
		slots, err := s.slotStore.ListByKind(ctx, k, "")
		if err != nil {
			return nil, err
		}
		all = append(all, slots...)
	}
	return all, nil
}

// applyLive hands a prospective slot set to the running stack. A server built
// without the composition root has no reloader, so the apply is a no-op and
// the store write alone stands.
func (s *Server) applyLive(ctx context.Context, slots []*corev1.Slot) error {
	if s.slotReloader == nil {
		return nil
	}
	return s.slotReloader(ctx, slots)
}

// upsertSlot returns base with slot inserted or replaced at its identity —
// kind plus owner plus id, the same key the store uses.
func upsertSlot(base []*corev1.Slot, slot *corev1.Slot) []*corev1.Slot {
	out := make([]*corev1.Slot, 0, len(base)+1)
	replaced := false
	for _, x := range base {
		if sameSlot(x, slot) {
			out = append(out, slot)
			replaced = true
			continue
		}
		out = append(out, x)
	}
	if !replaced {
		out = append(out, slot)
	}
	return out
}

// removeSlot returns base without the slot identified by kind, owner, and id.
func removeSlot(base []*corev1.Slot, kind corev1.SlotKind, owner, id string) []*corev1.Slot {
	out := make([]*corev1.Slot, 0, len(base))
	for _, x := range base {
		if x.GetKind() == kind && x.GetOwnerUserId() == owner && x.GetId() == id {
			continue
		}
		out = append(out, x)
	}
	return out
}

// sameSlot reports whether two slots occupy the same store key: one id per
// kind per owner (D8).
func sameSlot(a, b *corev1.Slot) bool {
	return a.GetKind() == b.GetKind() && a.GetOwnerUserId() == b.GetOwnerUserId() && a.GetId() == b.GetId()
}

// canManageSlot reports whether the caller may see or change a slot: an
// operator may manage any slot, a regular user only their own. A slot with no
// owner is operator-owned, so only an operator passes (PLAN.md §8.1: a slot
// outside the caller's reach answers NotFound, indistinguishable from one
// that does not exist).
func (s *Server) canManageSlot(ctx context.Context, slot *corev1.Slot, caller string) bool {
	if s.isOperator(ctx) {
		return true
	}
	if slot.GetOwnerUserId() == "" {
		return false
	}
	return slot.GetOwnerUserId() == caller
}

// isOperator reports whether the caller carries the operator role. It never
// errors: an unreadable user store or a missing user means "not an operator".
func (s *Server) isOperator(ctx context.Context) bool {
	uid, ok := UserIDFromContext(ctx)
	if !ok {
		return false
	}
	username, found := strings.CutPrefix(uid, "user:")
	if !found {
		return false
	}
	if s.stores.Users == nil {
		return false
	}
	data, err := auth.NewStoreUserStore(s.stores.Users).GetUser(username)
	if err != nil {
		return false
	}
	return data.Role == auth.RoleOperator
}
