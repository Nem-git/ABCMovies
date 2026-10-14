package apiserver_test

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
)

func TestGetPlayInfo_ReturnsStagedMenu(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	dm := &stubDelivery{
		playmenu: &apiserver.PlayMenu{
			SessionID:    "del-1",
			MemberUserID: "user-1",
			Container:    "mkv",
			Tracks: []apiserver.PlayMenuTrack{
				{TrackID: "v1", Track: videoTrack("v1"), RelayToken: "tok-v1"},
			},
		},
	}
	srv := apiserver.NewServer(apiserver.Settings{Bus: bus, Stores: testStores(t), Authenticator: authenticator, Session: session})
	srv.SetDelivery(dm)

	resp, err := srv.GetPlayInfo(ctxAs(session, "user-1"), &apiv1.GetPlayInfoRequest{SessionId: "del-1"})
	if err != nil {
		t.Fatalf("GetPlayInfo: %v", err)
	}
	if resp.GetContainer() != "mkv" {
		t.Fatalf("container = %q, want mkv", resp.GetContainer())
	}
	tracks := resp.GetRawTracks().GetTracks()
	if len(tracks) != 1 {
		t.Fatalf("tracks = %d, want 1", len(tracks))
	}
	pt := tracks[0]
	if pt.GetTrackId() != "v1" || pt.GetRelayUrl() != "/media/relay/tok-v1" {
		t.Fatalf("track = %+v", pt)
	}
	if v := pt.GetVideo(); v == nil || v.GetCodec() != "hevc" || v.GetWidth() != 3840 {
		t.Fatalf("mapped video = %+v", v)
	}
}

func TestGetPlayInfo_MissesAndAccess(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	dm := &stubDelivery{
		playmenu: &apiserver.PlayMenu{SessionID: "del-1", MemberUserID: "user-1"},
		menuErr:  apiserver.ErrPlayMenuNotFound,
	}
	srv := apiserver.NewServer(apiserver.Settings{Bus: bus, Stores: testStores(t), Authenticator: authenticator, Session: session})
	srv.SetDelivery(dm)

	// A session with no menu is a clean NotFound (recovery semantics §6.2).
	if _, err := srv.GetPlayInfo(ctxAs(session, "user-1"), &apiv1.GetPlayInfoRequest{SessionId: "del-x"}); status.Code(err) != codes.NotFound {
		t.Fatalf("miss code = %v, want NotFound", status.Code(err))
	}

	// An unarmed delivery engine is Unavailable.
	bare := apiserver.NewServer(apiserver.Settings{Bus: bus, Stores: testStores(t), Authenticator: authenticator, Session: session})
	if _, err := bare.GetPlayInfo(ctxAs(session, "user-1"), &apiv1.GetPlayInfoRequest{SessionId: "del-1"}); status.Code(err) != codes.Unavailable {
		t.Fatalf("unarmed code = %v, want Unavailable", status.Code(err))
	}
}

func TestGetPlayInfo_MemberScoped(t *testing.T) {
	bus := apiserver.NewInMemoryBus()
	defer bus.Close()
	authenticator, session := testAuth(t)
	dm := &stubDelivery{
		playmenu: &apiserver.PlayMenu{
			SessionID:    "del-1",
			MemberUserID: "user-1",
			Tracks:       []apiserver.PlayMenuTrack{{TrackID: "v1", Track: videoTrack("v1"), RelayToken: "t"}},
		},
	}
	srv := apiserver.NewServer(apiserver.Settings{Bus: bus, Stores: testStores(t), Authenticator: authenticator, Session: session})
	srv.SetDelivery(dm)

	// The menu belongs to the member; a different user must not recover it —
	// and gets the same NotFound as a missing menu (PLAN.md §8.1).
	if _, err := srv.GetPlayInfo(ctxAs(session, "user-2"), &apiv1.GetPlayInfoRequest{SessionId: "del-1"}); status.Code(err) != codes.NotFound {
		t.Fatalf("cross-user read code = %v, want NotFound", status.Code(err))
	}
}
