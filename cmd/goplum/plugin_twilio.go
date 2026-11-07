//go:build !notwilio

package main

import "chameth.com/goplum/plugins/twilio"

func init() {
	plugins["twilio"] = twilio.Plugin{}
}
