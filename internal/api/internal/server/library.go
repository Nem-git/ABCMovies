// Library routes: the session-protected proxy/delegation layer for the gRPC
// library service (see decisions/tech/0010 and tech/0011). Credentials in
// request bodies are input-only: they are forwarded to the core and never
// echoed, logged, or stored by the API. Responses are always
// Cache-Control: no-store (via writeJSON).
//
// Browser forms (the no-JS path) are handled in the web layer, which
// translates them to these JSON endpoints; the API itself is JSON-only.
package server

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	libraryv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/library/v1"
)

const maxLibraryBody = int64(4 << 10)

type libraryCredentials struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
}

type syncRequest struct {
	Credentials *libraryCredentials `json:"credentials,omitempty"`
}

type titleResponse struct {
	TitleID            string `json:"title_id"`
	SourceID           string `json:"source_id"`
	SourceDisplayName  string `json:"source_display_name"`
	ExternalID         string `json:"external_id"`
	Name               string `json:"name"`
	Kind               string `json:"kind"`
	Year               int    `json:"year"`
	Overview           string `json:"overview,omitempty"`
	Availability       string `json:"availability"`
	AvailabilityDetail string `json:"availability_detail,omitempty"`
}

type syncStateResponse struct {
	SourceID          string `json:"source_id"`
	SourceDisplayName string `json:"source_display_name"`
	LastOutcome       string `json:"last_outcome"`
	LastSyncedAt      string `json:"last_synced_at,omitempty"` // RFC3339 UTC; empty if never synced
	Detail            string `json:"detail,omitempty"`
	TitleCount        int    `json:"title_count"`
	InFlight          bool   `json:"in_flight,omitempty"`
}

type listTitlesResponse struct {
	Titles     []titleResponse     `json:"titles"`
	SyncStates []syncStateResponse `json:"sync_states"`
	TotalCount int32               `json:"total_count"`
}

type syncResponse struct {
	Outcome  string            `json:"outcome"` // "success" | "partial" | "failed"
	SourceID string            `json:"source_id"`
	State    syncStateResponse `json:"state"`
}

// registerLibraryRoutes wires the library routes onto mux.
func (s *Server) registerLibraryRoutes() {
	s.mux.HandleFunc("POST /api/library/source/{source_id}/sync", s.apiLibrarySync)
	s.mux.HandleFunc("GET /api/library", s.apiLibraryList)
	s.mux.HandleFunc("GET /api/library/search", s.apiLibrarySearch)
	s.mux.HandleFunc("GET /api/library/title/{title_id}", s.apiLibraryGetTitle)
}

func buildSyncRequest(actor, sourceID string, c *libraryCredentials) (*libraryv1.SyncSourceRequest, error) {
	req := &libraryv1.SyncSourceRequest{ActorUserId: actor, SourceId: sourceID}
	if c == nil {
		return req, nil // omitted entirely: core reuses the session cache
	}
	userSet := strings.TrimSpace(c.Username) != ""
	passSet := strings.TrimSpace(c.Password) != ""
	keySet := strings.TrimSpace(c.APIKey) != ""
	switch {
	case keySet && (userSet || passSet):
		return nil, errors.New("provide either username/password or an API key, not both")
	case keySet:
		return &libraryv1.SyncSourceRequest{
			ActorUserId: actor,
			SourceId:    sourceID,
			Credentials: &libraryv1.SyncSourceRequest_ApiKey{ApiKey: strings.TrimSpace(c.APIKey)},
		}, nil
	case userSet != passSet:
		// A half-filled password pair is an error, not a silent fall-back to
		// the session cache (the play endpoint already rejects this).
		return nil, errors.New("username and password are required together")
	case userSet && passSet:
		return &libraryv1.SyncSourceRequest{
			ActorUserId: actor,
			SourceId:    sourceID,
			Credentials: &libraryv1.SyncSourceRequest_Password{Password: &libraryv1.PasswordCredentials{
				Username: strings.TrimSpace(c.Username), Password: c.Password,
			}},
		}, nil
	default:
		// All-blank credentials object: use this session's cached credentials.
		return req, nil
	}
}

