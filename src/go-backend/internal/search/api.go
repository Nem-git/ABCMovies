package search

import (
	"github.com/nem-git/abcmovies/internal/show"
)

type SearchRequest struct {
	Query string
}

// Search Response
type SearchResponse struct {
	Query     string              `json:"query"`
	ShowCount int                 `json:"showCount"`
	Shows     []show.ShowResponse `json:"shows"`
}

func RequestHandler(query string) SearchRequest {
	return SearchRequest{
		Query: query,
	}
}
