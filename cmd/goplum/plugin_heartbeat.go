//go:build !noheartbeat

package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/heartbeat"
)

func init() {
	plugins["heartbeat"] = func() (goplum.Plugin, error) {
		return &heartbeat.Plugin{}, nil
	}
}
