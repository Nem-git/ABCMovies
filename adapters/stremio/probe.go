package stremio

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Prober validates a linked account against a Stremio addon: it fetches the
// addon's manifest URL and refuses anything that is not a well-shaped addon
// manifest. An addon has no user password — the probe proves the provider
// responds and is an addon, which is what the core can honestly confirm
// before vaulting anything (PLAN.md §3.5).
type Prober struct{}

func (Prober) Probe(ctx context.Context, baseURL, username string, password []byte) ([]byte, error) {
	url := strings.TrimRight(baseURL, "/")
	if url == "" {
		return nil, fmt.Errorf("stremio: manifest url required")
	}
	if !strings.HasSuffix(url, "/manifest.json") {
		url = url + "/manifest.json"
	}
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stremio: probe fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode/100 != 2 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("stremio: probe status %d", resp.StatusCode)
	}
	var m manifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("stremio: probe: not a manifest: %w", err)
	}
	if m.ID == "" && len(m.Catalogs) == 0 {
		return nil, fmt.Errorf("stremio: probe: manifest missing id and catalogs")
	}
	// The session blob the core vaults: just the canonical manifest URL. The
	// account is anonymous; there is no secret key material to carry.
	return json.Marshal(Account{ID: username, ManifestURL: url})
}
