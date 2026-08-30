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
// (vault-first custody model). Sharing is fixed at link time for M5.
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
		Visibility:           apiVisibility(req.GetVisibility()),
		SharedWith:           req.GetSharedWith(),
		MaxConcurrentStreams: req.GetMaxConcurrentStreams(),
		CreatedAt:            time.Now().UTC(),
	}
	if err := s.accounts.Add(ctx, rec); err != nil {
		// The probe's session must not outlive the record it belonged to.
		_ = s.accounts.Delete(ctx, id)
		return nil, status.Error(codes.Internal, "failed to persist the account record")
	}
	s.emitAccountEvent(uid, id, rec.Provider, corev1.EventType_EVENT_TYPE_ACCOUNT_SESSION_LINKED)
	return &apiv1.LinkAccountResponse{AccountId: id}, nil
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
		add(&apiv1.Account{
			AccountId:    rec.ID,
			Provider:     rec.Provider,
			BaseUrl:      rec.BaseURL,
			CallerLinked: rec.OwnerUserID == uid,
			Status:       apiStatus(rec.Status),
			Visibility:   apiVisibilityAPI(rec.Visibility),
			SharedWith:   rec.SharedWith,
			OwnerUserId:  rec.OwnerUserID,
		})
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
			SharedWith:   r.Members,
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
