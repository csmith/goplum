package internal

import (
	"github.com/csmith/goplum"
	"io/ioutil"
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
	p, err := plugin.Open(location)
	if err != nil {
		return nil
	}

	f, err := p.Lookup("Plum")
	if err != nil {
		return nil
	}

	provider, valid := f.(func() goplum.Plugin)
	if !valid {
		return nil
	}

	return provider()
}
