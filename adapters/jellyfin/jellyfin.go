// Package jellyfin implements the Jellyfin provider slot: a library-class
// adapter that speaks the whole-catalogue sync contract (PLAN.md §5.4) against
// a Jellyfin server's REST API. It maps Jellyfin's offset-paginated item index
// onto opaque continuation tokens, and its ProviderIds metadata onto
// provider-supplied external identity assertions (§5.3).
package jellyfin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// pageSize is how many catalogue items one upstream request fetches. It is an
// implementation detail behind the contract's opaque tokens.
const pageSize = 500

// Account is one operator-declared streaming-provider account (IMPLEMENTATION.md §3).
type Account struct {
	ID       string // the account_id callers pass to CatalogueSync
	URL      string // base URL of the Jellyfin server
	Username string // login name
	// PasswordEnv names the environment variable holding the password, and is
	// optional: an operator-declared account logs in through it, while a
	// linked account (PLAN.md §3.5) arrives with its validated session already
	// in the vault and never needs the password here. When it is empty the
	// session must come from the vault, or the account is treated as needing
	// a re-link.
	PasswordEnv string
}

// Option customizes the slot's HTTP behaviour (tests inject their server).
type Option func(*clientConfig)

type clientConfig struct {
	httpClient *http.Client
	deviceID   string
	vault      SessionVault
}

// WithHTTPClient overrides the HTTP client used for all upstream calls.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *clientConfig) { c.httpClient = hc }
}

// WithSessionVault persists each account's provider session token through the
// given sealed store, so a restart does not re-login against the provider
// (and does not need the password env var at all). The store is responsible
// for sealing; this package never handles key material.
func WithSessionVault(v SessionVault) Option {
	return func(c *clientConfig) { c.vault = v }
}

// SessionVault stores provider session credentials, keyed by account id. A
// missing credential returns ("", nil), never an error.
type SessionVault interface {
	Save(ctx context.Context, accountID string, blob []byte) error
	Load(ctx context.Context, accountID string) ([]byte, error)
}

// Slot is the Jellyfin provider slot. It implements the meta handshake and,
// when admitted, serves the whole-catalogue sync surface.
type Slot struct {
	corev1.UnimplementedMetaServiceServer
	slotsv1.UnimplementedProviderServiceServer

	mu       sync.Mutex
	accounts map[string]*session // by Account.ID
	opts     clientConfig
}

type session struct {
	account Account
	token   string
	userID  string
}

// NoSessionError marks an account that has no live session and no way to
// obtain one itself: its vault holds nothing usable and it is not backed by a
// password env (a linked account whose session died, PLAN.md §3.5/§7.5). The
// only recovery is a user re-link, so the failure is typed rather than folded
// into a generic login error — the re-auth flow keys off this.
type NoSessionError struct {
	AccountID string
	Username  string
	BaseURL   string
}

func (e NoSessionError) Error() string {
	return fmt.Sprintf("jellyfin: account %q (%s@%s) has no vaulted session and no password-env; re-link required",
		e.AccountID, e.Username, e.BaseURL)
}

// New builds a slot serving the given operator-declared accounts. Empty
// PasswordEnv is allowed: such an account is vault-first — its session must
// already be in the vault (a validated link), never re-logged-in from here.
func New(accounts []Account, opts ...Option) (*Slot, error) {
	if len(accounts) == 0 {
		return nil, fmt.Errorf("jellyfin: at least one account must be declared")
	}
	byID := make(map[string]*session, len(accounts))
	for _, a := range accounts {
		if a.ID == "" || a.URL == "" || a.Username == "" {
			return nil, fmt.Errorf("jellyfin: account %q: id, url, and username are required", a.ID)
		}
		if _, dup := byID[a.ID]; dup {
			return nil, fmt.Errorf("jellyfin: duplicate account id %q", a.ID)
		}
		byID[a.ID] = &session{account: a}
	}
	cfg := clientConfig{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		deviceID:   "abcmovies-core",
	}
	for _, o := range opts {
		o(&cfg)
	}
	return &Slot{accounts: byID, opts: cfg}, nil
}

