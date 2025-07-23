package goplum

import (
	"testing"
	"time"
)

func TestGroupAlertThrottling(t *testing.T) {
	t.Run("NoLimit", func(t *testing.T) {
		group := &Group{
			Name:        "test",
			AlertLimit:  0, // No limit
			AlertWindow: time.Minute,
		}

		// Should always allow alerts when no limit set
		canSend, isLast := group.canSendAlert()
		if !canSend || isLast {
			t.Errorf("Expected canSend=true, isLast=false for no limit, got %v, %v", canSend, isLast)
		}
	})

	t.Run("SingleAlertLimit", func(t *testing.T) {
		group := &Group{
			Name:        "test",
			AlertLimit:  1,
			AlertWindow: time.Minute,
		}

		// First alert should be allowed and marked as last
		canSend, isLast := group.canSendAlert()
		if !canSend || !isLast {
			t.Errorf("Expected canSend=true, isLast=true for first alert with limit=1, got %v, %v", canSend, isLast)
		}

		// Second alert should be blocked
		canSend, isLast = group.canSendAlert()
		if canSend {
			t.Errorf("Expected canSend=false for second alert with limit=1, got %v", canSend)
		}
	})

	t.Run("MultipleAlertLimit", func(t *testing.T) {
		group := &Group{
			Name:        "test",
			AlertLimit:  3,
			AlertWindow: time.Minute,
		}

		// First two alerts should be normal
		for i := 0; i < 2; i++ {
			canSend, isLast := group.canSendAlert()
			if !canSend || isLast {
				t.Errorf("Alert %d: Expected canSend=true, isLast=false, got %v, %v", i+1, canSend, isLast)
			}
		}

		// Third alert should be last
		canSend, isLast := group.canSendAlert()
		if !canSend || !isLast {
			t.Errorf("Third alert: Expected canSend=true, isLast=true, got %v, %v", canSend, isLast)
		}

		// Fourth alert should be blocked
		canSend, isLast = group.canSendAlert()
		if canSend {
			t.Errorf("Fourth alert: Expected canSend=false, got %v", canSend)
		}
	})
}
