package http

import "github.com/csmith/goplum"

type Plugin struct{}

func (h Plugin) Name() string {
	return "http"
}

func (h Plugin) Checks() []goplum.Check {
	return nil
}

func (h Plugin) Notifiers() []goplum.Notifier {
	return nil
}

