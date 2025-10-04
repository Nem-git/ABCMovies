package models

import "time"

type stream struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type videoTrack struct {
	Quality string `json:"quality"`
	Height  int    `json:"height"`
	Width   int    `json:"width"`
	Bitrate int    `json:"bitrate"`
}

type audioTrack struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	OriginalName string `json:"originalName"`
}

type textTrack struct {
	Type         string `json:"type"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	OriginalName string `json:"originalName"`
	TrackUrl     string `json:"trackUrl"`
}

type cuePoint struct {
	Name  string  `json:"name"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// Episode Response
type Episode struct {
	Streams              *[]stream     `json:"streams"`     ///
	BackdropUrl          string        `json:"backdropUrl"` ///
	Number               int           `json:"number"`      // ///
	Name                 string        `json:"name"`        // ///
	OriginalName         string        `json:"originalName"`
	Overview             string        `json:"overview"`  ///
	PosterUrl            string        `json:"posterUrl"` ///
	MediaType            string        `json:"mediaType"` //
	OriginalLanguage     string        `json:"originalLanguage"`
	OriginalLanguageCode string        `json:"originalLanguageCode"`
	CompletionTime       float64       `json:"completionTime"` // ///
	Cast                 []string      `json:"cast"`
	Directors            []string      `json:"directors"`
	FirstAirDate         time.Time     `json:"firstAirDate"`       //
	OriginCountry        string        `json:"originCountry"`      ///
	AvailabilityStatus   string        `json:"availabilityStatus"` // ///
	Described            bool          `json:"described"`
	VideoTracks          *[]videoTrack `json:"videoTracks"` ////
	AudioTracks          *[]audioTrack `json:"audioTracks"` ////
	TextTracks           *[]textTrack  `json:"textTracks"`  /// ////
	CuePoints            *[]cuePoint   `json:"cuePoints"`   /// ////
}

// Next Episode Response
type NextEpisode struct {
	Episode

	ShowID       string `json:"showID"`
	SeasonNumber int    `json:"seasonNumber"`
}
