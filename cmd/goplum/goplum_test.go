//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	mathrand "math/rand/v2"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"chameth.com/goplum"
	"chameth.com/goplum/api"
	"chameth.com/goplum/plugins/heartbeat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	checkInterval = 20 * time.Millisecond
	waitTimeout   = 15 * time.Second
	waitTick      = 5 * time.Millisecond

	heartbeatID = "bf7a7fa97f112bf949e6de4188d6a991"

	// runMainEnv makes the test binary behave as the real goplum binary, so main itself can be tested.
	runMainEnv = "GOPLUM_TEST_RUN_MAIN"
)

func TestMain(m *testing.M) {
	if os.Getenv(runMainEnv) == "1" {
		main()
		return
	}

	if err := flag.Set("quiet", "true"); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}

func Test_Run_ReturnsConfigErrors(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		included string
		want     []string
	}{
		{
			name:   "syntax error",
			config: "check http.get {",
			want:   []string{"unable to read config", "goplum.conf"},
		},
		{
			name:   "unknown plugin",
			config: `check nope.get "a" {}`,
			want:   []string{"unable to read config", "goplum.conf:1", "no plugin found with name nope"},
		},
		{
			name:   "check validation failure",
			config: "\n" + `check http.get "a" {}`,
			want:   []string{"goplum.conf:2", "missing required argument: url"},
		},
		{
			name:   "threshold out of range",
			config: `check http.get "a" { url = "http://localhost/"` + "\n" + `failing_threshold = 11 }`,
			want:   []string{"failing_threshold must be between 1 and 10, got 11"},
		},
		{
			name:   "duration without units",
			config: `check http.get "a" { url = "http://localhost/"` + "\n" + `interval = 30 }`,
			want:   []string{"expected a duration with units, such as 30s, got 30"},
		},
		{
			name:   "negative alert limit",
			config: `group "g" { alert_limit = -1 }`,
			want:   []string{"alert_limit can't be negative"},
		},
		{
			name:   "unknown group",
			config: `check http.get "a" { url = "http://localhost/"` + "\n" + `groups = ["nope"] }`,
			want:   []string{"no group named 'nope'"},
		},
		{
			name:   "missing include",
			config: `include "nope.conf"`,
			want:   []string{"goplum.conf:1", "nope.conf does not exist"},
		},
		{
			name:     "error in included file",
			config:   `include "included.conf"`,
			included: `check http.get "a" {}`,
			want:     []string{"included.conf:1", "missing required argument: url"},
		},
		{
			name:     "defaults in included file",
			config:   `include "included.conf"`,
			included: `defaults {}`,
			want:     []string{"included.conf", "defaults block not allowed in an included file"},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				path := writeConfig(t, tt.config)
				if tt.included != "" {
					writeFile(t, filepath.Join(filepath.Dir(path), "included.conf"), tt.included)
				}

				err := runGoplum(path, filepath.Join(t.TempDir(), "tomb"), certPaths{}, 0, make(chan os.Signal, 1))

				require.Error(t, err)
				for _, want := range tt.want {
					assert.ErrorContains(t, err, want)
				}
			},
		)
	}
}

