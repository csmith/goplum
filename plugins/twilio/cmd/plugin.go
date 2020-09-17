package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/twilio"
)

func Plum() goplum.Plugin {
	return twilio.Plugin{}
}
