package config

import (
	"os"
	"path"
	"strings"
	"testing"

	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_GoldenData(t *testing.T) {
	tests := []string{
		"full",
		"comments",
		"duplicate_defaults",
		"arrays",
		"arrays_mixed",
		"unexpected_int",
		"defaults_in_defaults",
		"defaults_in_check",
		"defaults_in_alert",
		"defaults_in_plugin",
		"group_with_defaults",
	}
	gold := goldie.New(t)

	for i := range tests {
		t.Run(
			tests[i],
			func(t *testing.T) {
				f, _ := os.Open(path.Join("testdata", tests[i]+".conf"))
				defer f.Close()

				parser := NewParser(f)

				var expected any
				if err := parser.Parse(); err != nil {
					expected = err.Error()
				} else {
					expected = parser
				}

				gold.AssertJson(t, tests[i]+".parser", expected)
			},
		)
	}
}

func TestParser_RecordsBlockPositions(t *testing.T) {
	parser := NewParser(
		strings.NewReader(
			`
alert debug.sysout "a" {}

check debug.random "c" {}
plugin debug {}
  group "g" {}
`,
		),
	)
	require.NoError(t, parser.Parse())

	assert.Equal(t, Position{Line: 2}, parser.AlertBlocks[0].Position)
	assert.Equal(t, Position{Line: 4}, parser.CheckBlocks[0].Position)
	assert.Equal(t, Position{Line: 5}, parser.PluginSettings[0].Position)
	assert.Equal(t, Position{Line: 6}, parser.GroupBlocks[0].Position)
}
