package page

import (
	categoryApi "github.com/nem-git/abcmovies/internal/recommendations/category/api"
)

// Page ID
type PageRequest struct {
	PageID string
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
