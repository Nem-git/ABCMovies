package models

type WidevineKeysRequest struct {
	Headers map[string]string `json:"headers"`
	Pssh    string            `json:"pssh"`
	Url     string            `json:"url"`
}

type WidevineKeysResponse struct {
	Keys []string `json:"keys"`
}

type WidevinePsshRequest struct {
	Headers    map[string]string `json:"headers"`
	SegHeaders map[string]string `json:"segHeaders"`
	Url        string            `json:"url"`
}

type WidevinePsshResponse struct {
	Pssh string `json:"pssh"`
}

type WidevineRemovalRequest struct {
	Init    string   `json:"init"`
	IsInit  bool     `json:"isInit"`
	Keys    []string `json:"keys"`
	Segment string   `json:"segment"`
}

type WidevineRemovalResponse struct {
	Init    string `json:"init"`
	Segment string `json:"segment"`
}
