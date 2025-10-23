package handler

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/plugin"
)

type Handler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	MapRequest(req *http.Request) error
}

type PluginHandler interface {
	GetPlugin() (*plugin.Plugin, error)

	Handler
}
