package slack

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"chameth.com/goplum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSend_KeepsLinkWithinTextLimit(t *testing.T) {
	var payload struct {
		Text string `json:"text"`
	}
	server := httptest.NewServer(
		http.HandlerFunc(
			func(_ http.ResponseWriter, r *http.Request) {
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			},
		),
	)
	defer server.Close()

	link := "https://example.com/runbook"
	require.NoError(
		t,
		MessageAlert{Url: server.URL}.Send(goplum.AlertDetails{Text: strings.Repeat("a", 50000), Link: link}),
	)

	assert.True(t, strings.HasSuffix(payload.Text, "\n"+link))
	assert.Equal(t, 40000, utf8.RuneCountInString(payload.Text))
}
