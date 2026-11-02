package prometheus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"chameth.com/goplum"
)

const maxSeriesInDetail = 5

type Plugin struct {
	Url string
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
	if len(p.Url) == 0 {
		return errors.New("url must be specified")
	}

	u, err := url.Parse(p.Url)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("url must be an absolute http or https URL")
	}

	return nil
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
		return goplum.FailingResult("Error building request: %v", withoutUrl(err))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return goplum.FailingResult("Error making request: %v", withoutUrl(err))
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

// withoutUrl strips the URL from a url.Error, as it may contain credentials.
func withoutUrl(err error) error {
	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		return urlErr.Err
	}
	return err
}
