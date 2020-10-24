package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/discord"
)

func Plum() goplum.Plugin {
	return discord.Plugin{}
}
