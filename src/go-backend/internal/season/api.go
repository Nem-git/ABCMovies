package season

import (
	"github.com/nem-git/abcmovies/internal/episode"
)

// Season Number
type SeasonSlugs struct {
	SeasonNumber int
}

// Season Response
type SeasonResponse struct {
	Adult              bool                      `json:"adult"`
	BackdropUrl        string                    `json:"backdropUrl"`
	Number             int                       `json:"number"`
	EpisodeCount       int                       `json:"episodeCount"`
	Name               string                    `json:"name"`
	OriginalName       string                    `json:"originalName"`
	Overview           string                    `json:"overview"`
	PosterUrl          string                    `json:"posterUrl"`
	FirstAirDate       string                    `json:"firstAirDate"`
	AvailabilityStatus string                    `json:"availabilityStatus"`
	Episodes           []episode.EpisodeResponse `json:"episodes"`
}
