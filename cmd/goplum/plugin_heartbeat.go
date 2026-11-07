//go:build !noheartbeat

package main

import "chameth.com/goplum/plugins/heartbeat"

func init() {
	plugins["heartbeat"] = &heartbeat.Plugin{}
}
