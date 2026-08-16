package middleware

import (
	"log"
	"net/http"
)

// statusRecorder captures the HTTP status code written by the wrapped handler.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// LogStatus logs any API response with a 4xx/5xx status code. This catches
// router-level responses such as 404/405 that never reach an ogen handler or
// the ogen middleware.
func LogStatus(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status >= 400 {
			log.Printf("api response error: %s %s -> %d", r.Method, r.URL.Path, rec.status)
		}
	})
}
