package models

// Page Response
type Page struct {
	BackdropURL string      `json:"backdropURL"`
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Overview    string      `json:"overview"`
	PosterURL   string      `json:"posterURL"`
	Categories  []Category `json:"categories"`
}

// Pages Response
type Pages struct {
	PageCount int     `json:"pageCount"`
	Pages     []Page `json:"pages"`
}
