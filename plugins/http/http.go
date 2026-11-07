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
	"slices"
	"strings"
	"time"

	"chameth.com/goplum"
	"chameth.com/goplum/internal"
)

// client is shared by checks and alerts. It has no overall timeout as checks are bounded by their context,
// which may be configured to be longer than any default we could pick here.
var client = newClient("tcp")

var networkClients = map[string]*http.Client{
	"tcp":  client,
	"tcp4": newClient("tcp4"),
	"tcp6": newClient("tcp6"),
}

var networkFamilies = map[string]string{
	"tcp4": "IPv4",
	"tcp6": "IPv6",
}

func newClient(network string) *http.Client {
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, _, addr string) (net.Conn, error) {
				conn, err := dialer.DialContext(ctx, network, addr)
				if family, ok := networkFamilies[network]; ok && isNoAddressError(err) {
					host, _, _ := net.SplitHostPort(addr)
					return nil, fmt.Errorf("no %s address for %s: %w", family, host, err)
				}
				return conn, err
			},
			ForceAttemptHTTP2:     true,
			DisableKeepAlives:     true,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

func isNoAddressError(err error) bool {
	var addrErr *net.AddrError
	var dnsErr *net.DNSError
	return (errors.As(err, &addrErr) && addrErr.Err == "no suitable address found") ||
		(errors.As(err, &dnsErr) && dnsErr.IsNotFound)
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
			Network:         "tcp",
			ContentExpected: true,
			MinStatusCode:   100,
			MaxStatusCode:   399,
		}
	case "healthcheck":
		return HealthCheck{Network: "tcp"}
	default:
		return nil
	}
}

type Credentials struct {
	Username string
	Password string `redact:"true"`
}

type BaseCheck struct {
	Url     string `redact:"url"`
	Auth    Credentials
	Network string
}

var validNetworks = []string{"tcp", "tcp4", "tcp6"}

func (b BaseCheck) validate() error {
	if err := internal.ValidateHTTPURL(b.Url); err != nil {
		return err
	}

	if !slices.Contains(validNetworks, b.Network) {
		return fmt.Errorf("invalid network %q: must be one of %s", b.Network, strings.Join(validNetworks, ", "))
	}

	return nil
}

func (b BaseCheck) client() *http.Client {
	if c, ok := networkClients[b.Network]; ok {
		return c
	}
	return client
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
		return goplum.FailingResult("Error building request: %v", internal.WithoutURL(err))
	}

	if len(g.Auth.Username) > 0 || len(g.Auth.Password) > 0 {
		req.SetBasicAuth(g.Auth.Username, g.Auth.Password)
	}

	r, err := g.client().Do(req)
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
	if err := g.validate(); err != nil {
		return err
	}

	if g.MinStatusCode < 100 || g.MinStatusCode > g.MaxStatusCode || g.MaxStatusCode > 599 {
		return errors.New("invalid status codes: must satisfy 100 <= min_status_code <= max_status_code <= 599")
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
		return goplum.FailingResult("Error building request: %v", internal.WithoutURL(err))
	}

	if len(h.Auth.Username) > 0 || len(h.Auth.Password) > 0 {
		req.SetBasicAuth(h.Auth.Username, h.Auth.Password)
	}

	r, err := h.client().Do(req)
	if err != nil {
		return goplum.FailingResult("Error making request: %v", err)
	}

	defer r.Body.Close()

	type healthResponse struct {
		Status string `json:"status"`
		Output string `json:"output"`
		Checks map[string][]struct {
			Status string `json:"status"`
			Output string `json:"output"`
		} `json:"checks"`
	}

	res := &healthResponse{}
	if err := json.NewDecoder(r.Body).Decode(res); err != nil {
		return goplum.FailingResult("Error decoding response (HTTP %s): %v", r.Status, err)
	}

	if r.StatusCode >= 400 {
		if res.Output == "" {
			return goplum.FailingResult("Bad response (HTTP %s)", r.Status)
		}
		return goplum.FailingResult("Bad response (HTTP %s): %s", r.Status, res.Output)
	}

	status := h.convert(res.Status)
	if status == goplum.StateIndeterminate {
		return goplum.FailingResult("Unrecognised status %q (HTTP %s)", res.Status, r.Status)
	}

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
	return h.validate()
}

func (h HealthCheck) convert(status string) goplum.CheckState {
	lower := strings.ToLower(status)
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

	if err := internal.AddHeaders(req.Header, w.Headers); err != nil {
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
	if err := internal.ValidateHTTPURL(w.Url); err != nil {
		return err
	}

	return internal.AddHeaders(http.Header{}, w.Headers)
}
