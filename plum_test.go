package goplum_test

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"chameth.com/goplum"
	"chameth.com/goplum/plugins/debug"
	"chameth.com/goplum/plugins/exec"
	"chameth.com/goplum/plugins/heartbeat"
	"chameth.com/goplum/plugins/http"
	"chameth.com/goplum/plugins/prometheus"
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
	assert.Equal(t, "plugin defined multiple times: http (at "+configPath+":1 and "+configPath+":9)", err.Error())
}

func TestReadConfig_RejectsDuplicateBlocksForUnloadedPlugin(t *testing.T) {
	configPath := path.Join(t.TempDir(), "goplum.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("plugin debug {}\nplugin debug {}\n"), 0600))

	plum := goplum.NewPlum()
	plum.RegisterPlugins(plugins)
	err := plum.ReadConfig(configPath)

	require.Error(t, err)
	assert.Equal(t, "plugin defined multiple times: debug (at "+configPath+":1 and "+configPath+":2)", err.Error())
}

func TestReadConfig_ResolvesReferencesAcrossIncludedFiles(t *testing.T) {
	dir := t.TempDir()
	writeConfig(
		t,
		dir,
		"goplum.conf",
		`defaults {
  alerts = ["shared"]
  groups = ["web"]
}

check debug.random "main" {}

include "conf.d/*.conf"
`,
	)
	writeConfig(
		t,
		dir,
		"conf.d/a.conf",
		`check debug.random "included" {
  alerts = ["late"]
}
`,
	)
	writeConfig(
		t,
		dir,
		"conf.d/b.conf",
		`alert debug.sysout "shared" {}
alert debug.sysout "late" {}

group "web" {
  defaults {
    interval = 2m
  }
}
`,
	)

	plum := goplum.NewPlum()
	plum.RegisterPlugins(plugins)
	require.NoError(t, plum.ReadConfig(filepath.Join(dir, "goplum.conf")))

	require.Contains(t, plum.Checks, "main")
	assert.Equal(t, []string{"shared"}, plum.Checks["main"].Config.Alerts)
	assert.Equal(t, 2*time.Minute, plum.Checks["main"].Config.Interval)
	require.Contains(t, plum.Checks, "included")
	assert.Equal(t, []string{"late"}, plum.Checks["included"].Config.Alerts)
	assert.Equal(t, 2*time.Minute, plum.Checks["included"].Config.Interval)
}

func TestReadConfig_NamesBothFilesForDuplicatesAcrossIncludes(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "goplum.conf", "include \"*.inc\"\n\ncheck debug.random \"dup\" {\n  alerts = [\"-\"]\n}\n")
	writeConfig(t, dir, "checks.inc", "\ncheck debug.random \"dup\" {\n  alerts = [\"-\"]\n}\n")

	plum := goplum.NewPlum()
	plum.RegisterPlugins(plugins)
	err := plum.ReadConfig(filepath.Join(dir, "goplum.conf"))

	require.Error(t, err)
	assert.Equal(
		t,
		"check defined multiple times: dup (at "+filepath.Join(dir, "goplum.conf")+":3 and "+
			filepath.Join(dir, "checks.inc")+":2)",
		err.Error(),
	)
}

func TestReadConfig_NamesIncludedFileInConfigurationErrors(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "goplum.conf", "include \"checks.inc\"\n")
	writeConfig(t, dir, "checks.inc", "\n\ncheck debug.random \"c\" {\n  bazinga = 1\n}\n")

	plum := goplum.NewPlum()
	plum.RegisterPlugins(plugins)
	err := plum.ReadConfig(filepath.Join(dir, "goplum.conf"))

	require.Error(t, err)
	assert.True(
		t,
		strings.HasPrefix(err.Error(), filepath.Join(dir, "checks.inc")+":3: error configuring check c:"),
		err.Error(),
	)
}

