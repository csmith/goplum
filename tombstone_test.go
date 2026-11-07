package goplum_test

import (
	"testing"
	"time"

	"chameth.com/goplum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTombStone_Restore(t *testing.T) {
	tests := map[string]struct {
		age      time.Duration
		restored bool
	}{
		"recent":      {age: time.Minute, restored: true},
		"too old":     {age: time.Hour, restored: false},
		"from future": {age: -time.Hour, restored: false},
	}

	for name, tt := range tests {
		t.Run(
			name,
			func(t *testing.T) {
				saved := time.Now().Add(-tt.age)
				ts := goplum.TombStone{
					Time: saved,
					Checks: map[string]goplum.CheckTombStone{
						"site": {LastRun: saved, State: goplum.StateFailing},
					},
				}
				check := &goplum.ScheduledCheck{Name: "site", State: goplum.StateIndeterminate}

				err := ts.Restore(map[string]*goplum.ScheduledCheck{"site": check})

				if tt.restored {
					require.NoError(t, err)
					assert.Equal(t, goplum.StateFailing, check.State)
					assert.True(t, saved.Equal(check.LastRun))
				} else {
					require.Error(t, err)
					assert.Equal(t, goplum.StateIndeterminate, check.State)
					assert.True(t, check.LastRun.IsZero())
				}
			},
		)
	}
}
