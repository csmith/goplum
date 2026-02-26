//go:build !nodebug

package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/debug"
)

func init() {
	plugins["debug"] = func() (goplum.Plugin, error) {
		return debug.Plugin{}, nil
	}
}
