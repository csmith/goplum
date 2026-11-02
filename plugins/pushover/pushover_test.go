package pushover

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateSettings_Priority(t *testing.T) {
	tests := []struct {
		priority int
		valid    bool
	}{
		{-3, false},
		{-2, true},
		{-1, true},
		{0, true},
		{1, true},
		{2, true},
		{3, false},
	}

	for _, tt := range tests {
		t.Run(
			fmt.Sprintf("priority %d", tt.priority),
			func(t *testing.T) {
				err := MessageAlert{}.validateSettings(
					PushSettings{Priority: tt.priority, Retry: 30 * time.Second, Expire: time.Hour},
				)
				if tt.valid {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			},
		)
	}
}

func TestValidateSettings_EmergencyRequiresRetryAndExpire(t *testing.T) {
	require.Error(t, MessageAlert{}.validateSettings(PushSettings{Priority: 2, Expire: time.Hour}))
	require.Error(t, MessageAlert{}.validateSettings(PushSettings{Priority: 2, Retry: 30 * time.Second}))
	require.NoError(
		t,
		MessageAlert{}.validateSettings(PushSettings{Priority: 2, Retry: 30 * time.Second, Expire: time.Hour}),
	)
}

func TestValidateSettings_RetryAndExpireBounds(t *testing.T) {
	require.Error(t, MessageAlert{}.validateSettings(PushSettings{Retry: 29 * time.Second}))
	require.NoError(t, MessageAlert{}.validateSettings(PushSettings{Retry: 30 * time.Second}))
	require.NoError(t, MessageAlert{}.validateSettings(PushSettings{Expire: 3 * time.Hour}))
	require.Error(t, MessageAlert{}.validateSettings(PushSettings{Expire: 3*time.Hour + time.Second}))
}
