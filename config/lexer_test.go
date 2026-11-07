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
		{"250ms", tokenDuration, 250 * time.Millisecond},
		{"1m30s", tokenDuration, 90 * time.Second},
		{"1s500ms", tokenDuration, 1500 * time.Millisecond},
		{"1h2m3ms", tokenDuration, time.Hour + 2*time.Minute + 3*time.Millisecond},
		{"1d1w", tokenDuration, 8 * 24 * time.Hour},
		{"106751d", tokenDuration, 106751 * 24 * time.Hour},
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
		"-":                     "invalid integer literal '-'",
		"--1":                   "invalid integer literal '-'",
		"1-":                    "invalid integer literal",
		"-5m":                   "durations can't be negative",
		"1m-5s":                 "durations can't be negative",
		"1.5s":                  "invalid duration quantity '1.5'",
		"1.2.3":                 "multiple periods",
		"1h30":                  "invalid duration quantity '30': missing unit",
		"1h30 ":                 "invalid duration quantity '30': missing unit",
		"1m.5":                  "invalid duration quantity '.5': missing unit",
		"1m30x":                 "invalid duration quantity '30': missing unit",
		"3000000h":              "invalid duration quantity '3000000': duration is too long",
		"9223372036854775807ms": "invalid duration quantity '9223372036854775807': duration is too long",
		"106751d1d":             "invalid duration quantity '1': duration is too long",
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

func TestLexer_PositionsAfterComments(t *testing.T) {
	tests := map[string]struct {
		input  string
		line   int
		column int
	}{
		"comment lines":              {"# one\n# two\n# three\ncheck {}", 4, 0},
		"comment after token":        {"{ # one\n  check {}", 2, 2},
		"crlf line endings":          {"# one\r\n# two\r\ncheck {}", 3, 0},
		"carriage return in comment": {"# one\rcheck\ncheck {}", 2, 0},
	}

	for name, tt := range tests {
		t.Run(
			name,
			func(t *testing.T) {
				tokens := lexAll(tt.input)
				require.NotEmpty(t, tokens)
				var check *token
				for i := range tokens {
					if tokens[i].Class == tokenCheck {
						check = &tokens[i]
						break
					}
				}
				require.NotNil(t, check, "tokens: %+v", tokens)
				assert.Equal(t, tt.line, check.Line)
				assert.Equal(t, tt.column, check.Column)
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
