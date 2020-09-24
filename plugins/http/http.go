package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/csmith/goplum"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

var client = http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
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
		return GetCheck{}
	default:
		return nil
	}
}

type GetCheck struct {
	Url                 string
	Content             string
	CertificateValidity time.Duration `config:"certificate_validity"`
}

func (g GetCheck) Execute(ctx context.Context) goplum.Result {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.Url, http.NoBody)
	if err != nil {
		goplum.FailingResult("Error building request: %v", err)
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
		if !strings.Contains(string(content), g.Content) {
			return goplum.FailingResult("Body does not contain '%s'", g.Content)
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
