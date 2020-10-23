package main

import (
	"flag"
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/debug"
	"github.com/csmith/goplum/plugins/exec"
	"github.com/csmith/goplum/plugins/heartbeat"
	"github.com/csmith/goplum/plugins/http"
	"github.com/csmith/goplum/plugins/msteams"
	"github.com/csmith/goplum/plugins/network"
	"github.com/csmith/goplum/plugins/pushover"
	"github.com/csmith/goplum/plugins/slack"
	"github.com/csmith/goplum/plugins/smtp"
	"github.com/csmith/goplum/plugins/twilio"
	"github.com/kouhin/envflag"
)

var (
	configPath = flag.String("config", "goplum.conf", "Path to the config file")
)

var plugins = map[string]goplum.PluginLoader{
	"exec": func() (goplum.Plugin, error) {
		return exec.Plugin{}, nil
	},
	"heartbeat": func() (goplum.Plugin, error) {
		return &heartbeat.Plugin{}, nil
	},
	"http": func() (goplum.Plugin, error) {
		return http.Plugin{}, nil
	},
	"msteams": func() (goplum.Plugin, error) {
		return msteams.Plugin{}, nil
	},
	"network": func() (goplum.Plugin, error) {
		return network.Plugin{}, nil
	},
	"pushover": func() (goplum.Plugin, error) {
		return pushover.Plugin{}, nil
	},
	"slack": func() (goplum.Plugin, error) {
		return slack.Plugin{}, nil
	},
	"smtp": func() (goplum.Plugin, error) {
		return smtp.Plugin{}, nil
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

	goplum.Run(plugins, *configPath)
}
