package page

import (
	"github.com/nem-git/abcmovies/internal/recommendations/category"
)

// Page ID
type PageSlugs struct {
	PageID string
}

// Page Response
type PageResponse struct {
	BackdropUrl string                      `json:"backdropUrl"`
	ID          string                      `json:"id"`
	Name        string                      `json:"name"`
	Overview    string                      `json:"overview"`
	PosterUrl   string                      `json:"posterUrl"`
	Categories  []category.CategoryResponse `json:"categories"`
}
