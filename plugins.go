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
	Create(config json.RawMessage) (Task, error)
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
	Name() string
	Create(config json.RawMessage) (Notification, error)
}

type Notification interface {
	Send(check *ScheduledCheck)
}
