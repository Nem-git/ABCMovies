package rcmedia

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nem-git/abcmovies/internal/providers/rcmedia/types"
)

const defaultTimeout = 30 * time.Second

const (
	userAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"
	catalogProdBase = "https://services.radio-canada.ca"
	streamMetaPath  = "/media/meta/v1/index.ashx"
	streamValPath   = "/media/validation/v2/"
)

type clientOptions struct {
	httpClient *http.Client
	baseURL    string
}

type client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	appCode    string
}

func newClient(appCode string, opts clientOptions) *client {
	hc := opts.httpClient
	if hc == nil {
		hc = &http.Client{Timeout: defaultTimeout}
	}
	base := opts.baseURL
	if base == "" {
		base = catalogProdBase
	}
	return &client{
		httpClient: hc,
		baseURL:    base,
		userAgent:  userAgent,
		appCode:    appCode,
	}
}

func (c *client) catalogURL(path string) string {
	return c.baseURL + "/ott/catalog/v2/" + c.appCode + path
}

func (c *client) getBrowse(ctx context.Context) error {
	body, _, err := c.getRaw(ctx, c.catalogURL("/browse"), map[string]string{"device": "web"})
	if err != nil {
		return err
	}
	body.Close()
	return nil
}

func (c *client) search(ctx context.Context, term string, page, pageSize int) (*types.SearchResponse, error) {
	var resp types.SearchResponse
	params := map[string]string{"term": term, "device": "web"}
	if page > 0 {
		params["page"] = strconv.Itoa(page)
	}
	if pageSize > 0 {
		params["pageSize"] = strconv.Itoa(pageSize)
	}
	if err := c.getJSON(ctx, c.catalogURL("/search"), params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *client) getCategory(ctx context.Context, category string, page, pageSize int) (*types.CategoryResponse, error) {
	u := c.catalogURL("/category/" + url.PathEscape(category))
	params := map[string]string{"device": "web"}
	if page > 0 {
		params["pageNumber"] = strconv.Itoa(page)
	}
	if pageSize > 0 {
		params["pageSize"] = strconv.Itoa(pageSize)
	}
	var resp types.CategoryResponse
	if err := c.getJSON(ctx, u, params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *client) getShow(ctx context.Context, showId string) (*types.ShowResponse, error) {
	u := c.catalogURL("/show/" + url.PathEscape(showId))
	var resp types.ShowResponse
	if err := c.getJSON(ctx, u, map[string]string{"device": "web"}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *client) getSeason(ctx context.Context, showId, seasonId string) (*types.ShowResponse, error) {
	u := strings.Join([]string{c.catalogURL(""), "show", url.PathEscape(showId), url.PathEscape(seasonId)}, "/")
	var resp types.ShowResponse
	if err := c.getJSON(ctx, u, map[string]string{"device": "web"}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *client) getStreamMeta(ctx context.Context, idMedia string) (*types.StreamMetaResponse, error) {
	var resp types.StreamMetaResponse
	params := map[string]string{
		"appCode": c.appCode,
		"idMedia": idMedia,
		"output":  "jsonObject",
	}
	if err := c.getJSON(ctx, c.baseURL+streamMetaPath, params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *client) getStreamValidation(ctx context.Context, idMedia, tech string) (*types.StreamValidationResponse, error) {
	var resp types.StreamValidationResponse
	params := map[string]string{
		"appCode":        c.appCode,
		"connectionType": "hd",
		"deviceType":     "multiams",
		"idMedia":        idMedia,
		"multibitrate":   "true",
		"output":         "json",
		"tech":           tech,
		"manifestType":   "desktop",
	}
	if err := c.getJSON(ctx, c.baseURL+streamValPath, params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *client) getRaw(ctx context.Context, rawURL string, params map[string]string) (io.ReadCloser, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	if len(params) > 0 {
		q := req.URL.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("executing request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, "", fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, req.URL.Redacted(), string(body))
	}
	return resp.Body, resp.Header.Get("Content-Type"), nil
}

func (c *client) getImage(ctx context.Context, imageURL string) (io.ReadCloser, string, error) {
	return c.getRaw(ctx, imageURL, nil)
}

func (c *client) getJSON(ctx context.Context, urlStr string, params map[string]string, dest any) error {
	body, _, err := c.getRaw(ctx, urlStr, params)
	if err != nil {
		return fmt.Errorf("fetching json: %w", err)
	}
	defer body.Close()

	return json.NewDecoder(body).Decode(dest)
}
