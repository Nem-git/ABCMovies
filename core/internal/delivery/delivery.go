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
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
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
type Session struct {
	ID      string
	Goal    Goal
	Context corev1.DeliveryContext
	Status  Status
	Error   string

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
	SessionTTL        time.Duration
	HeartbeatInterval time.Duration
	HeartbeatGrace    time.Duration
	SourceResolver    Resolver
	SinkFactory       SinkFactory
	RecordJob         func(*corev1.Job)
	Now               func() time.Time
	Logger            *slog.Logger
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

	resolver  Resolver
	sinkMaker SinkFactory
	recordJob func(*corev1.Job)
	now       func() time.Time
	logger    *slog.Logger

	stopCh  chan struct{}
	done    chan struct{}
	watchWG sync.WaitGroup
	watched atomic.Bool
}

// New builds a delivery engine. sessionTTL is the zombie cap; interval and
// grace bound play-session heartbeat liveness (TECHNICAL-DECISIONS.md §1.14).
func New(opts Options) *Engine {
	e := &Engine{
		sessions:  make(map[string]*Session),
		byAccount: make(map[accountKey]map[string]struct{}),
		ttl:       opts.SessionTTL,
		interval:  opts.HeartbeatInterval,
		grace:     opts.HeartbeatGrace,
		resolver:  opts.SourceResolver,
		sinkMaker: opts.SinkFactory,
		recordJob: opts.RecordJob,
		now:       opts.Now,
		logger:    opts.Logger,
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
	}
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
		return nil, fmt.Errorf("resolve sources: %w", err)
	}
	if err := ValidateManifest(src); err != nil {
		return nil, errInvalid("provider returned an invalid manifest: %v", err)
	}

	now := e.now()
	sess := &Session{
		ID:            e.newSessionID(),
		Goal:          req.Goal,
		Status:        StatusQueued,
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
		},
	}

	if e.sinkMaker != nil {
		sink, err := e.sinkMaker.NewSink(ctx, sess, src.GetTracks())
		if err != nil {
			return nil, fmt.Errorf("sink %q: %w", req.Sink, err)
		}
		sess.Sink = sink
	}

	key := accountKey{req.Provider, req.AccountID, req.MemberUserID}
	e.mu.Lock()
	e.sessions[sess.ID] = sess
	if e.byAccount[key] == nil {
		e.byAccount[key] = make(map[string]struct{})
	}
	e.byAccount[key][sess.ID] = struct{}{}
	e.mu.Unlock()

	sess.Status = StatusRunning
	e.recordJob(sess.toJob())
	return sess, nil
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
// paused session still heartbeats, so it is not killed.
func (e *Engine) Heartbeat(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	s, ok := e.sessions[id]
	if !ok {
		return errNotFound("session %q not found", id)
	}
	if !s.isActive() {
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
	if !s.isActive() {
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
		s.Status = StatusRevoked
		s.Error = "account revoked"
		if s.Sink != nil {
			s.Sink.Abort(context.Background(), s)
		}
		e.recordJob(s.toJob())
		killed++
	}
	delete(e.byAccount, key)
	e.mu.Unlock()
	return killed
}

// Complete marks a session done and finalizes its sink.
func (e *Engine) Complete(id string) error {
	e.mu.Lock()
	s, ok := e.sessions[id]
	if !ok {
		e.mu.Unlock()
		return errNotFound("session %q not found", id)
	}
	s.Status = StatusDone
	if s.Sink != nil {
		if err := s.Sink.Finalize(context.Background(), s); err != nil {
			s.Error = err.Error()
		}
	}
	e.mu.Unlock()
	e.recordJob(s.toJob())
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
		if !s.isActive() {
			continue
		}
		now := e.now()
		if s.Goal == GoalPlay {
			// A play session must heartbeat within interval+grace (§9.1).
			if now.Sub(s.lastHeartbeat) > e.interval+e.grace {
				s.Status = StatusFailed
				s.Error = "heartbeat timed out"
				dead = append(dead, s)
				continue
			}
		} else if now.Sub(s.lastProgress) > e.ttl {
			s.Status = StatusFailed
			s.Error = "download stalled"
			dead = append(dead, s)
			continue
		}
		if now.Sub(s.createdAt) > e.ttl {
			// TTL is the zombie cap for every session (§9.1).
			if s.isActive() {
				s.Status = StatusFailed
				s.Error = "session TTL expired"
				dead = append(dead, s)
			}
		}
		_ = id
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
	return s.Status == StatusQueued || s.Status == StatusRunning
}

// Job returns the session as its system-of-record Job (PLAN.md §9.1).
func (s *Session) Job() *corev1.Job { return s.toJob() }

// toJob renders the session as its system-of-record Job (PLAN.md §9.1; a
// delivery session is a kind of job, §2.3).
func (s *Session) toJob() *corev1.Job {
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

type deliveryError struct {
	code int
	msg  string
}

func (e *deliveryError) Error() string { return e.msg }

const (
	codeInvalid  = 3
	codeNotFound = 5
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
	return 2 // Unknown
}
