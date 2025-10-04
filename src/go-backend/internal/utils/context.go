package utils

import (
	"context"
	"net/http"

	"github.com/nem-git/abcmovies/internal/config"
)

func SetRequestContextValue(r *http.Request, v any) *http.Request {
	return SetCustomRequestContextValue(r, config.CONTEXT_REQUEST_KEY, v)
}

func SetPluginContextValue(r *http.Request, v any) *http.Request {
	return SetCustomRequestContextValue(r, config.CONTEXT_PLUGIN_KEY, v)
}

func SetPluginsContextValue(r *http.Request, v any) *http.Request {
	return SetCustomRequestContextValue(r, config.CONTEXT_PLUGINS_KEY, v)
}

func SetCustomRequestContextValue(r *http.Request, k any, v any) *http.Request {
	ctx := context.WithValue(r.Context(), k, v)
	return r.WithContext(ctx)
}

func GetRequestContextValue[T any](r *http.Request) (*T, error) {
	return GetCustomRequestContextValue[T](r, config.CONTEXT_REQUEST_KEY)
}

func GetPluginContextValue[T any](r *http.Request) (*T, error) {
	return GetCustomRequestContextValue[T](r, config.CONTEXT_PLUGIN_KEY)
}

func GetPluginsContextValue[T any](r *http.Request) (*T, error) {
	return GetCustomRequestContextValue[T](r, config.CONTEXT_PLUGINS_KEY)
}

func GetCustomRequestContextValue[T any](r *http.Request, k any) (*T, error) {
	v := r.Context().Value(k)
	if v == nil {
		return nil, ErrContextNotContainsKey
	}

	var result *T

	// If value is not pointer
	vRes, ok := v.(T)
	if ok {
		result = &vRes
	} else {
		// If value is a pointer
		result, ok = v.(*T)
		if !ok {
			return nil, ErrContextValueCouldNotBeCasted
		}
	}

	return result, nil
}
