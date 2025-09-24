package api

// Dash Manifest Request
type DashManifestRequest struct {
	// Manifest URL
	URL string `json:"url"`

	// Manifest Content
	Content string `json:"content"`
}

// Dash Manifest Response
type DashManifestResponse struct {
	// Modified Manifest Content
	Content string `json:"content"`
}
