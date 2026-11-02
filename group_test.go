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
			canSend, isLast := group.canSendAlert()
			assert.True(t, canSend)
			assert.False(t, isLast)
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
			canSend, isLast := group.canSendAlert()
			assert.True(t, canSend)
			assert.True(t, isLast)

			// Second alert should be blocked
			canSend, isLast = group.canSendAlert()
			assert.False(t, canSend)
			assert.False(t, isLast)
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
				canSend, isLast := group.canSendAlert()
				assert.True(t, canSend, "alert %d", i+1)
				assert.False(t, isLast, "alert %d", i+1)
			}

			// Third alert should be last
			canSend, isLast := group.canSendAlert()
			assert.True(t, canSend)
			assert.True(t, isLast)

			// Fourth alert should be blocked
			canSend, isLast = group.canSendAlert()
			assert.False(t, canSend)
			assert.False(t, isLast)
		},
	)
}
