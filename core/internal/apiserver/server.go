package apiserver

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/schema"
	"github.com/nem-git/abcmovies/core/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// AccountAttacher wires a freshly-linked account into the live provider slot
// that serves its server (PLAN.md §5.1): route the record to that slot, admit
// the account to it, and build its sync machinery. The composition root
// implements it; until armed, linking stores the record and the account waits
// for the next boot to come alive.
type AccountAttacher interface {
	AttachAccount(rec accounts.Record) error
}

// AccountDropper takes an unlinked account out of the live slot: the slot
// drops its cached session, its refresh job leaves the shared scheduler, and
// its source-cache rows are dropped. The api layer owns the record, session
// blob, reach and stream revocation; this is the slot side.
type AccountDropper interface {
	DropAccount(rec accounts.Record) error
}

// DeliveryManager is the delivery-engine surface the API layer calls
// (PLAN.md §6, §9.1). Exposing an interface keeps the apiserver decoupled
// from the engine's internals and lets the handlers be tested with a stub.
type DeliveryManager interface {
	Start(ctx context.Context, req delivery.StartRequest) (*delivery.Session, error)
	// Heartbeat(id, memberUserID) keeps the session alive only when its
	// session member matches memberUserID; the engine rejects otherwise
	// (§2.2 member-scoping).
	Heartbeat(id string, memberUserID string) error
	PlayMenu(sessionID string) (*PlayMenu, error)
	// RevokeAllOnAccount ends every session routed through the account
	// (all members): removal of the record is a full revocation
	// (PLAN.md §7.5).
	RevokeAllOnAccount(accountID string) int
	// RevokeOthersOnAccount ends every session on the account whose member
	// is not in keepMembers: narrowing sharing revokes exactly the members
	// who lost access (§7.1).
	RevokeOthersOnAccount(provider, accountID string, keepMembers []string) int
	// ApplyAccountCap re-evaluates the account's effective stream allowance
	// and, on enforce-now, ends the excess sessions; it never touches a
	// session when the policy is new-sessions-only.
	ApplyAccountCap(ctx context.Context, provider, accountID string, enforceNow bool) (int, error)
}

// Server implements the CoreService (PLAN.md §8).
type Server struct {
	apiv1.UnimplementedCoreServiceServer
	bus      Bus
	stores   config.Stores
	auth     *auth.CompositeAuthenticator
	session  auth.Session
	delivery DeliveryManager
	seq      atomic.Int64

	// library gates every read of the merged catalog and the delivery
	// authorization (PLAN.md §5.1); nil until armed, when absent the library
	// RPCs return Unavailable. accounts persists linked-account records and
	// their vaulted sessions; it is always available over the vault. probers
	// validate candidate linked-account credentials per provider (PLAN.md
	// §3.5: nothing is vaulted that was not probed); armed by wiring.
	library       LibrarySeam
	accounts      *accounts.Store
	probers       map[string]CredentialProber
	attacher      AccountAttacher
	dropper       AccountDropper
	userDirectory UserDirectory
	// capChangeDefault is the instance-wide cap-change behaviour accounts
	// inherit when they carry no choice of their own; armed by the
	// composition root from the delivery config, defaulting to
	// new-sessions-only.
	capChangeDefault accounts.CapChangePolicy
}

// SetCapChangeDefault arms the instance-wide default for what lowering an
// account's concurrent-stream cap does to running sessions (PLAN.md §7.2).
// An empty value is deliberately accepted and means "new-sessions-only":
// the shipped default never kills a running session.
func (s *Server) SetCapChangeDefault(p accounts.CapChangePolicy) {
	if p == accounts.CapChangePolicyDefault {
		s.capChangeDefault = accounts.CapChangePolicyNewSessionsOnly
		return
	}
	s.capChangeDefault = p
}

// NewServer returns a CoreService backed by the given bus, stores, and auth.
// An optional DeliveryManager may be supplied; when absent the delivery RPCs
// return Unavailable.
func NewServer(bus Bus, stores config.Stores, authenticator *auth.CompositeAuthenticator, session auth.Session, dm ...DeliveryManager) *Server {
	var d DeliveryManager
	if len(dm) > 0 {
		d = dm[0]
	}
	return &Server{
		bus:      bus,
		stores:   stores,
		auth:     authenticator,
		session:  session,
		delivery: d,
		// The accounts store is owned here, over the same vault the wiring's
		// session-vault recovery reads, so a link made through the API is
		// picked up by slot provisioning exactly as an operator-configured
		// account would be (PLAN.md §3.5).
		accounts: accounts.NewStore(stores.Vault, nil),
		probers:  map[string]CredentialProber{},
		// The shipped default never kills a running session: a cap change
		// applies from the next session (TECHNICAL-DECISIONS.md).
		capChangeDefault: accounts.CapChangePolicyNewSessionsOnly,
	}
}

