//go:build !nopushover

package main

import "chameth.com/goplum/plugins/pushover"

func init() {
	plugins["pushover"] = pushover.Plugin{}
}
