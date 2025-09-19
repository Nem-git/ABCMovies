package api

// Category ID
type CategorySlugs struct {
	ID string
}

// Category Response
type CategoryResponse struct {
	BackdropUrl string         `json:"backdropUrl"`
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Overview    string         `json:"overview"`
	PosterUrl   string         `json:"posterUrl"`
	Shows       []ShowResponse `json:"shows"`
}
