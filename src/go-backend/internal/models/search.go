package models

// Search Response
type Search struct {
	Query     string  `json:"query"`
	ShowCount int     `json:"showCount"`
	Shows     []*Show `json:"shows"`
}
