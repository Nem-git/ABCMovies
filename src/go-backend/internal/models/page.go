package models

// Page Response
type Page struct {
	BackdropUrl string     `json:"backdropUrl"`
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Overview    string     `json:"overview"`
	PosterUrl   string     `json:"posterUrl"`
	Categories  []Category `json:"categories"`
}

// Pages Response
type Pages struct {
	PageCount int    `json:"pageCount"`
	Pages     []Page `json:"pages"`
}
