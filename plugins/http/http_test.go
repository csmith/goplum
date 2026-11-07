package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestWebHookAlert_ValidateAndSendRejectMalformedHeaders(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	headers := map[string]string{
		"missing colon":     "Authorization Bearer secret",
		"empty name":        ": secret",
		"blank name":        "   : secret",
		"space in name":     "X Custom: secret",
		"newline in value":  "X-Custom: secret\nX-Injected: secret",
		"control in value":  "X-Custom: sec\x7fret",
		"non-token in name": "X-Cüstom: secret",
	}

	for name, header := range headers {
		t.Run(
			name,
			func(t *testing.T) {
				alert := WebHookAlert{Url: server.URL, Headers: []string{"X-Good: value", header}}

				err := alert.Validate()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid header at position 2")
				assert.NotContains(t, err.Error(), "secret")

				assert.EqualError(t, alert.Send(goplum.AlertDetails{Text: "test"}), err.Error())
			},
		)
	}
}

func TestWebHookAlert_ValidateAndSendAcceptWellFormedHeaders(t *testing.T) {
	var received http.Header

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				received = r.Header
				w.WriteHeader(http.StatusOK)
			},
		),
	)
	defer server.Close()

	alert := WebHookAlert{
		Url: server.URL,
		Headers: []string{
			"X-Plain: value",
			"  X-Padded  :  value  ",
			"X-Empty:",
			"X-Colons: a:b:c",
			"X-Tab: a\tb",
			"X-Utf8: café",
			"X-Multi: a",
			"X-Multi: b",
			"!#$%&'*+-.^_`|~: value",
		},
	}

	require.NoError(t, alert.Validate())
	require.NoError(t, alert.Send(goplum.AlertDetails{Text: "test"}))

	assert.Equal(t, "value", received.Get("X-Plain"))
	assert.Equal(t, "value", received.Get("X-Padded"))
	assert.Equal(t, []string{""}, received.Values("X-Empty"))
	assert.Equal(t, "a:b:c", received.Get("X-Colons"))
	assert.Equal(t, "a\tb", received.Get("X-Tab"))
	assert.Equal(t, "café", received.Get("X-Utf8"))
	assert.Equal(t, []string{"a", "b"}, received.Values("X-Multi"))
	assert.Equal(t, "value", received.Get("!#$%&'*+-.^_`|~"))
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

func TestHealthCheck_ReadsStatusAndComponents(t *testing.T) {
	body, err := os.ReadFile("testdata/health.json")
	require.NoError(t, err)

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/health+json")
				_, _ = w.Write(body)
			},
		),
	)
	defer server.Close()

	tests := []struct {
		name            string
		checkComponents bool
		state           goplum.CheckState
		detail          string
	}{
		{"overall", false, goplum.StateGood, "all good"},
		{"components", true, goplum.StateFailing, "component db:connections: replica down"},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				check := HealthCheck{Url: server.URL, CheckComponents: tt.checkComponents}

				result := check.Execute(context.Background())
				assert.Equal(t, tt.state, result.State)
				assert.Equal(t, tt.detail, result.Detail)
			},
		)
	}
}

func TestHealthCheck_FailsOnBadResponses(t *testing.T) {
	tests := []struct {
		name   string
		code   int
		body   string
		detail string
	}{
		{
			"html error page", http.StatusBadGateway, "<html>Bad Gateway</html>",
			"Error decoding response (HTTP 502 Bad Gateway): ",
		},
		{"empty body", http.StatusServiceUnavailable, "", "Error decoding response (HTTP 503 Service Unavailable): "},
		{
			"passing body with error status", http.StatusServiceUnavailable, `{"status": "pass", "output": "draining"}`,
			"Bad response (HTTP 503 Service Unavailable): draining",
		},
		{
			"passing body without output", http.StatusNotFound, `{"status": "pass"}`,
			"Bad response (HTTP 404 Not Found)",
		},
		{"missing status", http.StatusOK, `{"output": "hello"}`, `Unrecognised status "" (HTTP 200 OK)`},
		{"unknown status", http.StatusOK, `{"status": "maybe"}`, `Unrecognised status "maybe" (HTTP 200 OK)`},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				server := httptest.NewServer(
					http.HandlerFunc(
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(tt.code)
							_, _ = w.Write([]byte(tt.body))
						},
					),
				)
				defer server.Close()

				result := HealthCheck{Url: server.URL}.Execute(context.Background())
				assert.Equal(t, goplum.StateFailing, result.State)
				assert.Contains(t, result.Detail, tt.detail)
			},
		)
	}
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

