package api

import (
	"github.com/nem-git/abcmovies/internal/show/api"
)

type SearchRequest struct {
	Query string
}

// Search Response
type SearchResponse struct {
	Query     string             `json:"query"`
	ShowCount int                `json:"showCount"`
	Shows     []api.ShowResponse `json:"shows"`
}

func RequestHandler(query string) SearchRequest {
	return SearchRequest{
		Query: query,
	}
}
