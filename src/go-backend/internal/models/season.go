package models

import (
	"time"
)

// URL params
type SeasonRequest struct {
	ServiceTag   string
	ShowID       string
	SeasonNumber int
}

// Season Response
type Season struct {
	BackdropURL        string    `json:"backdropURL"`
	Number             int       `json:"number"`
	EpisodeCount       int       `json:"episodeCount"`
	Name               string    `json:"name"`
	OriginalName       string    `json:"originalName"`
	Overview           string    `json:"overview"`
	PosterURL          string    `json:"posterURL"`
	FirstAirDate       time.Time `json:"firstAirDate"`
	AvailabilityStatus string    `json:"availabilityStatus"`
	Episodes           []Episode `json:"episodes"`
}
