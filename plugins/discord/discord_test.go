package discord

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

func TestSend_KeepsLinkWithinContentLimit(t *testing.T) {
	var payload struct {
		Content string `json:"content"`
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
		MessageAlert{Url: server.URL}.Send(goplum.AlertDetails{Text: strings.Repeat("a", 3000), Link: link}),
	)

	assert.True(t, strings.HasSuffix(payload.Content, "\n"+link))
	assert.Equal(t, 2000, utf8.RuneCountInString(payload.Content))
}
