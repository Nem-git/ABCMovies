package types

type CategoriesResponse struct {
	Formats     []Format     `json:"formats"`
	Networks    []Network    `json:"networks"`
	Collections []Collection `json:"collections"`
	Genres      []Genre      `json:"genres"`
	HTMLMeta    *HTMLMeta    `json:"htmlMeta"`
}

type Format struct {
	Title string `json:"title"`
	Image *Image `json:"image"`
	URL   string `json:"url"`
}

type Background struct {
	SolidColor   string `json:"solidColor"`
	GradientType string `json:"gradientType"`
	ColorStart   string `json:"colorStart"`
	ColorEnd     string `json:"colorEnd"`
}

type Network struct {
	Title      string      `json:"title"`
	Image      *Image      `json:"image"`
	Background *Background `json:"background"`
	URL        string      `json:"url"`
}

type Collection struct {
	Title string `json:"title"`
	Image *Image `json:"image"`
	URL   string `json:"url"`
}

type Genre struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type CategoryResponse struct {
	Title    string            `json:"title"`
	Header   *Header           `json:"header"`
	Contents []CategoryContent `json:"content"`
	Actions  []Action          `json:"actions"`
	HTMLMeta *HTMLMeta         `json:"htmlMeta"`
	HasAds   bool              `json:"hasAds"`
}

type Header struct {
	Title         string   `json:"title"`
	Key           string   `json:"key"`
	Items         []Result `json:"items"`
	CardImageType string   `json:"cardImageType"`
	LayoutType    string   `json:"layoutType"`
	LineupType    string   `json:"lineupType"`
}

type CategoryContent struct {
	Key           string          `json:"key"`
	Items         *SearchResponse `json:"items"`
	CardImageType string          `json:"cardImageType"`
	LineupType    string          `json:"lineupType"`
}

type Action struct {
	Title      string `json:"title"`
	URL        string `json:"url"`
	IsSelected bool   `json:"isSelected"`
}
