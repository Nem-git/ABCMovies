package jellyfin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const (
	libraryIncludeItemTypes = "Movie,Series"
	libraryPageSize         = 200
	libraryMaxPages         = 100
)

// LibraryItem is a non-sensitive library title from the upstream source.
type LibraryItem struct {
	ExternalID string // upstream item ID (non-secret)
	Name       string
	Kind       string // "movie" | "series" | "other"
	Year       int32  // 0 if unknown
	Overview   string
}

// LibrarySnapshot is the result of a fetch of the source's library.
type LibrarySnapshot struct {
	Items   []LibraryItem
	Skipped int // titles skipped because of unusable upstream data
}

// FetchLibrary authenticates (transient credentials) and returns the source's
// library titles. The token is held only in memory and is never returned.
//
// Skipped items (empty name or ID) do not fail the fetch; the returned
// snapshot reports how many were skipped. Any transport/auth failure returns
// an error and no snapshot, so the caller does not apply partial data.
func (c *Client) FetchLibrary(ctx context.Context, endpoint string, creds Credentials) (*LibrarySnapshot, *Error) {
	endpoint, nerr := normalizeEndpoint(endpoint)
	if nerr != nil {
		return nil, nerr
	}
	creds, authErr := credentialsForRequest(creds)
	if authErr != nil {
		return nil, authErr
	}

	var token, userID string
	if creds.Password != nil {
		auth, err := c.authorize(ctx, endpoint, creds.Password.Username, creds.Password.Password)
		if err != nil {
			return nil, err
		}
		token, userID = auth.Token, auth.UserID
	} else {
		token = creds.APIKey
	}

	pageSize := int32(libraryPageSize)
	total := int32(-1)
	snapshot := &LibrarySnapshot{}
	for page := int32(0); ; page++ {
		result, err := c.fetchLibraryPage(ctx, endpoint, token, userID, page, pageSize)
		if err != nil {
			return nil, err
		}
		if total < 0 {
			total = result.TotalRecordCount
		}
		for _, raw := range result.Items {
			item, ok := raw.toLibraryItem()
			if !ok {
				snapshot.Skipped++
				continue
			}
			snapshot.Items = append(snapshot.Items, item)
		}
		fetched := len(snapshot.Items) + snapshot.Skipped
		done := total >= 0 && int64(fetched) >= int64(total)
		if !done && len(result.Items) == 0 {
			done = true
		}
		if !done && total < 0 && len(result.Items) < int(pageSize) {
			done = true
		}
		if !done && page+1 >= libraryMaxPages {
			done = true
		}
		if done {
			break
		}
	}
	return snapshot, nil
}

type itemDTO struct {
	Name           string `json:"Name"`
	Id             string `json:"Id"`
	Type           string `json:"Type"`
	ProductionYear int    `json:"ProductionYear"`
	Overview       string `json:"Overview"`
}

func (d itemDTO) toLibraryItem() (LibraryItem, bool) {
	id := strings.TrimSpace(d.Id)
	name := strings.TrimSpace(d.Name)
	if id == "" || name == "" {
		return LibraryItem{}, false
	}
	kind := "other"
	switch strings.ToLower(strings.TrimSpace(d.Type)) {
	case "movie":
		kind = "movie"
	case "series":
		kind = "series"
	}
	return LibraryItem{
		ExternalID: id,
		Name:       name,
		Kind:       kind,
		Year:       int32(d.ProductionYear),
		Overview:   d.Overview,
	}, true
}

type itemsResultDTO struct {
	Items            []itemDTO `json:"Items"`
	TotalRecordCount int32     `json:"TotalRecordCount"`
}

func (c *Client) fetchLibraryPage(ctx context.Context, endpoint, token, userID string, page, pageSize int32) (*itemsResultDTO, *Error) {
	start := page * pageSize
	q := url.Values{}
	q.Set("includeItemTypes", libraryIncludeItemTypes)
	q.Set("recursive", "true")
	q.Set("sortBy", "Name")
	q.Set("sortOrder", "Ascending")
	q.Set("fields", "Overview")
	q.Set("enableImages", "false")
	q.Set("enableUserData", "false")
	q.Set("enableTotalRecordCount", "true")
	q.Set("startIndex", fmt.Sprintf("%d", start))
	q.Set("limit", fmt.Sprintf("%d", pageSize))
	if strings.TrimSpace(userID) != "" {
		q.Set("userId", userID)
	}
	payload, err := c.doJSON(ctx, http.MethodGet, joinURL(endpoint, "/Items?"+q.Encode()), nil, c.authHeader(token))
	if err != nil {
		return nil, err
	}
	type itemsEnvelope struct {
		Items            []itemDTO `json:"Items"`
		TotalRecordCount int32     `json:"TotalRecordCount"`
	}
	var result itemsEnvelope
	if uerr := json.Unmarshal(payload, &result); uerr != nil {
		return nil, &Error{Kind: ErrKindMisconfig, Message: "the server returned an unexpected response while reading the library"}
	}
	return &itemsResultDTO{Items: result.Items, TotalRecordCount: result.TotalRecordCount}, nil
}
