package http

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"chameth.com/goplum"
	"github.com/nelkinda/health-go"
)

// client is shared by checks and alerts. It has no overall timeout as checks are bounded by their context,
// which may be configured to be longer than any default we could pick here.
var client = http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:     true,
		DisableKeepAlives:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

var webhookTimeout = 20 * time.Second

type Plugin struct{}

func (p Plugin) Alert(kind string) goplum.Alert {
	switch kind {
	case "webhook":
		return WebHookAlert{}
	default:
		return nil
	}
}

func (p Plugin) Check(kind string) goplum.Check {
	switch kind {
	case "get":
		return GetCheck{
			ContentExpected: true,
			MinStatusCode:   100,
			MaxStatusCode:   399,
		}
	case "healthcheck":
		return HealthCheck{}
	default:
		return nil
	}
}

type Credentials struct {
	Username string
	Password string `redact:"true"`
}

type BaseCheck struct {
	Url  string `redact:"url"`
	Auth Credentials
}

type GetCheck struct {
	BaseCheck           `config:",squash"`
	Content             string
	ContentExpected     bool          `config:"content_expected"`
	CertificateValidity time.Duration `config:"certificate_validity"`
	MinStatusCode       int           `config:"min_status_code"`
	MaxStatusCode       int           `config:"max_status_code"`
}

func (g GetCheck) Execute(ctx context.Context) goplum.Result {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.Url, http.NoBody)
	if err != nil {
		return goplum.FailingResult("Error building request: %v", withoutUrl(err))
	}

	if len(g.Auth.Username) > 0 || len(g.Auth.Password) > 0 {
		req.SetBasicAuth(g.Auth.Username, g.Auth.Password)
	}

	r, err := client.Do(req)
	if err != nil {
		return goplum.FailingResult("Error making request: %v", err)
	}

	defer r.Body.Close()

	if r.StatusCode < g.MinStatusCode || r.StatusCode > g.MaxStatusCode {
		return goplum.FailingResult("Bad status code: %d", r.StatusCode)
	}

	if len(g.Content) > 0 {
		content, err := io.ReadAll(r.Body)
		if err != nil {
			return goplum.FailingResult("Error reading response body: %v", err)
		}

		// TODO: It would be nice to scan the body instead of having to read it all into memory
		// TODO: Add options around case sensitivity/consider allowing regexp
		found := strings.Contains(string(content), g.Content)
		if !found && g.ContentExpected {
			return goplum.FailingResult("Body does not contain '%s'", g.Content)
		} else if found && !g.ContentExpected {
			return goplum.FailingResult("Body contains '%s'", g.Content)
		}
	}

	if g.CertificateValidity > 0 {
		if r.TLS == nil {
			return goplum.FailingResult("Connection did not use TLS")
		}

		remaining := time.Until(r.TLS.PeerCertificates[0].NotAfter)
		if remaining < g.CertificateValidity {
			return goplum.FailingResult("Certificate expires in %s", remaining)
		}
	}

	return goplum.GoodResult()
}

func (g GetCheck) Validate() error {
	return validateUrl(g.Url)
}

type HealthCheck struct {
	BaseCheck       `config:",squash"`
	CheckComponents bool `config:"check_components"`
}

func (h HealthCheck) Execute(ctx context.Context) goplum.Result {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.Url, http.NoBody)
	if err != nil {
		return goplum.FailingResult("Error building request: %v", withoutUrl(err))
	}

	if len(h.Auth.Username) > 0 || len(h.Auth.Password) > 0 {
		req.SetBasicAuth(h.Auth.Username, h.Auth.Password)
	}

	r, err := client.Do(req)
	if err != nil {
		return goplum.FailingResult("Error making request: %v", err)
	}

	defer r.Body.Close()

	res := &health.Health{}
	if err := json.NewDecoder(r.Body).Decode(res); err != nil {
		return goplum.FailingResult("Error decoding response: %v", err)
	}

	status := h.convert(res.Status)
	detail := res.Output

	if status == goplum.StateGood && h.CheckComponents {
		for name, checks := range res.Checks {
			for i := range checks {
				check := checks[i]
				checkStatus := h.convert(check.Status)
				if checkStatus > status {
					status = checkStatus
					detail = fmt.Sprintf("component %s: %s", name, check.Output)
				}
			}
		}
	}

	return goplum.Result{State: status, Detail: detail, Time: time.Now()}
}

func (h HealthCheck) Validate() error {
	return validateUrl(h.Url)
}

func (h HealthCheck) convert(status health.Status) goplum.CheckState {
	lower := strings.ToLower(string(status))
	if lower == "pass" || lower == "ok" || lower == "up" {
		return goplum.StateGood
	} else if lower == "fail" || lower == "error" || lower == "down" {
		return goplum.StateFailing
	} else if lower == "warn" {
		return goplum.StateGood // TODO: Change when goplum implements warnings
	} else {
		return goplum.StateIndeterminate
	}
}

type WebHookAlert struct {
	Url     string
	Headers []string
	Secret  string
}

func (w WebHookAlert) Send(details goplum.AlertDetails) error {
	b, err := json.Marshal(details)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), webhookTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.Url, bytes.NewReader(b))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	if len(w.Secret) > 0 {
		mac := hmac.New(sha256.New, []byte(w.Secret))
		mac.Write(b)
		req.Header.Set("X-Goplum-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}

	if err := addHeaders(req.Header, w.Headers); err != nil {
		return err
	}

	res, err := client.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return fmt.Errorf("bad response from webhook: HTTP %d", res.StatusCode)
	}

	return nil
}

func (w WebHookAlert) Validate() error {
	if err := validateUrl(w.Url); err != nil {
		return err
	}

	return addHeaders(http.Header{}, w.Headers)
}

const headerNameChars = "!#$%&'*+-.^_`|~0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// addHeaders parses "Name: Value" entries into dst. It rejects anything net/http would refuse to send, so entries
// that pass here can't fail later for being malformed. Errors give the entry's position instead of its content, as
// it may contain credentials.
func addHeaders(dst http.Header, headers []string) error {
	for i, h := range headers {
		name, value, ok := strings.Cut(h, ":")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)

		invalidValue := strings.ContainsFunc(
			value,
			func(r rune) bool {
				return (r < ' ' && r != '\t') || r == 0x7f
			},
		)
		if !ok || len(name) == 0 || strings.Trim(name, headerNameChars) != "" || invalidValue {
			return fmt.Errorf("invalid header at position %d, expected format \"Name: Value\"", i+1)
		}

		dst.Add(name, value)
	}

	return nil
}

func validateUrl(u string) error {
	if len(u) == 0 {
		return errors.New("missing required argument: url")
	}

	if _, err := url.Parse(u); err != nil {
		return fmt.Errorf("invalid url: %w", withoutUrl(err))
	}

	return nil
}

// withoutUrl strips the URL from a url.Error, as it may contain credentials.
func withoutUrl(err error) error {
	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		return urlErr.Err
	}
	return err
}
