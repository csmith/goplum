package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"

	"chameth.com/goplum"
	"github.com/stretchr/testify/assert"
)

func TestGetCheck_FollowsRedirectsEachTime(t *testing.T) {
	var server *httptest.Server
	var urls []string

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		urls = append(urls, r.URL.Path)
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/redirect", http.StatusPermanentRedirect)
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer server.Close()

	check := &GetCheck{
		BaseCheck: BaseCheck{
			Url: server.URL,
		},
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

func TestWebHookAlert_SendsPostWithJsonBody(t *testing.T) {
	var method string
	var contentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	alert := WebHookAlert{Url: server.URL}
	err := alert.Send(goplum.AlertDetails{Text: "test"})
	assert.NoError(t, err)
	assert.Equal(t, http.MethodPost, method)
	assert.Equal(t, "application/json", contentType)
}

func TestWebHookAlert_SendsCustomHeaders(t *testing.T) {
	var authHeader, customHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		customHeader = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	alert := WebHookAlert{
		Url:     server.URL,
		Headers: []string{"Authorization: Bearer token123", "X-Custom: value"},
	}
	err := alert.Send(goplum.AlertDetails{Text: "test"})
	assert.NoError(t, err)
	assert.Equal(t, "Bearer token123", authHeader)
	assert.Equal(t, "value", customHeader)
}

func TestWebHookAlert_SendsSignatureWhenSecretSet(t *testing.T) {
	var sig string
	var body []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sig = r.Header.Get("X-Goplum-Signature")
		body = make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	secret := "my-secret"
	alert := WebHookAlert{Url: server.URL, Secret: secret}
	details := goplum.AlertDetails{Text: "hello"}
	err := alert.Send(details)
	assert.NoError(t, err)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	assert.Equal(t, expected, sig)
}

func TestWebHookAlert_NoSignatureWhenSecretEmpty(t *testing.T) {
	var sig string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sig = r.Header.Get("X-Goplum-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	alert := WebHookAlert{Url: server.URL}
	err := alert.Send(goplum.AlertDetails{Text: "test"})
	assert.NoError(t, err)
	assert.Empty(t, sig)
}

func TestWebHookAlert_ReturnsErrorOnBadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	alert := WebHookAlert{Url: server.URL}
	err := alert.Send(goplum.AlertDetails{Text: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 500")
}

func TestWebHookAlert_ReturnsErrorOnInvalidHeader(t *testing.T) {
	alert := WebHookAlert{Url: "http://localhost", Headers: []string{"NoColon"}}
	err := alert.Send(goplum.AlertDetails{Text: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid header")
}
