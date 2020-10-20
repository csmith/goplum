package goplum_test

import (
	"fmt"
	"github.com/csmith/goplum"
	"github.com/csmith/goplum/plugins/debug"
	"github.com/csmith/goplum/plugins/http"
	"github.com/sebdah/goldie/v2"
	"path"
	"testing"
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
		"unknown-alert",
		"unknown-check",
		"unknown-field",
		"unknown-plugin",
		"validation-error",
	}
	gold := goldie.New(t)

	for i := range tests {
		t.Run(tests[i], func(t *testing.T) {
			plum := goplum.NewPlum()

			plum.RegisterPlugins(plugins)
			err := plum.ReadConfig(path.Join("testdata", fmt.Sprintf("%s.conf", tests[i])))

			var actual interface{}
			if err == nil {
				actual = plum
			} else {
				actual = err.Error()
			}

			gold.AssertJson(t, tests[i], actual)
		})
	}
}
