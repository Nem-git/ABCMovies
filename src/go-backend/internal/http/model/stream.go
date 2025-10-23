package model

// URL params
type StreamRequest struct {
	ServiceTag     string
	ShowID         string
	SeasonNumber   int
	EpisodeNumber  int
	StreamType     string
	StreamFileName string
	StreamURL      string

	// Dash

	StreamMediaType string
	StreamID        string
}

// Stream Response
type Stream []byte
