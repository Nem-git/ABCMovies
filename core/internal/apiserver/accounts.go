package apiserver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/schema"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CredentialProber validates a candidate linked-account credential against a
// provider and returns the provider session blob to vault (PLAN.md §3.5: the
// core never vaults material it has not confirmed works — the probe is the
// confirmation, and nothing that fails the probe is persisted). Each provider
// slot arms one prober; absent a prober for a provider, linking is
// Unavailable.
type CredentialProber interface {
	Probe(ctx context.Context, baseURL, username string, password []byte) ([]byte, error)
}

// SetProber arms the credential prober for one provider slot (PLAN.md §3.5).
func (s *Server) SetProber(provider string, p CredentialProber) {
	if provider == "" || p == nil {
		return
	}
	s.probers[provider] = p
}

// LinkAccount links a provider account to the caller (PLAN.md §3.5, §7.5):
// the credentials are first probed against the provider slot — nothing is
// vaulted that the probe rejected — and the confirmed session is vaulted
// under a freshly minted account id. The record is persisted alongside the
// session, so the link survives the widget's own session lifetime and is
// usable by slot provisioning at the next boot without a re-login
// (vault-first custody model). Sharing starts at link time and can change
// afterwards through UpdateAccount (§7.1: the owner can revoke a member at
// any time).
func (s *Server) LinkAccount(ctx context.Context, req *apiv1.LinkAccountRequest) (*apiv1.LinkAccountResponse, error) {
	if err := schema.ValidateLinkAccountRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if req.GetProvider() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider is required")
	}
	prober, ok := s.probers[req.GetProvider()]
	if !ok {
		return nil, status.Error(codes.Unavailable, "no live provider slot to probe: "+req.GetProvider())
	}
	uid, _ := UserIDFromContext(ctx)
	baseURL := strings.TrimRight(req.GetBaseUrl(), "/")
	vis := apiVisibility(req.GetVisibility())
	if err := s.validateSharing(ctx, vis, req.GetSharedWith()); err != nil {
		return nil, err
	}
	blob, err := prober.Probe(ctx, baseURL, req.GetPassword().GetUsername(), req.GetPassword().GetPassword())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "provider rejected the credentials")
	}
	id := accounts.NewID()
	if err := s.accounts.Save(ctx, id, blob); err != nil {
		return nil, status.Error(codes.Internal, "failed to store the account session")
	}
	rec := accounts.Record{
		ID:                   id,
		Provider:             req.GetProvider(),
		BaseURL:              baseURL,
		Username:             req.GetPassword().GetUsername(),
		OwnerUserID:          uid,
		Status:               accounts.StatusLinked,
		Visibility:           vis,
		SharedWith:           req.GetSharedWith(),
		MaxConcurrentStreams: req.GetMaxConcurrentStreams(),
		CreatedAt:            time.Now().UTC(),
	}
	if err := s.accounts.Add(ctx, rec); err != nil {
		// The probe's session must not outlive the record it belonged to.
		_ = s.accounts.Delete(ctx, id)
		return nil, status.Error(codes.Internal, "failed to persist the account record")
	}
	if s.attacher != nil {
		if err := s.attacher.AttachAccount(rec); err != nil {
			// The account could not be wired into a live slot: take back the
			// record and its session so the user is not left with an account
			// that stores but never serves. The attacher cleans the slot side.
			_ = s.accounts.Delete(ctx, id)
			return nil, status.Error(codes.Internal, "failed to wire the account into its slot: "+err.Error())
		}
	}
	s.emitAccountEvent(uid, id, rec.Provider, corev1.EventType_EVENT_TYPE_ACCOUNT_SESSION_LINKED)
	return &apiv1.LinkAccountResponse{AccountId: id}, nil
}

