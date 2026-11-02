package prometheus

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"chameth.com/goplum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type receivedRequest struct {
	path  string
	query string
}

func runQuery(t *testing.T, status int, fixture string) (goplum.Result, receivedRequest) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", fixture))
	require.NoError(t, err)

	var received receivedRequest
	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				received = receivedRequest{path: r.URL.Path, query: r.PostFormValue("query")}
				w.WriteHeader(status)
				_, _ = w.Write(body)
			},
		),
	)
	defer server.Close()

	check := (&Plugin{Url: server.URL + "/prefix/"}).Check("query").(QueryCheck)
	check.Query = "up{job='node'}"
	return check.Execute(context.Background()), received
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

	check := (&Plugin{Url: "http://user:secret@" + server.Listener.Addr().String()}).Check("query").(QueryCheck)
	check.Query = "up"
	result := check.Execute(context.Background())
	assert.Equal(t, goplum.StateFailing, result.State)
	assert.NotContains(t, result.Detail, "secret")
}

func TestPlugin_ValidateRequiresUrl(t *testing.T) {
	require.Error(t, (&Plugin{}).Validate())
	require.Error(t, (&Plugin{Url: "prometheus.example.com"}).Validate())
	require.NoError(t, (&Plugin{Url: "https://example.com/"}).Validate())
}
