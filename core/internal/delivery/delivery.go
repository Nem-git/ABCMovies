// Package delivery implements the delivery engine (PLAN.md §6): the core-side
// representation and supervision of play and download sessions. A delivery
// session pipes bytes from a provider to a sink; both play and download run
// the same flow — resolve a manifest, then deliver its tracks (§6.2) — with
// the goal governing how much is delivered.
//
// The engine owns the session→account index (PLAN.md §2.3, §6.1): the in-memory
// table it uses to police quotas, attribute audit, and — crucially — end
// sessions on revocation (§7.1). It also owns liveness (§9.1): play sessions
// must heartbeat, downloads prove liveness by progress, and every session is
// bounded by a TTL, so no phantom session blocks a concurrent-stream limit
// (TECHNICAL-DECISIONS.md §1.14).
//
// The engine is provider- and sink-agnostic: it resolves manifests through a
// Resolver (a provider slot's produce-sources surface) and pushes bytes
// through a SinkFactory. It never talks to a frontend and never reaches out to
// a provider directly — it composes whoever is wired.
package delivery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/policy"
)

// Goal is the delivery goal — play or download (PLAN.md §6.1). The goal is a
// hint that seeds defaults: play delivers the whole menu and lets the player
// choose; download delivers the user's recipe into a container.
type Goal string

const (
	GoalPlay     Goal = "play"
	GoalDownload Goal = "download"
)

// Status is the visible delivery-session lifecycle, mirroring the job status
// vocabulary (PLAN.md §9.1) plus the engine-side revoked terminal state
// (§7.1).
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusDone      Status = "done"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
	StatusRevoked   Status = "revoked"
)

// Session is one delivery session: a single play or download (§6, §2.3). It
// carries the account context the engine needs to police, audit, and revoke.
//
// Session is handed out of the engine to callers (Start's return value, Get),
// so its mutable state — Status, Error, Menu, and the liveness timestamps —
// is guarded by mu, not by the engine's sessions lock. The engine acquires
// e.mu before s.mu, never the reverse.
type Session struct {
	mu sync.RWMutex

	ID      string
	Goal    Goal
	Context corev1.DeliveryContext
	Status  Status
	Error   string

	// Plan is the pipeline-selection result for this session (PLAN.md §6.3):
	// which pipeline runs and the container the deliverable is named as. It
	// is set once at Start, before any sink work, so it seeds the resume key.
	Plan Plan

	// Menu is the staged play menu for a play session, set once at Start
	// (PLAN.md §6.1): the manifest tracks that were delivered to the sink and
	// are ready to pull. It is nil for downloads. GetPlayInfo recovers the
	// menu from here — the payload keyed by the session id (PLAN.md §6.2).
	Menu []*corev1.Track

	Sink Sink

	createdAt     time.Time
	lastHeartbeat time.Time
	lastProgress  time.Time
}

// accountKey is the index key revocation is grounded in: a provider account
// as a member uses it (PLAN.md §2.3, §7.1). Revocation finds every session
// whose memberUserId matches the revoked member on that account.
type accountKey struct {
	provider     string
	accountID    string
	memberUserID string
}

// Resolver resolves a manifest for one item through a provider slot's
// produce-sources surface (PLAN.md §3.2, §6.2). Provider-specific details
// (URLs, auth, expiry) stay inside the source; the engine only consumes the
// validated manifest.
type Resolver interface {
	ProduceSources(ctx context.Context, provider, accountID, nativeID string) (*corev1.MediaSource, error)
}

// noSession is the structural contract a typed "this account has no live
// session and cannot obtain one; only a human re-link recovers it" failure
// satisfies (PLAN.md §7.5). Adapters mark their own error type with
// NoSession() bool — structural typing keeps the core free of adapter
// imports while any adapter can participate.
type noSession interface{ NoSession() bool }

// IsNoSession reports whether err, or anything it wraps, is a typed
// no-live-session failure: the signal that the re-auth flow must run, never
// an ordinary provider error.
func IsNoSession(err error) bool {
	var marker noSession
	return errors.As(err, &marker)
}

