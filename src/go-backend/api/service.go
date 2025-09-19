package api

// Streaming Service Tag
type StreamingServiceSlugs struct {
	StreamingServiceTag string
}

// Streaming Service Response
type StreamingServiceResponse struct {
	BackdropUrl        string   `json:"backdropUrl"`
	Id                 string   `json:"id"`
	Name               string   `json:"name"`
	OriginalName       string   `json:"originalName"`
	Overview           string   `json:"overview"`
	PosterUrl          string   `json:"posterUrl"`
	MediaTypes         []string `json:"mediaTypes"`
	OriginalLanguage   string   `json:"originalLanguage"`
	HomePage           string   `json:"homePage"`
	OriginCountry      string   `json:"originCountry"`
	AvailabilityStatus string   `json:"availabilityStatus"`
}
