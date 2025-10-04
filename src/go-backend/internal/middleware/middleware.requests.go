package middleware

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/requests"
	"github.com/nem-git/abcmovies/internal/utils"
)

func RequestsParsingMiddleware(next http.Handler, request requests.IRequest) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := request.Map(r); err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		r = utils.SetRequestContextValue(r, request)

		next.ServeHTTP(w, r)
	})
}
