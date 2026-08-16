package drm

import (
	"bytes"
	"encoding/binary"
	"net/http"

	"github.com/Eyevinn/mp4ff/mp4"
)

// buildPSSHBox wraps a protection-system payload in a standard v1 PSSH box:
// size (4) | "pssh" (4) | version/flags (4) | system ID (16) | data size (4) | data.
func buildPSSHBox(systemID mp4.UUID, payload []byte) []byte {
	const headerLen = 32
	size := headerLen + len(payload)
	buf := make([]byte, size)
	binary.BigEndian.PutUint32(buf[0:4], uint32(size))
	copy(buf[4:8], []byte("pssh"))
	buf[8] = 1 // version 1
	copy(buf[12:28], systemID)
	binary.BigEndian.PutUint32(buf[28:32], uint32(len(payload)))
	copy(buf[32:], payload)
	return buf
}

// equalBytes compares two byte slices.
func equalBytes(a, b []byte) bool {
	return bytes.Equal(a, b)
}

// httpHeaders merges request headers over configured headers.
func httpHeaders(headers http.Header) http.Header {
	out := http.Header{}
	for k, vs := range headers {
		for _, v := range vs {
			out.Add(k, v)
		}
	}
	return out
}

// headersFromMap converts a string map into an http.Header.
func headersFromMap(headers http.Header, cfg map[string]string) http.Header {
	out := http.Header{}
	if headers != nil {
		for k, vs := range headers {
			for _, v := range vs {
				out.Add(k, v)
			}
		}
	}
	for k, v := range cfg {
		if out.Get(k) == "" {
			out.Set(k, v)
		}
	}
	return out
}
