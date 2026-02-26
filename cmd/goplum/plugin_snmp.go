//go:build !nosnmp

package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/snmp"
)

func init() {
	plugins["snmp"] = func() (goplum.Plugin, error) {
		return snmp.Plugin{}, nil
	}
}
