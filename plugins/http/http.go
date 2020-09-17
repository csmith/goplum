package http

import (
	"encoding/json"
	"github.com/csmith/goplum"
	"net/http"
)

type Plugin struct{}

func (h Plugin) Name() string {
	return "http"
}

func (h Plugin) Checks() []goplum.CheckType {
	return []goplum.CheckType{Check{}}
}

func (h Plugin) Alerts() []goplum.AlertType {
	return nil
}

type params struct {
	Url string `json:"url"`
}

type Check struct{}

func (c Check) Name() string {
	return "get"
}

func (c Check) Create(config json.RawMessage) (goplum.Check, error) {
	p := params{}
	err := json.Unmarshal(config, &p)
	if err != nil {
		return nil, err
	}

	return Task{p}, nil
}

type Task struct {
	params params
}

func (t Task) Execute() goplum.Result {
	r, err := http.Get(t.params.Url)
	return goplum.ResultFor(err == nil && r.StatusCode < 400)
}
