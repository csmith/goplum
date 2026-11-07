//go:build !nosnmp

package main

import "chameth.com/goplum/plugins/snmp"

func init() {
	plugins["snmp"] = snmp.Plugin{}
}
