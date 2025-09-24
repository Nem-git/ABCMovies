package api

// Request Slug
type ServiceRequest struct {
	ServiceTag string
}

// Service Response
type ServiceResponse struct {
	BackdropUrl        string   `json:"backdropUrl"`
	Id                 string   `json:"id"`
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

func RequestHandler(tag string) ServiceRequest {
	return ServiceRequest{
		ServiceTag: tag,
	}
}
