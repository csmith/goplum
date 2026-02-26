package heartbeat

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/csmith/goplum"
)

var checks = make(map[string]*ReceivedCheck)

type Plugin struct {
	Port int
	Path string
}

func (p *Plugin) Alert(_ string) goplum.Alert {
	return nil
}

func (p *Plugin) Check(kind string) goplum.Check {
	switch kind {
	case "received":
		return &ReceivedCheck{
			created: time.Now(),
		}
	default:
		return nil
	}
}

func (p *Plugin) Validate() error {
	if p.Port == 0 {
		return fmt.Errorf("port must be specified")
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", p.Port))
	if err != nil {
		return err
	}
	log.Printf("Heartbeat plugin listening on port %d", p.Port)

	p.Path = strings.ReplaceAll(fmt.Sprintf("/%s/", p.Path), "//", "/")

	go http.Serve(l, p)
	return nil
}

func (p *Plugin) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !strings.HasPrefix(request.URL.Path, p.Path) {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	id := strings.ToLower(strings.TrimPrefix(request.URL.Path, p.Path))
	if check, ok := checks[id]; ok {
		log.Printf("Received heartbeat with ID %s", id)
		check.received = time.Now()
		writer.WriteHeader(http.StatusAccepted)
		return
	}

	writer.WriteHeader(http.StatusNotFound)
}

type ReceivedCheck struct {
	ID     string `config:"id"`
	Within time.Duration

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
		return fmt.Errorf("id must be a 32-character hexadecimal string")
	}

	if g.Within < 30*time.Second {
		return fmt.Errorf("within must be at least 30 seconds")
	}

	checks[g.ID] = g
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
