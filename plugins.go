package goplum

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Plugin is the API between plugins and the core. Plugins must provide an exported "Plum()" method in the
// main package which returns an instance of Plugin.
//
// The Check and Alert funcs should provide new instances of the named type, or nil if such a type does not
// exist. Exported fields of the checks and alerts will then be populated according to the user's config, and
// the Validate() func will be called.
type Plugin interface {
	Check(kind string) Check
	Alert(kind string) Alert
}

// Check defines the method to see if a service is up or not. The check is persistent - its
// Execute method will be called repeatedly over the lifetime of the application.
type Check interface {
	// Execute performs the actual check to see if the service is up or not.
	// It should block until a result is available or the passed context is cancelled.
	Execute(ctx context.Context) Result

	// Validate checks the configuration of this check and returns any errors.
	Validate() error
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

	// Validate checks the configuration of this alert and returns any errors.
	Validate() error
}
