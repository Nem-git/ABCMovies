// Package m5_test holds the milestone acceptance tests for M5: the
// connected-account surface (link, list, remove) and the derived library,
// plus one play session run end-to-end (plane: StartDelivery → GetPlayInfo →
// relay pull). Unlike a unit probe, the harness lives behind the production
// seams that compose in release: instance config defaults (all in-memory), the
// slotwiring provider/sink wiring, the library service, the delivery engine,
// and the API service arming every surface it exposes. The provider is an
// HTTP-level Jellyfin fake; the mock boundary is the adapter's own REST
// contract, not a Go interface.
package m5_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/auth"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/itemregistry"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/metadatacache"
	"github.com/nem-git/abcmovies/core/internal/registry"
	"github.com/nem-git/abcmovies/core/internal/slotwiring"
	"github.com/nem-git/abcmovies/core/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// fakeJellyfin serves the adapter's REST contract at the HTTP boundary
// (PLAN.md §3.2): password auth, the movie+series index, per-item playback
// info, and a direct stream URL. It is the M5 mock — the Jellyfin adapter is
// exercised unmodified against it, exactly as it would reach a real server.
type fakeJellyfin struct {
	mu           sync.Mutex
	srv          *httptest.Server
	items        []jfCatalogItem
	streamBytes  map[string][]byte
	rejectedUser string // username whose auth attempt must 401
	streamHits   map[string]int
}

// jfCatalogItem is the subset of Jellyfin's BaseItemDto the fake index holds.
type jfCatalogItem struct {
	ID        string            `json:"Id"`
	Type      string            `json:"Type"`
	Name      string            `json:"Name"`
	Year      int               `json:"ProductionYear"`
	Providers map[string]string `json:"ProviderIds"`
}

// fakeJellyfinServer starts the fake and returns it. The catalog matches the
// seeded linked account: three titles across movie and series kinds.
func fakeJellyfinServer(t *testing.T) *fakeJellyfin {
	t.Helper()
	f := &fakeJellyfin{
		items: []jfCatalogItem{
			{
				ID: "movie-gondwana", Type: "Movie", Name: "The Last Gondwana Gardener", Year: 2021,
				Providers: map[string]string{"Imdb": "tt-gondwana", "Tmdb": "12"},
			},
			{
				ID: "movie-coral", Type: "Movie", Name: "Coral Skies", Year: 2019,
				Providers: map[string]string{"Tmdb": "23"},
			},
			{
				ID: "series-tidal", Type: "Series", Name: "Tidal Station", Year: 2022,
				Providers: map[string]string{"Tvdb": "99"},
			},
		},
		streamBytes: map[string][]byte{
			"movie-gondwana": []byte("P5-gondwana-mkv-bytes"),
			"movie-coral":    []byte("P5-coral-mkv-bytes"),
			"series-tidal":   []byte("P5-tidal-mkv-bytes"),
		},
		rejectedUser: "rejected-user",
		streamHits:   map[string]int{},
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.ServeHTTP))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeJellyfin) URL() string { return f.srv.URL }

