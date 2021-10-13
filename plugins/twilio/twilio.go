package twilio

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/csmith/goplum"
)

var client = http.Client{Timeout: 20 * time.Second}

type Plugin struct{}

func (p Plugin) Alert(kind string) goplum.Alert {
	switch kind {
	case "sms":
		return SmsAlert{}
	case "call":
		return CallAlert{}
	default:
		return nil
	}
}

func (p Plugin) Check(_ string) goplum.Check {
	return nil
}

type BaseAlert struct {
	To    string
	From  string
	Sid   string
	Token string
}

func (b BaseAlert) Validate() error {
	if len(b.To) == 0 {
		return fmt.Errorf("missing required argument: to")
	}

	if len(b.From) == 0 {
		return fmt.Errorf("missing required argument: from")
	}

	if len(b.Sid) == 0 {
		return fmt.Errorf("missing required argument: sid")
	}

	if len(b.Token) == 0 {
		return fmt.Errorf("missing required argument: token")
	}

	return nil
}

type SmsAlert struct {
	BaseAlert `config:",squash"`
}

func (s SmsAlert) Send(details goplum.AlertDetails) error {
	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", s.Sid),
		strings.NewReader(url.Values{
			"To":   []string{s.To},
			"From": []string{s.From},
			"Body": []string{details.Text},
		}.Encode()),
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(s.Sid, s.Token)
	res, err := client.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return fmt.Errorf("bad response from Twilio: HTTP %d", res.StatusCode)
	}

	return nil
}

func (s SmsAlert) Validate() error {
	return s.BaseAlert.Validate()
}

type CallAlert struct {
	BaseAlert `config:",squash"`
}

func (c CallAlert) Send(details goplum.AlertDetails) error {
	var b bytes.Buffer
	if err := xml.EscapeText(&b, []byte(details.Text)); err != nil {
		return err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Calls.json", c.Sid),
		strings.NewReader(url.Values{
			"To":    []string{c.To},
			"From":  []string{c.From},
			"Twiml": []string{fmt.Sprintf("<Response><Say>Go plum alert: %s</Say></Response>", b.String())},
		}.Encode()),
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(c.Sid, c.Token)
	res, err := client.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return fmt.Errorf("bad response from Twilio: HTTP %d", res.StatusCode)
	}

	return nil
}

func (c CallAlert) Validate() error {
	return c.BaseAlert.Validate()
}