func Test_Run_ReturnsErrorIfConfigMissing(t *testing.T) {
	err := runGoplum(
		filepath.Join(t.TempDir(), "missing.conf"),
		filepath.Join(t.TempDir(), "tomb"),
		certPaths{},
		0,
		make(chan os.Signal, 1),
	)

	assert.ErrorContains(t, err, "unable to read config")
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func Test_Run_ReturnsErrorIfConfigIsDirectory(t *testing.T) {
	err := runGoplum(t.TempDir(), filepath.Join(t.TempDir(), "tomb"), certPaths{}, 0, make(chan os.Signal, 1))

	assert.ErrorContains(t, err, "unable to read config")
}

func Test_Run_ReturnsErrorIfApiPortInUse(t *testing.T) {
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()
	port := listener.Addr().(*net.TCPAddr).Port

	err = runGoplum(
		writeConfig(t, ""),
		filepath.Join(t.TempDir(), "tomb"),
		generateCerts(t),
		port,
		make(chan os.Signal, 1),
	)

	assert.ErrorContains(t, err, fmt.Sprintf("unable to listen on port %d", port))
}

func Test_Run_AlertsOnFailureAndRecovery(t *testing.T) {
	sink := startAlertSink(t)
	site := startSite(t)

	startGoplum(t, writeConfig(t, sink.config(), httpCheck("site", site.URL)), tombstonePath(t)).
		waitForState(t, "site", api.Status_GOOD)
	assert.Empty(t, sink.alerts(), "settling as good should not raise an alert")

	site.setStatus(http.StatusInternalServerError)
	failed := sink.waitForAlert(t, 1)
	assert.Equal(t, "Check 'site' is now failing (Bad status code: 500), was good.", failed.Text)
	assert.Equal(t, "site", failed.Name)
	assert.Equal(t, "http.get", failed.Type)
	assert.Equal(t, goplum.StateGood, failed.PreviousState)
	assert.Equal(t, goplum.StateFailing, failed.NewState)
	assert.False(t, failed.IsReminder)
	assert.Equal(t, goplum.StateFailing, failed.LastResult.State)
	assert.Equal(t, "Bad status code: 500", failed.LastResult.Detail)

	site.setStatus(http.StatusOK)
	recovered := sink.waitForAlert(t, 2)
	assert.Equal(t, "Check 'site' is now good, was failing.", recovered.Text)
	assert.Equal(t, goplum.StateFailing, recovered.PreviousState)
	assert.Equal(t, goplum.StateGood, recovered.NewState)
	assert.False(t, recovered.IsReminder)

	site.waitForMoreRequests(t, 5)
	assert.Len(t, sink.alerts(), 2)
}

func Test_Run_AlertsWhenFirstSettlingAsFailing(t *testing.T) {
	sink := startAlertSink(t)
	site := startSite(t)
	site.setStatus(http.StatusBadGateway)

	startGoplum(t, writeConfig(t, sink.config(), httpCheck("site", site.URL)), tombstonePath(t))

	alert := sink.waitForAlert(t, 1)
	assert.Equal(t, "Check 'site' is now failing (Bad status code: 502), was indeterminate.", alert.Text)
	assert.Equal(t, goplum.StateIndeterminate, alert.PreviousState)
	assert.False(t, alert.IsReminder)
}

func Test_Run_HonoursThresholds(t *testing.T) {
	sink := startAlertSink(t)
	site := startSite(t)
	sink.observeRequests(site)

	startGoplum(
		t,
		writeConfig(t, sink.config(), httpCheck("site", site.URL, "failing_threshold = 3", "good_threshold = 4")),
		tombstonePath(t),
	).waitForState(t, "site", api.Status_GOOD)

	// Too few failures in a row shouldn't change anything.
	site.setStatusFor(http.StatusInternalServerError, 2)
	site.waitForMoreRequests(t, 8)
	assert.Empty(t, sink.alerts())

	// An alert is sent before the check next runs, so the number of requests seen by the time it's delivered tells
	// us exactly which execution triggered it.
	site.setStatus(http.StatusInternalServerError)
	sink.waitForAlert(t, 1)
	assert.Equal(t, 3, sink.requestsAtAlert(0))

	site.setStatus(http.StatusOK)
	sink.waitForAlert(t, 2)
	assert.Equal(t, 4, sink.requestsAtAlert(1))
}

func Test_Run_SendsRemindersWhileFailing(t *testing.T) {
	const reminder = 100 * time.Millisecond

	sink := startAlertSink(t)
	site := startSite(t)
	site.setStatus(http.StatusInternalServerError)

	startGoplum(
		t,
		writeConfig(t, sink.config(), httpCheck("site", site.URL, "reminder = "+millis(reminder))),
		tombstonePath(t),
	)

	first := sink.waitForAlert(t, 1)
	assert.False(t, first.IsReminder)

	second := sink.waitForAlert(t, 2)
	assert.True(t, second.IsReminder)
	assert.Equal(t, "Check 'site' is still failing (Bad status code: 500).", second.Text)
	assert.Equal(t, goplum.StateFailing, second.NewState)

	third := sink.waitForAlert(t, 3)
	assert.True(t, third.IsReminder)

	times := sink.times()
	assert.GreaterOrEqual(t, times[1].Sub(times[0]), reminder)
	assert.GreaterOrEqual(t, times[2].Sub(times[1]), reminder)

	site.setStatus(http.StatusOK)
	require.Eventually(
		t,
		func() bool {
			alerts := sink.alerts()
			return alerts[len(alerts)-1].NewState == goplum.StateGood
		},
		waitTimeout,
		waitTick,
	)
	alerts := sink.alerts()
	assert.False(t, alerts[len(alerts)-1].IsReminder)

	// Requests are at least an interval apart, so this is long enough for another reminder to have been due.
	site.waitForMoreRequests(t, int(reminder/checkInterval)+2)
	assert.Len(t, sink.alerts(), len(alerts))
}

func Test_Run_LimitsAlertsPerGroup(t *testing.T) {
	sink := startAlertSink(t)

	var sites []*site
	config := []string{sink.config(), `group "limited" {` + "\n alert_limit = 2\n alert_window = 1h\n}\n"}
	for i := range 3 {
		s := startSite(t)
		s.setStatus(http.StatusInternalServerError)
		sites = append(sites, s)
		config = append(config, httpCheck(fmt.Sprintf("limited%d", i), s.URL, `groups = ["limited"]`))
	}

	ungrouped := startSite(t)
	ungrouped.setStatus(http.StatusInternalServerError)
	config = append(config, httpCheck("ungrouped", ungrouped.URL))

	g := startGoplum(t, writeConfig(t, config...), tombstonePath(t))

	sink.waitForAlert(t, 3)
	for i := range sites {
		g.waitForState(t, fmt.Sprintf("limited%d", i), api.Status_FAILING)
		sites[i].waitForMoreRequests(t, 3)
	}

	var limited []goplum.AlertDetails
	var others []goplum.AlertDetails
	for _, alert := range sink.alerts() {
		if alert.Name == "ungrouped" {
			others = append(others, alert)
		} else {
			limited = append(limited, alert)
		}
	}

	assert.Len(t, others, 1)
	require.Len(t, limited, 2)

	// Checks run concurrently, so the alerts could arrive in either order.
	warnings := 0
	for _, alert := range limited {
		if strings.HasSuffix(alert.Text, "[GROUP ALERT LIMIT REACHED: limited]") {
			warnings++
		}
	}
	assert.Equal(t, 1, warnings, "only the last alert should warn about the limit")
}

func Test_Run_ResumesAlertsAfterGroupWindow(t *testing.T) {
	const window = 500 * time.Millisecond

	sink := startAlertSink(t)
	site := startSite(t)
	site.setStatus(http.StatusInternalServerError)

	startGoplum(
		t,
		writeConfig(
			t,
			sink.config(),
			fmt.Sprintf("group \"limited\" {\n alert_limit = 1\n alert_window = %s\n}\n", millis(window)),
			httpCheck("site", site.URL, `groups = ["limited"]`, "reminder = "+millis(checkInterval)),
		),
		tombstonePath(t),
	)

	sink.waitForAlert(t, 1)
	assert.True(t, sink.waitForAlert(t, 2).IsReminder)

	// The window starts just before the first alert is delivered, so allow for some slop in the delivery times.
	times := sink.times()
	assert.Greater(t, times[1].Sub(times[0]), window/2)
}

func Test_Run_Heartbeat_FailsWhenMissingAndRecoversWhenReceived(t *testing.T) {
	sink := startAlertSink(t)
	clock := startSite(t)
	port := freePort(t)
	tombstone := tombstonePath(t)

	// The check won't accept a window shorter than 30 seconds, so start with one that's almost used up.
	writeHeartbeatTombstone(
		t,
		tombstone,
		heartbeat.SavedState{
			Created:  time.Now().Add(-time.Hour),
			Received: time.Now().Add(-30*time.Second + 200*time.Millisecond),
		},
	)

	g := startGoplum(t, writeConfig(t, sink.config(), heartbeatConfig(port), httpCheck("clock", clock.URL)), tombstone)

	failed := sink.waitForAlert(t, 1)
	assert.Equal(t, "beat", failed.Name)
	assert.Contains(t, failed.Text, "No heartbeat received in")
	assert.Equal(t, goplum.StateGood, failed.PreviousState)
	assert.Equal(t, goplum.StateFailing, failed.NewState)

	assert.Equal(t, http.StatusAccepted, sendHeartbeat(t, port, heartbeatID))

	recovered := sink.waitForAlert(t, 2)
	assert.Equal(t, "Check 'beat' is now good, was failing.", recovered.Text)

	clock.waitForMoreRequests(t, 5)
	assert.Len(t, sink.alerts(), 2)
	assert.Equal(t, api.Status_GOOD, g.check(t, "beat").GetState())
}

func Test_Run_Heartbeat_GoodOnceReceived(t *testing.T) {
	sink := startAlertSink(t)
	port := freePort(t)

	g := startGoplum(t, writeConfig(t, sink.config(), heartbeatConfig(port)), tombstonePath(t))

	assert.Equal(t, http.StatusNotFound, sendHeartbeat(t, port, strings.Repeat("0", 32)))
	assert.Equal(t, http.StatusAccepted, sendHeartbeat(t, port, heartbeatID))

	g.waitForState(t, "beat", api.Status_GOOD)
	assert.Empty(t, sink.alerts())
}

func Test_Run_Tombstone_RestartDoesNotReAlert(t *testing.T) {
	sink := startAlertSink(t)
	site := startSite(t)
	site.setStatus(http.StatusInternalServerError)

	// The long reminder ensures the time of the last alert is restored too, or we'd get a reminder straight away.
	config := writeConfig(t, sink.config(), httpCheck("site", site.URL, "reminder = 1h"))
	tombstone := tombstonePath(t)

	first := startGoplum(t, config, tombstone)
	sink.waitForAlert(t, 1)
	first.stop(t)
	require.FileExists(t, tombstone)

	second := startGoplum(t, config, tombstone)
	restored := second.check(t, "site")
	assert.Equal(t, api.Status_FAILING, restored.GetState())
	assert.True(t, restored.GetSettled())

	site.waitForMoreRequests(t, 5)
	assert.Len(t, sink.alerts(), 1)

	site.setStatus(http.StatusOK)
	recovered := sink.waitForAlert(t, 2)
	assert.Equal(t, "Check 'site' is now good, was failing.", recovered.Text)
	second.stop(t)

	third := startGoplum(t, config, tombstone)
	assert.Equal(t, api.Status_GOOD, third.check(t, "site").GetState())
	site.waitForMoreRequests(t, 5)
	assert.Len(t, sink.alerts(), 2)
}

func Test_Run_Tombstone_IgnoredIfTooOld(t *testing.T) {
	sink := startAlertSink(t)
	site := startSite(t)
	site.setStatus(http.StatusInternalServerError)
	tombstone := tombstonePath(t)

	old := goplum.TombStone{
		Time: time.Now().Add(-time.Hour),
		Checks: map[string]goplum.CheckTombStone{
			"site": {State: goplum.StateFailing, Settled: true, LastAlertTime: time.Now().Add(-time.Hour)},
		},
	}
	require.NoError(t, old.Save(tombstone))

	startGoplum(t, writeConfig(t, sink.config(), httpCheck("site", site.URL)), tombstone)

	assert.Equal(t, goplum.StateIndeterminate, sink.waitForAlert(t, 1).PreviousState)
}

func Test_Run_Tombstone_CorruptFileIsIgnored(t *testing.T) {
	sink := startAlertSink(t)
	site := startSite(t)
	tombstone := tombstonePath(t)
	writeFile(t, tombstone, "not json")

	startGoplum(t, writeConfig(t, sink.config(), httpCheck("site", site.URL)), tombstone).
		waitForState(t, "site", api.Status_GOOD)
}

func Test_Run_Tombstone_UnwritablePathStillShutsDown(t *testing.T) {
	sink := startAlertSink(t)
	site := startSite(t)
	tombstone := filepath.Join(t.TempDir(), "no", "such", "directory", "tomb")

	g := startGoplum(t, writeConfig(t, sink.config(), httpCheck("site", site.URL)), tombstone)
	g.waitForState(t, "site", api.Status_GOOD)

	g.stop(t)
	assert.NoFileExists(t, tombstone)
}

func Test_Run_Api_ListsChecks(t *testing.T) {
	sink := startAlertSink(t)
	good := startSite(t)
	bad := startSite(t)
	bad.setStatus(http.StatusInternalServerError)

	g := startGoplum(
		t,
		writeConfig(t, sink.config(), httpCheck("good", good.URL), httpCheck("bad", bad.URL)),
		tombstonePath(t),
	)
	g.waitForState(t, "good", api.Status_GOOD)
	g.waitForState(t, "bad", api.Status_FAILING)

	list, err := g.client.GetChecks(context.Background(), &api.Empty{})
	require.NoError(t, err)

	checks := map[string]*api.Check{}
	for _, c := range list.GetChecks() {
		checks[c.GetName()] = c
	}
	require.Len(t, checks, 2)

	assert.Equal(t, "http.get", checks["good"].GetType())
	assert.Equal(t, api.Status_GOOD, checks["good"].GetState())
	assert.True(t, checks["good"].GetSettled())
	assert.Positive(t, checks["good"].GetLastRun())
	assert.False(t, checks["good"].GetSuspended())
	assert.Equal(t, api.Status_FAILING, checks["bad"].GetState())

	_, err = g.client.GetCheck(context.Background(), &api.CheckName{Name: "nope"})
	assert.ErrorContains(t, err, "no check found with name: nope")
}

func Test_Run_Api_SuspendsAndResumesChecks(t *testing.T) {
	sink := startAlertSink(t)
	target := startSite(t)

	config := writeConfig(t, sink.config(), httpCheck("target", target.URL))
	tombstone := tombstonePath(t)

	g := startGoplum(t, config, tombstone)
	g.waitForState(t, "target", api.Status_GOOD)

	suspended, err := g.client.SuspendCheck(context.Background(), &api.CheckName{Name: "target"})
	require.NoError(t, err)
	assert.True(t, suspended.GetSuspended())
	assert.True(t, g.check(t, "target").GetSuspended())

	// The check may have already been running or queued when it was suspended.
	time.Sleep(5 * checkInterval)
	before := target.requests()
	time.Sleep(5 * checkInterval)
	assert.Equal(t, before, target.requests())

	// A suspended check shouldn't raise alerts even if the service goes down.
	target.setStatus(http.StatusInternalServerError)
	time.Sleep(5 * checkInterval)
	assert.Empty(t, sink.alerts())

	g.stop(t)
	g = startGoplum(t, config, tombstone)
	assert.True(t, g.check(t, "target").GetSuspended(), "suspension should survive a restart")

	resumed, err := g.client.ResumeCheck(context.Background(), &api.CheckName{Name: "target"})
	require.NoError(t, err)
	assert.False(t, resumed.GetSuspended())
	assert.False(t, g.check(t, "target").GetSuspended())

	assert.Equal(t, goplum.StateFailing, sink.waitForAlert(t, 1).NewState)

	_, err = g.client.SuspendCheck(context.Background(), &api.CheckName{Name: "nope"})
	assert.ErrorContains(t, err, "no check found with name: nope")
	_, err = g.client.ResumeCheck(context.Background(), &api.CheckName{Name: "nope"})
	assert.ErrorContains(t, err, "no check found with name: nope")
}

func Test_Run_Api_RejectsUntrustedClients(t *testing.T) {
	g := startGoplum(t, writeConfig(t), tombstonePath(t))

	untrusted := generateCerts(t)
	client := dial(t, g.apiPort, certPaths{ca: g.certs.ca, cert: untrusted.cert, key: untrusted.key})

	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()

	_, err := client.GetChecks(ctx, &api.Empty{})
	assert.Error(t, err)
}

func Test_Run_Api_MissingCertificatesLeaveChecksRunning(t *testing.T) {
	site := startSite(t)
	sink := startAlertSink(t)
	port := freePort(t)

	signals := make(chan os.Signal, 1)
	done := make(chan error, 1)
	config := writeConfig(t, sink.config(), httpCheck("site", site.URL))
	tombstone := tombstonePath(t)
	missing := filepath.Join(t.TempDir(), "missing")

	go func() {
		done <- runGoplum(config, tombstone, certPaths{ca: missing, cert: missing, key: missing}, port, signals)
	}()

	site.waitForMoreRequests(t, 2)

	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err == nil {
		_ = conn.Close()
	}
	assert.Error(t, err, "the API port should not be open")

	signals <- syscall.SIGTERM
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(waitTimeout):
		require.Fail(t, "goplum didn't stop")
	}
}

