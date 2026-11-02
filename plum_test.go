package goplum_test

import (
	"fmt"
	"os"
	"path"
	"testing"

	"chameth.com/goplum"
	"chameth.com/goplum/plugins/debug"
	"chameth.com/goplum/plugins/http"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var plugins = map[string]goplum.PluginLoader{
	"http": func() (goplum.Plugin, error) {
		return http.Plugin{}, nil
	},
	"debug": func() (goplum.Plugin, error) {
		return debug.Plugin{}, nil
	},
}

func TestReadConfig_GoldenData(t *testing.T) {
	tests := []string{
		"defaults",
		"duplicate-alert",
		"duplicate-check",
		"duplicate-group",
		"unknown-alert",
		"unknown-check",
		"unknown-field",
		"unknown-plugin",
		"unknown-group",
		"unrecognised-alert-multiple",
		"unrecognised-alert-single",
		"unrecognised-alert-wildcard",
		"validation-error",
		"valid-group",
		"group-defaults-inheritance",
		"multiple-groups-inheritance",
		"groups-in-defaults",
		"http-get-keys",
		"invalid-group-in-defaults",
	}
	gold := goldie.New(t)

	for i := range tests {
		t.Run(tests[i], func(t *testing.T) {
			plum := goplum.NewPlum()

			plum.RegisterPlugins(plugins)
			err := plum.ReadConfig(path.Join("testdata", fmt.Sprintf("%s.conf", tests[i])))

			var actual any
			if err == nil {
				actual = plum
			} else {
				actual = err.Error()
			}

			gold.AssertJson(t, tests[i], actual)
		})
	}
}

func TestReadConfig_RejectsDuplicatePluginBlocks(t *testing.T) {
	configPath := path.Join(t.TempDir(), "goplum.conf")
	require.NoError(
		t,
		os.WriteFile(
			configPath,
			[]byte(
				`plugin http {
}

check http.get "example" {
  url = "https://example.com/"
  alerts = ["-"]
}

plugin http {
}
`,
			),
			0600,
		),
	)

	plum := goplum.NewPlum()
	plum.RegisterPlugins(plugins)
	err := plum.ReadConfig(configPath)

	require.Error(t, err)
	assert.Equal(t, "plugin defined multiple times: http (at line 1 and line 9)", err.Error())
}

func TestReadConfig_RejectsDuplicateBlocksForUnloadedPlugin(t *testing.T) {
	configPath := path.Join(t.TempDir(), "goplum.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("plugin debug {}\nplugin debug {}\n"), 0600))

	plum := goplum.NewPlum()
	plum.RegisterPlugins(plugins)
	err := plum.ReadConfig(configPath)

	require.Error(t, err)
	assert.Equal(t, "plugin defined multiple times: debug (at line 1 and line 2)", err.Error())
}
