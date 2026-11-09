// Package jellyfin verifies and describes a Jellyfin server without storing
// its credentials. It is used by core to implement connect/verify/update for
// REQ-SRC-01 (DEC-001).
//
// Credentials travel only in requests (Authorization header) and are never
// persisted, returned, or logged. Errors returned by this package are safe to
// show to users: they do not contain credentials, access tokens, or internal
// details beyond "what is wrong" and how to fix it.
package jellyfin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to the Jellyfin HTTP API.
type Client struct {
	HTTPClient    *http.Client
	Timeout       time.Duration
	ClientVersion string
	ClientName    string
}

// Credentials carries the access material for a Jellyfin server. It is used
// only for verification requests; callers must not persist it.
type Credentials struct {
	// Password authenticates username/password (a user access token is
	// obtained as part of the verification request and never returned).
	Password *PasswordCredentials
	// APIKey authenticates with an API key.
	APIKey string
}

// PasswordCredentials carries a Jellyfin username and password.
type PasswordCredentials struct {
	Username string
	Password string
}

// ServerInfo is a non-sensitive description of a verified Jellyfin server.
type ServerInfo struct {
	Endpoint    string // normalized base URL
	VerifiedID  string // server identity (`Id`), the stable upstream reference (DEC-001)
	DisplayName string // server name (`ServerName`)
	Version     string // server version (`Version`)
}

// Error is an operational (safe) error. It never carries credentials.
type Error struct {
	Kind       ErrorKind
	StatusCode int
	Message    string
	Retryable  bool
}

func (e *Error) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("%s (HTTP %d)", e.Message, e.StatusCode)
	}
	return e.Message
}

// ErrorKind classifies the failure so callers can map it to a user-facing
// message or a gRPC status code.
type ErrorKind string

const (
	ErrKindInvalidEndpoint ErrorKind = "invalid_endpoint" // malformed input, not validated yet
	ErrKindUnreachable     ErrorKind = "unreachable"      // network/timeout failure
	ErrKindUnauthorized    ErrorKind = "unauthorized"     // bad or expired credentials
	ErrKindNotFound        ErrorKind = "not_found"        // authenticated but endpoint missing
	ErrKindMisconfig       ErrorKind = "misconfig"        // unexpected/malformed response
	ErrKindUpstreamError   ErrorKind = "upstream_error"   // server-side failure
)

const (
	defaultTimeout       = 5 * time.Second
	defaultClientVersion = "0.0.0"
	defaultClientName    = "abcmovies"
	maxResponseBodyBytes = 1 << 20
)

// NewClient returns a client with sane defaults for tests and production use.
// The caller may override fields for testing or tailoring.
func NewClient() *Client {
	return &Client{
		Timeout:       defaultTimeout,
		ClientVersion: defaultClientVersion,
		ClientName:    defaultClientName,
	}
}

// NormalizeEndpoint validates and normalizes a user-supplied endpoint for
// storage and HTTP access. It is exported for callers that want to validate
// before any network access (e.g. connect forms). The result is a bare base
// URL with no credentials, query, or fragment.
func NormalizeEndpoint(raw string) (string, *Error) { return normalizeEndpoint(raw) }

