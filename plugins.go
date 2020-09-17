package goplum

import (
	"encoding/json"
	"fmt"
	"time"
)

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
	// Name returns a name for this type of check, which must be unique within the plugin.
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

// CheckState describes the state of a check.
type CheckState int

const (
	// StateIndeterminate indicates that it's not clear if the check passed or failed, e.g. it hasn't run yet.
	StateIndeterminate CheckState = iota
	// StateGood indicates the service is operating correctly.
	StateGood
	// StateFailing indicates a problem with the service.
	StateFailing
)

// String returns an english, lowercase name for the state.
func (c CheckState) String() string {
	switch c {
	case StateIndeterminate:
		return "indeterminate"
	case StateFailing:
		return "failing"
	case StateGood:
		return "good"
	default:
		return "unknown"
	}
}

func (c CheckState) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.String())
}

// Result contains information about a check that was performed.
type Result struct {
	// State gives the current state of the service.
	State CheckState `json:"state"`
	// Time is the time the check was performed.
	Time time.Time `json:"time"`
	// Detail is an short, optional explanation of the current state.
	Detail string `json:"detail,omitempty"`
}

// GoodResult creates a new result indicating the service is in a good state.
func GoodResult() Result {
	return Result{
		State: StateGood,
		Time:  time.Now(),
	}
}

// FailingResult creates a new result indicating the service is in a bad state.
func FailingResult(format string, a ...interface{}) Result {
	return Result{
		State:  StateFailing,
		Time:   time.Now(),
		Detail: fmt.Sprintf(format, a...),
	}
}

// AlertType is one way of notifying people when a service goes down or returns, e.g.
// posting a webhook, sending a message with Twilio
type AlertType interface {
	// Name returns a name for this type of alert, which must be unique within the plugin.
	Name() string
	// Create instantiates a new alert of this type, with the provided configuration.
	Create(config json.RawMessage) (Alert, error)
}

// AlertDetails contains information about a triggered alert
type AlertDetails struct {
	// Text is a short, pre-generated message describing the alert.
	Text string `json:"text"`
	// Name is the name of the check that transitioned.
	Name string `json:"name"`
	// Type is the type of check involved.
	Type string `json:"type"`
	// Config is the user-supplied parameters to the check.
	Config interface{} `json:"config"`
	// LastResult is the most recent result that caused the transition.
	LastResult *Result `json:"last_result"`
	// PreviousState is the state this check was previously in.
	PreviousState CheckState `json:"previous_state"`
	// NewState is the state this check is now in.
	NewState CheckState `json:"new_state"`
}

// Alert defines the method to inform the user of a change to a service - e.g. when it comes up or goes down.
type Alert interface {
	// Send dispatches an alert in relation to the given check event.
	Send(details AlertDetails) error
}
