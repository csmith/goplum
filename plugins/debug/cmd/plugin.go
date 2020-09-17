package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/debug"
)

func Plum() goplum.Plugin {
	return debug.Plugin{}
}
