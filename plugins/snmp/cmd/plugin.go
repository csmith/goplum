package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/snmp"
)

func Plum() goplum.Plugin {
	return snmp.Plugin{}
}
