package jellyfin

import (
	"net"
	"net/url"
	"strings"
)

// normalizeEndpoint validates and normalizes a user-supplied Jellyfin endpoint
// for storage, comparison, and HTTP access.
//
// Rules (DEC-001):
//   - an http(s) scheme is required; https is the default when absent
//   - scheme and host are lowercased; non-default ports are kept, the scheme's
//     default port (80 for http, 443 for https) is dropped
//   - userinfo is rejected: credentials must travel in the request
//     (Authorization header), never in a URL we store or compare
//   - query and fragment are stripped (the normal form is a base URL)
//   - a trailing slash on the path is dropped; a path of "/" is dropped
//   - host must be present; port must be numeric and in 1..65535
//
// The result contains no credentials, tokens, or query material.
func normalizeEndpoint(raw string) (string, *Error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", &Error{Kind: ErrKindInvalidEndpoint, Message: "endpoint is required"}
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", &Error{Kind: ErrKindInvalidEndpoint, Message: "endpoint URL could not be parsed"}
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", &Error{Kind: ErrKindInvalidEndpoint, Message: "endpoint scheme must be http or https"}
	}
	if u.User != nil {
		return "", &Error{Kind: ErrKindInvalidEndpoint, Message: "endpoint must not include a username or password; pass credentials separately"}
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", &Error{Kind: ErrKindInvalidEndpoint, Message: "endpoint must include a host"}
	}
	if p := u.Port(); p != "" {
		port, ok := parsePort(p)
		if !ok {
			return "", &Error{Kind: ErrKindInvalidEndpoint, Message: "endpoint port must be a number between 1 and 65535"}
		}
		switch {
		case scheme == "http" && port == 80, scheme == "https" && port == 443:
		default:
			host = net.JoinHostPort(host, p)
		}
	}

	p := u.EscapedPath()
	switch {
	case p == "/":
		p = ""
	case strings.HasSuffix(p, "/"):
		p = strings.TrimSuffix(p, "/")
	}

	norm := url.URL{Scheme: scheme, Host: host, Path: p}
	return norm.String(), nil
}

// joinURL appends path to base, preserving any configured path prefix.
func joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}

func parsePort(p string) (int, bool) {
	n := 0
	for _, r := range p {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
		if n > 65535 {
			return 0, false
		}
	}
	return n, n >= 1
}