func syncOutcomeString(o libraryv1.SyncOutcome) string {
	switch o {
	case libraryv1.SyncOutcome_SYNC_OUTCOME_SUCCESS:
		return "success"
	case libraryv1.SyncOutcome_SYNC_OUTCOME_PARTIAL:
		return "partial"
	case libraryv1.SyncOutcome_SYNC_OUTCOME_FAILED:
		return "failed"
	case libraryv1.SyncOutcome_SYNC_OUTCOME_INTERRUPTED:
		return "interrupted"
	default:
		return "unknown"
	}
}

func titleKindString(k libraryv1.TitleKind) string {
	switch k {
	case libraryv1.TitleKind_TITLE_KIND_MOVIE:
		return "movie"
	case libraryv1.TitleKind_TITLE_KIND_SERIES:
		return "series"
	default:
		return "other"
	}
}

func availabilityString(a libraryv1.AvailabilityState) string {
	switch a {
	case libraryv1.AvailabilityState_AVAILABILITY_STATE_AVAILABLE:
		return "available"
	case libraryv1.AvailabilityState_AVAILABILITY_STATE_UNAVAILABLE:
		return "unavailable"
	default:
		return "unknown"
	}
}

func titleToJSON(t *libraryv1.Title) titleResponse {
	if t == nil {
		return titleResponse{Kind: "other", Availability: "unknown"}
	}
	return titleResponse{
		TitleID:            t.GetTitleId(),
		SourceID:           t.GetSourceId(),
		SourceDisplayName:  t.GetSourceDisplayName(),
		ExternalID:         t.GetExternalId(),
		Name:               t.GetName(),
		Kind:               titleKindString(t.GetKind()),
		Year:               int(t.GetYear()),
		Overview:           t.GetOverview(),
		Availability:       availabilityString(t.GetAvailability()),
		AvailabilityDetail: t.GetAvailabilityDetail(),
	}
}

func titleToJSONList(ts []*libraryv1.Title) []titleResponse {
	out := make([]titleResponse, 0, len(ts))
	for _, t := range ts {
		out = append(out, titleToJSON(t))
	}
	return out
}

func syncStateToJSON(s *libraryv1.SourceSyncState) syncStateResponse {
	if s == nil {
		return syncStateResponse{LastOutcome: "unknown"}
	}
	out := syncStateResponse{
		SourceID:          s.GetSourceId(),
		SourceDisplayName: s.GetSourceDisplayName(),
		LastOutcome:       syncOutcomeString(s.GetLastOutcome()),
		Detail:            s.GetDetail(),
		TitleCount:        int(s.GetTitleCount()),
		InFlight:          s.GetInFlight(),
	}
	if s.GetLastSyncedAt() != nil && s.GetLastSyncedAt().IsValid() {
		out.LastSyncedAt = s.GetLastSyncedAt().AsTime().UTC().Format(time.RFC3339Nano)
	}
	return out
}

func syncStatesToJSON(ss []*libraryv1.SourceSyncState) []syncStateResponse {
	out := make([]syncStateResponse, 0, len(ss))
	for _, s := range ss {
		out = append(out, syncStateToJSON(s))
	}
	return out
}

