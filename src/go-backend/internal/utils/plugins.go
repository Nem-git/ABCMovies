package utils

import (
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"plugin"

	"github.com/nem-git/abcmovies/internal/config"
	iPlugin "github.com/nem-git/abcmovies/internal/plugin"
)

func GetPluginInterface(name string, p *plugin.Plugin) (*iPlugin.PluginInterface, error) {

	symbol, err := p.Lookup(name)
	if err != nil {
		return nil, fmt.Errorf("couldn't find method %v in plugin", name)
	}

	pluginInstance, ok := symbol.(iPlugin.PluginInterface)
	if !ok {
		return nil, fmt.Errorf("couldn't run function: %v", name)
	}

	return &pluginInstance, nil
}

func OpenPlugin(path string) (*plugin.Plugin, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("couldn't open plugin: %v", path)
	}

	return p, nil
}

func OpenPlugins() []*plugin.Plugin {

	var paths []string

	filepath.WalkDir(config.PLUGINS_PATH, func(s string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, s)
		}
		return nil
	})

	var plugins []*plugin.Plugin

	for _, path := range paths {
		p, err := OpenPlugin(path)
		if err == nil {
			log.Println("loaded plugin:", path)
			plugins = append(plugins, p)
		}
	}

	return plugins
}
