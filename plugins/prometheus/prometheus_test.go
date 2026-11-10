package prometheus

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"chameth.com/goplum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type receivedRequest struct {
	path    string
	query   string
	headers http.Header
}

func fixtureServer(t *testing.T, status int, fixture string) (*httptest.Server, *receivedRequest) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", fixture))
	require.NoError(t, err)

	received := &receivedRequest{}
	server := httptest.NewUnstartedServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				received.path = r.URL.Path
				received.query = r.PostFormValue("query")
				received.headers = r.Header.Clone()
				w.WriteHeader(status)
				_, _ = w.Write(body)
			},
		),
	)
	t.Cleanup(server.Close)
	return server, received
}

func execute(t *testing.T, plugin *Plugin) goplum.Result {
	t.Helper()
	require.NoError(t, plugin.Validate())
	check := plugin.Check("query").(QueryCheck)
	check.Query = "up{job='node'}"
	return check.Execute(context.Background())
}

func runQuery(t *testing.T, status int, fixture string) (goplum.Result, *receivedRequest) {
	t.Helper()
	server, received := fixtureServer(t, status, fixture)
	server.Start()
	return execute(t, &Plugin{Url: server.URL + "/prefix/"}), received
}

func TestQueryCheck_SendsQuery(t *testing.T) {
	_, req := runQuery(t, http.StatusOK, "empty.json")
	assert.Equal(t, "/prefix/api/v1/query", req.path)
	assert.Equal(t, "up{job='node'}", req.query)
}

func TestQueryCheck_Healthy(t *testing.T) {
	for _, fixture := range []string{"healthy.json", "healthy-scalar.json"} {
		t.Run(
			fixture,
			func(t *testing.T) {
				result, _ := runQuery(t, http.StatusOK, fixture)
				assert.Equal(t, goplum.StateGood, result.State)
			},
		)
	}
}

func TestQueryCheck_Failing(t *testing.T) {
	tests := []struct {
		fixture string
		status  int
		detail  string
	}{
		{"empty.json", http.StatusOK, "Query returned no results"},
		{"zero.json", http.StatusOK, `1 of 2 series failed: up{instance="b", job="node"} = 0`},
		{"zero-scalar.json", http.StatusOK, "1 of 1 series failed: {} = 0"},
		{"nan.json", http.StatusOK, "1 of 1 series failed: {} = NaN"},
		{
			"many-zero.json",
			http.StatusOK,
			`7 of 7 series failed: {i="1"} = 0, {i="2"} = 0, {i="3"} = 0, {i="4"} = 0, {i="5"} = 0 and 2 more`,
		},
		{"query-error.json", http.StatusUnprocessableEntity, "Query failed: bad_data: parse error"},
		{"bad-gateway.html", http.StatusBadGateway, "Bad status code: 502"},
		{"matrix.json", http.StatusOK, "unsupported result type: matrix"},
		{"non-prometheus-error.json", http.StatusUnauthorized, "Bad status code: 401"},
		{"non-prometheus-error.json", http.StatusOK, `Unexpected response status: ""`},
	}

	for _, tt := range tests {
		t.Run(
			fmt.Sprintf("%d %s", tt.status, tt.fixture),
			func(t *testing.T) {
				result, _ := runQuery(t, tt.status, tt.fixture)
				assert.Equal(t, goplum.StateFailing, result.State)
				assert.Equal(t, tt.detail, result.Detail)
			},
		)
	}
}

func TestQueryCheck_ConnectionError(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()

	result := execute(t, &Plugin{Url: "http://user:secret@" + server.Listener.Addr().String()})
	assert.Equal(t, goplum.StateFailing, result.State)
	assert.NotContains(t, result.Detail, "secret")
}

func TestQueryCheck_SendsHeaders(t *testing.T) {
	server, received := fixtureServer(t, http.StatusOK, "healthy.json")
	server.Start()

	result := execute(
		t,
		&Plugin{
			Url:     server.URL,
			Headers: []string{"X-Scope-OrgID: tenant-1", "X-Multi: a", "X-Multi: b"},
		},
	)
	assert.Equal(t, goplum.StateGood, result.State)
	assert.Equal(t, "tenant-1", received.headers.Get("X-Scope-Orgid"))
	assert.Equal(t, []string{"a", "b"}, received.headers.Values("X-Multi"))
	assert.Empty(t, received.headers.Get("Authorization"))
}

