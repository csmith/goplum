//go:build !nodiscord

package main

import "chameth.com/goplum/plugins/discord"

func init() {
	plugins["discord"] = discord.Plugin{}
}
