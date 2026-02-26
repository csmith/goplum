//go:build !nomsteams

package main

import (
	"chameth.com/goplum"
	"chameth.com/goplum/plugins/msteams"
)

func init() {
	plugins["msteams"] = func() (goplum.Plugin, error) {
		return msteams.Plugin{}, nil
	}
}
