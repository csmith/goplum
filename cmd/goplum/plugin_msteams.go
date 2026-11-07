//go:build !nomsteams

package main

import "chameth.com/goplum/plugins/msteams"

func init() {
	plugins["msteams"] = msteams.Plugin{}
}