// UpdateAccount changes a linked account's owner-owned settings after the
// fact (PLAN.md §7.1, §7.2): who may use it, the owner's concurrent-stream
// cap, and what lowering that cap does to running sessions. Only the owner
// may call it; operator-declared accounts are read-only through the API.
// The store is updated first — it is the record of truth — then the live
// side is brought into line: narrower sharing ends the de-authorized
// members' sessions on this account, and a lowered cap is enforced through
// the same allowance computation admission uses. A store failure changes
// nothing; the live effects are idempotent and re-run on the next edit.
func (s *Server) UpdateAccount(ctx context.Context, req *apiv1.UpdateAccountRequest) (*apiv1.UpdateAccountResponse, error) {
	if err := schema.ValidateUpdateAccountRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	uid, _ := UserIDFromContext(ctx)
	rec, err := s.accounts.Get(ctx, req.GetAccountId())
	if err != nil {
		if !errors.Is(err, accounts.ErrNotFound) {
			return nil, status.Error(codes.Internal, "failed to read the account")
		}
		if s.reachableFor(uid, req.GetAccountId()) {
			return nil, status.Error(codes.PermissionDenied, "operator-declared accounts cannot be updated through the API")
		}
		return nil, status.Error(codes.NotFound, "account not found")
	}
	if rec.OwnerUserID != uid {
		return nil, status.Error(codes.PermissionDenied, "only the account owner may update a linked account")
	}

	// Validate everything before anything changes: a rejected request must
	// leave the account as it was, never half-applied.
	if sh := req.GetSharing(); sh != nil {
		if err := s.validateSharing(ctx, apiVisibility(sh.GetVisibility()), sh.GetSharedWith()); err != nil {
			return nil, err
		}
	}
	if err := validateCapChangePolicyValue(req.GetCapChangePolicy()); err != nil {
		return nil, err
	}

	old := rec
	if sh := req.GetSharing(); sh != nil {
		if err := s.accounts.SetSharing(ctx, rec.ID, apiVisibility(sh.GetVisibility()), sh.GetSharedWith()); err != nil {
			return nil, status.Error(codes.Internal, "failed to update sharing")
		}
	}
	if req.MaxConcurrentStreams != nil {
		if err := s.accounts.SetMaxConcurrentStreams(ctx, rec.ID, req.GetMaxConcurrentStreams()); err != nil {
			// Never half-apply: put the earlier field back.
			_ = s.accounts.SetSharing(ctx, rec.ID, old.Visibility, old.SharedWith)
			return nil, status.Error(codes.Internal, "failed to update the stream cap")
		}
	}
	if req.GetCapChangePolicy() != apiv1.CapChangePolicy_CAP_CHANGE_POLICY_UNSPECIFIED {
		if err := s.accounts.SetCapChangePolicy(ctx, rec.ID, apiCapChangePolicy(req.GetCapChangePolicy())); err != nil {
			_ = s.accounts.SetSharing(ctx, rec.ID, old.Visibility, old.SharedWith)
			_ = s.accounts.SetMaxConcurrentStreams(ctx, rec.ID, old.MaxConcurrentStreams)
			return nil, status.Error(codes.Internal, "failed to update the cap policy")
		}
	}
	// Re-read the record so the view below, and the revoke keep-set, work
	// from the canonicalised stored form, not the request's echo.
	rec, err = s.accounts.Get(ctx, rec.ID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to re-read the account")
	}

	sharingEdited := req.GetSharing() != nil &&
		(rec.Visibility != old.Visibility || !sameMembers(old.SharedWith, rec.SharedWith))

	// The live reach must match the record before anyone else can use it
	// through the new audience — that is the API's only derivation and
	// delivery gate. A failed swap rolls the store's sharing change back so
	// the record and the reach can never disagree about who may use this.
	if sharingEdited && s.library != nil {
		if err := s.library.SetReachSharing(rec.ID, rec.Visibility, rec.SharedWith); err != nil {
			_ = s.accounts.SetSharing(ctx, rec.ID, old.Visibility, old.SharedWith)
			return nil, status.Error(codes.Internal, "failed to update account sharing live")
		}
	}

	// Now the live revoke. Narrower than "everyone" means some members may
	// have lost access: end exactly those sessions on this account. Public
	// visibility reaches everyone again, so no sweep is needed, and a
	// sharing edit that only widened the roster needed none either — the
	// sweep is gated on an actual narrowing, not on any sharing edit.
	if sharingEdited && s.delivery != nil && rec.Visibility != accounts.VisibilityPublic {
		keep := append([]string{rec.OwnerUserID}, rec.SharedWith...)
		s.delivery.RevokeOthersOnAccount(rec.Provider, rec.ID, keep)
	}
	if s.delivery != nil && req.MaxConcurrentStreams != nil {
		enforceNow := rec.CapChangePolicy == accounts.CapChangePolicyEnforceNow
		if rec.CapChangePolicy == accounts.CapChangePolicyDefault {
			enforceNow = s.capChangeDefault == accounts.CapChangePolicyEnforceNow
		}
		// The stored cap governs the next session regardless; the sweep over
		// the live set is idempotent and re-runs on the next edit.
		_, _ = s.delivery.ApplyAccountCap(ctx, rec.Provider, rec.ID, enforceNow)
	}

	return &apiv1.UpdateAccountResponse{Account: accountView(rec, uid, s.capChangeDefault)}, nil
}

