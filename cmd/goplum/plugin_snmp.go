//go:build !nosnmp

package main

import (
	"chameth.com/goplum"
	"chameth.com/goplum/plugins/snmp"
)

func init() {
	plugins["snmp"] = func() (goplum.Plugin, error) {
		return snmp.Plugin{}, nil
	}
}
