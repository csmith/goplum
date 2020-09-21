package twilio

import (
	"fmt"
	"github.com/csmith/goplum"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var client = http.Client{Timeout: 20 * time.Second}

type Plugin struct{}

func (p Plugin) Alert(kind string) goplum.Alert {
	switch kind {
	case "sms":
		return SmsAlert{}
	default:
		return nil
	}
}

func (p Plugin) Check(_ string) goplum.Check {
	return nil
}

type SmsAlert struct {
	To    string
	From  string
	Sid   string
	Token string
}

func (n SmsAlert) Send(details goplum.AlertDetails) error {
	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", n.Sid),
		strings.NewReader(url.Values{
			"To":   []string{n.To},
			"From": []string{n.From},
			"Body": []string{details.Text},
		}.Encode()),
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(n.Sid, n.Token)
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

func (n SmsAlert) Validate() error {
	if len(n.To) == 0 {
		return fmt.Errorf("missing required argument: to")
	}

	if len(n.From) == 0 {
		return fmt.Errorf("missing required argument: from")
	}

	if len(n.Sid) == 0 {
		return fmt.Errorf("missing required argument: sid")
	}

	if len(n.Token) == 0 {
		return fmt.Errorf("missing required argument: token")
	}

	return nil
}
