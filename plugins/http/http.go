package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/csmith/goplum"
	"github.com/nelkinda/health-go"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

var client = http.Client{
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		DisableKeepAlives:     true,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

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
		}
	case "healthcheck":
		return HealthCheck{}
	default:
		return nil
	}
}

type Credentials struct {
	Username string
	Password string
}

type BaseCheck struct {
	Url  string
	Auth Credentials
}

type GetCheck struct {
	BaseCheck           `config:",squash"`
	Content             string
	ContentExpected     bool          `config:"content_expected"`
	CertificateValidity time.Duration `config:"certificate_validity"`
}

func (g GetCheck) Execute(ctx context.Context) goplum.Result {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.Url, http.NoBody)
	if err != nil {
		goplum.FailingResult("Error building request: %v", err)
	}

	if len(g.Auth.Username) > 0 || len(g.Auth.Password) > 0 {
		req.SetBasicAuth(g.Auth.Username, g.Auth.Password)
	}

	r, err := client.Do(req)

	if err != nil {
		return goplum.FailingResult("Error making request: %v", err)
	} else if r.StatusCode >= 400 {
		return goplum.FailingResult("Bad status code: %d", r.StatusCode)
	}

	defer r.Body.Close()

	if len(g.Content) > 0 {
		content, err := ioutil.ReadAll(r.Body)
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

		remaining := r.TLS.PeerCertificates[0].NotAfter.Sub(time.Now())
		if remaining < g.CertificateValidity {
			return goplum.FailingResult("Certificate expires in %s", remaining)
		}
	}

	return goplum.GoodResult()
}

func (g GetCheck) Validate() error {
	if len(g.Url) == 0 {
		return fmt.Errorf("missing required argument: url")
	}

	return nil
}

type HealthCheck struct {
	BaseCheck       `config:",squash"`
	CheckComponents bool `config:"check_components"`
}

func (h HealthCheck) Execute(ctx context.Context) goplum.Result {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.Url, http.NoBody)
	if err != nil {
		goplum.FailingResult("Error building request: %v", err)
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
	if len(h.Url) == 0 {
		return fmt.Errorf("missing required argument: url")
	}

	return nil
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
	Url string
}

func (w WebHookAlert) Send(details goplum.AlertDetails) error {
	b, err := json.Marshal(details)
	if err != nil {
		return err
	}

	res, err := client.Post(w.Url, "application/json", bytes.NewReader(b))
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
	if len(w.Url) == 0 {
		return fmt.Errorf("missing required argument: url")
	}

	return nil
}
