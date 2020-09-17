package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/http"
	"github.com/csmith/goplum/plugins/twilio"
	"log"
)

func main() {
	plugins := []goplum.Plugin{
		http.Plugin{},
		twilio.Plugin{},
	}

	log.Printf("Loaded %d plugins\n", len(plugins))
	for i := range plugins {
		log.Printf("Plugin %d is '%s' with %d checks, %d notifiers\n", i, plugins[i].Name(), len(plugins[i].Checks()), len(plugins[i].Alerts()))
	}

	plum := &goplum.Plum{}
	plum.AddPlugins(plugins)
	plum.LoadConfig("config.json")
	plum.Run()
}
