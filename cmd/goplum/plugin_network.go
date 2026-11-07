//go:build !nonetwork

package main

import "chameth.com/goplum/plugins/network"

func init() {
	plugins["network"] = network.Plugin{}
}
