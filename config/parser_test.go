package config

import (
	"fmt"
	"github.com/sebdah/goldie/v2"
	"os"
	"path"
	"testing"
)

func TestParser_GoldenData(t *testing.T) {
	tests := []string{"full", "comments"}
	gold := goldie.New(t)

	for i := range tests {
		t.Run(tests[i], func(t *testing.T) {
			f, _ := os.Open(path.Join("testdata", fmt.Sprintf("%s.conf", tests[i])))
			defer f.Close()

			parser := NewParser(f)
			err := parser.Parse()
			if err != nil {
				t.Fatalf("Unable to parse test file: %v", err)
			}

			gold.AssertJson(t, fmt.Sprintf("%s.parser", tests[i]), parser)
		})
	}
}
