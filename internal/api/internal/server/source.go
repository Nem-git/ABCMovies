// Source routes: the session-protected proxy/delegation layer for the gRPC
// source service (see decisions/tech/0010). Credentials in request bodies are
// input-only: they are forwarded to the core and never echoed, logged, or
// stored by the API. Responses are always Cache-Control: no-store (via
// writeJSON).
package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	identityv1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/identity/v1"
	sourcev1 "github.com/Nem-git/ABCMovies/gen/go/abcmovies/source/v1"
)

const maxSourceBody = int64(4 << 10)

type sourceCredentials struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
}

type connectRequest struct {
	ConnectorID string             `json:"connector_id"`
	Endpoint    string             `json:"endpoint"`
	DisplayName string             `json:"display_name,omitempty"`
	Credentials *sourceCredentials `json:"credentials,omitempty"`
}

type updateRequest struct {
	Endpoint    string             `json:"endpoint,omitempty"`
	DisplayName string             `json:"display_name,omitempty"`
	Credentials *sourceCredentials `json:"credentials,omitempty"`
}

type sourceResponse struct {
	SourceID           string `json:"source_id"`
	ConnectorID        string `json:"connector_id"`
	VerifiedExternalID string `json:"verified_external_id"`
	Endpoint           string `json:"endpoint"`
	DisplayName        string `json:"display_name"`
	Status             string `json:"status"`
	StatusDetail       string `json:"status_detail,omitempty"`
}

type connectResponse struct {
	Outcome string         `json:"outcome"` // "connected" | "duplicate"
	Source  sourceResponse `json:"source"`
	Detail  string         `json:"detail,omitempty"`
}

type removeResponse struct {
	Outcome string `json:"outcome"` // "removed"
}

type listResponse struct {
	Sources []sourceResponse `json:"sources"`
}

// registerSourceRoutes wires the source routes onto mux.
func (s *Server) registerSourceRoutes() {
	s.mux.HandleFunc("POST /api/source/connect", s.apiSourceConnect)
	s.mux.HandleFunc("GET /api/source", s.apiSourceList)
	s.mux.HandleFunc("GET /api/source/{source_id}", s.apiSourceGet)
	s.mux.HandleFunc("POST /api/source/{source_id}/update", s.apiSourceUpdate)
	s.mux.HandleFunc("POST /api/source/{source_id}/remove", s.apiSourceRemove)
}

// requireSession resolves the session cookie to a valid actor user_id. If the
// session is missing/invalid it writes the 401 response and returns ok=false.
func (s *Server) requireSession(w http.ResponseWriter, r *http.Request) (actorID string, ok bool) {
	c, err := r.Cookie(SessionCookieName)
	if err != nil || c.Value == "" {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return "", false
	}
	sess, err := s.identityClient.ValidateSession(r.Context(), &identityv1.ValidateSessionRequest{SessionId: c.Value})
	if err != nil {
		slog.Error("core validate session failed", "err", err)
		writeError(w, http.StatusServiceUnavailable, "service unavailable")
		return "", false
	}
	if !sess.GetValid() {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return "", false
	}
	return sess.GetUserId(), true
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, maxBody int64, out any) bool {
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		writeError(w, http.StatusUnsupportedMediaType, "expected application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return false
	}
	return true
}

func buildConnectRequest(actor, connectorID, endpoint, displayName string, c *sourceCredentials) (*sourcev1.ConnectSourceRequest, error) {
	cr := &sourcev1.ConnectSourceRequest{
		ActorUserId: actor,
		ConnectorId: connectorID,
		Endpoint:    endpoint,
		DisplayName: displayName,
	}
	if err := applyConnectCredentials(cr, c); err != nil {
		return nil, err
	}
	return cr, nil
}

func buildUpdateRequest(actor, sourceID, endpoint, displayName string, c *sourceCredentials) (*sourcev1.UpdateSourceRequest, error) {
	ur := &sourcev1.UpdateSourceRequest{
		ActorUserId: actor,
		SourceId:    sourceID,
		Endpoint:    endpoint,
		DisplayName: displayName,
	}
	if err := applyUpdateCredentials(ur, c); err != nil {
		return nil, err
	}
	return ur, nil
}

func applyConnectCredentials(cr *sourcev1.ConnectSourceRequest, c *sourceCredentials) error {
	if c == nil {
		return errors.New("credentials are required")
	}
	hasPassword := strings.TrimSpace(c.Password) != ""
	hasAPIKey := strings.TrimSpace(c.APIKey) != ""
	switch {
	case hasPassword && hasAPIKey:
		return errors.New("provide either username/password or an API key, not both")
	case hasPassword:
		cr.Credentials = &sourcev1.ConnectSourceRequest_Password{Password: &sourcev1.PasswordCredentials{
			Username: strings.TrimSpace(c.Username), Password: c.Password,
		}}
		return nil
	case hasAPIKey:
		cr.Credentials = &sourcev1.ConnectSourceRequest_ApiKey{ApiKey: strings.TrimSpace(c.APIKey)}
		return nil
	default:
		return errors.New("credentials are required")
	}
}

func applyUpdateCredentials(cr *sourcev1.UpdateSourceRequest, c *sourceCredentials) error {
	if c == nil {
		return nil // credentials optional; core enforces when the endpoint changes
	}
	hasPassword := strings.TrimSpace(c.Password) != ""
	hasAPIKey := strings.TrimSpace(c.APIKey) != ""
	switch {
	case hasPassword && hasAPIKey:
		return errors.New("provide either username/password or an API key, not both")
	case hasPassword:
		cr.Credentials = &sourcev1.UpdateSourceRequest_Password{Password: &sourcev1.PasswordCredentials{
			Username: strings.TrimSpace(c.Username), Password: c.Password,
		}}
		return nil
	case hasAPIKey:
		cr.Credentials = &sourcev1.UpdateSourceRequest_ApiKey{ApiKey: strings.TrimSpace(c.APIKey)}
		return nil
	default:
		return nil
	}
}