func Test_Run_Includes(t *testing.T) {
	sink := startAlertSink(t)
	site := startSite(t)
	sink.observeRequests(site)

	config := writeConfig(
		t,
		`
include "conf.d/*.conf"
include "explicit.conf"

defaults {
  failing_threshold = 1
}
`,
	)
	dir := filepath.Dir(config)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "conf.d"), 0o700))
	writeFile(t, filepath.Join(dir, "conf.d", "alerts.conf"), sink.config())
	writeFile(t, filepath.Join(dir, "conf.d", "ignored.txt"), "this isn't valid config")
	writeFile(t, filepath.Join(dir, "explicit.conf"), httpCheck("included", site.URL))

	startGoplum(t, config, tombstonePath(t)).waitForState(t, "included", api.Status_GOOD)

	site.setStatus(http.StatusInternalServerError)

	alert := sink.waitForAlert(t, 1)
	assert.Equal(t, "included", alert.Name)
	assert.Equal(t, goplum.StateFailing, alert.NewState)
	assert.Equal(t, 1, sink.requestsAtAlert(0), "defaults from the main file should apply to included checks")
}

func Test_Run_PrometheusQuery(t *testing.T) {
	sink := startAlertSink(t)

	var (
		mu       sync.Mutex
		response = prometheusVector("1", "1")
		queries  []string
		tokens   []string
		paths    []string
	)
	setResponse := func(r string) {
		mu.Lock()
		defer mu.Unlock()
		response = r
	}

	server := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				_ = r.ParseForm()
				queries = append(queries, r.PostForm.Get("query"))
				tokens = append(tokens, r.Header.Get("Authorization"))
				paths = append(paths, r.Method+" "+r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(response))
			},
		),
	)
	t.Cleanup(server.Close)

	g := startGoplum(
		t,
		writeConfig(
			t,
			sink.config(),
			fmt.Sprintf("plugin prometheus {\n url = %q\n bearer_token = \"s3cret\"\n}\n", server.URL),
			fmt.Sprintf("check prometheus.query \"prom\" {\n query = \"up == 1\"\n interval = %s\n}\n", millis(checkInterval)),
		),
		tombstonePath(t),
	)
	g.waitForState(t, "prom", api.Status_GOOD)

	setResponse(prometheusVector("1", "0"))
	failed := sink.waitForAlert(t, 1)
	assert.Equal(t, goplum.StateFailing, failed.NewState)
	assert.Contains(t, failed.Text, `1 of 2 series failed: up{instance="b"} = 0`)

	setResponse(prometheusVector("1", "1"))
	assert.Equal(t, goplum.StateGood, sink.waitForAlert(t, 2).NewState)

	setResponse(`{"status":"error","errorType":"bad_data","error":"nope"}`)
	assert.Contains(t, sink.waitForAlert(t, 3).Text, "Query failed: bad_data: nope")

	g.stop(t)

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, queries)
	assert.Equal(t, "up == 1", queries[0])
	assert.Equal(t, "Bearer s3cret", tokens[0])
	assert.Equal(t, "POST /api/v1/query", paths[0])
}

