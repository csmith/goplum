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
	tests := []string{"defaults"}
	gold := goldie.New(t)

	for i := range tests {
		t.Run(tests[i], func(t *testing.T) {
			plum := goplum.NewPlum()

			plum.RegisterPlugins(plugins)

			if err := plum.ReadConfig(path.Join("testdata", fmt.Sprintf("%s.conf", tests[i]))); err != nil {
				t.Errorf("Error reading config: %v", err)
			}

			gold.AssertJson(t, tests[i], plum)
		})
	}
}
