package api

import (
	"github.com/nem-git/abcmovies/internal/show/api"
)

// All Categories
type CategoriesRequest struct {
}

// All Categories Response
type CategoriesResponse struct {
	CategoriesCount int                `json:"categoriesCount"`
	Categories      []CategoryResponse `json:"categories"`
}

// Category ID
type CategoryRequest struct {
	ID string
}

// Category Response
type CategoryResponse struct {
	BackdropUrl string             `json:"backdropUrl"`
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Overview    string             `json:"overview"`
	PosterUrl   string             `json:"posterUrl"`
	Shows       []api.ShowResponse `json:"shows"`
}
