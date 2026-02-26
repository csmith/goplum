//go:build !noslack

package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/slack"
)

func init() {
	plugins["slack"] = func() (goplum.Plugin, error) {
		return slack.Plugin{}, nil
	}
}
