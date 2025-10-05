package models

// Widevine Keys Response
type WidevineKeys struct {
	// Widevine Keys
	Keys []string `json:"keys"`
}

// Widevine Pssh Response
type WidevinePSSH struct {
	// Widevine Pssh
	PSSH string `json:"pssh"`
}

// Widevine Segment Response
type WidevineSegment struct {
	// Decrypted Segment
	Segment string `json:"segment"`
}
