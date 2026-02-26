//go:build !notwilio

package main

import (
	"chameth.com/goplum"
	"chameth.com/goplum/plugins/twilio"
)

func init() {
	plugins["twilio"] = func() (goplum.Plugin, error) {
		return twilio.Plugin{}, nil
	}
}