// Verify checks that the supplied credentials grant access to the Jellyfin
// server and returns its identity. It is used to implement connect and
// update verification (DEC-001).
//
// It never logs, stores, or returns credentials. The password-based token is
// held only in memory for the duration of the call.
func (c *Client) Verify(ctx context.Context, endpoint string, creds Credentials) (*ServerInfo, *Error) {
	endpoint, nerr := normalizeEndpoint(endpoint)
	if nerr != nil {
		return nil, nerr
	}
	if creds.APIKey != "" && creds.Password != nil {
		return nil, &Error{Kind: ErrKindInvalidEndpoint, Message: "provide either username/password or an API key, not both"}
	}
	if creds.APIKey == "" && creds.Password == nil {
		return nil, &Error{Kind: ErrKindUnauthorized, Message: "credentials are required"}
	}
	switch p := creds.Password; {
	case p != nil && strings.TrimSpace(p.Username) == "" && p.Password != "":
		return nil, &Error{Kind: ErrKindUnauthorized, Message: "a username is required when a password is provided"}
	case p != nil && p.Password == "" && strings.TrimSpace(p.Username) != "":
		return nil, &Error{Kind: ErrKindUnauthorized, Message: "a password is required when a username is provided"}
	case p != nil && strings.TrimSpace(p.Username) == "" && p.Password == "":
		return nil, &Error{Kind: ErrKindUnauthorized, Message: "username and password are required"}
	}

	var token string
	if creds.Password != nil {
		t, err := c.authorize(ctx, endpoint, creds.Password.Username, creds.Password.Password)
		if err != nil {
			return nil, err
		}
		token = t
	} else {
		token = creds.APIKey
	}

	info, err := c.fetchSystemInfo(ctx, endpoint, token)
	if err != nil {
		return nil, err
	}
	if info.VerifiedID == "" {
		return nil, &Error{Kind: ErrKindMisconfig, Message: "the server response did not include an id"}
	}
	if info.DisplayName == "" {
		// Name is not security-relevant; a blank default is acceptable rather
		// than failing the connection for a missing nicety. That is distinct
		// from a missing id, which breaks identity.
		info.DisplayName = endpoint
	}
	return info, nil
}

// authorize exchanges username/password for a user access token.
func (c *Client) authorize(ctx context.Context, endpoint, username, password string) (string, *Error) {
	body, err := json.Marshal(map[string]string{"Username": username, "Pw": password})
	if err != nil {
		return "", &Error{Kind: ErrKindMisconfig, Message: "could not encode the authentication request"}
	}
	type authResponse struct {
		AccessToken string `json:"AccessToken"`
	}
	payload, doErr := c.doJSON(ctx, http.MethodPost, joinURL(endpoint, "/Users/AuthenticateByName"), body, c.authHeader(""))
	if doErr != nil {
		return "", doErr
	}
	var ar authResponse
	if uerr := json.Unmarshal(payload, &ar); uerr != nil {
		return "", &Error{Kind: ErrKindMisconfig, Message: "the server returned an unexpected response while authenticating"}
	}
	if ar.AccessToken == "" {
		return "", &Error{Kind: ErrKindUnauthorized, Message: "invalid username or password"}
	}
	return ar.AccessToken, nil
}

// fetchSystemInfo reads the server's identity using the token.
func (c *Client) fetchSystemInfo(ctx context.Context, endpoint, token string) (*ServerInfo, *Error) {
	type systemInfo struct {
		ServerName string `json:"ServerName"`
		Id         string `json:"Id"`
		Version    string `json:"Version"`
	}
	payload, doErr := c.doJSON(ctx, http.MethodGet, joinURL(endpoint, "/System/Info"), nil, c.authHeader(token))
	if doErr != nil {
		return nil, doErr
	}
	var si systemInfo
	if uerr := json.Unmarshal(payload, &si); uerr != nil {
		return nil, &Error{Kind: ErrKindMisconfig, Message: "the server returned an unexpected response while reading system info"}
	}
	id := strings.TrimSpace(si.Id)
	name := strings.TrimSpace(si.ServerName)
	version := strings.TrimSpace(si.Version)
	return &ServerInfo{
		Endpoint:    endpoint,
		VerifiedID:  id,
		DisplayName: name,
		Version:     version,
	}, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	// nil Transport uses the default (environment proxies, DNS, TLS verification).
	return &http.Client{Timeout: timeout}
}

func (c *Client) authHeader(token string) string {
	part := func(k, v string) string {
		v = clientValue(v, "")
		if v == "" {
			return ""
		}
		return k + `=` + url.QueryEscape(`"`+v+`"`)
	}
	parts := []string{part("Client", clientValue(c.ClientName, defaultClientName)),
		part("Device", "abcmovies-server"),
		part("DeviceId", "abcmovies-source"),
		part("Version", clientValue(c.ClientVersion, defaultClientVersion))}
	if token != "" {
		parts = append(parts, part("Token", token))
	}
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return "MediaBrowser " + strings.Join(out, ", ")
}