func Test_Run_ShutdownInterruptsRunningChecks(t *testing.T) {
	sink := startAlertSink(t)

	var once sync.Once
	running := make(chan struct{})
	server := httptest.NewServer(
		http.HandlerFunc(
			func(_ http.ResponseWriter, r *http.Request) {
				once.Do(func() { close(running) })
				<-r.Context().Done()
			},
		),
	)
	t.Cleanup(server.Close)

	tombstone := tombstonePath(t)
	g := startGoplum(
		t,
		writeConfig(t, sink.config(), httpCheck("hung", server.URL, "failing_threshold = 1")),
		tombstone,
	)

	<-running
	start := time.Now()
	g.stop(t)

	assert.Less(t, time.Since(start), 4*time.Second, "shouldn't need to wait for the shutdown timeout")
	assert.Empty(t, sink.alerts(), "being interrupted shouldn't count as failing")

	saved, err := goplum.LoadTombStone(tombstone)
	require.NoError(t, err)
	assert.Equal(t, goplum.StateIndeterminate, saved.Checks["hung"].State)
	assert.Nil(t, saved.Checks["hung"].History[0])
}

func Test_Run_ShutdownDoesNotWaitForeverForAlerts(t *testing.T) {
	site := startSite(t)
	site.setStatus(http.StatusInternalServerError)

	var once sync.Once
	sending := make(chan struct{})
	release := make(chan struct{})
	hook := httptest.NewServer(
		http.HandlerFunc(
			func(_ http.ResponseWriter, _ *http.Request) {
				once.Do(func() { close(sending) })
				<-release
			},
		),
	)
	t.Cleanup(hook.Close)
	t.Cleanup(func() { close(release) })

	tombstone := tombstonePath(t)
	g := startGoplum(
		t,
		writeConfig(t, fmt.Sprintf("alert http.webhook \"hook\" {\n url = %q\n}\n", hook.URL), httpCheck("site", site.URL)),
		tombstone,
	)

	<-sending
	g.stop(t)

	saved, err := goplum.LoadTombStone(tombstone)
	require.NoError(t, err)
	assert.Equal(t, goplum.StateFailing, saved.Checks["site"].State)
}

