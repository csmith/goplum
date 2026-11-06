package smtp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"regexp"
	"strings"
	"time"

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
	return sendMail(
		s.Server,
		smtp.PlainAuth("", s.Username, s.Password, host),
		s.From,
		s.To,
		[]byte(
			fmt.Sprintf(
				"To: %s\r\nSubject: %s\r\nFrom: %s\r\n\r\n%s\r\n",
				s.To,
				subject(s.SubjectPrefix+details.Text),
				s.From,
				s.body(details),
			),
		),
	)
}

const maxSubjectLength = 200

var lineBreaks = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ")

// subject makes text fit on a single header line. The full check detail is still available in the message body.
func subject(text string) string {
	return goplum.Truncate(lineBreaks.Replace(text), maxSubjectLength)
}

var sendTimeout = 20 * time.Second

// sendMail behaves like smtp.SendMail, but bounds the whole exchange by sendTimeout.
func sendMail(addr string, auth smtp.Auth, from, to string, msg []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(sendTimeout)); err != nil {
		return err
	}

	host, _, _ := net.SplitHostPort(addr)
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.Hello("localhost"); err != nil {
		return err
	}

	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: host}); err != nil {
			return err
		}
	}

	if ok, _ := c.Extension("AUTH"); !ok {
		return errors.New("smtp: server doesn't support AUTH")
	}

	if err := c.Auth(auth); err != nil {
		return err
	}

	if err := c.Mail(from); err != nil {
		return err
	}

	if err := c.Rcpt(to); err != nil {
		return err
	}

	w, err := c.Data()
	if err != nil {
		return err
	}

	if _, err := w.Write(msg); err != nil {
		return err
	}

	if err := w.Close(); err != nil {
		return err
	}

	return c.Quit()
}

var basicEmailRegex = regexp.MustCompile(`^[^@]+@[^@]+$`)

func (s SendAlert) Validate() error {
	if _, _, err := net.SplitHostPort(s.Server); err != nil {
		return fmt.Errorf("invalid server: %v", err)
	}

	if !basicEmailRegex.MatchString(s.From) {
		return errors.New("invalid from address")
	}

	if !basicEmailRegex.MatchString(s.To) {
		return errors.New("invalid to address")
	}

	return nil
}

func (s SendAlert) body(details goplum.AlertDetails) string {
	settings := make(map[string]any)
	dec, _ := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			TagName:          "config",
			Result:           &settings,
			WeaklyTypedInput: true,
		},
	)
	_ = dec.Decode(details.Config)

	config := strings.Builder{}
	for k, v := range settings {
		config.WriteString(fmt.Sprintf("\t%s = %v\r\n", k, v))
	}

	link := ""
	if details.Link != "" {
		link = fmt.Sprintf("Link: %s\r\n\r\n", details.Link)
	}

	return fmt.Sprintf(
		"Check '%s' (type %s) is now %s (was: %s).\r\n\r\nCheck message: %s\r\n\r\n%sCheck config:\r\n%s\r\n",
		details.Name,
		details.Type,
		details.NewState,
		details.PreviousState,
		details.LastResult.Detail,
		link,
		config.String(),
	)
}
