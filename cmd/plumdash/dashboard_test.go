package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"chameth.com/goplum/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type fakeLister struct {
	checks []*api.Check
	err    error
}

func (f *fakeLister) GetChecks(context.Context, *api.Empty, ...grpc.CallOption) (*api.CheckList, error) {
	return &api.CheckList{Checks: f.checks}, f.err
}

func serve(t *testing.T, lister checkLister, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	newDashboard(lister).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	return rec
}

func TestDashboard_ShowsChecksSortedByName(t *testing.T) {
	rec := serve(
		t,
		&fakeLister{checks: []*api.Check{
			{Name: "zebra", Type: "http.get", State: api.Status_FAILING},
			{Name: "aardvark", Type: "network.connect", State: api.Status_GOOD, Suspended: true},
		}},
		"/",
	)

	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Regexp(t, `(?s)class="check Good".*aardvark.*Suspended · Good.*class="check Failing".*zebra`, body)
}

func TestDashboard_ShowsHistoryOldestFirst(t *testing.T) {
	rec := serve(
		t,
		&fakeLister{checks: []*api.Check{{
			Name: "web",
			History: []*api.Result{
				{Time: 300, Result: api.Status_FAILING, Detail: "timed out"},
				{Time: 200, Result: api.Status_GOOD},
				{Time: 100, Result: api.Status_INDETERMINATE},
			},
		}}},
		"/",
	)

	require.Equal(t, http.StatusOK, rec.Code)
	boxes := regexp.MustCompile(`<span class="(\w+)" title="([^"]*)">`).FindAllStringSubmatch(rec.Body.String(), -1)
	require.Len(t, boxes, 3)
	assert.Equal(t, []string{"Unknown", "Good", "Failing"}, []string{boxes[0][1], boxes[1][1], boxes[2][1]})
	assert.Equal(t, "1970-01-01 00:05:00 UTC: Failing - timed out", boxes[2][2])
}

func TestDashboard_EscapesCheckDetails(t *testing.T) {
	rec := serve(
		t,
		&fakeLister{checks: []*api.Check{{
			Name:    "<b>web</b>",
			History: []*api.Result{{Time: 100, Detail: `"><script>`}},
		}}},
		"/",
	)

	assert.NotContains(t, rec.Body.String(), "<b>web</b>")
	assert.NotContains(t, rec.Body.String(), `"><script>`)
}

func TestDashboard_ReturnsBadGatewayWhenApiFails(t *testing.T) {
	rec := serve(t, &fakeLister{err: errors.New("connection refused")}, "/")

	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestDashboard_OnlyServesRoot(t *testing.T) {
	rec := serve(t, &fakeLister{}, "/other")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
