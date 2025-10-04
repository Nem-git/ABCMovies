package middleware

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/api"
	"github.com/nem-git/abcmovies/internal/plugin"
	"github.com/nem-git/abcmovies/internal/requests"
	"github.com/nem-git/abcmovies/internal/utils"
)

func PluginMiddleware(next http.Handler, plugins []*plugin.IPlugin) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Get and validate service tag
		serviceRequest := &requests.ServiceRequest{}
		if err := serviceRequest.Map(r); err != nil {
			api.BadRequestErrorHandler(w, err)
		}

		// Get services' matching plugin
		p, err := plugin.GetByID(serviceRequest.ServiceTag, plugins)
		if err != nil {
			api.BadRequestErrorHandler(w, err)
			return
		}

		// Save plugin in ctx
		r = utils.SetPluginContextValue(r, p)

		next.ServeHTTP(w, r)
	})
}

func PluginsMiddleware(next http.Handler, plugins []*plugin.IPlugin) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = utils.SetPluginsContextValue(r, plugins)

		next.ServeHTTP(w, r)
	})
}
