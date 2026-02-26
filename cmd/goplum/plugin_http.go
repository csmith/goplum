//go:build !nohttp

package main

import (
	"chameth.com/goplum"
	"chameth.com/goplum/plugins/http"
)

func init() {
	plugins["http"] = func() (goplum.Plugin, error) {
		return http.Plugin{}, nil
	}
}
