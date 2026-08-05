package proxy

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// Fetcher knows how to retrieve upstream content.
type Fetcher interface {
	Fetch(ctx context.Context, rawURL string, headers http.Header, query url.Values) (io.ReadCloser, http.Header, error)
}

// HTTPFetcher fetches via HTTP/HTTPS.
type HTTPFetcher struct {
	Client *http.Client
}

// defaultUserAgent is used for upstream requests that don't specify one; some
// CDNs reject requests with no User-Agent.
const defaultUserAgent = "ABCMovies/1.0"

func NewHTTPFetcher(client *http.Client) *HTTPFetcher {
	return &HTTPFetcher{Client: client}
}

func (f *HTTPFetcher) Fetch(ctx context.Context, rawURL string, headers http.Header, query url.Values) (io.ReadCloser, http.Header, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, err
	}

	if query != nil {
		q := u.Query()
		maps.Copy(q, query)
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, nil, err
	}

	for k, vs := range headers {
		req.Header[k] = vs
	}

	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", defaultUserAgent)
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, nil, fmt.Errorf("upstream returned %d: %s", resp.StatusCode, string(body))
	}

	return resp.Body, resp.Header, nil
}

// FileFetcher fetches via file:// URLs. For disk-based providers.
type FileFetcher struct{}

func (f *FileFetcher) Fetch(_ context.Context, rawURL string, _ http.Header, _ url.Values) (io.ReadCloser, http.Header, error) {
	if !strings.HasPrefix(rawURL, "file://") {
		return nil, nil, fmt.Errorf("FileFetcher: unsupported scheme in %s", rawURL)
	}
	path := strings.TrimPrefix(rawURL, "file://")
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return file, nil, nil
}