func Test_Main_ExitsWithMessageOnConfigError(t *testing.T) {
	output, err := goplumCommand("--config", filepath.Join(t.TempDir(), "missing.conf")).CombinedOutput()

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Equal(t, 1, exitErr.ExitCode())
	assert.Contains(t, string(output), "unable to read config")
	assert.Contains(t, string(output), "missing.conf")
}

func Test_Main_SavesTombstoneOnSignal(t *testing.T) {
	tests := []struct {
		name      string
		signal    syscall.Signal
		tombstone func(cmd *exec.Cmd, path string)
	}{
		{
			name:   "SIGTERM with tombstone flag",
			signal: syscall.SIGTERM,
			tombstone: func(cmd *exec.Cmd, path string) {
				cmd.Args = append(cmd.Args, "--tombstone", path)
			},
		},
		{
			name:   "SIGINT with tombstone environment variable",
			signal: syscall.SIGINT,
			tombstone: func(cmd *exec.Cmd, path string) {
				cmd.Env = append(cmd.Env, "TOMBSTONE="+path)
			},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				site := startSite(t)
				sink := startAlertSink(t)
				tombstone := tombstonePath(t)
				missing := filepath.Join(t.TempDir(), "missing")

				var output bytes.Buffer
				cmd := goplumCommand(
					"--config",
					writeConfig(t, sink.config(), httpCheck("site", site.URL)),
					"--api-port",
					strconv.Itoa(freePort(t)),
					"--ca-cert",
					missing,
					"--cert",
					missing,
					"--key",
					missing,
					"--quiet",
				)
				tt.tombstone(cmd, tombstone)
				cmd.Stdout = &output
				cmd.Stderr = &output
				require.NoError(t, cmd.Start())

				site.waitForMoreRequests(t, 3)

				require.NoError(t, cmd.Process.Signal(tt.signal))
				assert.NoError(t, cmd.Wait(), output.String())

				saved, err := goplum.LoadTombStone(tombstone)
				require.NoError(t, err)
				assert.Equal(t, goplum.StateGood, saved.Checks["site"].State)
			},
		)
	}
}

