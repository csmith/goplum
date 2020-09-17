package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/slack"
)

func Plum() goplum.Plugin {
	return slack.Plugin{}
}
