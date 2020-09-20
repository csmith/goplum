package config

import (
	"fmt"
	"github.com/sebdah/goldie/v2"
	"os"
	"path"
	"testing"
)

func TestLexer_GoldenData(t *testing.T) {
	tests := []string{"full", "comments", "unterminated_string", "string_escaping"}
	gold := goldie.New(t)

	for i := range tests {
		t.Run(tests[i], func(t *testing.T) {
			f, _ := os.Open(path.Join("testdata", fmt.Sprintf("%s.conf", tests[i])))
			defer f.Close()
			lexer := NewLexer(f)
			go lexer.Lex()

			var actual []token
			for token := range lexer.Output() {
				actual = append(actual, token)
			}

			gold.AssertJson(t, tests[i], actual)
		})
	}
}
