package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// Error Response
type Error struct {
	// Error code
	Code int

	// Error message
	Message string
}

func writeError(w http.ResponseWriter, message string, code int) {

	resp := Error{
		Code:    code,
		Message: message,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	log.Printf("Error %v: %v\n", code, message)

	json.NewEncoder(w).Encode(resp)
}

var (
	BadRequestErrorHandler = func(w http.ResponseWriter, err error) {
		writeError(w, err.Error(), http.StatusBadRequest)
	}
	InternalErrorHandler = func(w http.ResponseWriter, err error) {
		writeError(w, err.Error(), http.StatusInternalServerError) // "an unexpected error occurred"
	}
	UnauthorizedErrorHandler = func(w http.ResponseWriter, err error) {
		writeError(w, err.Error(), http.StatusUnauthorized)
	}
	ForbiddenErrorHandler = func(w http.ResponseWriter, err error) {
		writeError(w, err.Error(), http.StatusForbidden)
	}
	NotFoundErrorHandler = func(w http.ResponseWriter, err error) {
		writeError(w, err.Error(), http.StatusNotFound)
	}
)
