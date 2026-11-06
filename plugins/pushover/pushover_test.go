package pushover

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"chameth.com/goplum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sentMessage struct {
	Message string `json:"message"`
}

func startServer(t *testing.T, status int) *[]sentMessage {
	t.Helper()

	var received []sentMessage
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				var message sentMessage
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&message))
				received = append(received, message)
				w.WriteHeader(status)
			},
		),
	)
	t.Cleanup(server.Close)

	old := apiURL
	apiURL = server.URL
	t.Cleanup(func() { apiURL = old })

	return &received
}

func TestSend_KeepsTryingAfterClientErrors(t *testing.T) {
	received := startServer(t, http.StatusBadRequest)
	alert := MessageAlert{Token: "token", Key: "key"}

	require.ErrorContains(t, alert.Send(goplum.AlertDetails{Text: "first"}), "HTTP 400")
	require.ErrorContains(t, alert.Send(goplum.AlertDetails{Text: "second"}), "HTTP 400")

	require.Len(t, *received, 2)
	assert.Equal(t, "first", (*received)[0].Message)
	assert.Equal(t, "second", (*received)[1].Message)
}

func TestSend_TruncatesLongMessages(t *testing.T) {
	tests := map[string]struct {
		text     string
		expected string
	}{
		"at the limit": {
			text:     strings.Repeat("a", 1024),
			expected: strings.Repeat("a", 1024),
		},
		"over the limit": {
			text:     strings.Repeat("a", 1025),
			expected: strings.Repeat("a", 1023) + "…",
		},
		"multi-byte characters": {
			text:     strings.Repeat("é", 2000),
			expected: strings.Repeat("é", 1023) + "…",
		},
	}

	for name, tt := range tests {
		t.Run(
			name,
			func(t *testing.T) {
				received := startServer(t, http.StatusOK)

				require.NoError(t, MessageAlert{Token: "token", Key: "key"}.Send(goplum.AlertDetails{Text: tt.text}))

				require.Len(t, *received, 1)
				assert.Equal(t, tt.expected, (*received)[0].Message)
				assert.LessOrEqual(t, utf8.RuneCountInString((*received)[0].Message), 1024)
			},
		)
	}
}

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
