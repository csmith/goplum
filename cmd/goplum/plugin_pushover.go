//go:build !nopushover

package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/pushover"
)

func init() {
	plugins["pushover"] = func() (goplum.Plugin, error) {
		return pushover.Plugin{}, nil
	}
}
