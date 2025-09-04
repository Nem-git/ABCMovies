package models

type DashManifestModificationRequest struct {
	Url     string `json:"url"`
	Content string `json:"content"`
}

type DashManifestModificationResponse struct {
	Manifest string `json:"content"`
}
