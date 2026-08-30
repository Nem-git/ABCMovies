// Package accounts stores the instance's linked provider accounts
// (PLAN.md §3.5). A link records two things: the non-secret metadata every
// view needs (provider, server, owner, status) and the validated provider
// session blob. Both live in the vault (PLAN.md §2.4: an account session is a
// durable, vaulted provider login); the metadata is non-secret but costs
// nothing extra at rest and keeps every piece of account state in one durable
// class. The blob is written under the account id — the same key family a
// provider slot's session vault already reads for operator-declared accounts
// — so a linked account is usable by wiring at the next boot with no re-login.
//
// The store deliberately holds no credentials: enabling the owner-sidecar
// custody axis (§3.5) later only changes who fills the blob, not how it is
// stored.
package accounts

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/nem-git/abcmovies/core/internal/store"
)

// ErrNotFound wraps store.ErrKeyNotFound for callers that want a sentinel
// without importing the store package.
var ErrNotFound = store.ErrKeyNotFound

// Status is the lifecycle state of a linked account. A session that stopped
// working moves the account to StatusExpired (the re-auth flow, PLAN.md §7.5)
// rather than silently unlinking it: the record still names the server and
// owner, so the right user can act on it.
type Status string

const (
	// StatusLinked means the account's vaulted session was last known live.
	StatusLinked Status = "linked"
	// StatusExpired means the vaulted session died and the account needs a
	// user re-link before it can be used again.
	StatusExpired Status = "expired"
)

// Visibility is who may derive a linked account's items into a library
// (PLAN.md §5.1). Owner-only is the default; an owner may widen their slot to
// named users or to everyone at link time. M6 introduces mutable sharing; M5
// fixes the choice at link time.
type Visibility string

const (
	// VisibilityPrivate restricts the account's reachable library to its
	// owner.
	VisibilityPrivate Visibility = "private"
	// VisibilityShared adds the named host users (SharedWith) to the owner.
	VisibilityShared Visibility = "shared"
	// VisibilityPublic makes the account's reachable library available to
	// everyone, matching how host-provided operator accounts behave (§2.2).
	VisibilityPublic Visibility = "public"
)

