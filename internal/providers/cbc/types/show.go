package types

import "time"

type ShowResponse struct {
	Title                   string              `json:"title"`
	OriginalTitle           string              `json:"originalTitle"`
	Description             string              `json:"description"`
	Badge                   *Badge              `json:"badge"`
	Images                  *Images             `json:"images"`
	Sponsors                *Sponsors           `json:"sponsors"`
	SelectedURL             string              `json:"selectedUrl"`
	ContentType             string              `json:"contentType"`
	RequestedType           string              `json:"requestedType"`
	ExternalSite            *ExternalSite       `json:"externalSite"`
	Recommendations         *Recommendations    `json:"recommendations"`
	NavigationFilters       []NavigationFilter  `json:"navigationFilters"`
	Messages                []Message           `json:"messages"`
	StructuredMetadata      *StructuredMetadata `json:"structuredMetadata"`
	Contents                []Content           `json:"content"`
	Metadata                *Metadata           `json:"metadata"`
	HTMLMeta                *HTMLMeta           `json:"htmlMeta"`
	Event                   *Event              `json:"event"`
	MandatoryAdsForAllUsers bool                `json:"mandatoryAdsForAllUsers"`
	HasOnlyATrailer         bool                `json:"hasOnlyATrailer"`
	IsFavouriteSupported    bool                `json:"isFavouriteSupported"`
	IsShareSupported        bool                `json:"isShareSupported"`
}

type Sponsors struct {
	Label    string    `json:"label"`
	Sponsors []Sponsor `json:"sponsors"`
}

type Sponsor struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	AltText string `json:"altText"`
}

type ExternalSite struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type Recommendations struct {
	Items         []RecommendationItem `json:"items"`
	CardImageType string               `json:"cardImageType"`
	LineupType    string               `json:"lineupType"`
}

type RecommendationItem struct {
	Title                     string  `json:"title"`
	InfoTitle                 string  `json:"infoTitle"`
	Description               string  `json:"description"`
	Tier                      string  `json:"tier"`
	Images                    *Images `json:"images"`
	URL                       string  `json:"url"`
	Badge                     *Badge  `json:"badge"`
	Type                      string  `json:"type"`
	GrantedRight              string  `json:"grantedRight"`
	ClosedCaptionAvailable    bool    `json:"closedCaptionAvailable"`
	VideoDescriptionAvailable bool    `json:"videoDescriptionAvailable"`
}

type NavigationFilter struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

type Message struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type StructuredMetadata struct {
	AtType           string           `json:"@type"`
	Actor            []Person         `json:"actor"`
	Director         []Person         `json:"director"`
	CountryOfOrigin  *CountryOfOrigin `json:"countryOfOrigin"`
	Duration         string           `json:"duration"`
	NumberOfEpisodes int              `json:"numberOfEpisodes"`
	SeasonNumber     int              `json:"seasonNumber"`
	NumberOfSeasons  int              `json:"numberOfSeasons"`
	ContainsSeason   []Season         `json:"containsSeason"`
	StartDate        time.Time        `json:"startDate"`
	PartofSeries     *PartofSeries    `json:"partOfSeries"`
	Trailer          Trailer          `json:"trailer"`
	Abstract         string           `json:"abstract"`
	Author           []Person         `json:"author"`
	Producers        []Producer       `json:"producer"`
	ContentRating    string           `json:"contentRating"`
	Genres           []string         `json:"genre"`
	InLanguage       string           `json:"inLanguage"`
	DatePublished    time.Time        `json:"datePublished"`
	DateCreated      string           `json:"dateCreated"` // unsure
	AtContext        string           `json:"@context"`
	Name             string           `json:"name"`
	AlternateName    string           `json:"alternateName"`
	URL              string           `json:"url"`
	Image            string           `json:"image"`
}

type Person struct {
	AtType    string `json:"@type"`
	AtContext string `json:"@context"`
	Name      string `json:"name"`
}

type CountryOfOrigin struct {
	Type    string `json:"@type"`
	Context string `json:"@context"`
	Name    string `json:"name"`
}

type Trailer struct {
	AtType    string `json:"@type"`
	Duration  string `json:"duration"`
	EmbedURL  string `json:"embedUrl"`
	AtContext string `json:"@context"`
}

type PartofSeries struct {
	AtType        string    `json:"@type"`
	StartDate     time.Time `json:"startDate"`
	Abstract      string    `json:"abstract"`
	Author        []Person  `json:"author"`
	ContentRating string    `json:"contentRating"`
	Genre         []string  `json:"genre"`
	InLanguage    string    `json:"inLanguage"`
	DatePublished time.Time `json:"datePublished"`
	AtContext     string    `json:"@context"`
	Name          string    `json:"name"`
	URL           string    `json:"url"`
	Image         string    `json:"image"`
}