// SetAttacher arms the runtime link path (PLAN.md §5.1): when armed, a link
// not only stores the record and session but wires the account into its live
// provider slot, so the account is reachable without a restart. Compose-time
// only, like SetProber/SetLibrary.
func (s *Server) SetAttacher(a AccountAttacher) {
	if a == nil {
		return
	}
	s.attacher = a
}

// SetDropper arms the live-removal counterpart to SetAttacher: when armed, an
// unlink also takes the account out of its running slot (cached session,
// refresh job, source-cache rows), not just out of the permission views.
func (s *Server) SetDropper(d AccountDropper) {
	if d == nil {
		return
	}
	s.dropper = d
}

// UserDirectory answers whether a user id names a real account. Sharing with
// a mistyped id would silently grant access to nobody, so the API refuses
// before recording it — the error names the user, not a generic roster
// rejection. Compose-time only, like SetProber/SetAttacher.
type UserDirectory interface {
	HasUser(userID string) (bool, error)
}

// userStoreDirectory adapts the auth user store to the UserDirectory seam.
// User ids are "user:"+username and the store keys users by the same id, so
// the check is a single lookup; any lookup failure reads as "no such user"
// — a user whose record cannot be read could not log in anyway, so the
// shares through it are nothing to grant.
type userStoreDirectory struct{ users auth.UserStore }

// NewUserDirectory adapts the given user store to the UserDirectory seam.
func NewUserDirectory(users auth.UserStore) UserDirectory {
	if users == nil {
		return nil
	}
	return userStoreDirectory{users: users}
}

func (d userStoreDirectory) HasUser(userID string) (bool, error) {
	uname, ok := strings.CutPrefix(userID, "user:")
	if !ok || uname == "" {
		return false, nil
	}
	if _, err := d.users.GetUser(uname); err != nil {
		return false, nil
	}
	return true, nil
}

// SetUserDirectory arms the user-existence check used by the accounts RPCs.
func (s *Server) SetUserDirectory(d UserDirectory) {
	if d == nil {
		return
	}
	s.userDirectory = d
}

// SetDelivery arms the delivery engine after construction — used when the
// engine is composed lazily (after slots are wired) rather than at NewServer
// time. Until SetDelivery is called the delivery RPCs return Unavailable.
func (s *Server) SetDelivery(dm DeliveryManager) {
	s.delivery = dm
}

// Delivery returns the currently armed delivery engine, or nil.
func (s *Server) Delivery() DeliveryManager { return s.delivery }

// GetJob returns a job's current state from the jobs store (PLAN.md §9.1).
// The job is visible only to its owner: the caller presenting a delivery job
// that belongs to another member gets PermissionDenied, the same boundary the
// play menu applies (§2.2 member-scoping).
func (s *Server) GetJob(ctx context.Context, req *apiv1.GetJobRequest) (*apiv1.GetJobResponse, error) {
	if err := schema.ValidateGetJobRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	uid, _ := UserIDFromContext(ctx)
	raw, err := s.stores.Jobs.Get(ctx, "job:"+req.GetJobId())
	if err == store.ErrKeyNotFound {
		return nil, status.Error(codes.NotFound, "job not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read job")
	}
	var job corev1.Job
	if err := proto.Unmarshal(raw, &job); err != nil {
		return nil, status.Error(codes.Internal, "corrupted job data")
	}
	// Jobs without an owner belong to no one; a delivery job always carries one.
	if owner := job.GetOwnerUserId(); owner != "" && owner != uid {
		return nil, status.Error(codes.PermissionDenied, "job belongs to another user")
	}
	return &apiv1.GetJobResponse{Job: &job}, nil
}

