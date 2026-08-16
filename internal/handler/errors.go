package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/ogen-go/ogen/ogenerrors"
)

func ErrorHandler(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("api error: %s %s: %v", r.Method, r.URL.Path, err)
	code := ogenerrors.ErrorCode(err)

	errorCode := "INTERNAL_ERROR"
	switch code {
	case http.StatusBadRequest:
		errorCode = "BAD_REQUEST"
	case http.StatusNotFound:
		errorCode = "NOT_FOUND"
	case http.StatusNotImplemented:
		errorCode = "NOT_IMPLEMENTED"
	case http.StatusUnsupportedMediaType:
		errorCode = "UNSUPPORTED_MEDIA_TYPE"
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(oas.Error{Code: errorCode, Message: err.Error()})
}