// mapLibraryError maps core gRPC errors to safe HTTP responses. Safe
// core-authored messages (InvalidArgument, FailedPrecondition) are passed
// through; everything else is replaced with a generic actionable message and
// logged.
func (s *Server) mapLibraryError(w http.ResponseWriter, err error, logMsg string) {
	code := status.Code(err)
	msg := status.Convert(err).Message()
	switch code {
	case codes.InvalidArgument:
		writeError(w, http.StatusBadRequest, msg)
	case codes.Unauthenticated:
		writeError(w, http.StatusUnauthorized, "invalid or expired credentials")
	case codes.NotFound:
		writeError(w, http.StatusNotFound, "not found")
	case codes.FailedPrecondition:
		writeError(w, http.StatusUnprocessableEntity, msg)
	case codes.Unavailable:
		writeError(w, http.StatusServiceUnavailable, "could not reach the server")
	default:
		slog.Error(logMsg, "err", err)
		writeError(w, http.StatusServiceUnavailable, "library operation failed")
	}
}

func clampLibraryLimit(limit int32) int32 {
	switch {
	case limit <= 0:
		return 50
	case limit > 200:
		return 200
	default:
		return limit
	}
}

func clampLibraryOffset(offset int32) int32 {
	if offset < 0 {
		return 0
	}
	return offset
}

func parseLimitOffset(r *http.Request) (int32, int32) {
	q := r.URL.Query()
	limit := int32(50)
	offset := int32(0)
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = clampLibraryLimit(int32(n))
		}
	}
	if v := strings.TrimSpace(q.Get("offset")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = int32(n)
		}
	}
	return limit, offset
}

func (s *Server) apiLibrarySync(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	sourceID := strings.TrimSpace(r.PathValue("source_id"))
	if sourceID == "" {
		writeError(w, http.StatusBadRequest, "source_id is required")
		return
	}
	var req syncRequest
	if !decodeJSONBody(w, r, maxLibraryBody, &req) {
		return
	}
	syncReq, err := buildSyncRequest(actor, sourceID, req.Credentials)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.libraryClient.SyncSource(r.Context(), syncReq)
	if err != nil {
		s.mapLibraryError(w, err, "core sync library failed")
		return
	}
	outcome := syncOutcomeString(resp.GetOutcome())
	statusCode := http.StatusOK
	writeJSON(w, statusCode, syncResponse{Outcome: outcome, SourceID: sourceID, State: syncStateToJSON(resp.GetState())})
}

func (s *Server) apiLibraryList(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	limit, offset := parseLimitOffset(r)
	resp, err := s.libraryClient.ListTitles(r.Context(), &libraryv1.ListTitlesRequest{ActorUserId: actor, Limit: limit, Offset: offset})
	if err != nil {
		s.mapLibraryError(w, err, "core list titles failed")
		return
	}
	out := listTitlesResponse{Titles: []titleResponse{}, SyncStates: []syncStateResponse{}, TotalCount: resp.GetTotalCount()}
	for _, t := range resp.GetTitles() {
		out.Titles = append(out.Titles, titleToJSON(t))
	}
	for _, s := range resp.GetSyncStates() {
		out.SyncStates = append(out.SyncStates, syncStateToJSON(s))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiLibrarySearch(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	limit, offset := parseLimitOffset(r)
	resp, err := s.libraryClient.SearchTitles(r.Context(), &libraryv1.SearchTitlesRequest{ActorUserId: actor, Query: q, Limit: limit, Offset: offset})
	if err != nil {
		s.mapLibraryError(w, err, "core search titles failed")
		return
	}
	out := listTitlesResponse{Titles: []titleResponse{}, SyncStates: []syncStateResponse{}, TotalCount: resp.GetTotalCount()}
	for _, t := range resp.GetTitles() {
		out.Titles = append(out.Titles, titleToJSON(t))
	}
	for _, s := range resp.GetSyncStates() {
		out.SyncStates = append(out.SyncStates, syncStateToJSON(s))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiLibraryGetTitle(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("title_id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "title_id is required")
		return
	}
	resp, err := s.libraryClient.GetTitle(r.Context(), &libraryv1.GetTitleRequest{ActorUserId: actor, TitleId: id})
	if err != nil {
		s.mapLibraryError(w, err, "core get title failed")
		return
	}
	writeJSON(w, http.StatusOK, titleToJSON(resp.GetTitle()))
}
