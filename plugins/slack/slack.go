package slack

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/csmith/goplum"
	"net/http"
	"time"
)

var client = http.Client{Timeout: 20 * time.Second}

type Plugin struct{}

func (h Plugin) Name() string {
	return "slack"
}

func (h Plugin) Checks() []goplum.CheckType {
	return nil
}

func (h Plugin) Alerts() []goplum.AlertType {
	return []goplum.AlertType{MessageAlertType{}}
}

type MessageParams struct {
	Url string `json:"url"`
}

type MessageAlertType struct{}

func (n MessageAlertType) Name() string {
	return "message"
}

func (n MessageAlertType) Create(config json.RawMessage) (goplum.Alert, error) {
	p := MessageParams{}
	err := json.Unmarshal(config, &p)
	if err != nil {
		return nil, err
	}

	return MessageAlert{p}, nil
}

type MessageAlert struct {
	params MessageParams
}

func (m MessageAlert) Send(details goplum.AlertDetails) error {
	payload, err := json.Marshal(struct {
		Text string `json:"text"`
	}{
		details.Text,
	})

	req, err := http.NewRequest(http.MethodPost, m.params.Url, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("bad response from Slack: HTTP %d", res.StatusCode)
	}

	return nil
}
