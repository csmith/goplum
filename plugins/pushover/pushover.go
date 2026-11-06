package pushover

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"chameth.com/goplum"
)

const (
	maxMessageLength = 1024
	maxURLLength     = 512
)

var (
	client = http.Client{Timeout: 20 * time.Second}
	apiURL = "https://api.pushover.net/1/messages.json"
)

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

type PushSettings struct {
	Priority int
	Sound    string
	Retry    time.Duration
	Expire   time.Duration
}

type MessageAlert struct {
	Token      string
	Key        string
	Devices    []string
	Failing    PushSettings
	Recovering PushSettings
}

func (m MessageAlert) Send(details goplum.AlertDetails) error {
	var settings PushSettings
	if details.NewState == goplum.StateFailing {
		settings = m.Failing
	} else {
		settings = m.Recovering
	}

	// Links go in Pushover's own URL fields, unless they're too long for it to accept.
	var link, linkTitle string
	if details.Link != "" && utf8.RuneCountInString(details.Link) <= maxURLLength {
		link, linkTitle = details.Link, "More details"
		details.Link = ""
	}

	payload, err := json.Marshal(
		struct {
			Token     string `json:"token"`
			User      string `json:"user"`
			Message   string `json:"message"`
			Device    string `json:"device,omitempty"`
			Priority  int    `json:"priority,omitempty"`
			Sound     string `json:"sound,omitempty"`
			Timestamp int64  `json:"timestamp"`
			Retry     int    `json:"retry,omitempty"`
			Expire    int    `json:"expire,omitempty"`
			URL       string `json:"url,omitempty"`
			URLTitle  string `json:"url_title,omitempty"`
		}{
			Token:     m.Token,
			User:      m.Key,
			Message:   details.TextWithLink(maxMessageLength),
			Device:    strings.Join(m.Devices, ","),
			Priority:  settings.Priority,
			Sound:     settings.Sound,
			Retry:     int(settings.Retry.Seconds()),
			Expire:    int(settings.Expire.Seconds()),
			Timestamp: time.Now().Unix(),
			URL:       link,
			URLTitle:  linkTitle,
		},
	)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		apiURL,
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode >= 400 && res.StatusCode < 500 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("error response from pushover: HTTP %d (%s)", res.StatusCode, body)
	} else if res.StatusCode >= 500 {
		return fmt.Errorf("bad response from pushover: HTTP %d", res.StatusCode)
	}

	return nil
}

func (m MessageAlert) Validate() error {
	if len(m.Token) == 0 {
		return errors.New("missing required argument: token")
	}
	if len(m.Key) == 0 {
		return errors.New("missing required argument: key")
	}
	if err := m.validateSettings(m.Failing); err != nil {
		return fmt.Errorf("failing block invalid: %v", err)
	}
	if err := m.validateSettings(m.Recovering); err != nil {
		return fmt.Errorf("recovering block invalid: %v", err)
	}
	return nil
}

func (m MessageAlert) validateSettings(settings PushSettings) error {
	if settings.Priority < -2 || settings.Priority > 2 {
		return errors.New("priority must be in range -2..+2")
	}

	if settings.Retry != 0 && settings.Retry < 30*time.Second {
		return errors.New("retry must be at least 30 seconds")
	}

	if settings.Expire > 10800*time.Second {
		return errors.New("expire must be at most 10800 seconds (3 hours)")
	}

	if settings.Priority == 2 && (settings.Expire == 0 || settings.Retry == 0) {
		return errors.New("expire and retry must be specified for emergency (2) priority")
	}

	return nil
}
