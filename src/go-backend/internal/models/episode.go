package models

import "time"

// URL params
type EpisodeRequest struct {
	ServiceTag    string
	ShowID        string
	SeasonNumber  int
	EpisodeNumber int
}

// Episode Response
type Episode struct {
	Streams            *[]EpisodeStream    `json:"streams"`
	BackdropURL        string              `json:"backdropURL"`
	Number             int                 `json:"number"`
	Name               string              `json:"name"`
	OriginalName       string              `json:"originalName"`
	Overview           string              `json:"overview"`
	PosterURL          string              `json:"posterURL"`
	MediaType          string              `json:"mediaType"`
	OriginalLanguage   string              `json:"originalLanguage"`
	Length             float64             `json:"length"`
	Cast               []string            `json:"cast"`
	Directors          []string            `json:"directors"`
	FirstAirDate       time.Time           `json:"firstAirDate"`
	OriginCountry      string              `json:"originCountry"`
	AvailabilityStatus string              `json:"availabilityStatus"`
	Described          bool                `json:"described"`
	VideoTracks        []EpisodeVideoTrack `json:"videoTracks"`
	AudioTracks        []EpisodeAudioTrack `json:"audioTracks"`
	TextTracks         []EpisodeTextTrack  `json:"textTracks"`
	CuePoints          []EpisodeCuePoint   `json:"cuePoints"`
}

// Next Episode Response
type NextEpisode struct {
	ShowID       string `json:"showID"`
	SeasonNumber int    `json:"seasonNumber"`

	Episode
}

type EpisodeStream struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type EpisodeVideoTrack struct {
	Name    string `json:"name"`
	Height  int    `json:"height"`
	Width   int    `json:"width"`
	Bitrate int    `json:"bitrate"`
}

type EpisodeAudioTrack struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	OriginalName string `json:"originalName"`
}

type EpisodeTextTrack struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Language string `json:"language"`
	TrackURL string `json:"trackURL"`
}

type EpisodeCuePoint struct {
	Name  string  `json:"name"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}
