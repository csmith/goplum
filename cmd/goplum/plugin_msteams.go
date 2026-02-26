//go:build !nomsteams

package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/msteams"
)

func init() {
	plugins["msteams"] = func() (goplum.Plugin, error) {
		return msteams.Plugin{}, nil
	}
}