// Record is one linked account. It is not secret: the session credential
// lives separately, sealed in the vault under the account id. OwnerUserID
// names the host user who linked the account — the only principal allowed to
// re-link or remove it, and the only principal able to see it when it is
// private (PLAN.md §7.5, §5.1). Sharing is fixed at link time for M5.
type Record struct {
	ID          string `json:"id"`
	Provider    string `json:"provider"`
	BaseURL     string `json:"base_url"`
	Username    string `json:"username"`
	OwnerUserID string `json:"owner_user_id"`
	Status      Status `json:"status"`
	// Visibility gates who may use the account's library; defaults to private.
	Visibility Visibility `json:"visibility"`
	// SharedWith lists the host users allowed when VisibilityShared
	// (canonicalized: sorted, deduplicated). Ignored otherwise.
	SharedWith []string `json:"shared_with,omitempty"`
	// MaxConcurrentStreams caps simultaneous delivery streams through this
	// account (0 = unlimited). The delivery quota gate enforces it; M6 makes
	// the value editable, M5 pins it at link time.
	MaxConcurrentStreams uint32    `json:"max_concurrent_streams,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
}

// metaPrefix scopes linked-account records below any other vault state. The
// session blob itself is keyed by the bare account id, exactly like operator-
// declared accounts, so a provider slot's session vault reads both kinds
// through the same key path.
const metaPrefix = "account/meta/"

// Store persists linked-account records over a durable, sealed store (the
// vault). It also serves as the provider slots' session vault: Save and Load
// read and write the session blob under the account id.
type Store struct {
	vault  store.Store
	logger *slog.Logger
	now    func() time.Time
}

// NewStore builds a store over the vault. now is injectable for tests; nil
// means time.Now.
func NewStore(vault store.Store, logger *slog.Logger) *Store {
	if logger == nil {
		logger = slog.Default()
	}
	return &Store{vault: vault, logger: logger, now: time.Now}
}

func (s *Store) metaKey(id string) string {
	return metaPrefix + id
}

// NewID returns an opaque linked-account identifier. The lnk_ prefix keeps
// the linked id namespace disjoint from operator-chosen account ids, so the
// two can never collide in the vault's shared session key space (a collision
// would make an operator account and a linked account read each other's
// session).
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure on the happy path is unrecoverable; surface it
		// as a clearly invalid id so the caller fails loudly.
		return "lnk_<rand-failed>"
	}
	return "lnk_" + hex.EncodeToString(b[:])
}

// Add stores a record. A duplicate id is an error, not a silent overwrite:
// ids are minted once per link and a second Add with the same id means a
// caller bug.
func (s *Store) Add(ctx context.Context, rec Record) error {
	if rec.ID == "" || rec.Provider == "" || rec.BaseURL == "" || rec.Username == "" {
		return fmt.Errorf("accounts: id, provider, base_url, and username are required")
	}
	if rec.Status == "" {
		rec.Status = StatusLinked
	}
	switch rec.Visibility {
	case "":
		// A link defaults to private: the owner's account feeds only their
		// library until they say otherwise (§5.1 sharing rule).
		rec.Visibility = VisibilityPrivate
	case VisibilityPrivate, VisibilityShared, VisibilityPublic:
	default:
		return fmt.Errorf("accounts: %q: invalid visibility %q", rec.ID, rec.Visibility)
	}
	if rec.Visibility != VisibilityShared {
		rec.SharedWith = nil
	} else if len(rec.SharedWith) == 0 {
		return fmt.Errorf("accounts: %q: shared visibility requires at least one shared_with user", rec.ID)
	} else {
		rec.SharedWith = canonUsers(rec.SharedWith)
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = s.now().UTC()
	}
	if _, err := s.Get(ctx, rec.ID); err == nil {
		return fmt.Errorf("accounts: %q already linked", rec.ID)
	} else if !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("accounts: read before add: %w", err)
	}
	blob, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("accounts: encode %q: %w", rec.ID, err)
	}
	if err := s.vault.Put(ctx, s.metaKey(rec.ID), blob); err != nil {
		return fmt.Errorf("accounts: add %q: %w", rec.ID, err)
	}
	return nil
}

// Get fetches one record. Ids that were never linked yield ErrNotFound.
func (s *Store) Get(ctx context.Context, id string) (Record, error) {
	var rec Record
	blob, err := s.vault.Get(ctx, s.metaKey(id))
	if err != nil {
		return rec, err
	}
	if err := json.Unmarshal(blob, &rec); err != nil {
		return rec, fmt.Errorf("accounts: decode %q: %w", id, err)
	}
	return rec, nil
}

// SetStatus updates a record's lifecycle status, leaving every other field
// untouched. Unknown ids are an error.
func (s *Store) SetStatus(ctx context.Context, id string, status Status) error {
	rec, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	rec.Status = status
	blob, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("accounts: encode %q: %w", id, err)
	}
	if err := s.vault.Put(ctx, s.metaKey(id), blob); err != nil {
		return fmt.Errorf("accounts: set status %q: %w", id, err)
	}
	return nil
}

// Delete removes a linked account: both its record and its session blob. The
// session is dead the moment the account is unlinked, so keeping it would be
// a stale live credential hoarding space for nothing.
func (s *Store) Delete(ctx context.Context, id string) error {
	if err := s.vault.Delete(ctx, s.metaKey(id)); err != nil {
		return fmt.Errorf("accounts: delete %q: %w", id, err)
	}
	if err := s.vault.Delete(ctx, id); err != nil {
		return fmt.Errorf("accounts: delete session %q: %w", id, err)
	}
	return nil
}

// List returns every linked account record, oldest first. Order is
// deterministic so the accounts view does not shuffle between renders.
func (s *Store) List(ctx context.Context) ([]Record, error) {
	keys, err := s.vault.List(ctx, metaPrefix)
	if err != nil {
		return nil, fmt.Errorf("accounts: list: %w", err)
	}
	out := make([]Record, 0, len(keys))
	for _, k := range keys {
		id := strings.TrimPrefix(k, metaPrefix)
		rec, err := s.Get(ctx, id)
		if err != nil {
			// A torn record must not hide every other account; report it and
			// keep going so the operator can see it in the accounts view.
			s.logger.Error("accounts: skipping unreadable record", "account", id, "error", err)
			continue
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

// canonUsers keeps the members list canonical: empty entries dropped,
// duplicates removed, deterministic (sorted) order — so the stored record is
// stable and comparing two records is meaningful.
func canonUsers(users []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(users))
	for _, u := range users {
		u = strings.TrimSpace(u)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		out = append(out, u)
	}
	sort.Strings(out)
	return out
}

// Save stores a provider session blob under the account id — the Surface the
// jellyfin adapter's session vault calls. The adapter guarantees the caller
// validated the material against the provider before vaulting it (PLAN.md
// §3.5: the core never vaults material it has not confirmed works); this
// store does not second-guess that.
func (s *Store) Save(ctx context.Context, accountID string, blob []byte) error {
	if err := s.vault.Put(ctx, accountID, blob); err != nil {
		return fmt.Errorf("accounts: save session %q: %w", accountID, err)
	}
	return nil
}

// Load restores a provider session blob for an account, or returns
// ErrNotFound when none has been vaulted (the store never fabricates an empty
// credential for a missing one).
func (s *Store) Load(ctx context.Context, accountID string) ([]byte, error) {
	blob, err := s.vault.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return blob, nil
}
