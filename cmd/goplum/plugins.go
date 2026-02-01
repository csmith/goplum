package main

import (
	"fmt"
	"path"
	"plugin"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/csmith/goplum"
)

func FindPlugins(pattern string) (map[string]goplum.PluginLoader, error) {
	matches, err := doublestar.FilepathGlob(pattern)
	if err != nil {
		return nil, err
	}

	plugins := make(map[string]goplum.PluginLoader)
	for i := range matches {
		name := path.Base(matches[i])
		if strings.HasSuffix(name, ".so") {
			location := matches[i]
			plugins[strings.TrimSuffix(name, ".so")] = func() (goplum.Plugin, error) {
				return loadPlugin(location)
			}
		}
	}

	return plugins, nil
}

func loadPlugin(location string) (goplum.Plugin, error) {
	p, err := plugin.Open(location)
	if err != nil {
		return nil, fmt.Errorf("unable to open plugin at %s: %v", location, err)
	}

	f, err := p.Lookup("Plum")
	if err != nil {
		return nil, fmt.Errorf("plugin at %s doesn't export Plum() func: %v", location, err)
	}

	provider, valid := f.(func() goplum.Plugin)
	if !valid {
		return nil, fmt.Errorf("plugin at %s has Plum() func with incorrect return type: %v", location, err)
	}

	return provider(), nil
}
