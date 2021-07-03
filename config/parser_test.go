package config

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/sebdah/goldie/v2"
)

func TestParser_GoldenData(t *testing.T) {
	tests := []string{"full", "comments", "duplicate_defaults", "arrays", "arrays_mixed"}
	gold := goldie.New(t)

	for i := range tests {
		t.Run(tests[i], func(t *testing.T) {
			f, _ := os.Open(path.Join("testdata", fmt.Sprintf("%s.conf", tests[i])))
			defer f.Close()

			parser := NewParser(f)

			var expected interface{}
			if err := parser.Parse(); err != nil {
				expected = err.Error()
			} else {
				expected = parser
			}

			gold.AssertJson(t, fmt.Sprintf("%s.parser", tests[i]), expected)
		})
	}
}
