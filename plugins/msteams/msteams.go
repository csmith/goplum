package msteams

import (
	"fmt"
	"regexp"

	"chameth.com/goplum"
	goteamsnotify "github.com/dasrick/go-teams-notify/v2"
)

type Plugin struct{}

func (p Plugin) Alert(kind string) goplum.Alert {
	switch kind {
	case "message":
		return MessageAlert{
			Title: "Goplum alert",
			Theme: "#6c2b8f",
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
	Theme string
}

func (m MessageAlert) Send(details goplum.AlertDetails) error {
	mstClient := goteamsnotify.NewClient()

	card := goteamsnotify.NewMessageCard()
	card.Title = m.Title
	card.Text = details.Text
	card.ThemeColor = m.Theme

	return mstClient.Send(m.Url, card)
}

var themeRegex = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

func (m MessageAlert) Validate() error {
	if len(m.Url) == 0 {
		return fmt.Errorf("missing required argument: url")
	}

	if !themeRegex.MatchString(m.Theme) {
		return fmt.Errorf("theme must be a six-character hex colour starting with '#'")
	}

	return nil
}