// CapabilityQuery answers the meta-contract: the slot speaks the meta-contract
// and declares its whole-catalogue sync surface as capability "browse" v1 plus
// its produce-sources surface as capability "produce-sources" v1 (PLAN.md
// §3.2: nothing is assumed, everything is asked). The refresh cadence is
// declared here too (PLAN.md §5.4 scheduler rules): operators may override it
// per instance in config, but the provider states its own polite default.
func (s *Slot) CapabilityQuery(_ context.Context, _ *corev1.CapabilityQueryRequest) (*corev1.CapabilityQueryResponse, error) {
	return &corev1.CapabilityQueryResponse{
		Capabilities: []*corev1.Capability{
			{Name: "meta", Version: 1},
			{Name: "browse", Version: 1},
			{Name: "produce-sources", Version: 1},
		},
		Policy: map[string]string{
			"browse.sync-cadence": declaredCadence.String(),
		},
	}, nil
}

// declaredCadence is this adapter's handshake-declared default sync cadence;
// the value's home is TECHNICAL-DECISIONS.md (§1.14/§1.22).
var declaredCadence = 6 * time.Hour

// Authenticate performs the upfront login for every declared account. It
// exists so the composition root can fail fast on bad credentials instead of
// discovering them at the first sync.
func (s *Slot) Authenticate(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.accounts {
		if _, err := s.ensureSessionLocked(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// ensureSessionLocked returns a live session, restoring it from the vault
// when available. A vault-first account (no password env) whose vault holds
// nothing usable has no way to proceed: that is a NoSessionError, the signal
// that a user re-link is required. An account backed by a password env falls
// back to logging in (then vaulting the result). Callers must hold s.mu.
func (s *Slot) ensureSessionLocked(ctx context.Context, accountID string) (*session, error) {
	sess, ok := s.accounts[accountID]
	if !ok {
		return nil, fmt.Errorf("jellyfin: unknown account %q", accountID)
	}
	if sess.token == "" && s.opts.vault != nil {
		if blob, err := s.opts.vault.Load(ctx, accountID); err == nil && len(blob) > 0 {
			var restored authResult
			if json.Unmarshal(blob, &restored) == nil && restored.AccessToken != "" && restored.User.ID != "" {
				sess.token = restored.AccessToken
				sess.userID = restored.User.ID
			}
		}
	}
	if sess.token == "" {
		if sess.account.PasswordEnv == "" {
			return nil, NoSessionError{
				AccountID: sess.account.ID,
				Username:  sess.account.Username,
				BaseURL:   sess.account.URL,
			}
		}
		password := os.Getenv(sess.account.PasswordEnv)
		auth, err := s.authenticate(ctx, sess.account.URL, sess.account.Username, password)
		if err != nil {
			return nil, fmt.Errorf("jellyfin: login %q: %w", accountID, err)
		}
		sess.token = auth.AccessToken
		sess.userID = auth.User.ID
		if s.opts.vault != nil {
			if blob, mErr := json.Marshal(auth); mErr == nil {
				if vErr := s.opts.vault.Save(ctx, accountID, blob); vErr != nil {
					// A failed save must not fail the login: the session is
					// live; we merely fall back to re-login next start.
					fmt.Fprintf(os.Stderr, "jellyfin: vault save %q: %v\n", accountID, vErr)
				}
			}
		}
	}
	return sess, nil
}

// invalidate drops a stale token — including any vaulted copy, which is dead
// once the server rejects it — so the next call re-authenticates fresh.
func (s *Slot) invalidate(ctx context.Context, accountID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.accounts[accountID]; ok {
		sess.token = ""
		if s.opts.vault != nil {
			_ = s.opts.vault.Save(ctx, accountID, nil)
		}
	}
}

// authResult is the subset of Jellyfin's AuthenticationResult we consume.
type authResult struct {
	AccessToken string `json:"AccessToken"`
	User        struct {
		ID string `json:"Id"`
	} `json:"User"`
}

// authenticate POSTs /Users/AuthenticateByName. The Authorization header
// carries the MediaBrowser scheme; the access token arrives in the body, not
// the header (Jellyfin 10.9+ removed X-Emby-Token).
func (s *Slot) authenticate(ctx context.Context, baseURL, username, password string) (*authResult, error) {
	body := fmt.Sprintf(`{"Username":%q,"Pw":%q}`, username, password)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(baseURL, "/")+"/Users/AuthenticateByName",
		strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization",
		fmt.Sprintf(`MediaBrowser Client="ABCMovies", Device="abcmovies-core", DeviceId=%q, Version="0.0.1"`, s.opts.deviceID))

	resp, err := s.opts.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("credentials rejected")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var out authResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.AccessToken == "" || out.User.ID == "" {
		return nil, fmt.Errorf("server returned an incomplete authentication result")
	}
	return &out, nil
}

// mediaItem is the subset of Jellyfin's BaseItemDto we consume.
type mediaItem struct {
	Id             string            `json:"Id"`
	Type           string            `json:"Type"`
	Name           string            `json:"Name"`
	ProductionYear int               `json:"ProductionYear"`
	ProviderIds    map[string]string `json:"ProviderIds"`
}

// itemsPage is the subset of BaseItemDtoQueryResult we consume.
type itemsPage struct {
	Items            []mediaItem `json:"Items"`
	TotalRecordCount int         `json:"TotalRecordCount"`
	StartIndex       int         `json:"StartIndex"`
}

// getItems fetches one upstream page of the account's movie+series index.
// ProviderIds must be requested explicitly on Jellyfin 10.9+; without the
// fields parameter the item index omits them, silently stripping every
// identity assertion from the catalogue.
func (s *Slot) getItems(ctx context.Context, sess *session, offset int) (*itemsPage, error) {
	url := fmt.Sprintf("%s/Items?userId=%s&includeItemTypes=Movie,Series&recursive=true&sortBy=SortName&sortOrder=Ascending&startIndex=%d&limit=%d&fields=ProviderIds",
		strings.TrimRight(sess.account.URL, "/"), sess.userID, offset, pageSize)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization",
		fmt.Sprintf(`MediaBrowser Token=%q, Client="ABCMovies", Device="abcmovies-core", DeviceId=%q, Version="0.0.1"`,
			sess.token, s.opts.deviceID))

	resp, err := s.opts.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		// fall through to decode
	case http.StatusUnauthorized:
		return nil, errUnauthorized
	default:
		return nil, fmt.Errorf("jellyfin: /Items: unexpected status %d", resp.StatusCode)
	}
	var out itemsPage
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// errUnauthorized signals a stale token; CatalogueSync re-authenticates once
// on it rather than surfacing a spurious failure.
var errUnauthorized = fmt.Errorf("jellyfin: unauthorized")

// CatalogueSync serves one page of the whole-catalogue sync contract
// (PLAN.md §5.4). Continuation tokens are this adapter's private encoding of
// the upstream offset; iteration ends at the first empty next_page_token.
func (s *Slot) CatalogueSync(ctx context.Context, req *slotsv1.CatalogueSyncRequest) (*slotsv1.CatalogueSyncResponse, error) {
	offset, err := decodePageToken(req.GetPageToken())
	if err != nil {
		return nil, err
	}

	page, err := s.syncOnce(ctx, req.GetAccountId(), offset)
	if err == nil {
		return encodePage(offset, page), nil
	}
	if err != errUnauthorized {
		return nil, err
	}

	// One transparent retry after re-login: a token can expire mid-sync.
	s.invalidate(ctx, req.GetAccountId())
	page, err = s.syncOnce(ctx, req.GetAccountId(), offset)
	if err != nil {
		return nil, err
	}
	return encodePage(offset, page), nil
}

func (s *Slot) syncOnce(ctx context.Context, accountID string, offset int) (*itemsPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, err := s.ensureSessionLocked(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return s.getItems(ctx, sess, offset)
}

// decodePageToken parses this adapter's opaque tokens ("offset=N"; empty means
// first page). Anything unparseable is a caller bug and is rejected.
func decodePageToken(token string) (int, error) {
	if token == "" {
		return 0, nil
	}
	var offset int
	if _, err := fmt.Sscanf(token, "offset=%d", &offset); err != nil || offset < 0 {
		return 0, fmt.Errorf("catalogue sync: malformed page_token")
	}
	return offset, nil
}

// encodePage converts an upstream page into a contract response, mapping
// kinds, metadata (title and production year inside the embedded
// TitleMetadata), and ProviderIds (namespace lower-cased; empty assertions
// dropped), and minting the next token from the upstream offset arithmetic.
func encodePage(offset int, page *itemsPage) *slotsv1.CatalogueSyncResponse {
	out := &slotsv1.CatalogueSyncResponse{}
	for _, it := range page.Items {
		item := &slotsv1.CatalogueItem{
			NativeId: it.Id,
			Metadata: &corev1.TitleMetadata{Title: it.Name},
		}
		if it.ProductionYear > 0 {
			item.Metadata.Year = uint32(it.ProductionYear) //nolint:gosec // guarded above
		}
		switch it.Type {
		case "Movie":
			item.Kind = slotsv1.ItemKind_ITEM_KIND_MOVIE
		case "Series":
			item.Kind = slotsv1.ItemKind_ITEM_KIND_SERIES
		default:
			continue // defensive: we only ever ask for Movie,Series
		}
		for ns, val := range it.ProviderIds {
			if val == "" {
				continue
			}
			item.ExternalIds = append(item.ExternalIds, &slotsv1.ExternalId{
				Namespace: strings.ToLower(ns),
				Value:     val,
			})
		}
		out.Items = append(out.Items, item)
	}
	if end := offset + len(page.Items); end < page.TotalRecordCount && len(out.Items) > 0 {
		out.NextPageToken = fmt.Sprintf("offset=%d", end)
	}
	return out
}

// ProduceSources serves the produce-sources capability (PLAN.md §3.2, §6.2):
// it returns the manifest for one item, resolved through the account's live
// session. Jellyfin has no pre-built manifest; we call PlaybackInfo and map
// the first media source's streams onto a WHOLE_MUX manifest — the source is
// a muxed container fetched as a unit, with a video container track carrying
// its audio and subtitle streams (§6.2). Selection stays engine-side.
func (s *Slot) ProduceSources(ctx context.Context, req *slotsv1.ProduceSourcesRequest) (*slotsv1.ProduceSourcesResponse, error) {
	if req.GetAccountId() == "" || req.GetNativeId() == "" {
		return nil, fmt.Errorf("jellyfin: produce-sources requires account_id and native_id")
	}
	pb, err := s.playbackInfo(ctx, req.GetAccountId(), req.GetNativeId())
	if err == errUnauthorized {
		s.invalidate(ctx, req.GetAccountId())
		pb, err = s.playbackInfo(ctx, req.GetAccountId(), req.GetNativeId())
	}
	if err != nil {
		return nil, err
	}
	src, err := sourceFromPlayback(pb)
	if err != nil {
		return nil, err
	}
	return &slotsv1.ProduceSourcesResponse{Source: src}, nil
}

// playbackInfoResult is the subset of Jellyfin's PlaybackInfo response we
// consume to build a manifest: the first media source's container and streams,
// plus a direct stream URL for the source.
type playbackInfoResult struct {
	Container string
	StreamURL string
	Streams   []playbackStream
}

// playbackStream mirrors one item of a media source's MediaStreams array.
type playbackStream struct {
	Type          string `json:"Type"` // Video, Audio, Subtitle
	Codec         string `json:"Codec"`
	Width         int    `json:"Width"`
	Height        int    `json:"Height"`
	BitRate       int    `json:"BitRate"`
	Language      string `json:"Language"`
	Channels      int    `json:"Channels"`
	ChannelLayout string `json:"ChannelLayout"`
	IsForced      bool   `json:"IsForced"`
}

// playbackInfo calls Jellyfin's PlaybackInfo for one item and returns the
// first media source with a direct stream URL.
func (s *Slot) playbackInfo(ctx context.Context, accountID, itemID string) (*playbackInfoResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, err := s.ensureSessionLocked(ctx, accountID)
	if err != nil {
		return nil, err
	}
	body := fmt.Sprintf(`{"UserId":%q,"DeviceId":%q,"StartTimeTicks":0}`, sess.userID, s.opts.deviceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/Items/%s/PlaybackInfo", strings.TrimRight(sess.account.URL, "/"), itemID),
		strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization",
		fmt.Sprintf(`MediaBrowser Token=%q, Client="ABCMovies", Device="abcmovies-core", DeviceId=%q, Version="0.0.1"`,
			sess.token, s.opts.deviceID))

	resp, err := s.opts.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		// fall through to decode
	case http.StatusUnauthorized:
		return nil, errUnauthorized
	default:
		return nil, fmt.Errorf("jellyfin: /PlaybackInfo: unexpected status %d", resp.StatusCode)
	}

	var out struct {
		MediaSources []struct {
			Id        string           `json:"Id"`
			Container string           `json:"Container"`
			Streams   []playbackStream `json:"MediaStreams"`
		} `json:"MediaSources"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.MediaSources) == 0 {
		return nil, fmt.Errorf("jellyfin: playback info returned no media sources")
	}
	src := out.MediaSources[0]
	streamURL := fmt.Sprintf("%s/Videos/%s/stream?Static=true&MediaSourceId=%s&DeviceId=%s&api_key=%s",
		strings.TrimRight(sess.account.URL, "/"), itemID, src.Id, s.opts.deviceID, sess.token)
	return &playbackInfoResult{
		Container: src.Container,
		StreamURL: streamURL,
		Streams:   src.Streams,
	}, nil
}

// sourceFromPlayback maps a Jellyfin media source onto a WHOLE_MUX manifest:
// the muxed container is the fetch unit, carried by a video container track,
// with audio and subtitle tracks referencing it (PLAN.md §6.2).
func sourceFromPlayback(pb *playbackInfoResult) (*corev1.MediaSource, error) {
	if pb == nil {
		return nil, fmt.Errorf("jellyfin: nil playback info")
	}
	carrierID := "container"
	tracks := []*corev1.Track{
		{Id: carrierID, Delivery: &corev1.TrackDelivery{Locations: []string{pb.StreamURL}}},
	}
	for i, st := range pb.Streams {
		switch st.Type {
		case "Video":
			tracks[0].Media = &corev1.Track_Video{Video: &corev1.VideoTrack{
				Codec:  st.Codec,
				Width:  uint32(st.Width),
				Height: uint32(st.Height),
				Range:  corev1.VideoRange_VIDEO_RANGE_SDR,
			}}
			if st.BitRate > 0 {
				tracks[0].GetVideo().Bitrate = uint32(st.BitRate)
			}
		case "Audio":
			lang := st.Language
			if lang == "" {
				lang = "und"
			}
			tracks = append(tracks, &corev1.Track{
				Id:       fmt.Sprintf("audio-%d", i),
				Media:    &corev1.Track_Audio{Audio: &corev1.AudioTrack{Codec: st.Codec, Language: lang, ChannelLayout: st.ChannelLayout, Role: corev1.AudioRole_AUDIO_ROLE_MAIN}},
				Delivery: &corev1.TrackDelivery{CarriedIn: carrierID},
			})
		case "Subtitle":
			lang := st.Language
			if lang == "" {
				lang = "und"
			}
			tracks = append(tracks, &corev1.Track{
				Id:       fmt.Sprintf("subtitle-%d", i),
				Media:    &corev1.Track_Subtitle{Subtitle: &corev1.SubtitleTrack{Format: st.Codec, Language: lang, Role: corev1.SubtitleRole_SUBTITLE_ROLE_SUBTITLE, Forced: st.IsForced}},
				Delivery: &corev1.TrackDelivery{CarriedIn: carrierID},
			})
		default:
			// Unknown stream types are ignored, not failed: Jellyfin may
			// carry auxiliary streams (data, attachments) we do not model.
		}
	}
	return &corev1.MediaSource{
		Type:        corev1.MediaSourceType_MEDIA_SOURCE_TYPE_STATIC,
		Seekable:    corev1.Seekable_SEEKABLE_FULL,
		Addressable: corev1.Addressable_ADDRESSABLE_WHOLE_MUX,
		Tracks:      tracks,
	}, nil
}
