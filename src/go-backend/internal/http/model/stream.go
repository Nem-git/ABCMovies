package model

// URL params
type StreamRequest struct {
	ServiceTag    string
	ShowID        string
	SeasonNumber  int
	EpisodeNumber int
	StreamType    string

	IsPlaylist bool
	IsMedia    bool

	Playlist StreamPlaylist
	Media    StreamMedia
}

type StreamPlaylist struct {
	FileName string
}

type StreamMedia struct {
	URL string

	Dash StreamDashMedia
}

type StreamDashMedia struct {
	ID   string
	Type string
}

// Stream Response
type Stream []byte
