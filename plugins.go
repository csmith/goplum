package goplum

import "encoding/json"

type Plugin interface {
	Name() string
	Checks() []Check
	Notifiers() []Notifier
}

// Check is one type of check that may be performed to determine the status of a service
// e.g. making a HTTP request, or opening a TCP socket.
type Check interface {
	Name() string
	Help() string
	Create(config json.RawMessage) Task
}

type Task interface {
	Execute() Result
}

type Result struct {
	Good bool
}

// Notifier is one way of notifying people when a service goes down or returns, e.g.
// posting a webhook, sending a message with Twilio
type Notifier interface {
	// TODO: Fill this in - similar to the Check interface
}

func NewPlugin(name string, checks []Check, notifiers []Notifier) Plugin {
	return SimplePlugin{name, checks, notifiers}
}

type SimplePlugin struct {
	name      string
	checks    []Check
	notifiers []Notifier
}

func (s SimplePlugin) Name() string {
	return s.name
}

func (s SimplePlugin) Checks() []Check {
	return s.checks
}

func (s SimplePlugin) Notifiers() []Notifier {
	return s.notifiers
}