func sourceOutcomeAndDetail(out sourcev1.ConnectSourceOutcome) (outcome, detail string) {
	switch out {
	case sourcev1.ConnectSourceOutcome_CONNECT_SOURCE_OUTCOME_CONNECTED:
		return "connected", ""
	case sourcev1.ConnectSourceOutcome_CONNECT_SOURCE_OUTCOME_DUPLICATE:
		return "duplicate", "already connected"
	default:
		return "unknown", ""
	}
}

func sourceStatusString(s sourcev1.SourceStatus) string {
	switch s {
	case sourcev1.SourceStatus_SOURCE_STATUS_ACTIVE:
		return "active"
	case sourcev1.SourceStatus_SOURCE_STATUS_NEEDS_REVERIFICATION:
		return "needs_reverification"
	case sourcev1.SourceStatus_SOURCE_STATUS_ERROR:
		return "error"
	case sourcev1.SourceStatus_SOURCE_STATUS_DISCONNECTED:
		return "disconnected"
	default:
		return "unknown"
	}
}

func sourceToJSON(s *sourcev1.Source) sourceResponse {
	if s == nil {
		return sourceResponse{}
	}
	return sourceResponse{
		SourceID:           s.GetSourceId(),
		ConnectorID:        s.GetConnectorId(),
		VerifiedExternalID: s.GetVerifiedExternalId(),
		Endpoint:           s.GetEndpoint(),
		DisplayName:        s.GetDisplayName(),
		Status:             sourceStatusString(s.GetStatus()),
		StatusDetail:       s.GetStatusDetail(),
	}
}

// mapSourceError maps core gRPC errors to safe HTTP responses. The messages
// passed through (InvalidArgument, FailedPrecondition) are authored by core to
// contain no secrets; everything else is replaced with a generic actionable
// message and logged.
func (s *Server) mapSourceError(w http.ResponseWriter, err error, logMsg string) {
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
		writeError(w, http.StatusServiceUnavailable, "source operation failed")
	}
}

func (s *Server) apiSourceConnect(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	var req connectRequest
	if !decodeJSONBody(w, r, maxSourceBody, &req) {
		return
	}
	connectorID := strings.TrimSpace(req.ConnectorID)
	endpointRaw := strings.TrimSpace(req.Endpoint)
	if connectorID == "" || endpointRaw == "" {
		writeError(w, http.StatusBadRequest, "connector_id and endpoint are required")
		return
	}
	cr, err := buildConnectRequest(actor, connectorID, endpointRaw, strings.TrimSpace(req.DisplayName), req.Credentials)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.sourceClient.ConnectSource(r.Context(), cr)
	if err != nil {
		s.mapSourceError(w, err, "core connect source failed")
		return
	}
	outcome, detail := sourceOutcomeAndDetail(resp.GetOutcome())
	statusCode := http.StatusCreated
	if outcome != "connected" {
		statusCode = http.StatusOK
	}
	writeJSON(w, statusCode, connectResponse{Outcome: outcome, Source: sourceToJSON(resp.GetSource()), Detail: detail})
}

func (s *Server) apiSourceList(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	resp, err := s.sourceClient.ListSources(r.Context(), &sourcev1.ListSourcesRequest{ActorUserId: actor})
	if err != nil {
		s.mapSourceError(w, err, "core list sources failed")
		return
	}
	out := listResponse{Sources: []sourceResponse{}}
	for _, src := range resp.GetSources() {
		out.Sources = append(out.Sources, sourceToJSON(src))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) apiSourceGet(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("source_id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "source_id is required")
		return
	}
	resp, err := s.sourceClient.GetSource(r.Context(), &sourcev1.GetSourceRequest{ActorUserId: actor, SourceId: id})
	if err != nil {
		s.mapSourceError(w, err, "core get source failed")
		return
	}
	writeJSON(w, http.StatusOK, sourceToJSON(resp.GetSource()))
}

// apiSourceRemove disconnects a source. It takes no credentials (a local
// action) and is idempotent from the caller's point of view: an unknown
// source reports 404.
func (s *Server) apiSourceRemove(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("source_id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "source_id is required")
		return
	}
	_, err := s.sourceClient.RemoveSource(r.Context(), &sourcev1.RemoveSourceRequest{ActorUserId: actor, SourceId: id})
	if err != nil {
		s.mapSourceError(w, err, "core remove source failed")
		return
	}
	writeJSON(w, http.StatusOK, removeResponse{Outcome: "removed"})
}

func (s *Server) apiSourceUpdate(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.requireSession(w, r)
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("source_id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "source_id is required")
		return
	}
	var req updateRequest
	if !decodeJSONBody(w, r, maxSourceBody, &req) {
		return
	}
	ur, err := buildUpdateRequest(actor, id, strings.TrimSpace(req.Endpoint), strings.TrimSpace(req.DisplayName), req.Credentials)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	resp, err := s.sourceClient.UpdateSource(r.Context(), ur)
	if err != nil {
		s.mapSourceError(w, err, "core update source failed")
		return
	}
	writeJSON(w, http.StatusOK, sourceToJSON(resp.GetSource()))
}
