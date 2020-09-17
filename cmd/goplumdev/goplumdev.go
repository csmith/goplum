package main

import (
	"flag"
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/debug"
	"github.com/csmith/goplum/plugins/http"
	"github.com/csmith/goplum/plugins/slack"
	"github.com/csmith/goplum/plugins/twilio"
	"github.com/kouhin/envflag"
	"log"
)

var (
	configPath = flag.String("config", "config.json", "Path to the config file")
)

func main() {
	if err := envflag.Parse(); err != nil {
		panic(err)
	}

	plugins := []goplum.Plugin{
		http.Plugin{},
		slack.Plugin{},
		twilio.Plugin{},
		debug.Plugin{},
	}

	log.Printf("Loaded %d plugins\n", len(plugins))
	for i := range plugins {
		log.Printf("Plugin %d is '%s' with %d check types, %d alert types\n", i, plugins[i].Name(), len(plugins[i].Checks()), len(plugins[i].Alerts()))
	}

	plum := &goplum.Plum{}
	plum.AddPlugins(plugins)
	plum.LoadConfig(*configPath)
	plum.Run()
}
