//go:build !noexec

package main

import "chameth.com/goplum/plugins/exec"

func init() {
	plugins["exec"] = exec.Plugin{}
}
