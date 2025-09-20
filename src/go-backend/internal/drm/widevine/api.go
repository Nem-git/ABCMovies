package widevine

// Widevine Keys Request
type WidevineKeysRequest struct {
	// PSSH Data
	Pssh string `json:"pssh"`

	// License Server URL
	URL string `json:"url"`

	// License Request Headers
	Headers map[string]string `json:"headers"`
}

// Widevine Keys Response
type WidevineKeysResponse struct {
	// Widevine Keys
	Keys []string `json:"keys"`
}

// Widevine Pssh Request
type WidevinePsshRequest struct {
	// URL
	URL string `json:"url"`

	// Headers
	Headers map[string]string `json:"headers"`

	// Segment Headers
	SegHeaders map[string]string `json:"segHeaders"`
}

// Widevine Pssh Response
type WidevinePsshResponse struct {
	// Widevine Pssh
	Pssh string `json:"pssh"`
}

// Widevine Segment Request
type WidevineSegmentRequest struct {
	// Init Data
	InitStr string `json:"init"`

	// Segment Data
	SegmentStr string `json:"segment"`

	// Decryption Keys
	Keys []string `json:"keys"`
}

// Widevine Segment Response
type WidevineSegmentResponse struct {
	// Decrypted Segment
	Segment string `json:"segment"`
}