// StartDelivery begins a play or download session and returns its job
// (PLAN.md §6, §9.1). Start is a plain "create" — each call makes a new
// session (TECHNICAL-DECISIONS.md §1.30).
func (s *Server) StartDelivery(ctx context.Context, req *apiv1.StartDeliveryRequest) (*apiv1.StartDeliveryResponse, error) {
	if err := schema.ValidateStartDeliveryRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if s.delivery == nil {
		return nil, status.Error(codes.Unavailable, "delivery engine not configured")
	}
	goal, err := deliveryGoal(req.GetGoal())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	// The member the session is attributed to is the authenticated caller —
	// the quota and policy keys this delivery is counted against attach to
	// that identity (§2.2). Account reachability is verified next: an
	// authenticated user may only start a delivery through an account they
	// may derive a library from.
	uid, _ := UserIDFromContext(ctx)
	if s.library == nil {
		return nil, status.Error(codes.Unavailable, "library engine not configured")
	}
	if _, ok := s.library.ReachAuthorized(req.GetAccountId(), uid); !ok {
		return nil, status.Error(codes.PermissionDenied, "account not reachable by the caller")
	}
	sess, err := s.delivery.Start(ctx, delivery.StartRequest{
		Goal:           goal,
		MemberUserID:   uid,
		Provider:       req.GetProvider(),
		AccountID:      req.GetAccountId(),
		NativeID:       req.GetNativeId(),
		Sink:           req.GetSink(),
		SelectedTarget: req.GetSelectedTarget(),
		Container:      req.GetContainer(),
	})
	if err != nil {
		return nil, status.Error(codes.Code(delivery.Code(err)), err.Error())
	}
	s.recordJob(ctx, sess.Job())
	return &apiv1.StartDeliveryResponse{Job: sess.Job()}, nil
}

// Heartbeat keeps a play session alive (PLAN.md §9.1).
func (s *Server) Heartbeat(ctx context.Context, req *apiv1.HeartbeatRequest) (*apiv1.HeartbeatResponse, error) {
	if err := schema.ValidateHeartbeatRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if s.delivery == nil {
		return nil, status.Error(codes.Unavailable, "delivery engine not configured")
	}
	uid, _ := UserIDFromContext(ctx)
	if err := s.delivery.Heartbeat(req.GetSessionId(), uid); err != nil {
		return nil, status.Error(codes.Code(delivery.Code(err)), err.Error())
	}
	return &apiv1.HeartbeatResponse{SessionId: req.GetSessionId()}, nil
}

// deliveryGoal maps the API goal enum to the engine's goal.
func deliveryGoal(g apiv1.DeliveryGoal) (delivery.Goal, error) {
	switch g {
	case apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY:
		return delivery.GoalPlay, nil
	case apiv1.DeliveryGoal_DELIVERY_GOAL_DOWNLOAD:
		return delivery.GoalDownload, nil
	default:
		return "", status.Error(codes.InvalidArgument, "unknown delivery goal")
	}
}

// recordJob persists the job so GetJob reflects the new state. Delivery-job
// status events are announced by the engine's RecordJob hook (wired in
// core/app production code): the engine observes every transition —
// including the ones no client requested — while the API layer only ever
// sees StartDelivery, so the engine is the one announcer.
func (s *Server) recordJob(ctx context.Context, job *corev1.Job) {
	if job == nil {
		return
	}
	raw, err := proto.Marshal(job)
	if err != nil {
		return
	}
	_ = s.stores.Jobs.Put(ctx, "job:"+job.GetId(), raw)
}

// RecordJobStatus persists a delivery job's state and announces its
// transition on the bus. It is the single announcer for delivery-job status:
// production wires it as the engine's RecordJob hook, and test harnesses
// wire the same function, so no fixture reconstructs the envelope by hand.
func RecordJobStatus(ctx context.Context, st store.Store, b Bus, job *corev1.Job) {
	if job == nil {
		return
	}
	raw, err := proto.Marshal(job)
	if err != nil {
		return
	}
	_ = st.Put(ctx, "job:"+job.GetId(), raw)
	b.Publish(&corev1.EventEnvelope{
		Id:       fmt.Sprintf("evt-delivery-%s", job.GetId()),
		Type:     corev1.EventType_EVENT_TYPE_JOB_STATUS,
		Audience: corev1.EventAudience_EVENT_AUDIENCE_USER,
		UserId:   job.GetOwnerUserId(),
		Payload: &corev1.EventEnvelope_JobStatus{
			JobStatus: &corev1.JobStatusEvent{
				JobId:  job.GetId(),
				Status: job.GetStatus(),
			},
		},
		EmittedAt: timestamppb.Now(),
	})
}

