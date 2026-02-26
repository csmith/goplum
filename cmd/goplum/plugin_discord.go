//go:build !nodiscord

package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/discord"
)

func init() {
	plugins["discord"] = func() (goplum.Plugin, error) {
		return discord.Plugin{}, nil
	}
}
