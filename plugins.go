package goplum

import "encoding/json"

// Plugin is the API between plugins and the core. Plugins must provide an exported "Plum()" method in the
// main package which returns an instance of Plugin. The Plugin in turn then provides its name and the
// check and alerts it makes available.
type Plugin interface {
	Name() string
	Checks() []CheckType
	Alerts() []AlertType
}

// CheckType is one type of check that may be performed to determine the status of a service
// e.g. making a HTTP request, or opening a TCP socket.
type CheckType interface {
	// Name returns a globally unique name for this type of check.
	Name() string
	// Create instantiates a new check of this type, with the provided configuration.
	Create(config json.RawMessage) (Check, error)
}

// Check defines the method to see if a service is up or not. The check is persistent - its
// Execute method will be called repeatedly over the lifetime of the application.
type Check interface {
	// Execute performs the actual check to see if the service is up or not.
	// It should block until a result is available.
	Execute() Result
}

// Result contains information about a check that was performed.
type Result struct {
	// Good indicates that the service is in a good state (or not).
	Good bool
}

// AlertType is one way of notifying people when a service goes down or returns, e.g.
// posting a webhook, sending a message with Twilio
type AlertType interface {
	// Name returns a globally unique name for this type of alert.
	Name() string
	// Create instantiates a new alert of this type, with the provided configuration.
	Create(config json.RawMessage) (Alert, error)
}

// Alert defines the method to inform the user of a change to a service - e.g. when it comes up or goes down.
type Alert interface {
	// Send dispatches an alert in relation to the given check
	Send(check *ScheduledCheck) error
}
