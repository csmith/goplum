package goplum

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGroupAlertThrottling(t *testing.T) {
	t.Run(
		"NoLimit",
		func(t *testing.T) {
			group := &Group{
				Name:        "test",
				AlertLimit:  0, // No limit
				AlertWindow: time.Minute,
			}

			// Should always allow alerts when no limit set
			assert.True(t, group.canSendAlert())
			assert.False(t, group.recordAlert())
			assert.True(t, group.canSendAlert())
		},
	)

	t.Run(
		"SingleAlertLimit",
		func(t *testing.T) {
			group := &Group{
				Name:        "test",
				AlertLimit:  1,
				AlertWindow: time.Minute,
			}

			// First alert should be allowed and marked as last
			assert.True(t, group.canSendAlert())
			assert.True(t, group.recordAlert())

			// Second alert should be blocked
			assert.False(t, group.canSendAlert())
		},
	)

	t.Run(
		"MultipleAlertLimit",
		func(t *testing.T) {
			group := &Group{
				Name:        "test",
				AlertLimit:  3,
				AlertWindow: time.Minute,
			}

			// First two alerts should be normal
			for i := range 2 {
				assert.True(t, group.canSendAlert(), "alert %d", i+1)
				assert.False(t, group.recordAlert(), "alert %d", i+1)
			}

			// Third alert should be last
			assert.True(t, group.canSendAlert())
			assert.True(t, group.recordAlert())

			// Fourth alert should be blocked
			assert.False(t, group.canSendAlert())
		},
	)

	t.Run(
		"CheckingDoesNotUseQuota",
		func(t *testing.T) {
			group := &Group{
				Name:        "test",
				AlertLimit:  1,
				AlertWindow: time.Minute,
			}

			for range 3 {
				assert.True(t, group.canSendAlert())
			}
			assert.True(t, group.recordAlert())
		},
	)

	t.Run(
		"AlertsExpireAfterWindow",
		func(t *testing.T) {
			group := &Group{
				Name:         "test",
				AlertLimit:   1,
				AlertWindow:  time.Minute,
				alertHistory: []time.Time{time.Now().Add(-2 * time.Minute)},
			}

			assert.True(t, group.canSendAlert())
			assert.True(t, group.recordAlert())
			assert.Len(t, group.alertHistory, 1)
		},
	)
}

func TestShouldSendAlert(t *testing.T) {
	newPlum := func() *Plum {
		p := NewPlum()
		p.Groups["ga"] = &Group{Name: "ga", AlertLimit: 3, AlertWindow: time.Hour}
		p.Groups["gb"] = &Group{Name: "gb", AlertLimit: 1, AlertWindow: time.Hour}
		return p
	}

	t.Run(
		"SuppressedAlertsDoNotUseOtherGroupsQuota",
		func(t *testing.T) {
			p := newPlum()
			p.Groups["gb"].recordAlert()

			for range 10 {
				send, _, suppressor := p.shouldSendAlert([]string{"ga", "gb"})
				assert.False(t, send)
				assert.Equal(t, "gb", suppressor)
			}

			assert.Empty(t, p.Groups["ga"].alertHistory)
			send, warning, _ := p.shouldSendAlert([]string{"ga"})
			assert.True(t, send)
			assert.Empty(t, warning)
		},
	)

	t.Run(
		"RecordsInEveryGroupWhenSent",
		func(t *testing.T) {
			p := newPlum()

			send, warning, _ := p.shouldSendAlert([]string{"ga", "gb"})
			assert.True(t, send)
			assert.Equal(t, " [GROUP ALERT LIMIT REACHED: gb]", warning)
			assert.Len(t, p.Groups["ga"].alertHistory, 1)
			assert.Len(t, p.Groups["gb"].alertHistory, 1)
		},
	)

	t.Run(
		"DuplicateGroupUsesOneSlot",
		func(t *testing.T) {
			p := newPlum()

			send, _, _ := p.shouldSendAlert([]string{"ga", "ga"})
			assert.True(t, send)
			assert.Len(t, p.Groups["ga"].alertHistory, 1)
		},
	)

	t.Run(
		"NoGroups",
		func(t *testing.T) {
			send, warning, suppressor := newPlum().shouldSendAlert(nil)
			assert.True(t, send)
			assert.Empty(t, warning)
			assert.Empty(t, suppressor)
		},
	)
}
