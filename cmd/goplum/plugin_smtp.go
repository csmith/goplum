//go:build !nosmtp

package main

import "chameth.com/goplum/plugins/smtp"

func init() {
	plugins["smtp"] = smtp.Plugin{}
}
