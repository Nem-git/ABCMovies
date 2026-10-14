package apiserver

import (
	"context"
	"errors"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/schema"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PlayMenuTrack is one staged play-menu track recovered through GetPlayInfo
// (PLAN.md §6.2): its media descriptor plus the relay token that serves its
// bytes under /media/relay/. The token is minted by the relay at delivery
// time; the frontend prepends its own origin to the stamped URL.
type PlayMenuTrack struct {
	TrackID    string
	Track      *corev1.Track
	RelayToken string
}

// PlayMenu is a delivery session's staged play menu (PLAN.md §6.2).
type PlayMenu struct {
	SessionID    string
	MemberUserID string
	// Container is the source's container name (e.g. "mkv", "mp4"), when the
	// plan resolved it; empty otherwise.
	Container string
	// Format is the session's packaging format (PLAN.md §6.1): empty for a raw
	// per-track menu; a manifest format when the Package step ran. It decides
	// which GetPlayInfo menu branch is set.
	Format delivery.PackagingFormat
	// ManifestToken is the relay token serving the session's manifest, set only
	// when Format is non-empty. Empty for a raw per-track menu.
	ManifestToken string
	Tracks        []PlayMenuTrack
}

// ErrPlayMenuNotFound marks a delivery-manager PlayMenu miss: the session has
// no staged menu (or the session id is unknown). GetPlayInfo maps it to
// NotFound (PLAN.md §6.2 recovery semantics: a subscriber that missed the
// ready event queries and gets a clean miss).
var ErrPlayMenuNotFound = errors.New("play menu not found")

// relayPrefix is the stable relay mount the play menu stamps onto its tracks;
// it is the delivery package's exported constant so the URL shape has exactly
// one definition (the serving layer mounts the handler under it).
const relayPrefix = delivery.RelayPrefix

// GetPlayInfo recovers a delivery session's staged play menu (PLAN.md §6.2).
// The ready event is the at-most-once notification; a subscriber that missed
// it queries here. The menu carries the relay URLs, which the engine stamps
// with the session's relay tokens — the caller never touches the provider.
func (s *Server) GetPlayInfo(ctx context.Context, req *apiv1.GetPlayInfoRequest) (*apiv1.GetPlayInfoResponse, error) {
	if err := schema.ValidateGetPlayInfoRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if s.delivery == nil {
		return nil, status.Error(codes.Unavailable, "delivery engine not configured")
	}
	menu, err := s.delivery.PlayMenu(req.GetSessionId())
	if err != nil {
		if errors.Is(err, ErrPlayMenuNotFound) {
			return nil, status.Error(codes.NotFound, "play menu not found")
		}
		return nil, status.Error(codes.Code(delivery.Code(err)), err.Error())
	}
	uid, _ := UserIDFromContext(ctx)
	// Another member's menu answers exactly as a missing one, so play-menu
	// ids cannot be probed (PLAN.md §8.1, §2.2).
	if menu.MemberUserID != "" && menu.MemberUserID != uid {
		return nil, status.Error(codes.NotFound, "play menu not found")
	}
	if menu.Format != "" {
		// Packaged: the menu is a manifest the Package step granted. The
		// manifest references the same per-track relay tokens the raw branch
		// would list; the player opens it and pulls the renditions it selects.
		return &apiv1.GetPlayInfoResponse{
			Menu:      &apiv1.GetPlayInfoResponse_ManifestUrl{ManifestUrl: relayPrefix + menu.ManifestToken},
			Container: menu.Container,
		}, nil
	}
	tracks := make([]*apiv1.PlayTrack, 0, len(menu.Tracks))
	for _, t := range menu.Tracks {
		var media *apiv1.PlayTrack
		switch m := t.Track.GetMedia().(type) {
		case *corev1.Track_Video:
			media = &apiv1.PlayTrack{Media: &apiv1.PlayTrack_Video{Video: m.Video}}
		case *corev1.Track_Audio:
			media = &apiv1.PlayTrack{Media: &apiv1.PlayTrack_Audio{Audio: m.Audio}}
		case *corev1.Track_Subtitle:
			media = &apiv1.PlayTrack{Media: &apiv1.PlayTrack_Subtitle{Subtitle: m.Subtitle}}
		default:
			// A track whose media is not one of the closed set is dropped:
			// the contract binds exactly one media to a track.
			continue
		}
		media.TrackId = t.TrackID
		media.RelayUrl = relayPrefix + t.RelayToken
		tracks = append(tracks, media)
	}
	return &apiv1.GetPlayInfoResponse{Menu: &apiv1.GetPlayInfoResponse_RawTracks{RawTracks: &apiv1.PlayMenuList{Tracks: tracks}}, Container: menu.Container}, nil
}
