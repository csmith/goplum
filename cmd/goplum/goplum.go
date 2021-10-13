package main

import (
	"flag"
	"log"

	"github.com/csmith/goplum"
	"github.com/kouhin/envflag"
)

var (
	pluginsPattern = flag.String("plugins", "plugins/**.so", "Glob pattern used to locate plugins")
	configPath     = flag.String("config", "goplum.conf", "Path to the config file")
)

func main() {
	if err := envflag.Parse(); err != nil {
		panic(err)
	}

	plugins, err := FindPlugins(*pluginsPattern)
	if err != nil {
		panic(err)
	}

	log.Printf("Found %d plugins\n", len(plugins))

	goplum.Run(plugins, *configPath)
}
