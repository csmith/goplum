package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/pushover"
)

func Plum() goplum.Plugin {
	return pushover.Plugin{}
}
