package show

import (
	"github.com/nem-git/abcmovies/internal/season"
)

// Show ID
type ShowSlugs struct {
	ShowID string
}

type Genre struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Show Response
type ShowResponse struct {
	Adult                bool                    `json:"adult"`
	BackdropUrl          string                  `json:"backdropUrl"`
	Id                   string                  `json:"id"`
	SeasonCount          string                  `json:"seasonCount"`
	Name                 string                  `json:"name"`
	OriginalName         string                  `json:"originalName"`
	Overview             string                  `json:"overview"`
	PosterUrl            string                  `json:"posterUrl"`
	MediaType            string                  `json:"mediaType"`
	OriginalLanguage     string                  `json:"originalLanguage"`
	OriginalLanguageCode string                  `json:"originalLanguageCode"`
	Genres               []Genre                 `json:"genres"`
	Cast                 []string                `json:"cast"`
	Directors            []string                `json:"directors"`
	FirstAirDate         string                  `json:"firstAirDate"`
	OriginCountry        string                  `json:"originCountry"`
	AvailabilityStatus   string                  `json:"availabilityStatus"`
	Seasons              []season.SeasonResponse `json:"seasons"`
}
