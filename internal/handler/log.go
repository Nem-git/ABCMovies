package handler

import (
	"log"

	"github.com/ogen-go/ogen/middleware"
)

// LogErrors logs every error returned by an API handler before it is encoded
// into the HTTP response. It is wired via oas.WithMiddleware.
func LogErrors(req middleware.Request, next middleware.Next) (middleware.Response, error) {
	resp, err := next(req)
	if err != nil {
		op := req.OperationID
		if op == "" {
			op = req.OperationName
		}
		path := ""
		if req.Raw != nil {
			path = req.Raw.URL.Path
		}
		log.Printf("api error: op=%s path=%s: %v", op, path, err)
	}
	return resp, err
}
