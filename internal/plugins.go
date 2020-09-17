package internal

import (
	"github.com/csmith/goplum"
	"io/ioutil"
	"log"
	"path"
	"plugin"
)

func LoadPlugins(dir string) []goplum.Plugin {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil
	}

	var plugins []goplum.Plugin
	for i := range files {
		location := path.Join(dir, files[i].Name())
		if files[i].IsDir() {
			plugins = append(plugins, LoadPlugins(location)...)
		} else if p := loadPlugin(location); p != nil {
			plugins = append(plugins, p)
		}
	}

	return plugins
}

func loadPlugin(location string) goplum.Plugin {
	log.Printf("Attempting to load plugin from %s\n", location)
	p, err := plugin.Open(location)
	if err != nil {
		log.Printf("Plugin at %s couldn't be opened: %v\n", location, err)
		return nil
	}

	f, err := p.Lookup("Plum")
	if err != nil {
		log.Printf("Plugin at %s doesn't export Plum() func: %v\n", location, err)
		return nil
	}

	provider, valid := f.(func() goplum.Plugin)
	if !valid {
		log.Printf("Plugin at %s has Plum() func with incorrect return type: %#v\n", location, f)
		return nil
	}

	return provider()
}
