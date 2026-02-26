//go:build !nonetwork

package main

import (
	"chameth.com/goplum"
	"chameth.com/goplum/plugins/network"
)

func init() {
	plugins["network"] = func() (goplum.Plugin, error) {
		return network.Plugin{}, nil
	}
}
