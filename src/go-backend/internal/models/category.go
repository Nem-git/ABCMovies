package models

// URL params
type CategoryRequest struct {
	ServiceTag string
	CategoryID string
}

type ServiceCategoriesRequest struct {
	ServiceTag string
}

// Category Response
type Category struct {
	BackdropURL string `json:"backdropURL"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Overview    string `json:"overview"`
	PosterURL   string `json:"posterURL"`
	Shows       []Show `json:"shows"`
}

// Categories Response
type Categories struct {
	CategoryCount int        `json:"categoryCount"`
	Categories    []Category `json:"categories"`
}