func TestQueryCheck_SendsBasicAuth(t *testing.T) {
	server, received := fixtureServer(t, http.StatusOK, "healthy.json")
	server.Start()

	execute(t, &Plugin{Url: server.URL, Auth: Credentials{Username: "user", Password: "pass"}})
	assert.Equal(t, "Basic dXNlcjpwYXNz", received.headers.Get("Authorization"))
}

func TestQueryCheck_SendsBearerToken(t *testing.T) {
	server, received := fixtureServer(t, http.StatusOK, "healthy.json")
	server.Start()

	execute(t, &Plugin{Url: server.URL, BearerToken: "token123"})
	assert.Equal(t, "Bearer token123", received.headers.Get("Authorization"))
}

type tokenRequest struct {
	form     url.Values
	username string
	password string
}

func tokenServer(t *testing.T, status int) (*httptest.Server, *[]tokenRequest) {
	t.Helper()
	var requests []tokenRequest
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				assert.NoError(t, r.ParseForm())
				username, password, _ := r.BasicAuth()
				requests = append(requests, tokenRequest{form: r.PostForm, username: username, password: password})

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				_, _ = fmt.Fprintf(w, `{"access_token":"token-%d","token_type":"Bearer","expires_in":3600}`, len(requests))
			},
		),
	)
	t.Cleanup(server.Close)
	return server, &requests
}

func oauth2Plugin(apiURL, tokenURL string) *Plugin {
	return &Plugin{
		Url: apiURL,
		OAuth2: OAuth2Config{
			ClientID:     "goplum",
			ClientSecret: "secret",
			TokenURL:     tokenURL,
			Scopes:       []string{"metrics:read"},
			Params:       []string{"resource=https://metrics.example.com"},
		},
	}
}

func TestQueryCheck_OAuth2(t *testing.T) {
	api, received := fixtureServer(t, http.StatusOK, "healthy.json")
	api.Start()
	tokens, requests := tokenServer(t, http.StatusOK)

	plugin := oauth2Plugin(api.URL, tokens.URL)
	require.NoError(t, plugin.Validate())
	check := plugin.Check("query").(QueryCheck)
	check.Query = "up"

	assert.Equal(t, goplum.StateGood, check.Execute(context.Background()).State)
	assert.Equal(t, "Bearer token-1", received.headers.Get("Authorization"))
	require.Len(t, *requests, 1)
	assert.Equal(t, "goplum", (*requests)[0].username)
	assert.Equal(t, "secret", (*requests)[0].password)
	assert.Equal(t, "client_credentials", (*requests)[0].form.Get("grant_type"))
	assert.Equal(t, "metrics:read", (*requests)[0].form.Get("scope"))
	assert.Equal(t, "https://metrics.example.com", (*requests)[0].form.Get("resource"))

	check.Execute(context.Background())
	assert.Equal(t, "Bearer token-1", received.headers.Get("Authorization"))
	assert.Len(t, *requests, 1)

	plugin.tokens.token.Expiry = time.Now().Add(-time.Minute)
	check.Execute(context.Background())
	assert.Equal(t, "Bearer token-2", received.headers.Get("Authorization"))
	assert.Len(t, *requests, 2)
}

func TestQueryCheck_OAuth2Failure(t *testing.T) {
	api, received := fixtureServer(t, http.StatusOK, "healthy.json")
	api.Start()
	tokens, _ := tokenServer(t, http.StatusUnauthorized)

	result := execute(t, oauth2Plugin(api.URL, tokens.URL))
	assert.Equal(t, goplum.StateFailing, result.State)
	assert.Contains(t, result.Detail, "Error fetching oauth2 token")
	assert.Nil(t, received.headers)
}

func TestQueryCheck_TLS(t *testing.T) {
	dir := t.TempDir()
	clientCert, clientKey, clientPool := generateClientCert(t, dir)

	tests := map[string]struct {
		requireClientCert bool
		trustServer       bool
		sendClientCert    bool
		expected          goplum.CheckState
	}{
		"untrusted server":           {trustServer: false, expected: goplum.StateFailing},
		"trusted server":             {trustServer: true, expected: goplum.StateGood},
		"missing client certificate": {requireClientCert: true, trustServer: true, expected: goplum.StateFailing},
		"client certificate": {
			requireClientCert: true,
			trustServer:       true,
			sendClientCert:    true,
			expected:          goplum.StateGood,
		},
	}

	for name, tt := range tests {
		t.Run(
			name,
			func(t *testing.T) {
				server, _ := fixtureServer(t, http.StatusOK, "healthy.json")
				if tt.requireClientCert {
					server.TLS = &tls.Config{ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: clientPool}
				}
				server.StartTLS()

				plugin := &Plugin{Url: server.URL}
				if tt.trustServer {
					plugin.TLS.CAFile = writePEM(t, dir, "ca.pem", "CERTIFICATE", server.Certificate().Raw)
				}
				if tt.sendClientCert {
					plugin.TLS.CertFile = clientCert
					plugin.TLS.KeyFile = clientKey
				}

				assert.Equal(t, tt.expected, execute(t, plugin).State)
			},
		)
	}
}

