package models

type Request any

type Response any

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}
