package http

import (
	"encoding/json"
	"github.com/csmith/goplum"
	"io/ioutil"
	"net/http"
	"strings"
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
	Url     string `json:"url"`
	Content string `json:"content"`
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

	if err != nil || r.StatusCode >= 400 {
		return goplum.Result{State: goplum.StateFailing}
	}

	if len(t.params.Content) > 0 {
		defer r.Body.Close()
		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return goplum.Result{State: goplum.StateFailing}
		}

		// TODO: It would be nice to scan the body instead of having to read it all into memory
		// TODO: Add options around case sensitivity/consider allowing regexp
		if !strings.Contains(string(content), t.params.Content) {
			return goplum.Result{State: goplum.StateFailing}
		}
	}

	return goplum.Result{State: goplum.StateGood}
}
