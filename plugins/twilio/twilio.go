package twilio

import (
	"encoding/json"
	"fmt"
	"github.com/csmith/goplum"
	"net/http"
	"net/url"
	"strings"
)

type Plugin struct{}

func (h Plugin) Name() string {
	return "twilio"
}

func (h Plugin) Checks() []goplum.CheckType {
	return nil
}

func (h Plugin) Alerts() []goplum.AlertType {
	return []goplum.AlertType{SmsAlertType{}}
}

type SmsParams struct {
	To    string `json:"to"`
	From  string `json:"from"`
	SID   string `json:"sid"`
	Token string `json:"token"`
}

type SmsAlertType struct{}

func (n SmsAlertType) Name() string {
	return "sms"
}

func (n SmsAlertType) Create(config json.RawMessage) (goplum.Alert, error) {
	p := SmsParams{}
	err := json.Unmarshal(config, &p)
	if err != nil {
		return nil, err
	}

	return SmsAlert{p}, nil
}

type SmsAlert struct {
	params SmsParams
}

func (n SmsAlert) Send(name string, _ *goplum.Result, previousState, newState goplum.CheckState) error {
	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", n.params.SID),
		strings.NewReader(url.Values{
			"To":   []string{n.params.To},
			"From": []string{n.params.From},
			"Body": []string{fmt.Sprintf("Check '%s' is now %s, was %s.", name, newState.Name(), previousState.Name())},
		}.Encode()),
	)
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(n.params.SID, n.params.Token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("bad response from Twilio: HTTP %d", res.StatusCode)
	}

	return nil
}
