package smtp

import (
	"fmt"
	"net"
	"net/smtp"
	"regexp"
	"strings"

	"chameth.com/goplum"
	"github.com/mitchellh/mapstructure"
)

type Plugin struct{}

func (p Plugin) Alert(kind string) goplum.Alert {
	switch kind {
	case "send":
		return SendAlert{
			SubjectPrefix: "Goplum alert: ",
		}
	default:
		return nil
	}
}

func (p Plugin) Check(_ string) goplum.Check {
	return nil
}

type SendAlert struct {
	Server        string
	Username      string
	Password      string
	SubjectPrefix string `config:"subject_prefix"`
	From          string
	To            string
}

func (s SendAlert) Send(details goplum.AlertDetails) error {
	host, _, _ := net.SplitHostPort(s.Server)
	auth := smtp.PlainAuth("", s.Username, s.Password, host)
	body := fmt.Sprintf(
		"To: %s\r\nSubject: %s%s\r\nFrom: %s\r\n\r\n%s\r\n",
		s.To,
		s.SubjectPrefix,
		details.Text,
		s.From,
		s.body(details),
	)
	return smtp.SendMail(s.Server, auth, s.From, []string{s.To}, []byte(body))
}

func (s SendAlert) body(details goplum.AlertDetails) string {
	settings := make(map[string]any)
	dec, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:          "config",
		Result:           &settings,
		WeaklyTypedInput: true,
	})
	_ = dec.Decode(details.Config)

	config := strings.Builder{}
	for k, v := range settings {
		config.WriteString(fmt.Sprintf("\t%s = %v\r\n", k, v))
	}

	return fmt.Sprintf(
		"Check '%s' (type %s) is now %s (was: %s).\r\n\r\nCheck message: %s\r\n\r\nCheck config:\r\n%s\r\n",
		details.Name,
		details.Type,
		details.NewState,
		details.PreviousState,
		details.LastResult.Detail,
		config.String(),
	)
}

var basicEmailRegex = regexp.MustCompile(`^[^@]+@[^@]+$`)

func (s SendAlert) Validate() error {
	if _, _, err := net.SplitHostPort(s.Server); err != nil {
		return fmt.Errorf("invalid server: %v", err)
	}

	if !basicEmailRegex.MatchString(s.From) {
		return fmt.Errorf("invalid from address")
	}

	if !basicEmailRegex.MatchString(s.To) {
		return fmt.Errorf("invalid to address")
	}

	return nil
}
