package config

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		"include",
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

func TestParser_RejectsTrailingWordAtEndOfFile(t *testing.T) {
	for _, word := range []string{"check", "chek"} {
		t.Run(
			word,
			func(t *testing.T) {
				parser := NewParser(strings.NewReader("check debug.random \"a\" {}\n" + word))
				assert.Error(t, parser.Parse())
			},
		)
	}
}

func TestParser_RejectsTopLevelOnlyBlocksInIncludedFiles(t *testing.T) {
	tests := map[string]string{
		"defaults": "check debug.random \"c\" {}\ndefaults {}\n",
		"include":  "\ninclude \"other.conf\"\n",
	}

	for name, input := range tests {
		t.Run(
			name,
			func(t *testing.T) {
				parser := NewParser(strings.NewReader(input))
				parser.included = true

				err := parser.Parse()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "not allowed in an included file at line 2")
			},
		)
	}
}

func TestLoad_MergesIncludedFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(
		t,
		dir,
		"goplum.conf",
		`
defaults {
  interval = 1m
}

include "conf.d/*.conf"
alert debug.sysout "main" {}
include "extra.conf"
`,
	)
	writeFile(t, dir, "conf.d/b.conf", "check debug.random \"b\" {}\n")
	writeFile(
		t,
		dir,
		"conf.d/a.conf",
		`
check debug.random "a" {}
group "g" {
  defaults {
    interval = 2m
  }
}
`,
	)
	writeFile(t, dir, "extra.conf", "plugin debug {}\nalert debug.sysout \"extra\" {}\n")

	cfg, err := Load(filepath.Join(dir, "goplum.conf"))
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"interval": time.Minute}, cfg.DefaultSettings)
	require.Len(t, cfg.CheckBlocks, 2)
	assert.Equal(t, "a", cfg.CheckBlocks[0].Name)
	assert.Equal(t, Position{File: filepath.Join(dir, "conf.d/a.conf"), Line: 2}, cfg.CheckBlocks[0].Position)
	assert.Equal(t, "b", cfg.CheckBlocks[1].Name)
	require.Len(t, cfg.AlertBlocks, 2)
	assert.Equal(t, Position{File: filepath.Join(dir, "goplum.conf"), Line: 7}, cfg.AlertBlocks[0].Position)
	assert.Equal(t, Position{File: filepath.Join(dir, "extra.conf"), Line: 2}, cfg.AlertBlocks[1].Position)
	require.Len(t, cfg.GroupBlocks, 1)
	assert.Equal(t, map[string]any{"interval": 2 * time.Minute}, cfg.GroupBlocks[0].Settings["defaults"])
	require.Len(t, cfg.PluginSettings, 1)
}

func TestLoad_AllowsGlobWithNoMatches(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "goplum.conf", "include \"conf.d/*.conf\"\n")

	cfg, err := Load(filepath.Join(dir, "goplum.conf"))
	require.NoError(t, err)
	assert.Empty(t, cfg.CheckBlocks)
}

func TestLoad_RejectsMissingLiteralInclude(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "goplum.conf", "\ninclude \"secrets.conf\"\n")

	_, err := Load(filepath.Join(dir, "goplum.conf"))
	require.Error(t, err)
	assert.Equal(
		t,
		filepath.Join(dir, "goplum.conf")+":2: included file "+filepath.Join(dir, "secrets.conf")+" does not exist",
		err.Error(),
	)
}

func TestLoad_ResolvesAbsoluteIncludes(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	writeFile(t, other, "checks.conf", "check debug.random \"c\" {}\n")
	writeFile(t, dir, "goplum.conf", "include \""+filepath.Join(other, "*.conf")+"\"\n")

	cfg, err := Load(filepath.Join(dir, "goplum.conf"))
	require.NoError(t, err)
	require.Len(t, cfg.CheckBlocks, 1)
	assert.Equal(t, filepath.Join(other, "checks.conf"), cfg.CheckBlocks[0].Position.File)
}

func TestLoad_NamesIncludedFileInParseErrors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "goplum.conf", "include \"*.inc\"\n")
	writeFile(t, dir, "bad.inc", "defaults {}\n")

	_, err := Load(filepath.Join(dir, "goplum.conf"))
	require.Error(t, err)
	assert.Equal(
		t,
		"unable to parse config file "+filepath.Join(dir, "bad.inc")+
			": defaults block not allowed in an included file at line 1",
		err.Error(),
	)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0700))
	require.NoError(t, os.WriteFile(p, []byte(content), 0600))
}
