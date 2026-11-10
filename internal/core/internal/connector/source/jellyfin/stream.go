package jellyfin

import (
	"context"
	"net/url"
)

// StreamURL builds the upstream direct-play stream URL for an item. It serves
// the original file (static=true); negotiation and codec compatibility beyond
// that are the client's and source's business (REQ-PLY-02, best effort). The
// URL carries no credentials.
func StreamURL(endpoint, itemID string) (string, *Error) {
	endpoint, nerr := normalizeEndpoint(endpoint)
	if nerr != nil {
		return "", nerr
	}
	if itemID == "" {
		return "", &Error{Kind: ErrKindInvalidEndpoint, Message: "an item id is required to build a stream URL"}
	}
	return joinURL(endpoint, "/Videos/"+url.PathEscape(itemID)+"/stream?static=true"), nil
}

// Authorize authenticates against the server and returns the Authorization
// header value to use for upstream requests on the user's behalf. The header
// embeds a source access token; it must be used server-to-server only and
// never returned to browsers. Password-derived tokens are held in memory for
// the duration of the caller's use and are never persisted or logged.
func (c *Client) Authorize(ctx context.Context, endpoint string, creds Credentials) (string, *Error) {
	endpoint, nerr := normalizeEndpoint(endpoint)
	if nerr != nil {
		return "", nerr
	}
	creds, credErr := credentialsForRequest(creds)
	if credErr != nil {
		return "", credErr
	}
	if creds.Password != nil {
		auth, err := c.authorize(ctx, endpoint, creds.Password.Username, creds.Password.Password)
		if err != nil {
			return "", err
		}
		return c.authHeader(auth.Token), nil
	}
	return c.authHeader(creds.APIKey), nil
}
