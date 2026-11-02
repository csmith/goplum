package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"chameth.com/goplum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCheck_FollowsRedirectsEachTime(t *testing.T) {
	var server *httptest.Server
	var urls []string

	server = httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				urls = append(urls, r.URL.Path)
				if r.URL.Path == "/" {
					http.Redirect(w, r, "/redirect", http.StatusPermanentRedirect)
				} else {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("ok"))
				}
			},
		),
	)
	defer server.Close()

	check := &GetCheck{
		Url:             server.URL,
		ContentExpected: true,
		Content:         "ok",
		MinStatusCode:   200,
		MaxStatusCode:   399,
	}

	result := check.Execute(context.Background())
	assert.Equal(t, goplum.StateGood, result.State)
	assert.Equal(t, []string{"/", "/redirect"}, urls)

	urls = []string{}
	result = check.Execute(context.Background())
	assert.Equal(t, goplum.StateGood, result.State)
	assert.Equal(t, []string{"/", "/redirect"}, urls)
}

func TestGetCheck_ClosesBodyOnBadStatusCode(t *testing.T) {
	disconnected := make(chan struct{})
	finished := make(chan struct{})
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
				w.(http.Flusher).Flush()
				// Hold the response open until the client gives up on it.
				select {
				case <-r.Context().Done():
					close(disconnected)
				case <-finished:
				}
			},
		),
	)
	defer server.Close()
	defer close(finished)

	check := &GetCheck{
		Url:           server.URL,
		MinStatusCode: 200,
		MaxStatusCode: 399,
	}

	result := check.Execute(context.Background())
	assert.Equal(t, goplum.StateFailing, result.State)

	select {
	case <-disconnected:
	case <-time.After(5 * time.Second):
		assert.Fail(t, "response body was not closed")
	}
}

func TestWebHookAlert_SendsPostWithJsonBody(t *testing.T) {
	var method string
	var contentType string

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				method = r.Method
				contentType = r.Header.Get("Content-Type")
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	alert := WebHookAlert{Url: server.URL}
	err := alert.Send(goplum.AlertDetails{Text: "test"})
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, method)
	assert.Equal(t, "application/json", contentType)
}

func TestWebHookAlert_SendsCustomHeaders(t *testing.T) {
	var authHeader, customHeader string

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				authHeader = r.Header.Get("Authorization")
				customHeader = r.Header.Get("X-Custom")
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	alert := WebHookAlert{
		Url:     server.URL,
		Headers: []string{"Authorization: Bearer token123", "X-Custom: value"},
	}
	err := alert.Send(goplum.AlertDetails{Text: "test"})
	require.NoError(t, err)
	assert.Equal(t, "Bearer token123", authHeader)
	assert.Equal(t, "value", customHeader)
}

func TestWebHookAlert_SendsSignatureWhenSecretSet(t *testing.T) {
	var sig string
	var body []byte

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				sig = r.Header.Get("X-Goplum-Signature")
				body = make([]byte, r.ContentLength)
				_, _ = r.Body.Read(body)
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	secret := "my-secret"
	alert := WebHookAlert{Url: server.URL, Secret: secret}
	err := alert.Send(goplum.AlertDetails{Text: "hello"})
	require.NoError(t, err)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expected, sig)
}

func TestWebHookAlert_NoSignatureWhenSecretEmpty(t *testing.T) {
	var sig string

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				sig = r.Header.Get("X-Goplum-Signature")
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	alert := WebHookAlert{Url: server.URL}
	err := alert.Send(goplum.AlertDetails{Text: "test"})
	require.NoError(t, err)
	assert.Empty(t, sig)
}

func TestWebHookAlert_ReturnsErrorOnBadStatus(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
		),
	)
	defer server.Close()

	alert := WebHookAlert{Url: server.URL}
	err := alert.Send(goplum.AlertDetails{Text: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

func TestWebHookAlert_ReturnsErrorOnInvalidHeader(t *testing.T) {
	alert := WebHookAlert{Url: "http://localhost", Headers: []string{"NoColon"}}
	err := alert.Send(goplum.AlertDetails{Text: "test"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid header")
}

func TestWebHookAlert_TimesOutIfServerNeverResponds(t *testing.T) {
	done := make(chan struct{})
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				<-done
			},
		),
	)
	defer server.Close()
	defer close(done)

	oldTimeout := webhookTimeout
	webhookTimeout = 100 * time.Millisecond
	defer func() { webhookTimeout = oldTimeout }()

	start := time.Now()
	err := WebHookAlert{Url: server.URL}.Send(goplum.AlertDetails{})
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 5*time.Second)
}

const unparseableUrl = "http://user:secret@example.com/%zz"

func TestGetCheck_FailsWithoutPanicOnUnparseableUrl(t *testing.T) {
	check := GetCheck{Url: unparseableUrl}

	result := check.Execute(context.Background())
	assert.Equal(t, goplum.StateFailing, result.State)
	assert.Contains(t, result.Detail, "Error building request")
	assert.NotContains(t, result.Detail, "secret")
}

func TestHealthCheck_FailsWithoutPanicOnUnparseableUrl(t *testing.T) {
	check := HealthCheck{Url: unparseableUrl}

	result := check.Execute(context.Background())
	assert.Equal(t, goplum.StateFailing, result.State)
	assert.Contains(t, result.Detail, "Error building request")
	assert.NotContains(t, result.Detail, "secret")
}

func TestValidate_RejectsUnparseableUrl(t *testing.T) {
	validators := map[string]interface{ Validate() error }{
		"get":         GetCheck{Url: unparseableUrl},
		"healthcheck": HealthCheck{Url: unparseableUrl},
		"webhook":     WebHookAlert{Url: unparseableUrl},
	}

	for name, v := range validators {
		t.Run(
			name,
			func(t *testing.T) {
				err := v.Validate()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid url")
				assert.NotContains(t, err.Error(), "secret")
			},
		)
	}
}

func TestValidate_AcceptsValidUrl(t *testing.T) {
	validators := map[string]interface{ Validate() error }{
		"get":         GetCheck{Url: "https://example.com/"},
		"healthcheck": HealthCheck{Url: "https://example.com/"},
		"webhook":     WebHookAlert{Url: "https://example.com/"},
	}

	for name, v := range validators {
		t.Run(
			name,
			func(t *testing.T) {
				assert.NoError(t, v.Validate())
			},
		)
	}
}
