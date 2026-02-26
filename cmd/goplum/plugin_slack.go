//go:build !noslack

package main

import (
	"chameth.com/goplum"
	"chameth.com/goplum/plugins/slack"
)

func init() {
	plugins["slack"] = func() (goplum.Plugin, error) {
		return slack.Plugin{}, nil
	}
}
