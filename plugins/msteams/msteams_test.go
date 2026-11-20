package msteams

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

type sentPayload struct {
	Type        string `json:"type"`
	Attachments []struct {
		ContentType string `json:"contentType"`
		Content     struct {
			Type string `json:"type"`
			Body []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"body"`
		} `json:"content"`
	} `json:"attachments"`
}

func capture(t *testing.T, payload *sentPayload) *httptest.Server {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.NoError(t, json.NewDecoder(r.Body).Decode(payload))
				w.WriteHeader(http.StatusAccepted)
			},
		),
	)
	t.Cleanup(server.Close)
	return server
}

func TestSend_PostsAdaptiveCard(t *testing.T) {
	var payload sentPayload
	server := capture(t, &payload)

	require.NoError(
		t,
		MessageAlert{Url: server.URL, Title: "Service alert"}.Send(
			goplum.AlertDetails{Text: "Check is down", Link: "https://example.com/runbook"},
		),
	)

	assert.Equal(t, "message", payload.Type)
	require.Len(t, payload.Attachments, 1)
	assert.Equal(t, "application/vnd.microsoft.card.adaptive", payload.Attachments[0].ContentType)
	card := payload.Attachments[0].Content
	assert.Equal(t, "AdaptiveCard", card.Type)
	require.Len(t, card.Body, 2)
	assert.Equal(t, "Service alert", card.Body[0].Text)
	assert.Equal(t, "Check is down\nhttps://example.com/runbook", card.Body[1].Text)
}

func TestSend_KeepsLinkWithinTextLimit(t *testing.T) {
	var payload sentPayload
	server := capture(t, &payload)

	link := "https://example.com/runbook"
	require.NoError(
		t,
		MessageAlert{Url: server.URL}.Send(goplum.AlertDetails{Text: strings.Repeat("a", 50000), Link: link}),
	)

	require.Len(t, payload.Attachments, 1)
	require.Len(t, payload.Attachments[0].Content.Body, 2)
	text := payload.Attachments[0].Content.Body[1].Text
	assert.True(t, strings.HasSuffix(text, "\n"+link))
	assert.Equal(t, maxTextLength, utf8.RuneCountInString(text))
}

func TestSend_ReportsErrorStatus(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
		),
	)
	defer server.Close()

	err := MessageAlert{Url: server.URL}.Send(goplum.AlertDetails{Text: "alert"})
	assert.EqualError(t, err, "bad response from Teams: HTTP 400")
}

func TestSend_ErrorOmitsURL(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	target := server.URL + "/workflows/secret-signature"
	server.Close()

	err := MessageAlert{Url: target}.Send(goplum.AlertDetails{Text: "alert"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret-signature")
}

func TestValidate(t *testing.T) {
	tests := map[string]struct {
		url string
		ok  bool
	}{
		"workflows url": {url: "https://example.com/workflows/abc/triggers/manual/paths/invoke?sig=xyz", ok: true},
		"missing url":   {url: ""},
		"relative url":  {url: "/webhook"},
		"other scheme":  {url: "ftp://example.com/webhook"},
	}

	for name, tt := range tests {
		t.Run(
			name,
			func(t *testing.T) {
				err := MessageAlert{Url: tt.url}.Validate()
				if tt.ok {
					assert.NoError(t, err)
				} else {
					assert.Error(t, err)
				}
			},
		)
	}
}
