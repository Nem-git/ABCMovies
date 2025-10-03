package api

import "time"

type EpisodeRequest struct {
	ServiceTag    string
	ShowID        string
	SeasonNumber  int
	EpisodeNumber int
}

type Stream struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type VideoTrack struct {
	Quality string `json:"quality"`
	Height  int    `json:"height"`
	Width   int    `json:"width"`
	Bitrate int    `json:"bitrate"`
}

type AudioTrack struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	OriginalName string `json:"originalName"`
}

type TextTrack struct {
	Type         string `json:"type"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	OriginalName string `json:"originalName"`
	TrackUrl     string `json:"trackUrl"`
}

type CuePoint struct {
	Name  string  `json:"name"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// Episode Response
type EpisodeResponse struct {
	Streams              *[]Stream     `json:"streams"` ///
	BackdropUrl          string        `json:"backdropUrl"` ///
	Number               int           `json:"number"` // ///
	Name                 string        `json:"name"` // ///
	OriginalName         string        `json:"originalName"`
	Overview             string        `json:"overview"` ///
	PosterUrl            string        `json:"posterUrl"` ///
	MediaType            string        `json:"mediaType"` //
	OriginalLanguage     string        `json:"originalLanguage"`
	OriginalLanguageCode string        `json:"originalLanguageCode"`
	CompletionTime       float64       `json:"completionTime"` // ///
	Cast                 []string      `json:"cast"`
	Directors            []string      `json:"directors"`
	FirstAirDate         time.Time     `json:"firstAirDate"` //
	OriginCountry        string        `json:"originCountry"` ///
	AvailabilityStatus   string        `json:"availabilityStatus"` // ///
	Described            bool          `json:"described"`
	VideoTracks          *[]VideoTrack `json:"videoTracks"` ////
	AudioTracks          *[]AudioTrack `json:"audioTracks"` ////
	TextTracks           *[]TextTrack  `json:"textTracks"` /// ////
	CuePoints            *[]CuePoint   `json:"cuePoints"` /// ////
}

// Next Episode Response
type NextEpisodeResponse struct {
	EpisodeResponse

	ShowID       string `json:"showID"`
	SeasonNumber int    `json:"seasonNumber"`
}

func EpisodeRequestHandler(serviceTag string, showID string, seasonNumber int, number int) EpisodeRequest {
	return EpisodeRequest{
		ServiceTag:    serviceTag,
		ShowID:        showID,
		SeasonNumber:  seasonNumber,
		EpisodeNumber: number,
	}
}
