package api

import (
	episodeApi "github.com/nem-git/abcmovies/internal/episode/api"
)

type SeasonRequest struct {
	ServiceTag   string
	ShowID       string
	SeasonNumber int
}

// Season Response
type SeasonResponse struct {
	Adult              bool                         `json:"adult"`
	BackdropUrl        string                       `json:"backdropUrl"`
	Number             int                          `json:"number"`
	EpisodeCount       int                          `json:"episodeCount"`
	Name               string                       `json:"name"`
	OriginalName       string                       `json:"originalName"`
	Overview           string                       `json:"overview"`
	PosterUrl          string                       `json:"posterUrl"`
	FirstAirDate       string                       `json:"firstAirDate"`
	AvailabilityStatus string                       `json:"availabilityStatus"`
	Episodes           []episodeApi.EpisodeResponse `json:"episodes"`
}

func SeasonRequestHandler(serviceTag string, showID string, number int) SeasonRequest {
	return SeasonRequest{
		ServiceTag:   serviceTag,
		ShowID:       showID,
		SeasonNumber: number,
	}
}
