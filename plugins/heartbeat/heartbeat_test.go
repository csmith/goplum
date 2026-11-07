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

func TestPlugin_ServeHTTP_doesNotLogID(t *testing.T) {
	const id = "0123456789abcdef0123456789abcdef"
	check := &ReceivedCheck{ID: id, Within: time.Minute}
	require.NoError(t, check.Validate())
	t.Cleanup(func() { delete(checks, id) })

	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	p := &Plugin{Path: "/heartbeat/"}
	recorder := httptest.NewRecorder()
	p.ServeHTTP(recorder, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/heartbeat/"+id, nil))

	assert.Equal(t, http.StatusAccepted, recorder.Code)
	assert.False(t, check.received.IsZero())
	assert.Contains(t, logs.String(), "Received heartbeat")
	assert.NotContains(t, logs.String(), id)
}
