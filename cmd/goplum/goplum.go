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
	quiet         = flag.Bool("quiet", false, "Deprecated: use --log.level warn instead")
)

var plugins = map[string]goplum.Plugin{}

func main() {
	envflag.Parse()
	var logOpts []slogflags.Option
	if *quiet {
		logOpts = append(logOpts, slogflags.WithDefaultLogLevel(slog.LevelWarn))
	}
	slogflags.Logger(append(logOpts, slogflags.WithSetDefault(true))...)
	if *quiet {
		slog.Warn("The quiet flag is deprecated and will be removed in a future release, use --log.level warn instead")
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	if err := goplum.Run(plugins, *configPath, *tombStonePath, signals); err != nil {
		slog.Error("goplum failed", "error", err)
		os.Exit(1)
	}
}
