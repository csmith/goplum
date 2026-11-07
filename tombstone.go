package goplum

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"
)

const maxTombStoneAge = 10 * time.Minute

type TombStone struct {
	Time   time.Time                 `json:"Time"`
	Checks map[string]CheckTombStone `json:"Checks"`
}

type CheckTombStone struct {
	LastRun       time.Time       `json:"LastRun"`
	Settled       bool            `json:"Settled"`
	State         CheckState      `json:"State"`
	Suspended     bool            `json:"Suspended"`
	LastAlertTime time.Time       `json:"last_alert_time,omitzero"`
	History       ResultHistory   `json:"History"`
	PluginState   json.RawMessage `json:"plugin_state,omitempty"`
}

func NewTombStone(checks map[string]*ScheduledCheck) *TombStone {
	ts := &TombStone{
		Time:   time.Now(),
		Checks: make(map[string]CheckTombStone),
	}

	for i := range checks {
		var state []byte
		check := checks[i]

		if stateful, ok := check.Check.(Stateful); ok {
			var err error
			state, err = json.Marshal(stateful.Save())
			if err != nil {
				slog.Error("Unable to save state of check", "check", check.Name, "error", err)
			}
		}

		ts.Checks[check.Name] = CheckTombStone{
			LastRun:       check.LastRun,
			Settled:       check.Settled,
			State:         check.State,
			Suspended:     check.Suspended,
			LastAlertTime: check.LastAlertTime,
			History:       check.History,
			PluginState:   state,
		}
	}

	return ts
}

func LoadTombStone(path string) (*TombStone, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	tombStone := &TombStone{}

	return tombStone, json.NewDecoder(f).Decode(tombStone)
}

func (ts *TombStone) Save(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(0600))
	if err != nil {
		return err
	}

	defer f.Close()

	return json.NewEncoder(f).Encode(ts)
}

func (ts *TombStone) Restore(checks map[string]*ScheduledCheck) error {
	if time.Since(ts.Time) >= maxTombStoneAge {
		return fmt.Errorf("tombstone too old: %s", ts.Time)
	}

	for i := range checks {
		check := checks[i]
		if saved, ok := ts.Checks[check.Name]; ok {
			check.LastRun = saved.LastRun
			check.Settled = saved.Settled
			check.State = saved.State
			check.Suspended = saved.Suspended
			check.LastAlertTime = saved.LastAlertTime
			check.History = saved.History

			if stateful, ok := check.Check.(Stateful); ok && saved.PluginState != nil {
				stateful.Restore(
					func(i any) {
						if err := json.Unmarshal(saved.PluginState, i); err != nil {
							slog.Warn("Unable to restore state of check", "check", check.Name, "error", err)
						}
					},
				)
			}
		}
	}

	return nil
}
