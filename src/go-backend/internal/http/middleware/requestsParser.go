package middleware

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/http/api"
	"github.com/nem-git/abcmovies/internal/http/handler"
	"github.com/nem-git/abcmovies/internal/utils"
)

func RequestsParsingMiddleware(next handler.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := next.MapRequest(r); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		r = utils.SetRequestContextValue(r, next)

		next.ServeHTTP(w, r)
	})
}