func generateClientCert(t *testing.T, dir string) (certFile, keyFile string, pool *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "goplum"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		IsCA:         true,

		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	keyDer, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	pool = x509.NewCertPool()
	pool.AddCert(cert)
	certFile = writePEM(t, dir, "client.pem", "CERTIFICATE", der)
	keyFile = writePEM(t, dir, "client-key.pem", "PRIVATE KEY", keyDer)
	return certFile, keyFile, pool
}

func writePEM(t *testing.T, dir, name, blockType string, der []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}), 0o600))
	return path
}

func TestPlugin_Validate(t *testing.T) {
	tests := map[string]struct {
		plugin Plugin
		valid  bool
	}{
		"url only":           {Plugin{Url: "https://example.com/"}, true},
		"missing url":        {Plugin{}, false},
		"url without scheme": {Plugin{Url: "prometheus.example.com"}, false},
		"auth and bearer token": {
			Plugin{Url: "https://example.com/", Auth: Credentials{Username: "user"}, BearerToken: "token"},
			false,
		},
		"oauth2": {*oauth2Plugin("https://example.com/", "https://id.example.com/token"), true},
		"oauth2 and bearer token": {
			func() Plugin {
				p := oauth2Plugin("https://example.com/", "https://id.example.com/token")
				p.BearerToken = "token"
				return *p
			}(),
			false,
		},
		"oauth2 and auth": {
			func() Plugin {
				p := oauth2Plugin("https://example.com/", "https://id.example.com/token")
				p.Auth.Username = "user"
				return *p
			}(),
			false,
		},
		"oauth2 without client_id": {
			Plugin{Url: "https://example.com/", OAuth2: OAuth2Config{ClientSecret: "s", TokenURL: "https://id.example.com/"}},
			false,
		},
		"oauth2 without client_secret": {
			Plugin{Url: "https://example.com/", OAuth2: OAuth2Config{ClientID: "c", TokenURL: "https://id.example.com/"}},
			false,
		},
		"oauth2 without token_url": {
			Plugin{Url: "https://example.com/", OAuth2: OAuth2Config{ClientID: "c", ClientSecret: "s"}},
			false,
		},
		"oauth2 malformed param": {
			func() Plugin {
				p := oauth2Plugin("https://example.com/", "https://id.example.com/token")
				p.OAuth2.Params = []string{"resource"}
				return *p
			}(),
			false,
		},
		"empty header name":    {Plugin{Url: "https://example.com/", Headers: []string{": value"}}, false},
		"malformed header":     {Plugin{Url: "https://example.com/", Headers: []string{"X-Scope-OrgID"}}, false},
		"space in header name": {Plugin{Url: "https://example.com/", Headers: []string{"Bad Name: value"}}, false},
		"control character in header value": {
			Plugin{Url: "https://example.com/", Headers: []string{"X-Name: a\nb"}},
			false,
		},
		"tab in header value": {Plugin{Url: "https://example.com/", Headers: []string{"X-Name: a\tb"}}, true},
		"cert without key":    {Plugin{Url: "https://example.com/", TLS: TLSConfig{CertFile: "cert.pem"}}, false},
		"missing ca file":     {Plugin{Url: "https://example.com/", TLS: TLSConfig{CAFile: "missing.pem"}}, false},
		"ca file without pem": {Plugin{Url: "https://example.com/", TLS: TLSConfig{CAFile: "testdata/empty.json"}}, false},
	}

	for name, tt := range tests {
		t.Run(
			name,
			func(t *testing.T) {
				if tt.valid {
					require.NoError(t, tt.plugin.Validate())
				} else {
					require.Error(t, tt.plugin.Validate())
				}
			},
		)
	}
}
