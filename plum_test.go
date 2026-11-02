package goplum_test

import (
	"encoding/json"
	"os"
	"path"
	"testing"

	"chameth.com/goplum"
	"chameth.com/goplum/plugins/debug"
	"chameth.com/goplum/plugins/exec"
	"chameth.com/goplum/plugins/heartbeat"
	"chameth.com/goplum/plugins/http"
	"chameth.com/goplum/plugins/snmp"
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
		t.Run(
			tests[i],
			func(t *testing.T) {
				plum := goplum.NewPlum()

				plum.RegisterPlugins(plugins)
				err := plum.ReadConfig(path.Join("testdata", tests[i]+".conf"))

				var actual any
				if err == nil {
					actual = plum
				} else {
					actual = err.Error()
				}

				gold.AssertJson(t, tests[i], actual)
			},
		)
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

type recordingAlert struct {
	details []goplum.AlertDetails
}

func (r *recordingAlert) Send(details goplum.AlertDetails) error {
	r.details = append(r.details, details)
	return nil
}

func TestRaiseAlerts_RedactsSecretsFromCheckConfig(t *testing.T) {
	tests := map[string]goplum.Check{
		"http.get": http.GetCheck{BaseCheck: http.BaseCheck{
			Url:  "https://example.com/",
			Auth: http.Credentials{Username: "user", Password: "secret-value"},
		}},
		"http.healthcheck": http.HealthCheck{BaseCheck: http.BaseCheck{
			Url:  "https://example.com/",
			Auth: http.Credentials{Username: "user", Password: "secret-value"},
		}},
		"snmp.string": snmp.StringCheck{BaseCheck: snmp.BaseCheck{Agent: "192.0.2.1", Community: "secret-value"}},
		"snmp.int":    snmp.IntCheck{BaseCheck: snmp.BaseCheck{Agent: "192.0.2.1", Community: "secret-value"}},
		"exec.command": exec.CommandCheck{
			Name:      "/usr/bin/check",
			Arguments: []string{"--token", "secret-value"},
		},
		"heartbeat.received": &heartbeat.ReceivedCheck{ID: "secret-value"},
	}

	for name, check := range tests {
		t.Run(
			name,
			func(t *testing.T) {
				alert := &recordingAlert{}
				plum := goplum.NewPlum()
				plum.Alerts["test"] = alert

				scheduled := &goplum.ScheduledCheck{
					Name:   "test",
					Type:   name,
					Config: &goplum.CheckSettings{Alerts: []string{"*"}},
					Check:  check,
					State:  goplum.StateFailing,
				}
				scheduled.AddResult(&goplum.Result{State: goplum.StateFailing})
				plum.RaiseAlerts(scheduled, goplum.StateGood)

				require.Len(t, alert.details, 1)
				b, err := json.Marshal(alert.details[0])
				require.NoError(t, err)
				assert.NotContains(t, string(b), "secret-value")
				assert.Contains(t, string(b), "[redacted]")

				original, err := json.Marshal(check)
				require.NoError(t, err)
				assert.Contains(t, string(original), "secret-value")
			},
		)
	}
}
