package utils

import "net/http"

func ByteResponse(w http.ResponseWriter, r []byte, ct string) {
	w.Header().Set("Content-Type", ct)
	w.Write(r)
}
