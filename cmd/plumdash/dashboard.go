package main

import (
	"bytes"
	"context"
	_ "embed"
	"html/template"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"chameth.com/goplum/api"
	"google.golang.org/grpc"
)

type checkLister interface {
	GetChecks(ctx context.Context, in *api.Empty, opts ...grpc.CallOption) (*api.CheckList, error)
}

//go:embed dashboard.html
var dashboardTemplate string

var tmpl = template.Must(template.New("dashboard").Parse(dashboardTemplate))

func newDashboard(client checkLister) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(
		"GET /{$}",
		func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()

			checks, err := client.GetChecks(ctx, &api.Empty{})
			if err != nil {
				slog.Error("Unable to retrieve checks", "error", err)
				http.Error(w, "Unable to retrieve checks from GoPlum", http.StatusBadGateway)
				return
			}

			buf := &bytes.Buffer{}
			if err := tmpl.Execute(buf, buildView(checks.GetChecks())); err != nil {
				slog.Error("Unable to render dashboard", "error", err)
				http.Error(w, "Unable to render dashboard", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = buf.WriteTo(w)
		},
	)
	return mux
}

type checkView struct {
	Name      string
	Type      string
	State     string
	Suspended bool
	LastRun   string
	History   []resultView
}

type resultView struct {
	State   string
	Summary string
}

func buildView(checks []*api.Check) []checkView {
	res := make([]checkView, 0, len(checks))
	for _, c := range checks {
		history := make([]resultView, 0, len(c.GetHistory()))
		for _, r := range c.GetHistory() {
			summary := formatTime(r.GetTime()) + ": " + stateName(r.GetResult())
			if r.GetDetail() != "" {
				summary += " - " + r.GetDetail()
			}
			history = append(
				history,
				resultView{
					State:   stateName(r.GetResult()),
					Summary: summary,
				},
			)
		}
		// The API gives the newest result first, but the dashboard reads left to right.
		slices.Reverse(history)

		res = append(
			res,
			checkView{
				Name:      c.GetName(),
				Type:      c.GetType(),
				State:     stateName(c.GetState()),
				Suspended: c.GetSuspended(),
				LastRun:   formatTime(c.GetLastRun()),
				History:   history,
			},
		)
	}

	slices.SortFunc(
		res,
		func(a, b checkView) int {
			return strings.Compare(a.Name, b.Name)
		},
	)
	return res
}

func stateName(s api.Status) string {
	switch s {
	case api.Status_GOOD:
		return "Good"
	case api.Status_FAILING:
		return "Failing"
	default:
		return "Unknown"
	}
}

func formatTime(unix int64) string {
	if unix <= 0 {
		return "Never"
	}
	return time.Unix(unix, 0).UTC().Format("2006-01-02 15:04:05 UTC")
}
