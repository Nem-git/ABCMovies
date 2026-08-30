package apiserver_test

import (
	"context"
	"errors"
	"log/slog"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// ctxAs returns a context presenting uid as the authenticated principal.
func ctxAs(session auth.Session, uid string) context.Context {
	return apiserver.AuthContext(context.Background(), session, uid, "test-token")
}

// stubLibrary is a configurable LibrarySeam: it serves prebuilt entries,
// display metadata, and explicitly authorized reaches.
type stubLibrary struct {
	entries   []*corev1.LibraryEntry
	meta      map[string]*corev1.TitleMetadata
	reachable map[string][]string // account id -> users it is authorized for
	reaches   []library.Reach
	removed   []string
	err       error
}

func (l *stubLibrary) Library(context.Context, string) ([]*corev1.LibraryEntry, error) {
	return l.entries, l.err
}

func (l *stubLibrary) Metadata(_ context.Context, ref string) (*corev1.TitleMetadata, bool, error) {
	if l.meta == nil {
		return nil, false, nil
	}
	m, ok := l.meta[ref]
	return m, ok, nil
}

func (l *stubLibrary) ReachAuthorized(accountID, userID string) (library.Reach, bool) {
	for _, u := range l.reachable[accountID] {
		if u == userID {
			for _, r := range l.reaches {
				if r.AccountID == accountID {
					return r, true
				}
			}
		}
	}
	return library.Reach{}, false
}

func (l *stubLibrary) ReachesForUser(userID string) []library.Reach {
	var out []library.Reach
	for _, r := range l.reaches {
		for _, u := range l.reachable[r.AccountID] {
			if u == userID {
				out = append(out, r)
			}
		}
	}
	return out
}

func (l *stubLibrary) RemoveReach(accountID string) { l.removed = append(l.removed, accountID) }

// noopClient satisfies the source-cache client surface; the handler never
// syncs, so it is only a wrapper that makes a Synchronizer constructible.
type noopClient struct{}

func (noopClient) CatalogueSync(context.Context, *slotsv1.CatalogueSyncRequest) (*slotsv1.CatalogueSyncResponse, error) {
	return &slotsv1.CatalogueSyncResponse{}, nil
}

// operatorReach builds a reach as the wiring would for a host-provided,
// operator-declared account (§2.2): public, no owner.
func operatorReach(accountID string) library.Reach {
	sync, err := sourcecache.New("jellyfin", noopClient{}, store.NewInMemory(), slog.Default())
	if err != nil {
		panic(err)
	}
	return library.Reach{Sync: sync, AccountID: accountID, Visibility: accounts.VisibilityPublic}
}

// stubProber is a configurable CredentialProber.
type stubProber struct {
	accept  bool
	blob    []byte
	baseURL string
	user    string
	pass    []byte
	calls   int
}

func (p *stubProber) Probe(_ context.Context, baseURL, username string, password []byte) ([]byte, error) {
	p.calls++
	p.baseURL, p.user, p.pass = baseURL, username, password
	if !p.accept {
		return nil, errors.New("provider rejected")
	}
	return p.blob, nil
}

// videoTrack is a minimal video track descriptor for play-menu stubs.
func videoTrack(id string) *corev1.Track {
	return &corev1.Track{
		Id:    id,
		Media: &corev1.Track_Video{Video: &corev1.VideoTrack{Codec: "hevc", Width: 3840, Height: 2160}},
	}
}
