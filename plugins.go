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
//
// Checks may also implement the Validator interface to validate their arguments when configured.
type Check interface {
	// Execute performs the actual check to see if the service is up or not.
	// It should block until a result is available or the passed context is cancelled.
	Execute(ctx context.Context) Result
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

func (c *CheckState) UnmarshalJSON(val []byte) error {
	switch string(val) {
	case "\"indeterminate\"":
		*c = StateIndeterminate
	case "\"failing\"":
		*c = StateFailing
	case "\"good\"":
		*c = StateGood
	default:
		return fmt.Errorf("unknown value for CheckState: %s", val)
	}
	return nil
}

// Fact defines a type of information that may be returned in a Result.
//
// Fact names should consist of the package name that defines them, a '#' character, and
// then a short, human-friendly name for the metric in `snake_case`.
type Fact string

var (
	// ResponseTime denotes the length of time it took for a service to respond to a request.
	// Its value should be a time.Duration.
	ResponseTime Fact = "chameth.com/goplum#response_time"

	// CheckTime indicates how long the entire check took to invoke. Its value should be a time.Duration.
	CheckTime Fact = "chameth.com/goplum#check_time"
)

// Result contains information about a check that was performed.
type Result struct {
	// State gives the current state of the service.
	State CheckState `json:"state"`
	// Time is the time the check was performed.
	Time time.Time `json:"time"`
	// Detail is an short, optional explanation of the current state.
	Detail string `json:"detail,omitempty"`
	// Facts provides details about the check and/or the remote service, such as the response time or version.
	Facts map[Fact]any `json:"facts,omitempty"`
}

// GoodResult creates a new result indicating the service is in a good state.
func GoodResult() Result {
	return Result{
		State: StateGood,
		Time:  time.Now(),
	}
}

// IndeterminateResult creates a new result indicating the check wasn't able to compute a state.
func IndeterminateResult(format string, a ...any) Result {
	return Result{
		State:  StateIndeterminate,
		Time:   time.Now(),
		Detail: fmt.Sprintf(format, a...),
	}
}

// FailingResult creates a new result indicating the service is in a bad state.
func FailingResult(format string, a ...any) Result {
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
	Config any `json:"config"`
	// LastResult is the most recent result that caused the transition.
	LastResult *Result `json:"last_result"`
	// PreviousState is the state this check was previously in.
	PreviousState CheckState `json:"previous_state"`
	// NewState is the state this check is now in.
	NewState CheckState `json:"new_state"`
	// IsReminder indicates that this alert is a periodic reminder for an ongoing failure, not a new state change.
	IsReminder bool `json:"is_reminder"`
}

// Alert defines the method to inform the user of a change to a service - e.g. when it comes up or goes down.
//
// Alerts may also implement the Validator interface to validate their arguments when configured.
type Alert interface {
	// Send dispatches an alert in relation to the given check event.
	Send(details AlertDetails) error
}

// Validator is implemented by checks, alerts and plugins that wish to validate their own config.
type Validator interface {
	// Validate checks the configuration of the object and returns any errors.
	Validate() error
}

// LongRunning is implemented by checks that intentionally run for a long period of time. Checks that implement
// this interface won't be subject to user-defined timeouts.
type LongRunning interface {
	// Timeout specifies the upper-bound for how long the check will take.
	Timeout() time.Duration
}

// Stateful is implemented by checks that keep local state that should be persisted across restarts.
type Stateful interface {
	Save() any
	Restore(func(any))
}
