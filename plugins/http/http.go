package http

import (
	"encoding/json"
	"github.com/csmith/goplum"
	"net/http"
	"time"
)

var client = http.Client{Timeout: 20 * time.Second}

type Plugin struct{}

func (h Plugin) Name() string {
	return "http"
}

func (h Plugin) Checks() []goplum.CheckType {
	return []goplum.CheckType{GetCheckType{}}
}

func (h Plugin) Alerts() []goplum.AlertType {
	return nil
}

type GetParams struct {
	Url string `json:"url"`
}

type GetCheckType struct{}

func (c GetCheckType) Name() string {
	return "get"
}

func (c GetCheckType) Create(config json.RawMessage) (goplum.Check, error) {
	p := GetParams{}
	err := json.Unmarshal(config, &p)
	if err != nil {
		return nil, err
	}

	return GetCheck{p}, nil
}

type GetCheck struct {
	params GetParams
}

func (t GetCheck) Execute() goplum.Result {
	r, err := client.Get(t.params.Url)
	return goplum.ResultFor(err == nil && r.StatusCode < 400)
}
