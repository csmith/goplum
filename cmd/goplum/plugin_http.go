//go:build !nohttp

package main

import "chameth.com/goplum/plugins/http"

func init() {
	plugins["http"] = http.Plugin{}
}
