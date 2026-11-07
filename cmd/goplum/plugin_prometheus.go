//go:build !noprometheus

package main

import "chameth.com/goplum/plugins/prometheus"

func init() {
	plugins["prometheus"] = &prometheus.Plugin{}
}
