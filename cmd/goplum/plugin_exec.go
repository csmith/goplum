//go:build !noexec

package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/exec"
)

func init() {
	plugins["exec"] = func() (goplum.Plugin, error) {
		return exec.Plugin{}, nil
	}
}
