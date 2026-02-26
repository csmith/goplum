//go:build !noexec

package main

import (
	"chameth.com/goplum"
	"chameth.com/goplum/plugins/exec"
)

func init() {
	plugins["exec"] = func() (goplum.Plugin, error) {
		return exec.Plugin{}, nil
	}
}
