package goplum

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type RedactCredentials struct {
	Username string
	Password string `redact:"true"`
}

type RedactBase struct {
	Url  string `redact:"url"`
	Auth RedactCredentials
}

type redactCheck struct {
	RedactBase `config:",squash"`
	Token      string        `redact:"true"`
	Arguments  []string      `redact:"true"`
	Interval   time.Duration `redact:"true"`
	Optional   string        `redact:"true"`
	Plain      []string

	state string
}

func TestRedact_ReplacesTaggedFields(t *testing.T) {
	res := redact(
		redactCheck{
			Token:     "token",
			Arguments: []string{"--password", "hunter2"},
			Interval:  time.Minute,
			Plain:     []string{"a", "b"},
		},
	).(redactCheck)
	assert.Equal(t, "[redacted]", res.Token)
	assert.Equal(t, []string{"[redacted]", "[redacted]"}, res.Arguments)
	assert.Zero(t, res.Interval)
	assert.Empty(t, res.Optional)
	assert.Equal(t, []string{"a", "b"}, res.Plain)
}

func TestRedact_RecursesIntoStructs(t *testing.T) {
	res := redact(
		redactCheck{
			Url: "https://example.com/",
			Auth: RedactCredentials{
				Username: "user",
				Password: "pass",
			},
		},
	).(redactCheck)
	assert.Equal(t, "https://example.com/", res.Url)
	assert.Equal(t, "user", res.Auth.Username)
	assert.Equal(t, "[redacted]", res.Auth.Password)
}

func TestRedact_DoesNotModifyOriginal(t *testing.T) {
	check := &redactCheck{
		Token:     "token",
		Arguments: []string{"secret"},
	}

	res := redact(check).(redactCheck)
	assert.Equal(t, "[redacted]", res.Token)
	assert.Equal(t, "token", check.Token)
	assert.Equal(t, []string{"secret"}, check.Arguments)
}

func TestRedact_OmitsUnexportedFields(t *testing.T) {
	res := redact(redactCheck{state: "internal"}).(redactCheck)
	assert.Empty(t, res.state)
}

func TestRedact_ReturnsNonStructsUnchanged(t *testing.T) {
	assert.Equal(t, "value", redact("value"))
	assert.Nil(t, redact(nil))

	var check *redactCheck
	assert.Equal(t, check, redact(check))
}

func TestRedact_RemovesPasswordsFromUrls(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"no credentials", "https://example.com/path?q=1", "https://example.com/path?q=1"},
		{"username only", "https://user@example.com/", "https://user@example.com/"},
		{"username and password", "https://user:hunter2@example.com/", "https://user:xxxxx@example.com/"},
		{"unparseable", "https://user:hunter2@example.com:port/", "[redacted]"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				res := redact(redactCheck{Url: tt.url}).(redactCheck)
				assert.Equal(t, tt.want, res.Url)
			},
		)
	}
}

func TestAlertDetails_TextWithLink(t *testing.T) {
	const link = "https://example.com/runbook"

	tests := map[string]struct {
		text     string
		link     string
		limit    int
		expected string
	}{
		"no link":   {text: "alert", expected: "alert"},
		"with link": {text: "alert", link: link, expected: "alert\n" + link},
		"limit not reached": {
			text:     "alert",
			link:     link,
			limit:    1000,
			expected: "alert\n" + link,
		},
		"exactly at limit": {
			text:     strings.Repeat("a", 172),
			link:     link,
			limit:    200,
			expected: strings.Repeat("a", 172) + "\n" + link,
		},
		"text shortened to keep link": {
			text:     strings.Repeat("a", 173),
			link:     link,
			limit:    200,
			expected: strings.Repeat("a", 171) + "…\n" + link,
		},
		"limit without link": {
			text:     strings.Repeat("a", 201),
			limit:    200,
			expected: strings.Repeat("a", 199) + "…",
		},
		"link omitted when it would leave too little text": {
			text:     strings.Repeat("a", 300),
			link:     link,
			limit:    120,
			expected: strings.Repeat("a", 119) + "…",
		},
	}

	for name, tt := range tests {
		t.Run(
			name,
			func(t *testing.T) {
				details := AlertDetails{Text: tt.text, Link: tt.link}
				assert.Equal(t, tt.expected, details.TextWithLink(tt.limit))
			},
		)
	}
}