// effectiveCapPolicy resolves an account's own choice with the instance
// default the composition root armed. An account that carries no choice
// follows the instance default; anything else is the account's own.
func effectiveCapPolicy(rec accounts.Record, defaultPolicy accounts.CapChangePolicy) accounts.CapChangePolicy {
	if rec.CapChangePolicy != accounts.CapChangePolicyDefault {
		return rec.CapChangePolicy
	}
	return defaultPolicy
}

// sameMembers reports whether two canonicalised rosters name the same set
// of users. Both inputs come from store records, so both are already sorted
// and deduplicated; a set check against one and a length compare keeps the
// handler's sharing-changed gate exact.
func sameMembers(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, m := range a {
		counts[m]++
	}
	for _, m := range b {
		counts[m]--
		if counts[m] < 0 {
			return false
		}
	}
	return true
}

// accountView builds the API view of one account for its caller. The cap
// and policy fields are owner-only: a member learns that the account is
// shared, not the owner's usage budget.
func accountView(rec accounts.Record, uid string, defaultPolicy accounts.CapChangePolicy) *apiv1.Account {
	a := &apiv1.Account{
		AccountId:    rec.ID,
		Provider:     rec.Provider,
		BaseUrl:      rec.BaseURL,
		CallerLinked: rec.OwnerUserID == uid,
		Status:       apiStatus(rec.Status),
		Visibility:   apiVisibilityAPI(rec.Visibility),
		SharedWith:   rosterFor(rec, uid),
		OwnerUserId:  rec.OwnerUserID,
	}
	if rec.OwnerUserID == uid {
		a.MaxConcurrentStreams = rec.MaxConcurrentStreams
		a.CapChangePolicy = accountCapChangePolicy(rec.CapChangePolicy)
		a.EffectiveCapChangePolicy = accountCapChangePolicy(effectiveCapPolicy(rec, defaultPolicy))
	}
	return a
}

// ListAccounts returns every account the caller can deliver from (PLAN.md
// §7.5). The view is the union of the caller's linked accounts and the
// accounts reachable by them: an operator-declared account the caller cannot
// reach stays invisible, and a private linked account is visible only to its
// owner. Operator-declared accounts are always read-only (PUBLIC by
// definition), so they are reported with caller_linked false and no owner.
func (s *Server) ListAccounts(ctx context.Context, req *apiv1.ListAccountsRequest) (*apiv1.ListAccountsResponse, error) {
	if err := schema.ValidateListAccountsRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	uid, _ := UserIDFromContext(ctx)
	records, err := s.accounts.List(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read linked accounts")
	}
	out := make([]*apiv1.Account, 0, len(records))
	add := func(a *apiv1.Account) { out = append(out, a) }
	for _, rec := range records {
		if rec.OwnerUserID != uid && !s.reachableFor(uid, rec.ID) {
			continue
		}
		add(accountView(rec, uid, s.capChangeDefault))
	}
	for _, r := range s.reachesFor(uid) {
		if seen := containsAccount(out, r.AccountID); seen {
			continue
		}
		add(&apiv1.Account{
			AccountId:    r.AccountID,
			Provider:     providerOf(r),
			CallerLinked: false,
			Status:       apiv1.AccountStatus_ACCOUNT_STATUS_LINKED,
			Visibility:   apiVisibilityAPI(r.Visibility),
			// Reach records for host-provided accounts never carry a roster:
			// a host account is public by definition, and exposing its
			// member list would hand an anonymous visitor a set of usernames.
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].GetAccountId() < out[j].GetAccountId() })
	return &apiv1.ListAccountsResponse{Accounts: out}, nil
}

