package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"chameth.com/goplum"
	"github.com/csmith/envflag/v2"
	"github.com/csmith/slogflags"
)

var (
	configPath    = flag.String("config", "goplum.conf", "Path to the config file")
	tombStonePath = flag.String("tombstone", "/tmp/goplum.tomb", "Path to save tombstones to persist data across restarts")
)

var plugins = map[string]goplum.Plugin{}

func main() {
	envflag.Parse()
	slogflags.Logger(slogflags.WithSetDefault(true))

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	if err := goplum.Run(plugins, *configPath, *tombStonePath, signals); err != nil {
		slog.Error("goplum failed", "error", err)
		os.Exit(1)
	}
}
