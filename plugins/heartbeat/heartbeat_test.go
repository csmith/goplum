package heartbeat

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newReceivedCheck(p *Plugin, id string) *ReceivedCheck {
	check := p.Check("received").(*ReceivedCheck)
	check.ID = id
	check.Within = time.Minute
	return check
}

func TestReceiver_ServeHTTP_doesNotLogID(t *testing.T) {
	const id = "0123456789abcdef0123456789abcdef"
	p := &Plugin{}
	check := newReceivedCheck(p, id)
	require.NoError(t, check.Validate())

	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	r := &receiver{path: "/heartbeat/", checks: p.checks}
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/heartbeat/"+id, nil))

	assert.Equal(t, http.StatusAccepted, recorder.Code)
	assert.False(t, check.received.IsZero())
	assert.Contains(t, logs.String(), "Received heartbeat")
	assert.NotContains(t, logs.String(), id)
}

func TestReceivedCheck_Validate_rejectsDuplicateIDs(t *testing.T) {
	tests := map[string]string{
		"same case":      "0123456789abcdef0123456789abcdef",
		"different case": "0123456789ABCDEF0123456789ABCDEF",
	}

	for name, second := range tests {
		t.Run(
			name,
			func(t *testing.T) {
				p := &Plugin{}
				require.NoError(t, newReceivedCheck(p, "0123456789abcdef0123456789abcdef").Validate())

				err := newReceivedCheck(p, second).Validate()
				require.Error(t, err)
				assert.NotContains(t, err.Error(), "0123456789")
			},
		)
	}
}

func TestReceivedCheck_Validate_allowsSameCheckTwice(t *testing.T) {
	check := newReceivedCheck(&Plugin{}, "0123456789abcdef0123456789abcdef")
	require.NoError(t, check.Validate())
	assert.NoError(t, check.Validate())
}
