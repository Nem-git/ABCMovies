package dash

// Dash Manifest Request
type DashManifestRequest struct {
	// Manifest URL
	Url string

	// Manifest Content
	Content string
}

// Dash Manifest Response
type DashManifestResponse struct {
	// Modified Manifest Content
	Content string
}
