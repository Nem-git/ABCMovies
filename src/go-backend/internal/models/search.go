package models

// Search Response
type Search struct {
	Query     string          `json:"query"`
	ShowCount int             `json:"showCount"`
	Shows     *[]SearchResult `json:"shows"`
}

type SearchResult struct {
	Show

	ServiceTag string `json:"serviceTag"`
}