func TestValidate_RejectsUrlsThatCanNeverWork(t *testing.T) {
	for _, u := range []string{
		"example.com/hook",
		"ftp://user:secret@example.com/",
		"https://",
		"https:///path",
	} {
		validators := map[string]interface{ Validate() error }{
			"get":         GetCheck{Url: u},
			"healthcheck": HealthCheck{Url: u},
			"webhook":     WebHookAlert{Url: u},
		}

		for name, v := range validators {
			t.Run(
				name+"/"+u,
				func(t *testing.T) {
					err := v.Validate()
					require.ErrorContains(t, err, "must be an absolute http or https URL")
					assert.NotContains(t, err.Error(), "secret")
				},
			)
		}
	}
}

func TestValidate_AcceptsValidUrl(t *testing.T) {
	validators := map[string]interface{ Validate() error }{
		"get": GetCheck{
			BaseCheck:     BaseCheck{Url: "https://example.com/", Network: "tcp"},
			MinStatusCode: 100,
			MaxStatusCode: 399,
		},
		"healthcheck": HealthCheck{BaseCheck: BaseCheck{Url: "https://example.com/", Network: "tcp"}},
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

func TestGetCheck_ValidateStatusCodes(t *testing.T) {
	tests := []struct {
		name  string
		min   int
		max   int
		valid bool
	}{
		{"full range", 100, 599, true},
		{"single code", 418, 418, true},
		{"min above max", 400, 399, false},
		{"min below 100", 99, 399, false},
		{"negative min", -1, 399, false},
		{"max above 599", 200, 600, false},
		{"negative max", 100, -200, false},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				check := GetCheck{
					Url:           "https://example.com/",
					Network:       "tcp",
					MinStatusCode: tt.min,
					MaxStatusCode: tt.max,
				}

				err := check.Validate()
				if tt.valid {
					assert.NoError(t, err)
				} else {
					assert.ErrorContains(t, err, "invalid status codes")
				}
			},
		)
	}
}

func TestChecks_ConnectOverRequestedNetwork(t *testing.T) {
	tests := []struct {
		name    string
		listen  string
		network string
		detail  string
	}{
		{"ipv4 over tcp", "127.0.0.1:0", "tcp", ""},
		{"ipv4 over tcp4", "127.0.0.1:0", "tcp4", ""},
		{"ipv4 over tcp6", "127.0.0.1:0", "tcp6", "no IPv6 address for 127.0.0.1"},
		{"ipv6 over tcp", "[::1]:0", "tcp", ""},
		{"ipv6 over tcp6", "[::1]:0", "tcp6", ""},
		{"ipv6 over tcp4", "[::1]:0", "tcp4", "no IPv4 address for ::1"},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", tt.listen)
				if err != nil {
					t.Skipf("unable to listen on %s: %v", tt.listen, err)
				}

				server := httptest.NewUnstartedServer(
					http.HandlerFunc(
						func(w http.ResponseWriter, r *http.Request) {
							_, _ = w.Write([]byte(`{"status": "pass"}`))
						},
					),
				)
				server.Listener = listener
				server.Start()
				defer server.Close()

				base := BaseCheck{Url: server.URL, Network: tt.network}
				checks := map[string]goplum.Check{
					"get":         GetCheck{BaseCheck: base, MinStatusCode: 200, MaxStatusCode: 299},
					"healthcheck": HealthCheck{BaseCheck: base},
				}

				for name, check := range checks {
					result := check.Execute(context.Background())
					if tt.detail == "" {
						assert.Equal(t, goplum.StateGood, result.State, name)
					} else {
						assert.Equal(t, goplum.StateFailing, result.State, name)
						assert.Contains(t, result.Detail, tt.detail, name)
					}
				}
			},
		)
	}
}

func TestChecks_ValidateNetwork(t *testing.T) {
	tests := []struct {
		network string
		valid   bool
	}{
		{"tcp", true},
		{"tcp4", true},
		{"tcp6", true},
		{"", false},
		{"udp", false},
		{"TCP4", false},
	}

	for _, tt := range tests {
		t.Run(
			tt.network,
			func(t *testing.T) {
				base := BaseCheck{Url: "https://example.com/", Network: tt.network}
				validators := map[string]interface{ Validate() error }{
					"get":         GetCheck{BaseCheck: base, MinStatusCode: 100, MaxStatusCode: 399},
					"healthcheck": HealthCheck{BaseCheck: base},
				}

				for name, v := range validators {
					err := v.Validate()
					if tt.valid {
						assert.NoError(t, err, name)
					} else {
						assert.ErrorContains(t, err, "invalid network", name)
					}
				}
			},
		)
	}
}

func TestPlugin_DefaultNetworkIsValid(t *testing.T) {
	get := Plugin{}.Check("get").(GetCheck)
	get.Url = "https://example.com/"
	require.NoError(t, get.Validate())

	health := Plugin{}.Check("healthcheck").(HealthCheck)
	health.Url = "https://example.com/"
	require.NoError(t, health.Validate())
}
