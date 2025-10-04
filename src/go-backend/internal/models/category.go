package models

// Category Response
type Category struct {
	BackdropUrl string `json:"backdropUrl"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Overview    string `json:"overview"`
	PosterUrl   string `json:"posterUrl"`
	Shows       []Show `json:"shows"`
}

// Categories Response
type Categories struct {
	CategoriesCount int        `json:"categoriesCount"`
	Categories      []Category `json:"categories"`
}