func goplumCommand(args ...string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), runMainEnv+"=1")
	return cmd
}

type goplumInstance struct {
	apiPort int
	certs   certPaths
	client  api.GoPlumClient
	signals chan os.Signal
	done    chan error

	stopOnce sync.Once
}

func startGoplum(t *testing.T, config, tombstone string) *goplumInstance {
	t.Helper()

	g := &goplumInstance{
		apiPort: freePort(t),
		certs:   generateCerts(t),
		signals: make(chan os.Signal, 1),
		done:    make(chan error, 1),
	}
	t.Cleanup(func() { g.stop(t) })

	go func() {
		g.done <- runGoplum(config, tombstone, g.certs, g.apiPort, g.signals)
	}()

	require.Eventually(
		t,
		func() bool {
			conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(g.apiPort)))
			if err != nil {
				return false
			}
			_ = conn.Close()
			return true
		},
		waitTimeout,
		waitTick,
		"API didn't start listening",
	)

	g.client = dial(t, g.apiPort, g.certs)
	return g
}

func (g *goplumInstance) stop(t *testing.T) {
	t.Helper()

	g.stopOnce.Do(
		func() {
			g.signals <- syscall.SIGTERM
			select {
			case err := <-g.done:
				assert.NoError(t, err)
			case <-time.After(waitTimeout):
				assert.Fail(t, "goplum didn't stop")
			}
		},
	)
}

