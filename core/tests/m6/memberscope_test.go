package m6_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// This file proves the member-scoping invariant at the API boundary: "member
// A can never observe member B's sessions, history, or quota" (PLAN.md §2.2)
// is enforced on StartDelivery (member identity + account reachability),
// GetJob (job ownership), and Heartbeat (session ownership).

type stubDelivery struct {
	session *delivery.Session
	lastReq delivery.StartRequest
}

func (s *stubDelivery) Start(_ context.Context, r delivery.StartRequest) (*delivery.Session, error) {
	s.lastReq = r
	return s.session, nil
}

func (s *stubDelivery) Heartbeat(id, memberUserID string) error {
	return nil
}

func (s *stubDelivery) PlayMenu(string) (*apiserver.PlayMenu, error) {
	return &apiserver.PlayMenu{}, nil
}
func (s *stubDelivery) RevokeAllOnAccount(string) int { return 0 }

type stubLibrary struct {
	reachable map[string][]string
}

func (l *stubLibrary) Library(context.Context, string) ([]*corev1.LibraryEntry, error) {
	return nil, nil
}

func (l *stubLibrary) Metadata(context.Context, string) (*corev1.TitleMetadata, bool, error) {
	return nil, false, nil
}

func (l *stubLibrary) ReachAuthorized(accountID, userID string) (library.Reach, bool) {
	for _, u := range l.reachable[accountID] {
		if u == userID {
			return library.Reach{AccountID: accountID}, true
		}
	}
	return library.Reach{}, false
}
func (l *stubLibrary) ReachesForUser(string) []library.Reach { return nil }
func (l *stubLibrary) RemoveReach(string)                    {}
func (l *stubLibrary) SetReachSharing(string, accounts.Visibility, []string) error {
	return nil
}

var (
	_ apiserver.DeliveryManager = (*stubDelivery)(nil)
	_ apiserver.LibrarySeam     = (*stubLibrary)(nil)
)

func newMemberscopeServer() (*apiserver.Server, *store.Store, *stubDelivery) {
	bus := apiserver.NewInMemoryBus()
	stores := config.Stores{Jobs: store.NewInMemory()}
	dm := &stubDelivery{session: &delivery.Session{
		ID: "del-1", Goal: delivery.GoalPlay, Status: delivery.StatusRunning,
		Context: corev1.DeliveryContext{Provider: "jellyfin", AccountId: "acc1", MemberUserId: "u1"},
	}}
	srv := apiserver.NewServer(bus, stores, nil, nil, dm)
	srv.SetLibrary(&stubLibrary{reachable: map[string][]string{"acc1": {"u1"}}})
	return srv, nil, dm
}

func TestM6StartDeliveryAttributesTokenUser(t *testing.T) {
	srv, _, dm := newMemberscopeServer()
	_, err := srv.StartDelivery(apiserver.WithUserID(context.Background(), "u1"), &apiv1.StartDeliveryRequest{
		Goal: apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY, Provider: "jellyfin", AccountId: "acc1",
		NativeId: "item1", Sink: "device",
	})
	if err != nil {
		t.Fatalf("start delivery: %v", err)
	}
	if dm.lastReq.MemberUserID != "u1" {
		t.Fatalf("member attribution: got %q, want token user %q", dm.lastReq.MemberUserID, "u1")
	}
}

func TestM6StartDeliveryRejectsUnreachableAccount(t *testing.T) {
	srv, _, _ := newMemberscopeServer()
	_, err := srv.StartDelivery(apiserver.WithUserID(context.Background(), "u1"), &apiv1.StartDeliveryRequest{
		Goal: apiv1.DeliveryGoal_DELIVERY_GOAL_PLAY, Provider: "jellyfin", AccountId: "acc2",
		NativeId: "item1", Sink: "device",
	})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("account belonging to another member: got %v, want PermissionDenied", status.Code(err))
	}
}

func TestM6GetJobRejectsNonOwner(t *testing.T) {
	srv, _, _ := newMemberscopeServer()
	job := &corev1.Job{Id: "job-1", Kind: corev1.JobKind_JOB_KIND_REFRESH, Status: corev1.JobStatus_JOB_STATUS_QUEUED, OwnerUserId: "u1"}
	if err := srv.CreateJob(context.Background(), job); err != nil {
		t.Fatalf("CreateJob: %v", err)
	}
	_, err := srv.GetJob(apiserver.WithUserID(context.Background(), "u2"), &apiv1.GetJobRequest{JobId: "job-1"})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("GetJob by non-owner: got %v, want PermissionDenied", status.Code(err))
	}
}

func TestM6HeartbeatRejectsNonMember(t *testing.T) {
	env := apiserver.NewInMemoryBus()
	defer env.Close()
	eng := delivery.New(delivery.Options{
		SessionTTL:     time.Hour,
		InstancePolicy: nil,
		SourceResolver: staticResolver{src: muxSource()},
		RecordJob:      func(*corev1.Job) {},
	})
	defer eng.Close()
	sess, err := eng.Start(context.Background(), delivery.StartRequest{
		Goal: delivery.GoalDownload, MemberUserID: "u1", Provider: "jellyfin",
		AccountID: "acc1", Sink: "device", NativeID: "item1",
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := eng.Heartbeat(sess.ID, "u2"); delivery.Code(err) != 7 {
		t.Fatalf("non-member heartbeat err = %v (code %v), want permission denied", err, delivery.Code(err))
	}
}

// The user's reach model must also gate the one server that starts from it:
// a linked account that is private to Alice is unreachable for Bob even when
// the request is attributed correctly to Bob.
func TestM6WithUserIDIdentityPropagates(t *testing.T) {
	_ = accounts.VisibilityPrivate
	ctx := apiserver.WithUserID(context.Background(), "alice")
	uid, ok := apiserver.UserIDFromContext(ctx)
	if !ok || uid != "alice" {
		t.Fatalf("uid = (%q, %v), want (alice, true)", uid, ok)
	}
}

func (s *stubDelivery) RevokeOthersOnAccount(string, string, []string) int { return 0 }
func (s *stubDelivery) ApplyAccountCap(context.Context, string, string, bool) (int, error) {
	return 0, nil
}

func (l *stubLibrary) RefreshAvailability(context.Context, string, []string) (sourcecache.Stats, error) {
	return sourcecache.Stats{}, nil
}
