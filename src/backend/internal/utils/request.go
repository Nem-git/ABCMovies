package utils

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

func Get(u string, params map[string]any, headers map[string]string) (io.ReadCloser, error) {

	parsed, err := url.Parse(u)
	if err != nil {
		return nil, errors.New("couldn't parse url")
	}

	p := FormatParams(u, params)

	if parsed.RawQuery != "" {
		if p != "" {
			u += "&" + p
		}
	} else if p != "" {
		u += "?" + p
	}

	body, err := Request(u, headers, nil, http.MethodGet)
	if err != nil {
		return nil, errors.New("couldn't make the request")
	}

	return body, nil
}

func Post(u string, headers map[string]string, data []byte) (io.ReadCloser, error) {

	body, err := Request(u, headers, data, http.MethodPost)
	if err != nil {
		return nil, errors.New("couldn't make the request")
	}

	return body, nil
}

func Request(u string, headers map[string]string, data []byte, method string) (io.ReadCloser, error) {

	// Send request to get Segment content
	req, err := http.NewRequest(method, u, nil)
	if err != nil {
		return nil, errors.New("couldn't create request")
	}

	if method != http.MethodGet {
		req.Body = io.NopCloser(bytes.NewReader(data))
	}

	for key, value := range headers {
		if key != "" {
			req.Header.Set(key, value)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.New("request failed")
	}

	return resp.Body, nil
}

func FormatParams(u string, params map[string]any) string {

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return ""
	}

	if req.URL == nil {
		return ""
	}

	query := req.URL.Query()

	for k, v := range params {
		query.Add(k, fmt.Sprintf("%v", v))
	}

	return query.Encode()
}
