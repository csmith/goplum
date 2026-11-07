//go:build !noslack

package main

import "chameth.com/goplum/plugins/slack"

func init() {
	plugins["slack"] = slack.Plugin{}
}
