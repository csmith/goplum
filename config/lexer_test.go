package config

import (
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLexer_GoldenData(t *testing.T) {
	tests := []string{"full", "comments", "unterminated_string", "string_escaping"}
	gold := goldie.New(t)

	for i := range tests {
		t.Run(
			tests[i],
			func(t *testing.T) {
				f, _ := os.Open(path.Join("testdata", tests[i]+".conf"))
				defer f.Close()
				lexer := NewLexer(f)
				go lexer.Lex()

				var actual []token
				for token := range lexer.output {
					actual = append(actual, token)
				}

				gold.AssertJson(t, tests[i], actual)
			},
		)
	}
}

func TestLexer_Numbers(t *testing.T) {
	tests := []struct {
		input string
		class tokenClass
		value any
	}{
		{"30", tokenInt, 30},
		{"-2", tokenInt, -2},
		{"1.5", tokenFloat, 1.5},
		{"-0.25", tokenFloat, -0.25},
		{"-.5", tokenFloat, -0.5},
		{"30s", tokenDuration, 30 * time.Second},
		{"5m", tokenDuration, 5 * time.Minute},
		{"1m30s", tokenDuration, 90 * time.Second},
		{"1h2m3s", tokenDuration, time.Hour + 2*time.Minute + 3*time.Second},
		{"1d1w", tokenDuration, 8 * 24 * time.Hour},
		{"-5", tokenInt, -5},
	}

	for _, tt := range tests {
		for _, suffix := range []string{"", "\n"} {
			t.Run(
				tt.input+suffix,
				func(t *testing.T) {
					tokens := lexAll(tt.input + suffix)
					require.NotEmpty(t, tokens)
					assert.Equal(t, tt.class, tokens[0].Class, "token: %+v", tokens[0])
					assert.Equal(t, tt.value, tokens[0].Value)
					assert.Equal(t, tokenEOF, tokens[len(tokens)-1].Class)
				},
			)
		}
	}
}

func TestLexer_InvalidNumbers(t *testing.T) {
	tests := map[string]string{
		"-":     "invalid integer literal '-'",
		"--1":   "invalid integer literal '-'",
		"1-":    "invalid integer literal",
		"-5m":   "durations can't be negative",
		"1m-5s": "durations can't be negative",
		"1.5s":  "invalid duration quantity '1.5'",
		"1.2.3": "multiple periods",
	}

	for input, want := range tests {
		t.Run(
			input,
			func(t *testing.T) {
				tokens := lexAll(input)
				require.NotEmpty(t, tokens)
				last := tokens[len(tokens)-1]
				assert.Equal(t, tokenError, last.Class, "tokens: %+v", tokens)
				assert.Contains(t, last.Value, want)
			},
		)
	}
}

func lexAll(input string) []token {
	lexer := NewLexer(strings.NewReader(input))
	go lexer.Lex()

	var tokens []token
	for t := range lexer.output {
		tokens = append(tokens, t)
	}
	return tokens
}
