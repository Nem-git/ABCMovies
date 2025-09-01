package models

type DashManifestModificationRequest struct {
	Url     string `json:"url"`
	Content string `json:"content"`
}

type DashManifestModificationResponse struct {
	Error    string `json:"error"`
	Manifest string `json:"content"`
}
