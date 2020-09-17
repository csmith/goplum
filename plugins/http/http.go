package http

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	return []goplum.AlertType{WebHookAlertType{}}
}

type GetParams struct {
	Url         string              `json:"url"`
	Content     string              `json:"content"`
	Certificate *CertificateOptions `json:"certificate"`
}

type CertificateOptions struct {
	ValidFor goplum.Duration `json:"valid_for"`
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

	if err != nil {
		return goplum.FailingResult("Error making request: %v", err)
	} else if r.StatusCode >= 400 {
		return goplum.FailingResult("Bad status code: %d", r.StatusCode)
	}

	if len(t.params.Content) > 0 {
		defer r.Body.Close()
		content, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return goplum.FailingResult("Error reading response body: %v", err)
		}

		// TODO: It would be nice to scan the body instead of having to read it all into memory
		// TODO: Add options around case sensitivity/consider allowing regexp
		if !strings.Contains(string(content), t.params.Content) {
			return goplum.FailingResult("Body does not contain '%s'", t.params.Content)
		}
	}

	if t.params.Certificate != nil {
		if r.TLS == nil {
			return goplum.FailingResult("Connection did not use TLS")
		}

		remaining := r.TLS.PeerCertificates[0].NotAfter.Sub(time.Now())
		if remaining < time.Duration(t.params.Certificate.ValidFor) {
			return goplum.FailingResult("Certificate expires in %s", remaining)
		}
	}

	return goplum.GoodResult()
}

type WebHookParams struct {
	Url string `json:"url"`
}

type WebHookAlertType struct{}

func (w WebHookAlertType) Name() string {
	return "webhook"
}

func (w WebHookAlertType) Create(config json.RawMessage) (goplum.Alert, error) {
	p := WebHookParams{}
	err := json.Unmarshal(config, &p)
	if err != nil {
		return nil, err
	}

	return WebHookAlert{p}, nil
}

type WebHookAlert struct {
	params WebHookParams
}

func (w WebHookAlert) Send(name, checkType string, params interface{}, lastResult *goplum.Result, previousState, newState goplum.CheckState) error {
	type Transition struct {
		From string `json:"from"`
		To   string `json:"to"`
	}

	type Payload struct {
		Text       string         `json:"text"`
		Name       string         `json:"name"`
		Type       string         `json:"type"`
		Config     interface{}    `json:"config"`
		LastResult *goplum.Result `json:"last_result"`
		Transition Transition     `json:"transition"`
	}

	b, err := json.Marshal(Payload{
		Text:       fmt.Sprintf("Check '%s' is now %s, was %s.", name, newState.Name(), previousState.Name()),
		Name:       name,
		Type:       checkType,
		Config:     params,
		LastResult: lastResult,
		Transition: Transition{
			From: previousState.Name(),
			To:   newState.Name(),
		},
	})
	if err != nil {
		return err
	}

	res, err := http.Post(w.params.Url, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("bad response from webhook: HTTP %d", res.StatusCode)
	}

	return nil
}
