//go:build !nosmtp

package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/smtp"
)

func init() {
	plugins["smtp"] = func() (goplum.Plugin, error) {
		return smtp.Plugin{}, nil
	}
}
