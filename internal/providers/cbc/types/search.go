package types

type SearchResponse struct {
	TotalPages   int       `json:"totalPages"`
	TotalRecords int       `json:"totalRecords"`
	PageNumber   int       `json:"pageNumber"`
	PageSize     int       `json:"pageSize"`
	Results      *[]Result `json:"results"`
}

type Result struct {
	Title                     string   `json:"title"`
	Key                       string   `json:"key"`
	InfoTitle                 string   `json:"infoTitle"`
	Description               string   `json:"description"`
	Tier                      string   `json:"tier"`
	Images                    *Images  `json:"images"`
	URL                       string   `json:"url"`
	Badge                     *Badge   `json:"badge"`
	Type                      string   `json:"type"`
	GrantedRight              string   `json:"grantedRight"`
	ClosedCaptionAvailable    bool     `json:"closedCaptionAvailable"`
	VideoDescriptionAvailable bool     `json:"videoDescriptionAvailable"`
	CardOptions               []string `json:"cardOptions"`
	IsFavouriteSupported      bool     `json:"isFavouriteSupported"`
}

type SearchV1Response struct {
	TotalRecords int               `json:"totalRecords"`
	PageNumber   int               `json:"pageNumber"`
	TotalPages   int               `json:"totalPages"`
	PageSize     int               `json:"pageSize"`
	Results      *[]SearchV1Result `json:"result"`
}

type ImageURL struct {
	URL string `json:"url"`
}

type SearchV1Result struct {
	Title          string    `json:"title"`
	SearchableText string    `json:"searchableText"`
	Image          *ImageURL `json:"image"`
	Type           string    `json:"type"`
	URL            string    `json:"url"`
}
