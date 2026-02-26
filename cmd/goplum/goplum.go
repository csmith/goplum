package main

import (
	"flag"

	"chameth.com/goplum"
	"github.com/csmith/envflag/v2"
)

var (
	configPath = flag.String("config", "goplum.conf", "Path to the config file")
)

var plugins = map[string]goplum.PluginLoader{}

func main() {
	envflag.Parse()

	goplum.Run(plugins, *configPath)
}
