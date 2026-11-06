package twilio

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"chameth.com/goplum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func captureForm(t *testing.T) *url.Values {
	t.Helper()

	form := &url.Values{}
	old := client.Transport
	client.Transport = roundTripFunc(
		func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, err
			}
			*form, err = url.ParseQuery(string(body))
			if err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: http.StatusCreated, Body: http.NoBody}, nil
		},
	)
	t.Cleanup(func() { client.Transport = old })

	return form
}

func TestSmsAlert_AppendsLink(t *testing.T) {
	form := captureForm(t)
	link := "https://example.com/runbook"

	require.NoError(t, SmsAlert{}.Send(goplum.AlertDetails{Text: "alert", Link: link}))

	assert.Equal(t, "alert\n"+link, form.Get("Body"))
}

func TestSmsAlert_KeepsLinkWithinBodyLimit(t *testing.T) {
	form := captureForm(t)
	link := "https://example.com/runbook"

	require.NoError(t, SmsAlert{}.Send(goplum.AlertDetails{Text: strings.Repeat("a", 3000), Link: link}))

	assert.True(t, strings.HasSuffix(form.Get("Body"), "\n"+link))
	assert.Equal(t, 1600, utf8.RuneCountInString(form.Get("Body")))
}

func TestCallAlert_DoesNotReadOutLink(t *testing.T) {
	form := captureForm(t)

	require.NoError(t, CallAlert{}.Send(goplum.AlertDetails{Text: "alert", Link: "https://example.com/runbook"}))

	assert.Contains(t, form.Get("Twiml"), "alert")
	assert.NotContains(t, form.Get("Twiml"), "example.com")
}