// Sink is the byte destination for one delivery session (PLAN.md §6.4). One
// session drives exactly one sink.
type Sink interface {
	// Deliver pushes one track's bytes to the sink.
	Deliver(ctx context.Context, s *Session, track *corev1.Track, body io.Reader) (int64, error)
	// Finalize completes the session's deliverable.
	Finalize(ctx context.Context, s *Session) error
	// Abort discards the session's partial deliverable.
	Abort(ctx context.Context, s *Session)
}

// SinkFactory resolves a sink by session context (PLAN.md §6.4: sinks are
// declared in instance config).
type SinkFactory interface {
	NewSink(ctx context.Context, s *Session, tracks []*corev1.Track) (Sink, error)
}

// Options tunes the engine's liveness behaviour.
type Options struct {
	// SessionTTL is the play-session watchdog's idle ceiling (§1.14). Play
	// sessions refresh liveness with heartbeats; downloads with progress.
	SessionTTL        time.Duration
	HeartbeatInterval time.Duration
	HeartbeatGrace    time.Duration
	// InstancePolicy is the operator's usage policy (PLAN.md §7.2): parsed and
	// validated per policy-package rules. Nil resolves to the shipped defaults
	// (concurrentStreams: 3, bandwidth: unlimited, encode: disabled).
	InstancePolicy policy.Set
	// AccountConstraints resolves one account's declared constraints: the
	// policy override the owner set (applied here onto the instance policy)
	// and the provider-declared concurrency cap, kept as its own type so the
	// min(policy, provider_cap) rule reads as two different things
	// (PLAN.md §7.2). Nil admits the account with the instance policy only.
	AccountConstraints func(ctx context.Context, provider, accountID string) (override policy.Set, caps policy.ProviderCaps, err error)
	SourceResolver     Resolver
	SinkFactory        SinkFactory
	RecordJob          func(*corev1.Job)
	// MenuReady announces that a play session's menu is fully staged, once
	// Start has delivered every track to the sink. The app publishes the
	// delivery-play-menu-ready event through it (PLAN.md §6.2); nil mutes the
	// announcement.
	MenuReady func(*Session)
	// SessionDied reports that an account's provider session was discovered
	// dead during a delivery — a typed no-live-session failure (PLAN.md
	// §7.5). The composition root flips the account to EXPIRED, notifies the
	// owner, and opens the re-auth job; nil mutes the report, and the
	// delivery itself still fails normally.
	SessionDied func(accountID string)
	Now         func() time.Time
	Logger      *slog.Logger
}

// Engine owns the delivery sessions and the session→account index.
type Engine struct {
	mu        sync.Mutex
	sessions  map[string]*Session
	byAccount map[accountKey]map[string]struct{}
	seq       atomic.Int64

	ttl      time.Duration
	interval time.Duration
	grace    time.Duration

	resolver    Resolver
	sinkMaker   SinkFactory
	recordJob   func(*corev1.Job)
	menuReady   func(*Session)
	sessionDied func(accountID string)
	now         func() time.Time
	logger      *slog.Logger

	instancePolicy     policy.Set
	accountConstraints func(ctx context.Context, provider, accountID string) (override policy.Set, caps policy.ProviderCaps, err error)

	stopCh  chan struct{}
	done    chan struct{}
	watchWG sync.WaitGroup
	watched atomic.Bool
}

// New builds a delivery engine. sessionTTL is the zombie cap; interval and
// grace bound play-session heartbeat liveness (TECHNICAL-DECISIONS.md §1.14).
func New(opts Options) *Engine {
	e := &Engine{
		sessions:    make(map[string]*Session),
		byAccount:   make(map[accountKey]map[string]struct{}),
		ttl:         opts.SessionTTL,
		interval:    opts.HeartbeatInterval,
		grace:       opts.HeartbeatGrace,
		resolver:    opts.SourceResolver,
		sinkMaker:   opts.SinkFactory,
		recordJob:   opts.RecordJob,
		menuReady:   opts.MenuReady,
		sessionDied: opts.SessionDied,
		now:         opts.Now,
		logger:      opts.Logger,
		stopCh:      make(chan struct{}),
		done:        make(chan struct{}),
	}
	e.instancePolicy = opts.InstancePolicy
	if e.instancePolicy == nil {
		e.instancePolicy = policy.Defaults()
	}
	e.accountConstraints = opts.AccountConstraints
	if e.now == nil {
		e.now = time.Now
	}
	if e.logger == nil {
		e.logger = slog.Default()
	}
	return e
}

