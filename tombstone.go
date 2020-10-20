package goplum

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

var tombStonePath = flag.String("tombstone", "/tmp/goplum.tomb", "Path to save tombstones to persist data across restarts")

const maxTombStoneAge = 10 * time.Minute

type TombStone struct {
	Time   time.Time
	Checks map[string]CheckTombStone
}

type CheckTombStone struct {
	LastRun   time.Time
	Settled   bool
	State     CheckState
	Suspended bool
	History   ResultHistory
}

func NewTombStone(checks map[string]*ScheduledCheck) *TombStone {
	ts := &TombStone{
		Time:   time.Now(),
		Checks: make(map[string]CheckTombStone),
	}

	for i := range checks {
		check := checks[i]
		ts.Checks[check.Name] = CheckTombStone{
			LastRun:   check.LastRun,
			Settled:   check.Settled,
			State:     check.State,
			Suspended: check.Suspended,
			History:   check.History,
		}
	}

	return ts
}

func LoadTombStone() (*TombStone, error) {
	f, err := os.Open(*tombStonePath)
	if err != nil {
		return nil, err
	}

	defer f.Close()
	tombStone := &TombStone{}

	return tombStone, json.NewDecoder(f).Decode(tombStone)
}

func (ts *TombStone) Save() error {
	f, err := os.OpenFile(*tombStonePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(0600))
	if err != nil {
		return err
	}

	defer f.Close()

	return json.NewEncoder(f).Encode(ts)
}

func (ts *TombStone) Restore(checks map[string]*ScheduledCheck) error {
	if time.Now().Sub(ts.Time) >= maxTombStoneAge {
		return fmt.Errorf("tombstone too old: %s", ts.Time)
	}

	for i := range checks {
		check := checks[i]
		if saved, ok := ts.Checks[check.Name]; ok {
			check.LastRun = saved.LastRun
			check.Settled = saved.Settled
			check.State = saved.State
			check.Suspended = saved.Suspended
			check.History = saved.History
		}
	}

	return nil
}