func (g *goplumInstance) check(t *testing.T, name string) *api.Check {
	t.Helper()

	check, err := g.client.GetCheck(context.Background(), &api.CheckName{Name: name})
	require.NoError(t, err)
	return check
}

func (g *goplumInstance) waitForState(t *testing.T, name string, state api.Status) {
	t.Helper()

	require.Eventually(
		t,
		func() bool {
			check, err := g.client.GetCheck(context.Background(), &api.CheckName{Name: name})
			return err == nil && check.GetState() == state
		},
		waitTimeout,
		waitTick,
		"check %s never became %s",
		name,
		state,
	)
}

func runGoplum(config, tombstone string, certs certPaths, apiPort int, signals <-chan os.Signal) error {
	settings := map[string]string{
		"api-port": strconv.Itoa(apiPort),
		"ca-cert":  certs.ca,
		"cert":     certs.cert,
		"key":      certs.key,
	}
	for name, value := range settings {
		if err := flag.Set(name, value); err != nil {
			return err
		}
	}

	return goplum.Run(plugins, config, tombstone, signals)
}

// dial connects to the API in the same way as plumctl.
func dial(t *testing.T, port int, certs certPaths) api.GoPlumClient {
	t.Helper()

	conn, err := api.Connect(net.JoinHostPort("localhost", strconv.Itoa(port)), certs.cert, certs.key, certs.ca)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return api.NewGoPlumClient(conn)
}

type site struct {
	*httptest.Server

	mu           sync.Mutex
	status       int
	revertAfter  int
	revertStatus int
	total        int
	sinceLastSet int
}

func startSite(t *testing.T) *site {
	t.Helper()

	s := &site{status: http.StatusOK}
	s.Server = httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				s.mu.Lock()
				defer s.mu.Unlock()

				s.total++
				s.sinceLastSet++
				w.WriteHeader(s.status)

				if s.revertAfter > 0 {
					s.revertAfter--
					if s.revertAfter == 0 {
						s.status = s.revertStatus
					}
				}
			},
		),
	)
	t.Cleanup(s.Close)
	return s
}

func (s *site) setStatus(status int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.status = status
	s.revertAfter = 0
	s.sinceLastSet = 0
}

// setStatusFor serves the given status for the next count requests, before going back to the current one.
func (s *site) setStatusFor(status, count int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.revertStatus = s.status
	s.revertAfter = count
	s.status = status
}

func (s *site) requests() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.total
}

func (s *site) requestsSinceLastSet() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.sinceLastSet
}

func (s *site) waitForMoreRequests(t *testing.T, count int) {
	t.Helper()

	target := s.requests() + count
	require.Eventually(
		t,
		func() bool { return s.requests() >= target },
		waitTimeout,
		waitTick,
		"site didn't receive %d more requests",
		count,
	)
}

type alertSink struct {
	*httptest.Server

	mu         sync.Mutex
	received   []goplum.AlertDetails
	receivedAt []time.Time
	requests   []int
	observed   *site
}

func startAlertSink(t *testing.T) *alertSink {
	t.Helper()

	s := &alertSink{}
	s.Server = httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				var details goplum.AlertDetails
				if err := json.NewDecoder(r.Body).Decode(&details); err != nil {
					t.Errorf("Invalid alert payload: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				s.mu.Lock()
				defer s.mu.Unlock()

				s.received = append(s.received, details)
				s.receivedAt = append(s.receivedAt, time.Now())
				if s.observed != nil {
					s.requests = append(s.requests, s.observed.requestsSinceLastSet())
				}
			},
		),
	)
	t.Cleanup(s.Close)
	return s
}

func (s *alertSink) config() string {
	return fmt.Sprintf("alert http.webhook \"hook\" {\n url = %q\n}\n", s.URL)
}

