package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/network"
)

func Plum() goplum.Plugin {
	return network.Plugin{}
}
