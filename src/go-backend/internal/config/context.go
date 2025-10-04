package config

type ContextKey string

const (
	CONTEXT_PLUGIN_KEY  ContextKey = "plugin"
	CONTEXT_PLUGINS_KEY ContextKey = "plugins"
	CONTEXT_REQUEST_KEY ContextKey = "request"
)