func (f *fakeJellyfin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/Users/AuthenticateByName":
		f.authenticate(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/Items":
		f.index(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/Items/") && strings.HasSuffix(r.URL.Path, "/PlaybackInfo"):
		f.playbackInfo(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/Videos/") && strings.HasSuffix(r.URL.Path, "/stream"):
		f.stream(w, r)
	default:
		http.NotFound(w, r)
	}
}

// authenticate answers the adapter's POST /Users/AuthenticateByName. The
// device header is negotiated here; no bearer token exists yet (Jellyfin
// 10.9 returns the token in the body, not the header).
func (f *fakeJellyfin) authenticate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"Username"`
		Pw       string `json:"Pw"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Username == f.rejectedUser {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"Exception":{"Message":"Invalid username or password"}}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"AccessToken":"srv-access-token","User":{"Id":"jf-user-home"}}`))
}

// index answers the adapter's GET /Items. Every authenticated request must
// carry the MediaBrowser token header; the page respects startIndex/limit and
// the includeItemTypes filter.
func (f *fakeJellyfin) index(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Authorization"), "MediaBrowser Token=") {
		http.Error(w, "missing authorization", http.StatusUnauthorized)
		return
	}
	q := r.URL.Query()
	wantKinds := map[string]bool{}
	for _, k := range strings.Split(q.Get("includeItemTypes"), ",") {
		wantKinds[k] = true
	}
	all := make([]jfCatalogItem, 0, len(f.items))
	for _, it := range f.items {
		if len(wantKinds) == 0 || wantKinds[it.Type] {
			all = append(all, it)
		}
	}
	start, _ := strconv.Atoi(q.Get("startIndex"))
	if limit, err := strconv.Atoi(q.Get("limit")); err == nil && limit > 0 && start+limit < len(all) {
		all = all[start : start+limit]
	} else if start < len(all) {
		all = all[start:]
	} else {
		all = nil
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		Items            []jfCatalogItem `json:"Items"`
		TotalRecordCount int             `json:"TotalRecordCount"`
		StartIndex       int             `json:"StartIndex"`
	}{Items: all, TotalRecordCount: len(f.items), StartIndex: start})
}

// playbackInfo answers the adapter's POST /Items/{id}/PlaybackInfo with one
// muxed media source: hevc video, truehd audio, and an srt subtitle.
func (f *fakeJellyfin) playbackInfo(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.Header.Get("Authorization"), "MediaBrowser Token=") {
		http.Error(w, "missing authorization", http.StatusUnauthorized)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/Items/"), "/PlaybackInfo")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(struct {
		MediaSources []map[string]any `json:"MediaSources"`
	}{
		MediaSources: []map[string]any{{
			"Id":        "ms-" + id,
			"Container": "mkv",
			"MediaStreams": []map[string]any{
				{"Type": "Video", "Codec": "hevc", "Width": 1920, "Height": 1080, "Language": "eng"},
				{"Type": "Audio", "Codec": "truehd", "Channels": 8, "ChannelLayout": "7.1", "Language": "eng"},
				{"Type": "Subtitle", "Codec": "srt", "Language": "eng"},
			},
		}},
	})
}

// stream answers the adapter's GET /Videos/{id}/stream. The provider grants
// the direct stream through the api_key query parameter, not a header.
func (f *fakeJellyfin) stream(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("api_key") != "srv-access-token" {
		http.Error(w, "missing api key", http.StatusUnauthorized)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/Videos/"), "/stream")
	f.mu.Lock()
	f.streamHits[id]++
	body := append([]byte(nil), f.streamBytes[id]...)
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	_, _ = w.Write(body)
}

// m5Stack is the composed M5 harness. It is the same composition the release
// root wires — config defaults (in-memory stores), auth, the slot wiring, the
// library service, and the delivery engine — with the two user-facing seams
// (Jellyfin provider, frontend device) mocked at their network boundaries.
type m5Stack struct {
	stores  config.Stores
	bus     *apiserver.InMemoryBus
	server  *apiserver.Server
	session auth.Session
	relay   *delivery.Relay
	eng     *delivery.Engine
	// ns is the identity namespace the seeded linked account provisions as
	// (PLAN.md §1.25): the server identity, which delivery requests address.
	ns string
	// baseURL is the Jellyfin server the fake serves (trailing slash trimmed),
	// shared by the seeded link and every wire LinkAccount in the tests.
	baseURL    string
	alice, bob *auth.SignUpResult
	aliceToken string
	bobToken   string
}

// newM5Stack builds the composed harness. The kind blobs, the linked-account
// seeding, and the metadata alias table all precede slot wiring, mirroring a
// release boot where the vault already holds a user's validated link.
func newM5Stack(t *testing.T, jf *fakeJellyfin) *m5Stack {
	t.Helper()

	c := config.Default()
	stores, err := config.BuildStores(t.Context(), c, nil)
	if err != nil {
		t.Fatalf("BuildStores: %v", err)
	}
	stores.WatchHistory = store.NewUserBlobStore(stores.WatchHistory)

	users, tokens, deks, err := config.BuildAuth(stores.Users, stores.Sessions, c.Auth.DEKCache, nil)
	if err != nil {
		t.Fatalf("BuildAuth: %v", err)
	}
	composite, err := config.BuildAuthenticator([]string{"password"}, users)
	if err != nil {
		t.Fatalf("BuildAuthenticator: %v", err)
	}
	session := config.BuildSession(tokens, deks, time.Hour)

	bus := apiserver.NewInMemoryBus()
	t.Cleanup(bus.Close)
	srv := apiserver.NewServer(bus, stores, composite, session)

	alice := signUp(t, srv, "alice", "password123")
	bob := signUp(t, srv, "bob", "password123")

	// alice linked her home server to this instance before it booted; the
	// vault-first custody model means provisioning restores the session it
	// validated. The blob is the exact authResult JSON the Jellyfin adapter
	// re-uses for slot sessions (PLAN.md §3.5).
	accts := accounts.NewStore(stores.Vault, nil)
	rec := accounts.Record{
		ID:          "lnk_alice_home",
		Provider:    "jellyfin",
		BaseURL:     strings.TrimRight(jf.URL(), "/"),
		Username:    "alice.homeserver.user",
		OwnerUserID: alice.UserID,
		Status:      accounts.StatusLinked,
		Visibility:  accounts.VisibilityPrivate,
		CreatedAt:   time.Now().UTC(),
	}
	if err := accts.Add(t.Context(), rec); err != nil {
		t.Fatalf("seed linked account: %v", err)
	}
	blob, err := json.Marshal(map[string]any{
		"AccessToken": "srv-access-token",
		"User":        map[string]any{"Id": "jf-user-home"},
	})
	if err != nil {
		t.Fatalf("marshal vault blob: %v", err)
	}
	if err := accts.Save(t.Context(), rec.ID, blob); err != nil {
		t.Fatalf("seed vault blob: %v", err)
	}
	ns := slotwiring.ServerNamespace(rec)

	// The metadata cache carries the display records the release would fill by
	// enrichment (PLAN.md §5.2). Records are keyed by their canonical external
	// id; the provider's other identity assertions for the same film resolve
	// through aliases to that canonical key.
	meta, err := metadatacache.New(stores.MetadataCache, nil)
	if err != nil {
		t.Fatalf("metadatacache.New: %v", err)
	}
	records := []struct {
		ref string // canonical external id
		rec *corev1.TitleMetadata
	}{
		{"imdb:tt-gondwana", &corev1.TitleMetadata{
			Title:        "The Last Gondwana Gardener",
			Year:         2021,
			KindSpecific: &corev1.TitleMetadata_Movie{Movie: &corev1.MovieSpecific{}},
		}},
		{"tmdb:23", &corev1.TitleMetadata{
			Title:        "Coral Skies",
			Year:         2019,
			KindSpecific: &corev1.TitleMetadata_Movie{Movie: &corev1.MovieSpecific{}},
		}},
		{"tvdb:99", &corev1.TitleMetadata{
			Title:        "Tidal Station",
			Year:         2022,
			KindSpecific: &corev1.TitleMetadata_Series{Series: &corev1.SeriesSpecific{}},
		}},
	}
	for _, s := range records {
		if err := meta.PutRecord(t.Context(), s.ref, s.rec); err != nil {
			t.Fatalf("seed metadata %s: %v", s.ref, err)
		}
	}
	// movie-gondwana asserts both imdb and tmdb; both must resolve to one
	// record, so the tmdb assertion aliases the canonical imdb key.
	if err := meta.LinkAlias(t.Context(), "tmdb:12", "imdb:tt-gondwana"); err != nil {
		t.Fatalf("seed alias tmdb:12: %v", err)
	}

	// Instance wiring: the linked account provisions its own user-owned
	// server slot under the seeded namespace; that slot's sync fills the
	// source cache and item registry before any library read.
	reg := registry.NewInProcess()
	itemReg, err := itemregistry.New(stores.SourceCache, "")
	if err != nil {
		t.Fatalf("itemregistry.New: %v", err)
	}
	_, reaches, resolvers, err := slotwiring.SetupProviders(nil, slotwiring.Deps{
		Ctx:          t.Context(),
		Registry:     reg,
		Accounts:     accts,
		SourceCache:  stores.SourceCache,
		ItemRegistry: itemReg,
	})
	if err != nil {
		t.Fatalf("SetupProviders: %v", err)
	}

	librarySvc, err := library.NewService(reaches, itemReg, stores.SourceCache, nil,
		library.WithEnrichment(meta, nil))
	if err != nil {
		t.Fatalf("library.NewService: %v", err)
	}
	srv.SetLibrary(librarySvc)
	srv.SetProber("jellyfin", slotwiring.ProberForAdapter("jellyfin"))

	relay := delivery.NewRelay()
	sinks, err := slotwiring.SetupSinks([]config.SlotEntry{{
		ID: "device", Adapter: "device", Enabled: true,
	}}, relay)
	if err != nil {
		t.Fatalf("SetupSinks: %v", err)
	}
	eng := delivery.New(delivery.Options{
		SessionTTL:        time.Hour,
		ConcurrentStreams: 3,
		SourceResolver:    &namespaceResolver{bySlot: resolvers},
		SinkFactory:       sinks,
		// The API service's own persistDeliveryJob covers the store + event
		// in the handler; the engine's hook is a no-op here (M4 precedent).
		RecordJob: func(*corev1.Job) {},
		// MenuReady announces the staged play menu on the harness bus, the
		// subscriber notification a frontend reacts to (PLAN.md §6.2, §9.2);
		// it mirrors production, where armDelivery publishes on both the
		// slot runtime bus and the API bus.
		MenuReady: func(sess *delivery.Session) {
			bus.Publish(&corev1.EventEnvelope{
				Id:       fmt.Sprintf("evt-menu-%s", sess.ID),
				Type:     corev1.EventType_EVENT_TYPE_DELIVERY_PLAY_MENU_READY,
				Audience: corev1.EventAudience_EVENT_AUDIENCE_USER,
				UserId:   sess.Context.GetMemberUserId(),
				Payload: &corev1.EventEnvelope_PlayMenuReady{
					PlayMenuReady: &corev1.PlayMenuReadyEvent{JobId: sess.ID},
				},
				EmittedAt: timestamppb.Now(),
			})
		},
		Logger: slog.New(slog.DiscardHandler),
	})
	srv.SetDelivery(m5Delivery{eng: eng, relay: relay})

	return &m5Stack{
		stores:     stores,
		bus:        bus,
		server:     srv,
		session:    session,
		relay:      relay,
		eng:        eng,
		ns:         ns,
		baseURL:    strings.TrimRight(jf.URL(), "/"),
		alice:      alice,
		bob:        bob,
		aliceToken: login(t, srv, "alice", "password123"),
		bobToken:   login(t, srv, "bob", "password123"),
	}
}

// namespaceResolver routes produce-sources by the provider slot id, mirroring
// the release composition's compositeResolver (PLAN.md §6.2).
type namespaceResolver struct {
	bySlot slotwiring.Resolvers
}

func (r *namespaceResolver) ProduceSources(ctx context.Context, provider, accountID, nativeID string) (*corev1.MediaSource, error) {
	slot, ok := r.bySlot[provider]
	if !ok {
		return nil, fmt.Errorf("no resolver wired for provider %q", provider)
	}
	return slot.ProduceSources(ctx, provider, accountID, nativeID)
}

// m5Delivery is the play-menu seam the API service consumes: the engine's
// Start/Heartbeat plus PlayMenu over the staged menu and the relay tokens.
// It mirrors the release's managedDelivery (app delivers exactly this shape);
// the seam exists because the M5 surface predates the app package.
type m5Delivery struct {
	eng   *delivery.Engine
	relay *delivery.Relay
}

var _ apiserver.DeliveryManager = (*m5Delivery)(nil)

func (m m5Delivery) Start(ctx context.Context, req delivery.StartRequest) (*delivery.Session, error) {
	return m.eng.Start(ctx, req)
}

func (m m5Delivery) Heartbeat(id string) error { return m.eng.Heartbeat(id) }

func (m m5Delivery) PlayMenu(sessionID string) (*apiserver.PlayMenu, error) {
	sess, ok := m.eng.Get(sessionID)
	if !ok {
		return nil, apiserver.ErrPlayMenuNotFound
	}
	if sess.Goal != delivery.GoalPlay || sess.Sink == nil || len(sess.Menu) == 0 {
		return nil, apiserver.ErrPlayMenuNotFound
	}
	device, ok := sess.Sink.(*delivery.DeviceSink)
	if !ok {
		return nil, apiserver.ErrPlayMenuNotFound
	}
	menu := &apiserver.PlayMenu{
		SessionID:    sess.ID,
		MemberUserID: sess.Context.GetMemberUserId(),
		Container:    planContainer(sess.Plan),
	}
	for _, tr := range sess.Menu {
		token, ok := device.RelayToken(tr.GetId())
		if !ok {
			// A carried-in track has no location and thus no relay ticket; it
			// is read off the carrier track's delivery (WHOLE_MUX).
			continue
		}
		menu.Tracks = append(menu.Tracks, apiserver.PlayMenuTrack{TrackID: tr.GetId(), Track: tr, RelayToken: token})
	}
	return menu, nil
}

// planContainer extracts the deliverable's container from a play plan: the
// remux step that names it, or "" for passthrough (identical helper to the
// release composition's, where the play menu carries the source container).
func planContainer(p delivery.Plan) string {
	for i := range p.Steps {
		if p.Steps[i].Kind == delivery.StepRemux && p.Steps[i].Params.Remux != nil {
			return p.Steps[i].Params.Remux.Container
		}
	}
	return ""
}

// startWireServer serves the real CoreService behind the production auth
// interceptors on an in-memory connection.
func startWireServer(t *testing.T, stack *m5Stack) *grpc.ClientConn {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer(
		grpc.UnaryInterceptor(apiserver.AuthUnaryInterceptor(stack.session)),
		grpc.StreamInterceptor(apiserver.AuthStreamInterceptor(stack.session)),
	)
	apiv1.RegisterCoreServiceServer(gs, stack.server)
	go func() { _ = gs.Serve(lis) }()
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// authedCtx returns ctx carrying the bearer token, as a frontend would send it.
func authedCtx(ctx context.Context, token string) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
}

// signUp creates a user via the server and returns the response.
func signUp(t *testing.T, srv *apiserver.Server, username, password string) *auth.SignUpResult {
	t.Helper()
	resp, err := srv.SignUp(context.Background(), &apiv1.SignUpRequest{
		Username: username,
		AuthMethod: &apiv1.SignUpRequest_Password{
			Password: &apiv1.PasswordSignUp{Password: []byte(password)},
		},
	})
	if err != nil {
		t.Fatalf("SignUp(%s): %v", username, err)
	}
	return &auth.SignUpResult{UserID: resp.GetUserId(), RecoveryKey: resp.GetRecoveryKey()}
}

// login authenticates a user and returns the session token.
func login(t *testing.T, srv *apiserver.Server, username, password string) string {
	t.Helper()
	resp, err := srv.Login(context.Background(), &apiv1.LoginRequest{
		Username: username,
		AuthMethod: &apiv1.LoginRequest_Password{
			Password: &apiv1.PasswordLogin{Password: []byte(password)},
		},
	})
	if err != nil {
		t.Fatalf("Login(%s): %v", username, err)
	}
	return resp.GetToken()
}
