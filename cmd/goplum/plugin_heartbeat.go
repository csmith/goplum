//go:build !noheartbeat

package main

import (
	"chameth.com/goplum"
	"chameth.com/goplum/plugins/heartbeat"
)

func init() {
	plugins["heartbeat"] = func() (goplum.Plugin, error) {
		return &heartbeat.Plugin{}, nil
	}
}
