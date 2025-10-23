package model

import (
	"time"
)

// URL params
type ShowRequest struct {
	ServiceTag string
	ShowID     string
}

// Show Response
type Show struct {
	Adult              bool        `json:"adult"`
	BackdropURL        string      `json:"backdropURL"`
	ID                 string      `json:"id"`
	SeasonCount        int         `json:"seasonCount"`
	Name               string      `json:"name"`
	OriginalName       string      `json:"originalName"`
	Overview           string      `json:"overview"`
	PosterURL          string      `json:"posterURL"`
	MediaType          string      `json:"mediaType"`
	OriginalLanguage   string      `json:"originalLanguage"`
	Genres             []ShowGenre `json:"genres"`
	Cast               []string    `json:"cast"`
	Directors          []string    `json:"directors"`
	FirstAirDate       time.Time   `json:"firstAirDate"`
	OriginCountry      string      `json:"originCountry"`
	AvailabilityStatus string      `json:"availabilityStatus"`
	Seasons            []Season    `json:"seasons"`
}

type ShowGenre struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
