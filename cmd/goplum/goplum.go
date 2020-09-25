package main

import (
	"flag"
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/internal"
	"github.com/kouhin/envflag"
	"log"
)

var (
	pluginsPattern = flag.String("plugins", "plugins/**.so", "Glob pattern used to locate plugins")
	configPath = flag.String("config", "goplum.conf", "Path to the config file")
)

func main() {
	if err := envflag.Parse(); err != nil {
		panic(err)
	}

	plugins, err := internal.FindPlugins(*pluginsPattern)
	if err != nil {
		panic(err)
	}

	log.Printf("Found %d plugins\n", len(plugins))

	goplum.Run(plugins, *configPath)
}
