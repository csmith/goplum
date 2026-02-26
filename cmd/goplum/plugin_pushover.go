//go:build !nopushover

package main

import (
	"chameth.com/goplum"
	"chameth.com/goplum/plugins/pushover"
)

func init() {
	plugins["pushover"] = func() (goplum.Plugin, error) {
		return pushover.Plugin{}, nil
	}
}
