//go:build !nohttp

package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/http"
)

func init() {
	plugins["http"] = func() (goplum.Plugin, error) {
		return http.Plugin{}, nil
	}
}
