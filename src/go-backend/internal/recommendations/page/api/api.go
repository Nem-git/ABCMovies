package api

import (
	categoryApi "github.com/nem-git/abcmovies/internal/recommendations/category/api"
)

// Pages
type PagesRequest struct {
}

// Pages
type PagesResponse struct {
	PageCount int            `json:"pageCount"`
	Pages     []PageResponse `json:"pages"`
}

// Page ID
type PageRequest struct {
	ID string
}

// Page Response
type PageResponse struct {
	BackdropUrl string                         `json:"backdropUrl"`
	ID          string                         `json:"id"`
	Name        string                         `json:"name"`
	Overview    string                         `json:"overview"`
	PosterUrl   string                         `json:"posterUrl"`
	Categories  []categoryApi.CategoryResponse `json:"categories"`
}

func PageRequestHandler(id string) PageRequest {
	return PageRequest{
		ID: id,
	}
}