func writeConfig(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0700))
	require.NoError(t, os.WriteFile(p, []byte(content), 0600))
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
			Url:  "https://user:secret-value@example.com/",
			Auth: http.Credentials{Username: "user", Password: "secret-value"},
		}},
		"http.healthcheck": http.HealthCheck{BaseCheck: http.BaseCheck{
			Url:  "https://user:secret-value@example.com/",
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

func TestRaiseAlerts_OmitsPluginSettings(t *testing.T) {
	plugin := &prometheus.Plugin{
		Url:         "https://user:secret-value@example.com/",
		BearerToken: "secret-value",
		Headers:     []string{"X-Scope-OrgID: secret-value"},
	}
	check := plugin.Check("query")

	alert := &recordingAlert{}
	plum := goplum.NewPlum()
	plum.Alerts["test"] = alert

	scheduled := &goplum.ScheduledCheck{
		Name:   "test",
		Type:   "prometheus.query",
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
}

func TestScheduledCheck_AddResult_ReachesThresholdOfFullHistory(t *testing.T) {
	check := &goplum.ScheduledCheck{}
	thresholds := map[goplum.CheckState]int{goplum.StateGood: 10, goplum.StateFailing: 10}

	for range 9 {
		check.AddResult(&goplum.Result{State: goplum.StateGood})
	}
	assert.Equal(t, goplum.StateIndeterminate, check.History.State(thresholds))

	history := check.AddResult(&goplum.Result{State: goplum.StateGood})
	assert.Equal(t, goplum.StateGood, history.State(thresholds))
}

func TestReadConfig_RejectsTypeWithoutPluginPrefix(t *testing.T) {
	tests := map[string]struct {
		config   string
		expected string
	}{
		"alert": {
			config:   "alert debug \"x\" {}\n",
			expected: `invalid alert type "debug": expected <plugin>.<type>`,
		},
		"check": {
			config:   "check debug \"x\" {\n  alerts = [\"-\"]\n}\n",
			expected: `error configuring check x: invalid check type "debug": expected <plugin>.<type>`,
		},
	}

	for name, tt := range tests {
		t.Run(
			name,
			func(t *testing.T) {
				dir := t.TempDir()
				writeConfig(t, dir, "goplum.conf", tt.config)
				configPath := filepath.Join(dir, "goplum.conf")

				plum := goplum.NewPlum()
				plum.RegisterPlugins(plugins)
				err := plum.ReadConfig(configPath)

				require.Error(t, err)
				assert.Equal(t, configPath+":1: "+tt.expected, err.Error())
			},
		)
	}
}

func TestReadConfig_ValidatesThresholds(t *testing.T) {
	tests := map[string]struct {
		config string
		errMsg string
	}{
		"lowest allowed": {
			config: "check debug.random \"c\" {\n  alerts = [\"-\"]\n  good_threshold = 1\n  failing_threshold = 1\n}\n",
		},
		"highest allowed": {
			config: "check debug.random \"c\" {\n  alerts = [\"-\"]\n  good_threshold = 10\n  failing_threshold = 10\n}\n",
		},
		"good zero": {
			config: "check debug.random \"c\" {\n  alerts = [\"-\"]\n  good_threshold = 0\n}\n",
			errMsg: "good_threshold must be between 1 and 10, got 0",
		},
		"good too high": {
			config: "check debug.random \"c\" {\n  alerts = [\"-\"]\n  good_threshold = 11\n}\n",
			errMsg: "good_threshold must be between 1 and 10, got 11",
		},
		"failing zero": {
			config: "check debug.random \"c\" {\n  alerts = [\"-\"]\n  failing_threshold = 0\n}\n",
			errMsg: "failing_threshold must be between 1 and 10, got 0",
		},
		"failing too high": {
			config: "check debug.random \"c\" {\n  alerts = [\"-\"]\n  failing_threshold = 11\n}\n",
			errMsg: "failing_threshold must be between 1 and 10, got 11",
		},
		"inherited from defaults": {
			config: "defaults {\n  good_threshold = 11\n}\n\ncheck debug.random \"c\" {\n  alerts = [\"-\"]\n}\n",
			errMsg: "good_threshold must be between 1 and 10, got 11",
		},
		"inherited from group": {
			config: "group \"g\" {\n  defaults {\n    failing_threshold = 11\n  }\n}\n\n" +
				"check debug.random \"c\" {\n  alerts = [\"-\"]\n  groups = [\"g\"]\n}\n",
			errMsg: "failing_threshold must be between 1 and 10, got 11",
		},
		"overridden by check": {
			config: "defaults {\n  good_threshold = 11\n}\n\n" +
				"check debug.random \"c\" {\n  alerts = [\"-\"]\n  good_threshold = 3\n}\n",
		},
	}

	for name, tt := range tests {
		t.Run(
			name,
			func(t *testing.T) {
				dir := t.TempDir()
				writeConfig(t, dir, "goplum.conf", tt.config)

				plum := goplum.NewPlum()
				plum.RegisterPlugins(plugins)
				err := plum.ReadConfig(filepath.Join(dir, "goplum.conf"))

				if tt.errMsg == "" {
					assert.NoError(t, err)
				} else {
					assert.ErrorContains(t, err, tt.errMsg)
				}
			},
		)
	}
}

type fixedCheck struct {
	state goplum.CheckState
}

func (f *fixedCheck) Execute(context.Context) goplum.Result {
	return goplum.Result{State: f.state, Detail: "fixed detail"}
}

func newSettlingCheck(
	state goplum.CheckState,
	reminder time.Duration,
) (*goplum.Plum, *goplum.ScheduledCheck, *fixedCheck, *recordingAlert) {
	alert := &recordingAlert{}
	plum := goplum.NewPlum()
	plum.Alerts["test"] = alert

	check := &fixedCheck{state: state}
	return plum, &goplum.ScheduledCheck{
		Name:  "test",
		Type:  "test.fixed",
		Check: check,
		Config: &goplum.CheckSettings{
			Alerts:           []string{"*"},
			Timeout:          time.Second,
			Reminder:         reminder,
			GoodThreshold:    2,
			FailingThreshold: 2,
		},
	}, check, alert
}

func TestUpdateStatus_AlertsWhenFirstSettlingAsFailing(t *testing.T) {
	plum, scheduled, _, alert := newSettlingCheck(goplum.StateFailing, time.Hour)

	plum.RunCheck(scheduled)
	assert.Empty(t, alert.details)

	plum.RunCheck(scheduled)
	require.Len(t, alert.details, 1)
	assert.False(t, alert.details[0].IsReminder)
	assert.Equal(t, goplum.StateIndeterminate, alert.details[0].PreviousState)
	assert.Equal(t, goplum.StateFailing, alert.details[0].NewState)
	assert.Equal(t, "Check 'test' is now failing (fixed detail), was indeterminate.", alert.details[0].Text)

	plum.RunCheck(scheduled)
	assert.Len(t, alert.details, 1)
}

func TestUpdateStatus_RemindsOnceReminderIntervalPassesAfterSettling(t *testing.T) {
	plum, scheduled, _, alert := newSettlingCheck(goplum.StateFailing, time.Hour)

	plum.RunCheck(scheduled)
	plum.RunCheck(scheduled)
	require.Len(t, alert.details, 1)

	scheduled.LastAlertTime = time.Now().Add(-2 * time.Hour)
	plum.RunCheck(scheduled)

	require.Len(t, alert.details, 2)
	assert.True(t, alert.details[1].IsReminder)
}

func TestUpdateStatus_AlertsOnSettlingWithRemindersDisabled(t *testing.T) {
	plum, scheduled, _, alert := newSettlingCheck(goplum.StateFailing, 0)

	plum.RunCheck(scheduled)
	plum.RunCheck(scheduled)
	require.Len(t, alert.details, 1)
	assert.False(t, alert.details[0].IsReminder)

	scheduled.LastAlertTime = time.Now().Add(-2 * time.Hour)
	plum.RunCheck(scheduled)
	assert.Len(t, alert.details, 1)
}

func TestUpdateStatus_SettlesSilentlyAsGood(t *testing.T) {
	plum, scheduled, check, alert := newSettlingCheck(goplum.StateGood, time.Hour)

	plum.RunCheck(scheduled)
	plum.RunCheck(scheduled)
	assert.True(t, scheduled.Settled)
	assert.Empty(t, alert.details)

	check.state = goplum.StateFailing
	plum.RunCheck(scheduled)
	assert.Empty(t, alert.details)

	plum.RunCheck(scheduled)
	require.Len(t, alert.details, 1)
	assert.False(t, alert.details[0].IsReminder)
	assert.Equal(t, goplum.StateGood, alert.details[0].PreviousState)
	assert.Equal(t, goplum.StateFailing, alert.details[0].NewState)
}
