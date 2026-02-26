//go:build !nodiscord

package main

import (
	"chameth.com/goplum"
	"chameth.com/goplum/plugins/discord"
)

func init() {
	plugins["discord"] = func() (goplum.Plugin, error) {
		return discord.Plugin{}, nil
	}
}
