package handler

import (
	"net/http"

	"github.com/nem-git/abcmovies/internal/plugin"
)

type Handler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
	MapRequest(*http.Request) error
}

type PluginHandler interface {
	GetPlugin() (*plugin.Plugin, error)

	Handler
}
