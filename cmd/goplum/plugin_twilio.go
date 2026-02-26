//go:build !notwilio

package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/twilio"
)

func init() {
	plugins["twilio"] = func() (goplum.Plugin, error) {
		return twilio.Plugin{}, nil
	}
}
