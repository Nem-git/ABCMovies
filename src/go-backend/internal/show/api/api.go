package api

import (
	"time"

	"github.com/nem-git/abcmovies/internal/season/api"
)

// Show ID
type ShowRequest struct {
	ServiceTag string
	ShowID     string
}

type Genre struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Show Response
type ShowResponse struct {
	Adult                bool                 `json:"adult"`
	BackdropURL          string               `json:"backdropURL"`
	ID                   string               `json:"id"`
	SeasonCount          string               `json:"seasonCount"`
	Name                 string               `json:"name"`
	OriginalName         string               `json:"originalName"`
	Overview             string               `json:"overview"`
	PosterURL            string               `json:"posterURL"`
	MediaType            string               `json:"mediaType"`
	OriginalLanguage     string               `json:"originalLanguage"`
	OriginalLanguageCode string               `json:"originalLanguageCode"`
	Genres               []Genre              `json:"genres"`
	Cast                 []string             `json:"cast"`
	Directors            []string             `json:"directors"`
	FirstAirDate         time.Time            `json:"firstAirDate"`
	OriginCountry        string               `json:"originCountry"`
	AvailabilityStatus   string               `json:"availabilityStatus"`
	Seasons              []api.SeasonResponse `json:"seasons"`
}

func ShowRequestHandler(serviceTag string, id string) ShowRequest {
	return ShowRequest{
		ServiceTag: serviceTag,
		ShowID:     id,
	}
}
