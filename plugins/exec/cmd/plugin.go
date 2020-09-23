package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/exec"
)

func Plum() goplum.Plugin {
	return exec.Plugin{}
}
