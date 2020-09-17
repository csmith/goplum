package main

import (
	"flag"
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/internal"
	"github.com/kouhin/envflag"
	"log"
)

var (
	pluginsDir = flag.String("plugins", "plugins", "Directory to load plugins from")
)

func main() {
	if err := envflag.Parse(); err != nil {
		panic(err)
	}

	plugins := internal.LoadPlugins(*pluginsDir)

	log.Printf("Loaded %d plugins\n", len(plugins))
	for i := range plugins {
		log.Printf("Plugin %d is '%s' with %d checks, %d notifiers\n", i, plugins[i].Name(), len(plugins[i].Checks()), len(plugins[i].Alerts()))
	}

	plum := &goplum.Plum{}
	plum.AddPlugins(plugins)
	plum.LoadConfig("config.json")
	plum.Run()
}
