//go:build !nosmtp

package main

import (
	"chameth.com/goplum"
	"chameth.com/goplum/plugins/smtp"
)

func init() {
	plugins["smtp"] = func() (goplum.Plugin, error) {
		return smtp.Plugin{}, nil
	}
}
