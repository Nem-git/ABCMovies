package requests

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/errs"
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
		return errs.ErrEmptyWidevinePSSH
	}

	if r.URL == "" {
		return errs.ErrEmptyWidevineLicenseURL
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
		return errs.ErrEmptyWidevinePlaylistURL
	}

	return nil
}

type WidevineSegmentRequest struct {
	InitStr    string   `json:"init"`
	SegmentStr string   `json:"segment"`
	Keys       []string `json:"keys"`
	WantInit   bool     `json:"wantInit"`
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
	if r.Keys == nil {
		return errs.ErrEmptyWidevineKeys
	}

	if len(r.Keys) == 0 {
		return errs.ErrEmptyWidevineKeys
	}

	return nil
}
