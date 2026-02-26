package http

import (
	"context"
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