// CreateJob persists a job in the jobs store. This is an internal method for
// M0 — it proves the jobs storage class works end-to-end. A public CreateJob
// RPC is not yet exposed (the proto does not define one).
func (s *Server) CreateJob(ctx context.Context, job *corev1.Job) error {
	if err := schema.ValidateJob(job); err != nil {
		return err
	}
	raw, err := proto.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	if err := s.stores.Jobs.Put(ctx, "job:"+job.GetId(), raw); err != nil {
		return err
	}
	// Publish job-status event (PLAN.md §9.2, M0 acceptance).
	s.bus.Publish(&corev1.EventEnvelope{
		Id:       fmt.Sprintf("evt-job-%s", job.GetId()),
		Type:     corev1.EventType_EVENT_TYPE_JOB_STATUS,
		Audience: corev1.EventAudience_EVENT_AUDIENCE_USER,
		UserId:   job.GetOwnerUserId(),
		Payload: &corev1.EventEnvelope_JobStatus{
			JobStatus: &corev1.JobStatusEvent{
				JobId:  job.GetId(),
				Status: job.GetStatus(),
			},
		},
		EmittedAt: timestamppb.Now(),
	})
	return nil
}

// Subscribe streams events to the client. The stream stays open until the
// client disconnects.
func (s *Server) Subscribe(req *apiv1.SubscribeRequest, stream apiv1.CoreService_SubscribeServer) error {
	if err := schema.ValidateSubscribeRequest(req); err != nil {
		return status.Error(codes.InvalidArgument, err.Error())
	}
	uid, _ := UserIDFromContext(stream.Context())
	id := fmt.Sprintf("sub-%d", s.seq.Add(1))
	ch := s.bus.Subscribe(id, uid)
	defer s.bus.Unsubscribe(id)
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(&apiv1.SubscribeResponse{Event: event}); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

// SignUp creates a new user account. The auth method is determined by the
// oneof field in the request.
func (s *Server) SignUp(ctx context.Context, req *apiv1.SignUpRequest) (*apiv1.SignUpResponse, error) {
	if err := schema.ValidateSignUpRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	method := authMethod(req.GetPassword(), nil)
	a, ok := s.auth.Get(method)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported auth method: %s", method)
	}
	result, err := a.SignUp(req.GetUsername(), req.GetPassword().GetPassword())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	token, err := s.mintSession(result.UserID, result.DEK)
	if err != nil {
		return nil, err
	}
	return &apiv1.SignUpResponse{
		UserId:      result.UserID,
		RecoveryKey: result.RecoveryKey,
		Token:       token,
	}, nil
}

// mintSession issues a bearer token for uid and caches its unwrapped DEK
// (IMPLEMENTATION.md §1.3). Both auth entry points share this: signup logs
// the user in, and login re-issues the same shape of session.
func (s *Server) mintSession(uid string, dek []byte) (string, error) {
	token, err := s.session.Mint(uid)
	if err != nil {
		return "", status.Error(codes.Internal, "failed to create session")
	}
	if err := s.session.StoreDEK(token, dek); err != nil {
		return "", status.Error(codes.Internal, "failed to cache session key material")
	}
	return token, nil
}

// Login authenticates a user and returns a session token. The auth method
// is determined by the oneof field in the request.
func (s *Server) Login(ctx context.Context, req *apiv1.LoginRequest) (*apiv1.LoginResponse, error) {
	if err := schema.ValidateLoginRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	method := authMethod(nil, req.GetPassword())
	a, ok := s.auth.Get(method)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "unsupported auth method: %s", method)
	}
	result, err := a.Login(req.GetUsername(), req.GetPassword().GetPassword())
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	token, err := s.mintSession(result.UserID, result.DEK)
	if err != nil {
		return nil, err
	}
	return &apiv1.LoginResponse{
		Token:  token,
		UserId: result.UserID,
	}, nil
}

// authMethod returns the method name from the non-nil oneof field.
func authMethod(passwordSignUp *apiv1.PasswordSignUp, passwordLogin *apiv1.PasswordLogin) string {
	if passwordSignUp != nil {
		return "password"
	}
	if passwordLogin != nil {
		return "password"
	}
	return ""
}
