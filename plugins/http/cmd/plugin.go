package main

import (
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/http"
)

func Plum() goplum.Plugin {
	return http.Plugin{}
}
