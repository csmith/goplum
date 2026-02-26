//go:build !nonetwork

package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/network"
)

func init() {
	plugins["network"] = func() (goplum.Plugin, error) {
		return network.Plugin{}, nil
	}
}
