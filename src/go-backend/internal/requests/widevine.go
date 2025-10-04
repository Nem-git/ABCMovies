package requests

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/utils"
)

type WidevineKeysRequest struct {
	PSSH    string            `json:"pssh"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
}

func (r *WidevineKeysRequest) Map(req *http.Request) error {
	if err := utils.BindJSON(req, r); err != nil {
		return err
	}

	if err := r.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *WidevineKeysRequest) Validate() error {
	if r.PSSH == "" {
		return ErrInvalidWidevinePSSH
	}

	if r.URL == "" {
		return ErrEmptyWidevineLicenseURL
	}

	return nil
}

type WidevinePSSHRequest struct {
	URL        string            `json:"url"`
	Headers    map[string]string `json:"headers"`
	SegHeaders map[string]string `json:"segHeaders"`
}

func (r *WidevinePSSHRequest) Map(req *http.Request) error {
	if err := utils.BindJSON(req, r); err != nil {
		return err
	}

	if err := r.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *WidevinePSSHRequest) Validate() error {
	if r.URL == "" {
		return ErrEmptyWidevinePlaylistURL
	}

	return nil
}

type WidevineSegmentRequest struct {
	InitStr    string   `json:"init"`
	SegmentStr string   `json:"segment"`
	Keys       []string `json:"keys"`
}

func (r *WidevineSegmentRequest) Map(req *http.Request) error {
	if err := utils.BindJSON(req, r); err != nil {
		return err
	}

	if err := r.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *WidevineSegmentRequest) Validate() error {
	if len(r.Keys) == 0 {
		return ErrEmptyKeysWidevine
	}

	return nil
}
