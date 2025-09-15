package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// Streaming Service Tag
type StreamingServiceSlug struct {
	StreamingServiceTag string
}

// Streaming Service Response
type StreamingServiceResponse struct {
	// Success code
	Code int

	// Streaming Service
	StreamingService
}

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

	json.NewEncoder(w).Encode(resp)
}

var (
	RequestErrorHandler = func(w http.ResponseWriter, err error) {
		writeError(w, err.Error(), http.StatusBadRequest)
	}
	InternalErrorHandler = func(w http.ResponseWriter, err error) {
		log.Println("Error: %v", err)
		writeError(w, "an unexpected error occurred", http.StatusInternalServerError)
	}
)
