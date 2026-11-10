package prometheus

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"

	"chameth.com/goplum"
	"chameth.com/goplum/internal"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const maxSeriesInDetail = 5

type Plugin struct {
	Url         string
	Auth        Credentials
	BearerToken string       `config:"bearer_token"`
	OAuth2      OAuth2Config `config:"oauth2"`
	Headers     []string
	TLS         TLSConfig `config:"tls"`

	headers http.Header
	client  *http.Client
	tokens  *tokenCache
}

type Credentials struct {
	Username string
	Password string
}

type TLSConfig struct {
	CAFile   string `config:"ca_file"`
	CertFile string `config:"cert_file"`
	KeyFile  string `config:"key_file"`
}

func (p *Plugin) Alert(_ string) goplum.Alert {
	return nil
}

func (p *Plugin) Check(kind string) goplum.Check {
	switch kind {
	case "query":
		return QueryCheck{plugin: p}
	default:
		return nil
	}
}

func (p *Plugin) Validate() error {
	if err := internal.ValidateHTTPURL(p.Url); err != nil {
		return err
	}

	authMethods := 0
	for _, used := range []bool{
		len(p.Auth.Username) > 0 || len(p.Auth.Password) > 0,
		len(p.BearerToken) > 0,
		p.OAuth2.configured(),
	} {
		if used {
			authMethods++
		}
	}
	if authMethods > 1 {
		return errors.New("only one of auth, bearer_token and oauth2 can be specified")
	}

	p.tokens = nil
	if p.OAuth2.configured() {
		config, err := p.OAuth2.build()
		if err != nil {
			return fmt.Errorf("invalid oauth2 config: %w", err)
		}
		p.tokens = &tokenCache{config: config}
	}

	p.headers = make(http.Header)
	if err := internal.AddHeaders(p.headers, p.Headers); err != nil {
		return err
	}

	tlsConfig, err := p.TLS.build()
	if err != nil {
		return err
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	p.client = &http.Client{Transport: transport}
	return nil
}

func (t TLSConfig) build() (*tls.Config, error) {
	config := &tls.Config{}

	if len(t.CAFile) > 0 {
		pem, err := os.ReadFile(t.CAFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read ca_file: %w", err)
		}

		config.RootCAs = x509.NewCertPool()
		if !config.RootCAs.AppendCertsFromPEM(pem) {
			return nil, errors.New("no certificates found in ca_file")
		}
	}

	if len(t.CertFile) > 0 || len(t.KeyFile) > 0 {
		if len(t.CertFile) == 0 || len(t.KeyFile) == 0 {
			return nil, errors.New("cert_file and key_file must be specified together")
		}

		cert, err := tls.LoadX509KeyPair(t.CertFile, t.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("unable to load client certificate: %w", err)
		}
		config.Certificates = []tls.Certificate{cert}
	}

	return config, nil
}

type OAuth2Config struct {
	ClientID     string `config:"client_id"`
	ClientSecret string `config:"client_secret"`
	TokenURL     string `config:"token_url"`
	Scopes       []string
	Params       []string
}

func (o OAuth2Config) configured() bool {
	return len(o.ClientID) > 0 || len(o.ClientSecret) > 0 || len(o.TokenURL) > 0 ||
		len(o.Scopes) > 0 || len(o.Params) > 0
}

func (o OAuth2Config) build() (*clientcredentials.Config, error) {
	if len(o.ClientID) == 0 {
		return nil, errors.New("missing required argument: client_id")
	}

	if len(o.ClientSecret) == 0 {
		return nil, errors.New("missing required argument: client_secret")
	}

	if len(o.TokenURL) == 0 {
		return nil, errors.New("missing required argument: token_url")
	}

	if err := internal.ValidateHTTPURL(o.TokenURL); err != nil {
		return nil, fmt.Errorf("token_url: %w", err)
	}

	params := url.Values{}
	for i, param := range o.Params {
		name, value, ok := strings.Cut(param, "=")
		if !ok || len(name) == 0 {
			return nil, fmt.Errorf("invalid param at position %d, expected format \"name=value\"", i+1)
		}
		params.Add(name, value)
	}

	return &clientcredentials.Config{
		ClientID:       o.ClientID,
		ClientSecret:   o.ClientSecret,
		TokenURL:       o.TokenURL,
		Scopes:         o.Scopes,
		EndpointParams: params,
	}, nil
}

// tokenCache holds an access token until it expires. Unlike oauth2.ReuseTokenSource, it fetches
// new tokens with the caller's context, so a slow token endpoint can't outlast a check's timeout.
type tokenCache struct {
	config *clientcredentials.Config

	mutex sync.Mutex
	token *oauth2.Token
}

func (c *tokenCache) Token(ctx context.Context) (string, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.token.Valid() {
		token, err := c.config.Token(ctx)
		if err != nil {
			return "", err
		}
		c.token = token
	}

	return c.token.AccessToken, nil
}

type QueryCheck struct {
	Query string

	plugin *Plugin
}

func (q QueryCheck) Execute(ctx context.Context) goplum.Result {
	endpoint, err := url.JoinPath(q.plugin.Url, "api/v1/query")
	if err != nil {
		return goplum.FailingResult("Invalid url")
	}

	form := url.Values{"query": []string{q.Query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return goplum.FailingResult("Error building request: %v", internal.WithoutURL(err))
	}
	maps.Copy(req.Header, q.plugin.headers)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if len(q.plugin.Auth.Username) > 0 || len(q.plugin.Auth.Password) > 0 {
		req.SetBasicAuth(q.plugin.Auth.Username, q.plugin.Auth.Password)
	} else if len(q.plugin.BearerToken) > 0 {
		req.Header.Set("Authorization", "Bearer "+q.plugin.BearerToken)
	} else if q.plugin.tokens != nil {
		token, err := q.plugin.tokens.Token(ctx)
		if err != nil {
			return goplum.FailingResult("Error fetching oauth2 token: %v", internal.WithoutURL(err))
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := q.plugin.client.Do(req)
	if err != nil {
		return goplum.FailingResult("Error making request: %v", internal.WithoutURL(err))
	}
	defer func() { _ = res.Body.Close() }()

	type response struct {
		Status    string `json:"status"`
		ErrorType string `json:"errorType"`
		Error     string `json:"error"`
		Data      struct {
			ResultType string          `json:"resultType"`
			Result     json.RawMessage `json:"result"`
		} `json:"data"`
	}

	body := response{}
	decodeErr := json.NewDecoder(io.LimitReader(res.Body, 10<<20)).Decode(&body)
	if decodeErr == nil && body.Status == "error" {
		return goplum.FailingResult("Query failed: %s: %s", body.ErrorType, body.Error)
	}

	if res.StatusCode >= 300 {
		return goplum.FailingResult("Bad status code: %d", res.StatusCode)
	}

	if decodeErr != nil {
		return goplum.FailingResult("Error decoding response: %v", decodeErr)
	}

	if body.Status != "success" {
		return goplum.FailingResult("Unexpected response status: %q", body.Status)
	}

	results, err := parseResults(body.Data.ResultType, body.Data.Result)
	if err != nil {
		return goplum.FailingResult("%v", err)
	}

	return evaluate(results)
}

func (q QueryCheck) Validate() error {
	if len(q.Query) == 0 {
		return errors.New("missing required argument: query")
	}
	return nil
}

type sample struct {
	labels map[string]string
	value  float64
}

func parseResults(resultType string, raw json.RawMessage) ([]sample, error) {
	type rawSample struct {
		Metric map[string]string `json:"metric"`
		Value  [2]any            `json:"value"`
	}

	var raws []rawSample
	switch resultType {
	case "vector":
		if err := json.Unmarshal(raw, &raws); err != nil {
			return nil, fmt.Errorf("error decoding vector result: %v", err)
		}
	case "scalar":
		var value [2]any
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, fmt.Errorf("error decoding scalar result: %v", err)
		}
		raws = []rawSample{{Value: value}}
	default:
		return nil, fmt.Errorf("unsupported result type: %s", resultType)
	}

	samples := make([]sample, len(raws))
	for i := range raws {
		s, ok := raws[i].Value[1].(string)
		if !ok {
			return nil, fmt.Errorf("unexpected sample value: %v", raws[i].Value[1])
		}

		value, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("unexpected sample value: %q", s)
		}

		samples[i] = sample{labels: raws[i].Metric, value: value}
	}
	return samples, nil
}

func evaluate(samples []sample) goplum.Result {
	if len(samples) == 0 {
		return goplum.FailingResult("Query returned no results")
	}

	var bad []string
	for i := range samples {
		if samples[i].value == 0 || math.IsNaN(samples[i].value) {
			bad = append(bad, formatSample(samples[i]))
		}
	}

	if len(bad) == 0 {
		return goplum.GoodResult()
	}

	detail := strings.Join(bad[:min(len(bad), maxSeriesInDetail)], ", ")
	if len(bad) > maxSeriesInDetail {
		detail += fmt.Sprintf(" and %d more", len(bad)-maxSeriesInDetail)
	}
	return goplum.FailingResult("%d of %d series failed: %s", len(bad), len(samples), detail)
}

func formatSample(s sample) string {
	name := s.labels["__name__"]

	var labels []string
	for k, v := range s.labels {
		if k != "__name__" {
			labels = append(labels, fmt.Sprintf("%s=%q", k, v))
		}
	}
	slices.Sort(labels)

	return fmt.Sprintf("%s{%s} = %s", name, strings.Join(labels, ", "), strconv.FormatFloat(s.value, 'g', -1, 64))
}
