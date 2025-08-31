package models

type WidevineKeysRequest struct {
	Pssh    string            `json:"pssh"`
	Url     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

type WidevineKeysResponse struct {
	Error string   `json:"error"`
	Keys  []string `json:"keys"`
}

type WidevinePsshRequest struct {
	Url        string            `json:"url"`
	Headers    map[string]string `json:"headers"`
	SegHeaders map[string]string `json:"segheaders"`
}

type WidevinePsshResponse struct {
	Error string `json:"error"`
	Pssh  string `json:"pssh"`
}
