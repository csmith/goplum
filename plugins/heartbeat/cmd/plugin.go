package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/heartbeat"
)

func Plum() goplum.Plugin {
	return &heartbeat.Plugin{}
}
