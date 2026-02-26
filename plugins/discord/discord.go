package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"chameth.com/goplum"
)

var client = http.Client{Timeout: 20 * time.Second}

type Plugin struct{}

func (p Plugin) Alert(kind string) goplum.Alert {
	switch kind {
	case "message":
		return MessageAlert{}
	default:
		return nil
	}
}

func (p Plugin) Check(_ string) goplum.Check {
	return nil
}

type MessageAlert struct {
	Url string
}

func (m MessageAlert) Send(details goplum.AlertDetails) error {
	payload, err := json.Marshal(struct {
		Content string `json:"content"`
	}{
		details.Text,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, m.Url, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return fmt.Errorf("bad response from Discord: HTTP %d", res.StatusCode)
	}

	return nil
}

func (m MessageAlert) Validate() error {
	if len(m.Url) == 0 {
		return fmt.Errorf("missing required argument: url")
	}

	return nil
}
