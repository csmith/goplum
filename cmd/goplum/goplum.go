package main

import (
	"flag"

	"github.com/csmith/goplum"
	"github.com/kouhin/envflag"
)

var (
	configPath = flag.String("config", "goplum.conf", "Path to the config file")
)

var plugins = map[string]goplum.PluginLoader{}

func main() {
	if err := envflag.Parse(); err != nil {
		panic(err)
	}

	goplum.Run(plugins, *configPath)
}