// StartRequest names what to deliver and why (PLAN.md §6). Start is a plain
// "create": each Start makes a new session; a retried start is a new create,
// guarded against over-consume by the concurrent-session cap (§1.30, §1.14).
type StartRequest struct {
	Goal         Goal
	MemberUserID string
	Provider     string
	AccountID    string
	NativeID     string
	Sink         string
	// SelectedTarget and Container seed the pipeline and the resume key; both
	// apply to play (sink-compatibility remux/transcode) and download (recipe
	// composition) (PLAN.md §6.3, §6.5).
	SelectedTarget string
	Container      string
}

// Start resolves a manifest and begins a delivery session. The session is
// created, indexed by account for revocation, and announced as a job.
func (e *Engine) Start(ctx context.Context, req StartRequest) (*Session, error) {
	if req.Goal != GoalPlay && req.Goal != GoalDownload {
		return nil, errInvalid("goal must be play or download")
	}
	if req.Provider == "" || req.AccountID == "" || req.MemberUserID == "" {
		return nil, errInvalid("provider, account_id, and member_user_id are required")
	}
	if req.Sink == "" {
		return nil, errInvalid("sink is required")
	}

	if e.resolver == nil {
		return nil, errInvalid("no source resolver wired; cannot deliver")
	}
	src, err := e.resolver.ProduceSources(ctx, req.Provider, req.AccountID, req.NativeID)
	if err != nil {
		if IsNoSession(err) {
			e.reportSessionDied(req.AccountID)
		}
		return nil, fmt.Errorf("resolve sources: %w", err)
	}
	if err := ValidateManifest(src); err != nil {
		return nil, errInvalid("provider returned an invalid manifest: %v", err)
	}
	// The contract permits DRM (§6.2), but v1 has no key/license path (§6.6):
	// an encrypted track is refused loudly, never silently delivered without
	// license handling (PLAN.md §2.5 — reject, never downgrade).
	if err := RejectUnsupportedDRM(src); err != nil {
		e.logger.Error("delivery refused: manifest carries a DRM-encrypted track",
			"goal", req.Goal,
			"provider", req.Provider,
			"account", req.AccountID,
			"member", req.MemberUserID,
			"reason", err.Error(),
		)
		return nil, errInvalid("%v", err)
	}

	plan, err := SelectPipeline(req.Goal, src, req.Container)
	if err != nil {
		return nil, errInvalid("cannot satisfy delivery: %v", err)
	}
	// Walk the selected chain and refuse — loudly, with the full plan logged —
	// any step this v1 engine cannot genuinely execute. A step needing a real
	// transcoder or decryptor is declined, never downgraded to a passthrough
	// (PLAN.md §2.5).
	if err := ValidatePlan(plan); err != nil {
		e.logger.Error("delivery step not executable; refusing (not downgrading)",
			"goal", req.Goal,
			"provider", req.Provider,
			"account", req.AccountID,
			"member", req.MemberUserID,
			"chain", Lineage(plan),
			"reason", err.Error(),
		)
		return nil, errInvalid("cannot satisfy delivery: %v", err)
	}
	now := e.now()
	override, caps, err := e.resolveConstraints(ctx, req)
	if err != nil {
		return nil, errInvalid("account constraints: %v", err)
	}
	// Admission folds two different things (PLAN.md §7.2), and the types say
	// so: effectivePolicy is what the operator's instance policy plus this
	// account's override allow; caps is what the provider itself declared.
	// The account admits no more sessions than the lesser of the two, so
	// "policy allows 4, provider allows 2" never fails at the source. Neither
	// operand is invented here — both are resolved above, and an absent cap
	// simply leaves the policy in charge.
	effectivePolicy := e.instancePolicy.With(override)
	accountLimit := policy.EffectiveAccountLimit(effectivePolicy, caps)
	memberLimit, _ := e.instancePolicy.Streams()

	sess := &Session{
		ID:            e.newSessionID(),
		Goal:          req.Goal,
		Status:        StatusQueued,
		Plan:          plan,
		createdAt:     now,
		lastHeartbeat: now,
		lastProgress:  now,
		Context: corev1.DeliveryContext{
			Provider:       req.Provider,
			AccountId:      req.AccountID,
			MemberUserId:   req.MemberUserID,
			Sink:           req.Sink,
			SelectedTarget: req.SelectedTarget,
			Container:      req.Container,
			Policy:         &corev1.Policy{Limits: effectivePolicy},
			ProviderCap:    &corev1.Policy{Limits: caps.Limits()},
		},
	}

	if e.sinkMaker != nil {
		sink, err := e.sinkMaker.NewSink(ctx, sess, src.GetTracks())
		if err != nil {
			return nil, fmt.Errorf("sink %q: %w", req.Sink, err)
		}
		sess.Sink = sink
	}

	// Counting and admission must be atomic: a check-then-act pair that isn't
	// lets two concurrent starts both pass a limit that either would trip.
	key := accountKey{req.Provider, req.AccountID, req.MemberUserID}
	e.mu.Lock()
	memberActive := 0
	accountActive := 0
	for _, s := range e.sessions {
		if !s.isActive() {
			continue
		}
		if s.Context.GetMemberUserId() == req.MemberUserID {
			memberActive++
		}
		if s.Context.GetProvider() == req.Provider && s.Context.GetAccountId() == req.AccountID {
			accountActive++
		}
	}
	if memberActive >= memberLimit {
		e.mu.Unlock()
		return nil, &deliveryError{code: codeQuota, msg: fmt.Sprintf("member %s is at its concurrent-stream cap (%d)", req.MemberUserID, memberLimit)}
	}
	if accountActive >= accountLimit {
		e.mu.Unlock()
		return nil, &deliveryError{code: codeQuota, msg: fmt.Sprintf("account %s/%s is at its concurrent-stream cap (%d)", req.Provider, req.AccountID, accountLimit)}
	}
	e.sessions[sess.ID] = sess
	if e.byAccount[key] == nil {
		e.byAccount[key] = make(map[string]struct{})
	}
	e.byAccount[key][sess.ID] = struct{}{}
	e.mu.Unlock()

	// The session's mutable state is synchronized by the session's own lock:
	// the watchdog's sweep and the quota loops read it, so the queued→running
	// flip and the menu staging must land under it — never in the gap after
	// the maps unlock (-race proved why; PLAN.md §9.1).
	staged := req.Goal == GoalPlay && sess.Sink != nil && len(src.GetTracks()) > 0
	sess.mu.Lock()
	sess.Status = StatusRunning

	// A play session stages its menu before it is announced: every manifest
	// track with a location is delivered to the sink right away, so the
	// delivery-play-menu-ready event — and any GetPlayInfo recovery — always
	// finds a complete menu, and a start that cannot stage fails loudly here
	// rather than surface an empty player (PLAN.md §6.1, §6.2). Delivery-only
	// tracks (carried-in audio/subtitle) have no location and stage nothing;
	// the player reads them off the carrier track they name.
	if staged {
		sess.Menu = src.GetTracks()
		for _, tr := range src.GetTracks() {
			if len(tr.GetDelivery().GetLocations()) == 0 {
				continue
			}
			if _, err := sess.Sink.Deliver(ctx, sess, tr, nil); err != nil {
				sess.mu.Unlock()
				return nil, fmt.Errorf("stage play menu: %w", err)
			}
		}
	}
	sess.mu.Unlock()

	// Announce the complete menu only after the session lock is dropped:
	// subscribers read back into the engine (GetPlayInfo, Job) and must never
	// find the staging path holding the write lock.
	if staged && e.menuReady != nil {
		e.menuReady(sess)
	}

	e.recordJob(sess.toJob())
	return sess, nil
}

