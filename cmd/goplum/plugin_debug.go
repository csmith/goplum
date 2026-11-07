//go:build !nodebug

package main

import "chameth.com/goplum/plugins/debug"

func init() {
	plugins["debug"] = debug.Plugin{}
}
