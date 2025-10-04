package models

// Service Response
type Service struct {
	BackdropUrl        string   `json:"backdropUrl"`
	ID                 string   `json:"id"`
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

// Services Response
type Services struct {
	ServiceCount int        `json:"serviceCount"`
	Services     []*Service `json:"services"`
}
