package goplum

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type RedactCredentials struct {
	Username string
	Password string `redact:"true"`
}

type RedactBase struct {
	Url  string
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