// RemoveAccount unlinks one of the caller's linked accounts (PLAN.md §7.5):
// the record, its vaulted session, and the live reach it backed are all
// dropped. Operator-declared accounts cannot be removed through the API, and
// nobody but the owner may remove a linked account.
func (s *Server) RemoveAccount(ctx context.Context, req *apiv1.RemoveAccountRequest) (*apiv1.RemoveAccountResponse, error) {
	if err := schema.ValidateRemoveAccountRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	uid, _ := UserIDFromContext(ctx)
	rec, err := s.accounts.Get(ctx, req.GetAccountId())
	if err != nil {
		if !errors.Is(err, accounts.ErrNotFound) {
			return nil, status.Error(codes.Internal, "failed to read the account")
		}
		// No linked record — maybe an operator-declared account the caller is
		// allowed to reach? Those are read-only by contract.
		if s.reachableFor(uid, req.GetAccountId()) {
			return nil, status.Error(codes.PermissionDenied, "operator-declared accounts cannot be removed through the API")
		}
		return nil, status.Error(codes.NotFound, "account not found")
	}
	if rec.OwnerUserID != uid {
		return nil, status.Error(codes.PermissionDenied, "only the account owner may remove a linked account")
	}
	if err := s.accounts.Delete(ctx, rec.ID); err != nil {
		return nil, status.Error(codes.Internal, "failed to remove the account")
	}
	if s.library != nil {
		s.library.RemoveReach(rec.ID)
	}
	// The running slot must forget it too: drop the account's cached session,
	// retire its refresh job, and drop its source-cache rows — the record and
	// session blob are already gone, and a ghost account with a working token
	// must not linger until restart.
	if s.dropper != nil {
		_ = s.dropper.DropAccount(rec)
	}
	// Every session routed through this account must die with it: the engine
	// marks them revoked (and aborts their sinks) before we announce the
	// account's removal, so the notification never lags the cut-off
	// (PLAN.md T7/§7.5).
	if s.delivery != nil {
		s.delivery.RevokeAllOnAccount(rec.ID)
	}
	s.emitAccountEvent(uid, rec.ID, rec.Provider, corev1.EventType_EVENT_TYPE_ACCOUNT_SESSION_REVOKED)
	return &apiv1.RemoveAccountResponse{}, nil
}

func (s *Server) reachesFor(userID string) []library.Reach {
	if s.library == nil {
		return nil
	}
	return s.library.ReachesForUser(userID)
}

func (s *Server) reachableFor(userID, accountID string) bool {
	if s.library == nil {
		return false
	}
	_, ok := s.library.ReachAuthorized(accountID, userID)
	return ok
}

func providerOf(r library.Reach) string {
	if r.Sync == nil {
		return ""
	}
	return r.Sync.Provider()
}

func containsAccount(accs []*apiv1.Account, id string) bool {
	for _, a := range accs {
		if a.GetAccountId() == id {
			return true
		}
	}
	return false
}

// rosterFor is the only producer of an account's member list on the wire:
// a roster is visible to the account's owner and to nobody else. A member
// learns that an account is shared — Visibility tells them that — but never
// whom with. A private or host-provided account carries no roster at all
// (the store clears one once visibility leaves SHARED, and host accounts
// are public by definition), so leaving the field empty there is not a
// behaviour change for those shapes.
func rosterFor(rec accounts.Record, uid string) []string {
	if rec.OwnerUserID != "" && rec.OwnerUserID == uid {
		return rec.SharedWith
	}
	return nil
}

// apiVisibility maps the API visibility enum to the accounts store's
// visibility, defaulting an unspecified value to private (§5.1: a link the
// caller does not widen is owner-only).
func apiVisibility(v apiv1.AccountVisibility) accounts.Visibility {
	switch v {
	case apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED:
		return accounts.VisibilityShared
	case apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PUBLIC:
		return accounts.VisibilityPublic
	default:
		return accounts.VisibilityPrivate
	}
}

// apiVisibilityAPI maps the store's visibility back to the API enum.
func apiVisibilityAPI(v accounts.Visibility) apiv1.AccountVisibility {
	switch v {
	case accounts.VisibilityShared:
		return apiv1.AccountVisibility_ACCOUNT_VISIBILITY_SHARED
	case accounts.VisibilityPublic:
		return apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PUBLIC
	default:
		return apiv1.AccountVisibility_ACCOUNT_VISIBILITY_PRIVATE
	}
}

// apiStatus maps the store's lifecycle status to the API enum.
func apiStatus(v accounts.Status) apiv1.AccountStatus {
	switch v {
	case accounts.StatusExpired:
		return apiv1.AccountStatus_ACCOUNT_STATUS_EXPIRED
	default:
		return apiv1.AccountStatus_ACCOUNT_STATUS_LINKED
	}
}

// emitAccountEvent publishes an account-session lifecycle event to its owner
// (PLAN.md §7.5, §9.2): linked at link time, revoked at removal, expired when
// the wiring discovers a dead session.
func (s *Server) emitAccountEvent(uid, accountID, provider string, typ corev1.EventType) {
	s.bus.Publish(&corev1.EventEnvelope{
		Id:       fmt.Sprintf("evt-account-%s-%d", accountID, time.Now().UnixNano()),
		Type:     typ,
		Audience: corev1.EventAudience_EVENT_AUDIENCE_OWNER,
		UserId:   uid,
		Payload: &corev1.EventEnvelope_AccountSession{
			AccountSession: &corev1.AccountSessionEvent{
				AccountId: accountID,
				Provider:  provider,
			},
		},
		EmittedAt: timestamppb.Now(),
	})
}
