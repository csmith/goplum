package heartbeat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"chameth.com/goplum"
)

type Plugin struct {
	Port int
	Path string

	checks map[string]*ReceivedCheck
}

func (p *Plugin) Alert(_ string) goplum.Alert {
	return nil
}

func (p *Plugin) Check(kind string) goplum.Check {
	switch kind {
	case "received":
		return &ReceivedCheck{
			plugin:  p,
			created: time.Now(),
		}
	default:
		return nil
	}
}

func (p *Plugin) Validate() error {
	if p.Port == 0 {
		return errors.New("port must be specified")
	}

	l, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", fmt.Sprintf(":%d", p.Port))
	if err != nil {
		return err
	}
	slog.Info("Heartbeat plugin listening", "port", p.Port)

	p.Path = strings.ReplaceAll(fmt.Sprintf("/%s/", p.Path), "//", "/")

	// Hand the registered checks over to the server, so that checks from a later config load can reuse their IDs.
	handler := &receiver{path: p.Path, checks: p.checks}
	p.checks = nil

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		IdleTimeout:       time.Minute,
	}
	go server.Serve(l)
	return nil
}

type receiver struct {
	path   string
	checks map[string]*ReceivedCheck
}

func (r *receiver) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !strings.HasPrefix(request.URL.Path, r.path) {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	id := strings.ToLower(strings.TrimPrefix(request.URL.Path, r.path))
	if check, ok := r.checks[id]; ok {
		slog.Info("Received heartbeat")
		check.received = time.Now()
		writer.WriteHeader(http.StatusAccepted)
		return
	}

	writer.WriteHeader(http.StatusNotFound)
}

type ReceivedCheck struct {
	ID     string `config:"id" redact:"true"`
	Within time.Duration

	plugin   *Plugin
	created  time.Time
	received time.Time
}

func (g *ReceivedCheck) Execute(_ context.Context) goplum.Result {
	if g.received.IsZero() {
		// We've not received a heartbeat since we started
		if delta := time.Since(g.created); delta > g.Within {
			return goplum.FailingResult("No heartbeat received in %s", delta)
		}
		return goplum.IndeterminateResult("No heartbeat received since monitoring started at %s", g.created)
	}

	if delta := time.Since(g.received); delta > g.Within {
		return goplum.FailingResult("No heartbeat received in %s", delta)
	}
	return goplum.GoodResult()
}

var idRegex = regexp.MustCompile("^[0-9a-f]{32}$")

func (g *ReceivedCheck) Validate() error {
	g.ID = strings.ToLower(g.ID)
	if !idRegex.MatchString(g.ID) {
		return errors.New("id must be a 32-character hexadecimal string")
	}

	if g.Within < 30*time.Second {
		return errors.New("within must be at least 30 seconds")
	}

	if existing, ok := g.plugin.checks[g.ID]; ok && existing != g {
		return errors.New("id is already used by another heartbeat check")
	}

	if g.plugin.checks == nil {
		g.plugin.checks = make(map[string]*ReceivedCheck)
	}
	g.plugin.checks[g.ID] = g
	return nil
}

type SavedState struct {
	Created  time.Time
	Received time.Time
}

func (g *ReceivedCheck) Save() any {
	return SavedState{
		Created:  g.created,
		Received: g.received,
	}
}

func (g *ReceivedCheck) Restore(restorer func(any)) {
	state := SavedState{}
	restorer(&state)
	g.created = state.Created
	g.received = state.Received
}
