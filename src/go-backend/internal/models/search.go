package models

// URL params
type SearchRequest struct {
	Query string
}

// URL params
type ServiceSearchRequest struct {
	SearchRequest

	ServiceTag string
}

// Search Response
type Search struct {
	Query     string         `json:"query"`
	ShowCount int            `json:"showCount"`
	Shows     []SearchResult `json:"shows"`
}

type SearchResult struct {
	Show

	ServiceTag string `json:"serviceTag,omitempty"`
}