type Season struct {
	AtType           string    `json:"@type"`
	Actor            []Person  `json:"actor"`
	Director         []Person  `json:"director"`
	NumberOfEpisodes int       `json:"numberOfEpisodes"`
	SeasonNumber     int       `json:"seasonNumber"`
	StartDate        time.Time `json:"startDate"`
	Abstract         string    `json:"abstract"`
	Author           []Person  `json:"author"`
	ContentRating    string    `json:"contentRating"`
	InLanguage       string    `json:"inLanguage"`
	DatePublished    time.Time `json:"datePublished"`
	AtContext        string    `json:"@context"`
	Name             string    `json:"name"`
	URL              string    `json:"url"`
	Image            string    `json:"image"`
}

type Producer struct {
	Type    string `json:"@type"`
	Context string `json:"@context"`
	Name    string `json:"name"`
}

type Content struct {
	Title       string          `json:"title"`
	Items       *SearchResponse `json:"items"`
	Lineups     []Lineup        `json:"lineups"`
	SeasonTitle string          `json:"seasonTitle"`
}

type Lineup struct {
	Title              string `json:"title"`
	Tier               string `json:"tier"`
	SeasonNumber       int    `json:"seasonNumber"`
	SelectedTrailerURL string `json:"selectedTrailerUrl"`
	URL                string `json:"url"`
	Items              []Item `json:"items"`
}

type Item struct {
	IDMedia                   int       `json:"idMedia"`
	Title                     string    `json:"title"`
	InfoTitle                 string    `json:"infoTitle"`
	Description               string    `json:"description"`
	Tier                      string    `json:"tier"`
	Images                    *Images   `json:"images"`
	URL                       string    `json:"url"`
	CallToActionTitle         string    `json:"callToActionTitle"`
	EpisodeNumber             int       `json:"episodeNumber"`
	CompletionTime            int       `json:"completionTime"`
	Links                     *Links    `json:"links"`
	Metadata                  *Metadata `json:"metadata"`
	MediaType                 string    `json:"mediaType"`
	Type                      string    `json:"type"`
	FormattedIDMedia          string    `json:"formattedIdMedia"`
	UniqueIdentifier          string    `json:"uniqueIdentifier"`
	ClosedCaptionAvailable    bool      `json:"closedCaptionAvailable"`
	VideoDescriptionAvailable bool      `json:"videoDescriptionAvailable"`
	IsPlaybackStatusSupported bool      `json:"isPlaybackStatusSupported"`
}

type Links struct {
	Next             string `json:"next"`
	Previous         string `json:"previous"`
	Action           string `json:"action"`
	CallToActionNext string `json:"callToActionNext"`
}

type Metadata struct {
	Media            *Media   `json:"media"`
	Credits          []Credit `json:"credits"`
	Country          string   `json:"country"`
	AirDate          string   `json:"airDate"`
	AvailabilityDate string   `json:"availabilityDate"`
	ProductionYear   int      `json:"productionYear"`
	Copyright        string   `json:"copyright"`
	Duration         int      `json:"duration"`
	Rating           string   `json:"rating"`
}

type Media struct {
	Credits          []Credit `json:"credits"`
	Country          string   `json:"country"`
	AirDate          string   `json:"airDate"`
	AvailabilityDate string   `json:"availabilityDate"`
	ProductionYear   int      `json:"productionYear"`
	Copyright        string   `json:"copyright"`
	Duration         int      `json:"duration"`
	Rating           string   `json:"rating"`
}

type Credit struct {
	Title   string `json:"title"`
	Peoples string `json:"peoples"`
}

type Event struct {
	Key                       string    `json:"key"`
	IDMedia                   int       `json:"idMedia"`
	Title                     string    `json:"title"`
	InfoTitle                 string    `json:"infoTitle"`
	CallToActionTitle         string    `json:"callToActionTitle"`
	Description               string    `json:"description"`
	ClosedCaptionAvailable    bool      `json:"closedCaptionAvailable"`
	VideoDescriptionAvailable bool      `json:"videoDescriptionAvailable"`
	Tier                      string    `json:"tier"`
	Images                    *Images   `json:"images"`
	URL                       string    `json:"url"`
	Type                      string    `json:"type"`
	FeedType                  string    `json:"feedType"`
	AirDate                   time.Time `json:"airDate"`
	GrantedRight              string    `json:"grantedRight"`
	FormattedIDMedia          string    `json:"formattedIdMedia"`
	IsVodEnabled              bool      `json:"isVodEnabled"`
}
