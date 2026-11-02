//go:build !noprometheus

package main

import (
	"chameth.com/goplum"
	"chameth.com/goplum/plugins/prometheus"
)

func init() {
	plugins["prometheus"] = func() (goplum.Plugin, error) {
		return &prometheus.Plugin{}, nil
	}
}
