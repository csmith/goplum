package main

import (
	"flag"
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/debug"
	"github.com/csmith/goplum/plugins/http"
	"github.com/csmith/goplum/plugins/network"
	"github.com/csmith/goplum/plugins/slack"
	"github.com/csmith/goplum/plugins/twilio"
	"github.com/kouhin/envflag"
	"log"
)

var (
	configPath = flag.String("config", "goplum.conf", "Path to the config file")
)

var plugins = map[string]goplum.PluginLoader{
	"http": func() (goplum.Plugin, error) {
		return http.Plugin{}, nil
	},
	"network": func() (goplum.Plugin, error) {
		return network.Plugin{}, nil
	},
	"slack": func() (goplum.Plugin, error) {
		return slack.Plugin{}, nil
	},
	"twilio": func() (goplum.Plugin, error) {
		return twilio.Plugin{}, nil
	},
	"debug": func() (goplum.Plugin, error) {
		return debug.Plugin{}, nil
	},
}

func main() {
	if err := envflag.Parse(); err != nil {
		panic(err)
	}

	plum := goplum.NewPlum()

	plum.RegisterPlugins(plugins)

	if err := plum.ReadConfig(*configPath); err != nil {
		log.Fatalf("Unable to read config: %v", err)
	}

	plum.Run()
}
