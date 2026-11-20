package msteams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"chameth.com/goplum"
	"chameth.com/goplum/internal"
)

// maxTextLength keeps the message well within Teams' 28 KB limit, even if every character needs escaping.
const maxTextLength = 4000

var client = http.Client{Timeout: 20 * time.Second}

type Plugin struct{}

func (p Plugin) Alert(kind string) goplum.Alert {
	switch kind {
	case "message":
		return MessageAlert{
			Title: "Goplum alert",
		}
	default:
		return nil
	}
}

func (p Plugin) Check(_ string) goplum.Check {
	return nil
}

type MessageAlert struct {
	Url   string
	Title string
}

func (m MessageAlert) Send(details goplum.AlertDetails) error {
	type textBlock struct {
		Type   string `json:"type"`
		Text   string `json:"text"`
		Wrap   bool   `json:"wrap"`
		Size   string `json:"size,omitempty"`
		Weight string `json:"weight,omitempty"`
	}

	type card struct {
		Schema  string      `json:"$schema"`
		Type    string      `json:"type"`
		Version string      `json:"version"`
		Body    []textBlock `json:"body"`
	}

	type attachment struct {
		ContentType string `json:"contentType"`
		Content     card   `json:"content"`
	}

	payload, err := json.Marshal(
		struct {
			Type        string       `json:"type"`
			Attachments []attachment `json:"attachments"`
		}{
			Type: "message",
			Attachments: []attachment{
				{
					ContentType: "application/vnd.microsoft.card.adaptive",
					Content: card{
						Schema:  "http://adaptivecards.io/schemas/adaptive-card.json",
						Type:    "AdaptiveCard",
						Version: "1.2",
						Body: []textBlock{
							{Type: "TextBlock", Text: m.Title, Wrap: true, Size: "Medium", Weight: "Bolder"},
							{Type: "TextBlock", Text: details.TextWithLink(maxTextLength), Wrap: true},
						},
					},
				},
			},
		},
	)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, m.Url, bytes.NewReader(payload))
	if err != nil {
		return internal.WithoutURL(err)
	}

	req.Header.Add("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return internal.WithoutURL(err)
	}

	defer res.Body.Close()

	if res.StatusCode >= 400 {
		return fmt.Errorf("bad response from Teams: HTTP %d", res.StatusCode)
	}

	return nil
}

func (m MessageAlert) Validate() error {
	return internal.ValidateHTTPURL(m.Url)
}