// clientValue sanitizes a MediaBrowser header token value: values are wrapped
// in quotes by the caller, so just escape any embedded quote/backslash and
// reject control characters. Returns "" if input is unusable.
func clientValue(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	// Drop control characters; MediaBrowser values are display-only.
	return strings.Map(func(r rune) rune {
		if r < 32 {
			return -1
		}
		return r
	}, v)
}

func (c *Client) doJSON(ctx context.Context, method, rawURL string, body []byte, authHeader string) ([]byte, *Error) {
	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reqBody)
	if err != nil {
		return nil, &Error{Kind: ErrKindInvalidEndpoint, Message: "endpoint URL could not be used for a request"}
	}
	client := c.httpClient()
	req.Header.Set("User-Agent", defaultClientName+"/"+clientValue(c.ClientVersion, defaultClientVersion))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, classifyTransportError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		b, rerr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodyBytes))
		if rerr != nil {
			return nil, &Error{Kind: ErrKindUpstreamError, StatusCode: resp.StatusCode, Message: "the server response could not be read"}
		}
		// Drain any remainder so the connection can be reused.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBodyBytes))
		return b, nil
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		msg := "access denied"
		if method == http.MethodPost && strings.Contains(rawURL, "/Users/AuthenticateByName") {
			msg = "invalid username or password"
		}
		return nil, &Error{Kind: ErrKindUnauthorized, StatusCode: resp.StatusCode, Message: msg}
	case http.StatusNotFound:
		return nil, &Error{Kind: ErrKindNotFound, StatusCode: resp.StatusCode, Message: "the requested endpoint was not found"}
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return nil, &Error{Kind: ErrKindUpstreamError, StatusCode: resp.StatusCode, Message: "the server returned an error; retry later", Retryable: true}
	default:
		// The response body is dropped: it is untrusted and we have better
		// messages to report to the user than "the server said ...".
		return nil, &Error{Kind: ErrKindMisconfig, StatusCode: resp.StatusCode, Message: "the server returned an unexpected response"}
	}
}

func classifyTransportError(err error) *Error {
	var urlErr *url.Error
	msg := strings.ToLower(err.Error())
	switch {
	case errors.As(err, &urlErr) && urlErr.Timeout():
		return &Error{Kind: ErrKindUnreachable, Message: "the server did not respond in time", Retryable: true}
	case strings.Contains(msg, "no such host"):
		return &Error{Kind: ErrKindUnreachable, Message: "the server address could not be resolved", Retryable: true}
	case strings.Contains(msg, "connection refused"):
		return &Error{Kind: ErrKindUnreachable, Message: "the server refused the connection", Retryable: true}
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return &Error{Kind: ErrKindUnreachable, Message: "the server did not respond in time", Retryable: true}
	case strings.Contains(msg, "proxyconnect") || strings.Contains(msg, "proxy:"):
		return &Error{Kind: ErrKindInvalidEndpoint, Message: "the endpoint could not be used through the configured proxy"}
	case strings.Contains(msg, "tls:"), strings.Contains(msg, "x509"), strings.Contains(msg, "certificate"):
		return &Error{Kind: ErrKindInvalidEndpoint, Message: "the server's TLS certificate or handshake could not be verified"}
	case strings.Contains(msg, "eof"), strings.Contains(msg, "reset"), strings.Contains(msg, "broken pipe"):
		return &Error{Kind: ErrKindUnreachable, Message: "the connection to the server was interrupted", Retryable: true}
	case errors.As(err, &urlErr):
		switch {
		case strings.Contains(strings.ToLower(urlErr.Err.Error()), "proxy"):
			return &Error{Kind: ErrKindInvalidEndpoint, Message: "the endpoint could not be used through the configured proxy"}
		default:
			_ = urlErr
		}
	}
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return &Error{Kind: ErrKindUnreachable, Message: "the server could not be reached", Retryable: true}
	}
	// Conservative default: treat unknown transport failures as unreachable
	// (offer retry) rather than a misconfiguration verdict.
	return &Error{Kind: ErrKindUnreachable, Message: "the server could not be reached", Retryable: true}
}
