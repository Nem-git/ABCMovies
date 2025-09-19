package category

import (
	"github.com/nem-git/abcmovies/internal/show"
)

// Category ID
type CategorySlugs struct {
	ID string
}

// Category Response
type CategoryResponse struct {
	BackdropUrl string              `json:"backdropUrl"`
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Overview    string              `json:"overview"`
	PosterUrl   string              `json:"posterUrl"`
	Shows       []show.ShowResponse `json:"shows"`
}
