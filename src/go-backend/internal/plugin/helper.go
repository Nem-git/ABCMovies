package plugin

import (
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"plugin"
	"strings"

	"github.com/nem-git/abcmovies/internal/config"
)

func Load() ([]Plugin, error) {

	var plugins []Plugin

	availablePlugins := OpenPlugins()

	for _, p := range availablePlugins {
		pluginInstance, err := GetInterface("Plugin", p)
		if err == nil {
			plugins = append(plugins, pluginInstance)
		} else {
			log.Println("error getting instance of plugin:", p, err)
			return nil, err
		}
	}

	return plugins, nil
}

func GetByID(name string, plugins []Plugin) (Plugin, error) {

	for _, p := range plugins {
		if strings.EqualFold(name, (p).GetServiceID()) {
			return p, nil
		}
	}

	return nil, fmt.Errorf("no plugin found matching the id")
}

func GetInterface(name string, p plugin.Plugin) (Plugin, error) {

	symbol, err := p.Lookup(name)
	if err != nil {
		return nil, fmt.Errorf("couldn't find method %v in plugin", name)
	}

	pluginInstance, ok := symbol.(Plugin)
	if !ok {
		return nil, fmt.Errorf("couldn't run function: %v", name)
	}

	return pluginInstance, nil
}

func OpenPlugin(path string) (*plugin.Plugin, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("couldn't open plugin: %v", path)
	}

	return p, nil
}

func OpenPlugins() []plugin.Plugin {

	var paths []string

	filepath.WalkDir(config.PLUGINS_PATH, func(s string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			log.Println("found plugin file:", s)
			paths = append(paths, s)
		}
		return nil
	})

	var plugins []plugin.Plugin

	for _, path := range paths {
		p, err := OpenPlugin(path)
		if err == nil {
			log.Println("loaded plugin:", path)
			plugins = append(plugins, *p)
		} else {
			log.Println("error loading plugin:", path)
		}
	}

	return plugins
}
