package models

import (
	"time"
)

// Season Response
type Season struct {
	BackdropUrl        string     `json:"backdropUrl"`
	Number             int        `json:"number"`
	EpisodeCount       int        `json:"episodeCount"`
	Name               string     `json:"name"`
	OriginalName       string     `json:"originalName"`
	Overview           string     `json:"overview"`
	PosterUrl          string     `json:"posterUrl"`
	FirstAirDate       time.Time  `json:"firstAirDate"`
	AvailabilityStatus string     `json:"availabilityStatus"`
	Episodes           *[]Episode `json:"episodes"`
}
