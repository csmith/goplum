//go:build !nodebug

package main

import (
	"chameth.com/goplum"
	"chameth.com/goplum/plugins/debug"
)

func init() {
	plugins["debug"] = func() (goplum.Plugin, error) {
		return debug.Plugin{}, nil
	}
}
