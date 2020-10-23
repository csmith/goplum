package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/smtp"
)

func Plum() goplum.Plugin {
	return smtp.Plugin{}
}