// reportSessionDied surfaces a discovered dead session to the composition
// root, which runs the re-auth flow (PLAN.md §7.5). A nil hook mutes the
// report; the delivery itself still fails either way.
func (e *Engine) reportSessionDied(accountID string) {
	if e.sessionDied == nil {
		return
	}
	e.sessionDied(accountID)
}

func (e *Engine) resolveConstraints(ctx context.Context, req StartRequest) (override policy.Set, caps policy.ProviderCaps, err error) {
	if e.accountConstraints == nil {
		return nil, policy.ProviderCaps{}, nil
	}
	return e.accountConstraints(ctx, req.Provider, req.AccountID)
}

func (e *Engine) newSessionID() string {
	return "del-" + strconv.FormatInt(e.seq.Add(1), 10)
}

// Get returns a live session by id.
func (e *Engine) Get(id string) (*Session, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, ok := e.sessions[id]
	return s, ok
}

// Heartbeat proves a play session is alive (PLAN.md §9.1). A legitimately
// paused session still heartbeats, so it is not killed. The session must
// belong to memberUserID; otherwise another member could keep it alive
// (§2.2 member-scoping).
func (e *Engine) Heartbeat(id string, memberUserID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, ok := e.sessions[id]
	if !ok {
		return errNotFound("session %q not found", id)
	}
	if s.Context.GetMemberUserId() != memberUserID {
		// The refusal is the same answer as an unknown id: session ids cannot
		// be probed for existence (PLAN.md §8.1, §2.2).
		return errNotFound("session %q not found", id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isActiveLocked() {
		return errInvalid("session %q is %s and cannot heartbeat", id, s.Status)
	}
	s.lastHeartbeat = e.now()
	return nil
}

// Progress records download progress; for downloads, progress is the
// heartbeat (PLAN.md §9.1).
func (e *Engine) Progress(id string, percent uint32) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, ok := e.sessions[id]
	if !ok {
		return errNotFound("session %q not found", id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.isActiveLocked() {
		return errInvalid("session %q is %s and cannot progress", id, s.Status)
	}
	if percent > 100 {
		return errInvalid("progress percent must be at most 100")
	}
	s.lastProgress = e.now()
	return nil
}

// RevokeAccount kills every session a revoked member holds on an account
// (PLAN.md §7.1): the engine finds each session whose memberUserId matches the
// revoked member on that account and ends it mid-stream; a revoked member's
// in-progress downloads are discarded. It returns how many sessions ended.
func (e *Engine) RevokeAccount(provider, accountID, memberUserID string) int {
	key := accountKey{provider, accountID, memberUserID}
	e.mu.Lock()
	ids := make([]string, 0, len(e.byAccount[key]))
	for id := range e.byAccount[key] {
		ids = append(ids, id)
	}
	var killed int
	for _, id := range ids {
		s, ok := e.sessions[id]
		if !ok {
			continue
		}
		s.mu.Lock()
		s.Status = StatusRevoked
		s.Error = "account revoked"
		if s.Sink != nil {
			s.Sink.Abort(context.Background(), s)
		}
		job := s.toJobLocked()
		s.mu.Unlock()
		e.recordJob(job)
		killed++
	}
	delete(e.byAccount, key)
	e.mu.Unlock()
	return killed
}

// RevokeAllOnAccount kills every session routed through one account, for all
// members, on both the index and the live-session map. Removing an account is
// a full revocation of that account's access: sessions a member started
// through it must not outlive the record that vaulted its credential
// (PLAN.md §7.5). Completes on the first N, matching RevokeAccount's
// return-count contract.
func (e *Engine) RevokeAllOnAccount(accountID string) int {
	e.mu.Lock()
	var killed int
	for id, s := range e.sessions {
		if s.Context.GetAccountId() != accountID {
			continue
		}
		s.mu.Lock()
		s.Status = StatusRevoked
		s.Error = "account revoked"
		if s.Sink != nil {
			s.Sink.Abort(context.Background(), s)
		}
		job := s.toJobLocked()
		s.mu.Unlock()
		e.recordJob(job)
		delete(e.byAccount[accountKey{s.Context.GetProvider(), s.Context.GetAccountId(), s.Context.GetMemberUserId()}], id)
		killed++
	}
	e.mu.Unlock()
	return killed
}

// RevokeOthersOnAccount ends every session on (provider, accountID) whose
// member is not in keepMembers. Changing an account's sharing to something
// narrower uses this: whoever no longer holds the account loses their live
// sessions on it, and only they do — the owner and the newly shared-with
// members keep theirs.
func (e *Engine) RevokeOthersOnAccount(provider, accountID string, keepMembers []string) int {
	keep := make(map[string]bool, len(keepMembers))
	for _, m := range keepMembers {
		keep[m] = true
	}
	e.mu.Lock()
	var killed int
	for id, s := range e.sessions {
		if s.Context.GetProvider() != provider || s.Context.GetAccountId() != accountID || !s.isActive() {
			continue
		}
		if keep[s.Context.GetMemberUserId()] {
			continue
		}
		s.mu.Lock()
		s.Status = StatusRevoked
		s.Error = "account sharing narrowed"
		if s.Sink != nil {
			s.Sink.Abort(context.Background(), s)
		}
		job := s.toJobLocked()
		s.mu.Unlock()
		e.recordJob(job)
		delete(e.byAccount[accountKey{provider, accountID, s.Context.GetMemberUserId()}], id)
		killed++
	}
	e.mu.Unlock()
	return killed
}

// ApplyAccountCap re-evaluates one account's effective concurrent-stream
// allowance (the lesser of its policy and the provider's declared cap, the
// same computation admission uses) and, when enforceNow is set, ends the
// excess sessions on that account, oldest first. The zero allowance — an
// account whose recorded or provider cap was just lowered to zero — ends all
// of them. New-sessions-only behaviour never ends anything here.
func (e *Engine) ApplyAccountCap(ctx context.Context, provider, accountID string, enforceNow bool) (int, error) {
	override, caps, err := e.resolveConstraints(ctx, StartRequest{Provider: provider, AccountID: accountID})
	if err != nil {
		return 0, err
	}
	// The same two operands and the same min() as admission, so a cap lowered
	// here ends exactly the sessions admission would now refuse (PLAN.md §7.2).
	effectivePolicy := e.instancePolicy.With(override)
	accountLimit := policy.EffectiveAccountLimit(effectivePolicy, caps)

	type liveSession struct {
		sess *Session
		id   string
	}
	var active []liveSession
	e.mu.Lock()
	for id, s := range e.sessions {
		if !s.isActive() || s.Context.GetProvider() != provider || s.Context.GetAccountId() != accountID {
			continue
		}
		active = append(active, liveSession{sess: s, id: id})
	}
	e.mu.Unlock()
	if !enforceNow || len(active) <= accountLimit {
		return 0, nil
	}
	// Deterministic eviction: the session that started first loses first; ties
	// break on session id so the same state yields the same cut.
	sort.Slice(active, func(i, j int) bool {
		a, b := active[i].sess, active[j].sess
		if a.createdAt.Equal(b.createdAt) {
			return active[i].id < active[j].id
		}
		return a.createdAt.Before(b.createdAt)
	})

	kill := len(active) - accountLimit
	var killed int
	for _, ls := range active[:kill] {
		e.mu.Lock()
		s, ok := e.sessions[ls.id]
		if !ok || !s.isActive() {
			e.mu.Unlock()
			continue
		}
		s.mu.Lock()
		if !s.isActiveLocked() {
			s.mu.Unlock()
			e.mu.Unlock()
			continue
		}
		s.Status = StatusRevoked
		s.Error = "stream limit lowered by the account owner"
		if s.Sink != nil {
			s.Sink.Abort(context.Background(), s)
		}
		job := s.toJobLocked()
		s.mu.Unlock()
		e.recordJob(job)
		delete(e.byAccount[accountKey{provider, accountID, s.Context.GetMemberUserId()}], ls.id)
		killed++
		e.mu.Unlock()
	}
	return killed, nil
}

// Complete marks a session done and finalizes its sink. A sink that refuses
// to finalize did not complete: the session is marked failed with the sink's
// error rather than announced done (a downloaded file that would not close is
// not a finished download). The context bounds the finalize so a hung sink
// cannot pin the session.
func (e *Engine) Complete(ctx context.Context, id string) error {
	e.mu.Lock()
	s, ok := e.sessions[id]
	if !ok {
		e.mu.Unlock()
		return errNotFound("session %q not found", id)
	}
	s.mu.Lock()
	s.Status = StatusDone
	if s.Sink != nil {
		if err := s.Sink.Finalize(ctx, s); err != nil {
			s.Status = StatusFailed
			s.Error = err.Error()
		}
	}
	job := s.toJobLocked()
	s.mu.Unlock()
	e.mu.Unlock()
	e.recordJob(job)
	return nil
}

// LiveSessions returns the number of active (queued or running) sessions.
// Quotas count active sessions; idle silent sessions are evicted rather than
// blocking others (PLAN.md §9.1).
func (e *Engine) LiveSessions() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, s := range e.sessions {
		if s.isActive() {
			n++
		}
	}
	return n
}

// Watch runs the liveness watchdog until Close. The watchdog evicts sessions
// whose heartbeat timed out (play) or whose TTL expired (any session), so no
// phantom session lingers (PLAN.md §9.1).
func (e *Engine) Watch(ctx context.Context) {
	e.watchWG.Add(1)
	defer e.watchWG.Done()
	e.watched.Store(true)
	defer close(e.done)
	tick := e.ttl
	if tick <= 0 {
		tick = time.Minute
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			e.sweep()
		case <-e.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// sweep evicts sessions that are no longer live.
func (e *Engine) sweep() {
	e.mu.Lock()
	var dead []*Session
	for id, s := range e.sessions {
		s.mu.Lock()
		if !s.isActiveLocked() {
			s.mu.Unlock()
			continue
		}
		now := e.now()
		if s.Goal == GoalPlay {
			// A play session must heartbeat within interval+grace (§9.1).
			if now.Sub(s.lastHeartbeat) > e.interval+e.grace {
				s.Status = StatusFailed
				s.Error = "heartbeat timed out"
				dead = append(dead, s)
				s.mu.Unlock()
				continue
			}
		} else if now.Sub(s.lastProgress) > e.ttl {
			s.Status = StatusFailed
			s.Error = "download stalled"
			dead = append(dead, s)
			s.mu.Unlock()
			continue
		}
		if now.Sub(s.createdAt) > e.ttl {
			// TTL is the zombie cap for every session (§9.1).
			if s.isActiveLocked() {
				s.Status = StatusFailed
				s.Error = "session TTL expired"
				dead = append(dead, s)
			}
		}
		_ = id
		s.mu.Unlock()
	}
	for _, d := range dead {
		delete(e.sessions, d.ID)
		key := accountKey{d.Context.GetProvider(), d.Context.GetAccountId(), d.Context.GetMemberUserId()}
		delete(e.byAccount[key], d.ID)
		e.recordJob(d.toJob())
	}
	e.mu.Unlock()
}

// Close stops the watchdog. Safe to call whether or not Watch is running.
func (e *Engine) Close() {
	select {
	case <-e.stopCh:
	default:
		close(e.stopCh)
	}
	if e.watched.Load() {
		e.watchWG.Wait()
	}
}

// isActive reports whether the session is queued or running.
func (s *Session) isActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isActiveLocked()
}

// isActiveLocked is isActive for a caller already holding s.mu's write lock.
func (s *Session) isActiveLocked() bool {
	return s.Status == StatusQueued || s.Status == StatusRunning
}

// StagedMenu returns the session's staged play menu under the session's
// read lock, or nil when none was staged (a download, or a play with no
// location-bearing tracks). The returned slice shares its (immutable) track
// elements with the session.
func (s *Session) StagedMenu() []*corev1.Track {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Menu
}

// Job returns the session as its system-of-record Job (PLAN.md §9.1).
func (s *Session) Job() *corev1.Job { return s.toJob() }

// toJob renders the session as its system-of-record Job (PLAN.md §9.1; a
// delivery session is a kind of job, §2.3).
func (s *Session) toJob() *corev1.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.toJobLocked()
}

// toJobLocked is toJob for a caller already holding s.mu.
func (s *Session) toJobLocked() *corev1.Job {
	j := &corev1.Job{
		Id:          s.ID,
		Kind:        corev1.JobKind_JOB_KIND_DELIVERY,
		OwnerUserId: s.Context.GetMemberUserId(),
		Delivery:    cloneContext(&s.Context),
	}
	switch s.Status {
	case StatusQueued:
		j.Status = corev1.JobStatus_JOB_STATUS_QUEUED
	case StatusRunning:
		j.Status = corev1.JobStatus_JOB_STATUS_RUNNING
	case StatusDone:
		j.Status = corev1.JobStatus_JOB_STATUS_DONE
	case StatusFailed, StatusRevoked:
		j.Status = corev1.JobStatus_JOB_STATUS_FAILED
		j.Error = s.Error
	case StatusCancelled:
		j.Status = corev1.JobStatus_JOB_STATUS_CANCELLED
	}
	return j
}

func cloneContext(d *corev1.DeliveryContext) *corev1.DeliveryContext {
	if d == nil {
		return nil
	}
	return proto.Clone(d).(*corev1.DeliveryContext)
}

// ValidateManifest asserts a resolved manifest conforms to the MediaSource
// contract's essential rules before any session runs on it (PLAN.md §2.5:
// reject, never downgrade). The rules mirror PLAN.md §6.2; full validation
// lives in the schema package, which the engine deliberately does not import
// to avoid a dependency cycle at the packet boundary.
func ValidateManifest(ms *corev1.MediaSource) error {
	if ms == nil {
		return fmt.Errorf("manifest: nil")
	}
	if ms.GetType() == corev1.MediaSourceType_MEDIA_SOURCE_TYPE_UNSPECIFIED {
		return fmt.Errorf("manifest: type is required")
	}
	if ms.GetSeekable() == corev1.Seekable_SEEKABLE_UNSPECIFIED {
		return fmt.Errorf("manifest: seekable is required")
	}
	if ms.GetAddressable() == corev1.Addressable_ADDRESSABLE_UNSPECIFIED {
		return fmt.Errorf("manifest: addressable is required")
	}
	if len(ms.GetTracks()) == 0 {
		return fmt.Errorf("manifest: at least one track is required")
	}
	return nil
}

// RejectUnsupportedDRM refuses a manifest that carries a DRM-encrypted track.
// The MediaSource contract permits DRM (media_source.proto §TrackDrm), but a
// v1 engine has no key or license handling (PLAN.md §6.6), so the only honest
// outcome is refusal — never a silent skip that streams an encrypted track
// unlicensed (PLAN.md §2.5). The schema keeps accepting the field; it is this
// layer that declines delivery.
func RejectUnsupportedDRM(ms *corev1.MediaSource) error {
	for _, tr := range ms.GetTracks() {
		if d := tr.GetDelivery(); d != nil && d.GetDrm() != nil {
			return fmt.Errorf("track %q is DRM-encrypted; DRM delivery is not supported in v1 (requires §6.6 key/license handling)", tr.GetId())
		}
	}
	return nil
}

type deliveryError struct {
	code codes.Code
	msg  string
}

func (e *deliveryError) Error() string { return e.msg }

// codeInvalid, codeNotFound, and codeQuota are the gRPC status codes delivery
// errors carry. They are named constants from the canonical set rather than
// bare integers, so a reader — and the apiserver, which re-raises them
// verbatim — sees the same vocabulary everywhere. Delivery never answers
// PermissionDenied: a refusal outside the caller's reach is NotFound
// (PLAN.md §8.1).
const (
	codeInvalid  = codes.InvalidArgument
	codeNotFound = codes.NotFound
	codeQuota    = codes.ResourceExhausted
)

func errInvalid(format string, a ...any) error {
	return &deliveryError{code: codeInvalid, msg: fmt.Sprintf(format, a...)}
}

func errNotFound(format string, a ...any) error {
	return &deliveryError{code: codeNotFound, msg: fmt.Sprintf(format, a...)}
}

// Code returns the gRPC-style status code for a delivery error.
func Code(err error) int32 {
	if e, ok := err.(*deliveryError); ok {
		return int32(e.code)
	}
	if err == nil {
		return 0
	}
	return int32(codes.Unknown)
}