// observeRequests records how many requests the site has received since its status last changed whenever an alert
// is delivered.
func (s *alertSink) observeRequests(site *site) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.observed = site
}

func (s *alertSink) requestsAtAlert(index int) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.requests[index]
}

func (s *alertSink) alerts() []goplum.AlertDetails {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]goplum.AlertDetails(nil), s.received...)
}

func (s *alertSink) times() []time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]time.Time(nil), s.receivedAt...)
}

// waitForAlert waits until at least count alerts have been received, and returns the one with that (1-based) position.
func (s *alertSink) waitForAlert(t *testing.T, count int) goplum.AlertDetails {
	t.Helper()

	require.Eventually(
		t,
		func() bool { return len(s.alerts()) >= count },
		waitTimeout,
		waitTick,
		"didn't receive %d alerts",
		count,
	)
	return s.alerts()[count-1]
}

func httpCheck(name, url string, settings ...string) string {
	return fmt.Sprintf(
		"check http.get %q {\n url = %q\n interval = %s\n %s\n}\n",
		name,
		url,
		millis(checkInterval),
		strings.Join(settings, "\n "),
	)
}

func heartbeatConfig(port int) string {
	return fmt.Sprintf(
		"plugin heartbeat {\n port = %d\n path = \"heartbeat\"\n}\n"+
			"check heartbeat.received \"beat\" {\n id = %q\n within = 30s\n interval = %s\n failing_threshold = 1\n good_threshold = 1\n}\n",
		port,
		heartbeatID,
		millis(checkInterval),
	)
}

func writeHeartbeatTombstone(t *testing.T, path string, state heartbeat.SavedState) {
	t.Helper()

	pluginState, err := json.Marshal(state)
	require.NoError(t, err)

	ts := goplum.TombStone{
		Time: time.Now(),
		Checks: map[string]goplum.CheckTombStone{
			"beat": {State: goplum.StateGood, Settled: true, PluginState: pluginState},
		},
	}
	require.NoError(t, ts.Save(path))
}

func sendHeartbeat(t *testing.T, port int, id string) int {
	t.Helper()

	res, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/heartbeat/%s", port, id))
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	return res.StatusCode
}

func prometheusVector(a, b string) string {
	return fmt.Sprintf(
		`{"status":"success","data":{"resultType":"vector","result":[`+
			`{"metric":{"__name__":"up","instance":"a"},"value":[1700000000,%q]},`+
			`{"metric":{"__name__":"up","instance":"b"},"value":[1700000000,%q]}]}}`,
		a,
		b,
	)
}

func millis(d time.Duration) string {
	return strconv.FormatInt(d.Milliseconds(), 10) + "ms"
}

func writeConfig(t *testing.T, parts ...string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "goplum.conf")
	writeFile(t, path, strings.Join(parts, "\n"))
	return path
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func tombstonePath(t *testing.T) string {
	t.Helper()

	return filepath.Join(t.TempDir(), "goplum.tomb")
}

// freePort picks a port below the ephemeral range. A port handed out by the kernel could be reused for one of the many
// outgoing connections the tests make before goplum gets around to listening on it.
func freePort(t *testing.T) int {
	t.Helper()

	for range 100 {
		port := 20000 + mathrand.IntN(10000)
		listener, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", strconv.Itoa(port)))
		if err == nil {
			_ = listener.Close()
			return port
		}
	}

	require.Fail(t, "unable to find a free port")
	return 0
}

type certPaths struct {
	ca   string
	cert string
	key  string
}

// generateCerts creates a new certificate authority and a certificate signed by it, usable by both client and server.
func generateCerts(t *testing.T) certPaths {
	t.Helper()

	dir := t.TempDir()
	paths := certPaths{
		ca:   filepath.Join(dir, "ca.crt"),
		cert: filepath.Join(dir, "goplum.crt"),
		key:  filepath.Join(dir, "goplum.key"),
	}

	notBefore := time.Now().Add(-time.Hour)
	notAfter := time.Now().Add(time.Hour)

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ca"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, caKey.Public(), caKey)
	require.NoError(t, err)
	writePEM(t, paths.ca, "CERTIFICATE", caDER)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.CreateCertificate(
		rand.Reader,
		&x509.Certificate{
			SerialNumber: big.NewInt(2),
			Subject:      pkix.Name{CommonName: "localhost"},
			DNSNames:     []string{"localhost"},
			NotBefore:    notBefore,
			NotAfter:     notAfter,
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		},
		caTemplate,
		key.Public(),
		caKey,
	)
	require.NoError(t, err)
	writePEM(t, paths.cert, "CERTIFICATE", der)

	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	writePEM(t, paths.key, "EC PRIVATE KEY", keyDER)

	return paths
}

func writePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()

	data := pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der})
	require.NoError(t, os.WriteFile(path, data, 0o600))
}
