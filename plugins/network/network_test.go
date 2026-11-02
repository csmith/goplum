package network

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"chameth.com/goplum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortScanCheck_SinglePortOpen(t *testing.T) {
	port := listenRange(t, 1)

	result := executeWithGuard(t, context.Background(), portScan(port, port))

	assert.Equal(t, goplum.StateFailing, result.State)
	assert.Equal(t, fmt.Sprintf("Unexpected open ports: [%d]", port), result.Detail)
}

func TestPortScanCheck_AllPortsOpen(t *testing.T) {
	start := listenRange(t, 3)

	result := executeWithGuard(t, context.Background(), portScan(start, start+2))

	assert.Equal(t, goplum.StateFailing, result.State)
	assert.Equal(t, fmt.Sprintf("Unexpected open ports: [%d %d %d]", start, start+1, start+2), result.Detail)
}

func TestPortScanCheck_AllowedPortsOpen(t *testing.T) {
	start := listenRange(t, 3)
	check := portScan(start, start+2)
	check.Allow = []int{start, start + 1, start + 2}

	result := executeWithGuard(t, context.Background(), check)

	assert.Equal(t, goplum.StateGood, result.State)
}

func TestPortScanCheck_CancelledContext(t *testing.T) {
	port := listenRange(t, 1)
	check := portScan(port, port)
	check.Allow = []int{port}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := executeWithGuard(t, ctx, check)

	assert.Equal(t, goplum.StateIndeterminate, result.State)
}

func TestPortScanCheck_TimeoutCoversRangesSmallerThanConcurrency(t *testing.T) {
	check := portScan(1000, 1000)
	check.ConcurrentConnections = 100

	assert.GreaterOrEqual(t, check.Timeout(), check.ConnectionTimeout)
}

func TestPortScanCheck_ValidateRejectsZeroConnectionTimeout(t *testing.T) {
	check := portScan(1, 10)
	check.ConnectionTimeout = 0

	assert.Error(t, check.Validate())
}

func portScan(start, end int) PortScanCheck {
	return PortScanCheck{
		Network:               "tcp",
		Address:               "127.0.0.1",
		Start:                 start,
		End:                   end,
		ConcurrentConnections: 2,
		ConnectionTimeout:     time.Second,
	}
}

func executeWithGuard(t *testing.T, ctx context.Context, check PortScanCheck) goplum.Result {
	t.Helper()
	require.NoError(t, check.Validate())

	done := make(chan goplum.Result, 1)
	go func() {
		done <- check.Execute(ctx)
	}()

	select {
	case result := <-done:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("port scan did not finish")
		return goplum.Result{}
	}
}

// listenRange opens listeners on count consecutive local ports and returns the first port.
func listenRange(t *testing.T, count int) int {
	t.Helper()
	for range 20 {
		base, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
		require.NoError(t, err)
		listeners := []net.Listener{base}
		start := base.Addr().(*net.TCPAddr).Port

		for p := start + 1; p < start+count && p <= 65535; p++ {
			l, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(p)))
			if err != nil {
				break
			}
			listeners = append(listeners, l)
		}

		if len(listeners) == count {
			t.Cleanup(
				func() {
					for _, l := range listeners {
						_ = l.Close()
					}
				},
			)
			return start
		}

		for _, l := range listeners {
			_ = l.Close()
		}
	}
	t.Skip("unable to find a range of free consecutive ports")
	return 0
}

func TestConnectCheck_ValidateNetwork(t *testing.T) {
	tests := []struct {
		network string
		valid   bool
	}{
		{"tcp", true},
		{"tcp4", true},
		{"tcp6", true},
		{"udp", false},
		{"udp6", false},
		{"tpc", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(
			tt.network,
			func(t *testing.T) {
				check := ConnectCheck{Network: tt.network, Address: "127.0.0.1:80"}
				if tt.valid {
					require.NoError(t, check.Validate())
				} else {
					require.Error(t, check.Validate())
				}
			},
		)
	}
}

func TestPortScanCheck_ValidateNetwork(t *testing.T) {
	tests := []struct {
		network string
		valid   bool
	}{
		{"tcp", true},
		{"tcp4", true},
		{"tcp6", true},
		{"udp", false},
		{"udp4", false},
		{"tpc", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(
			tt.network,
			func(t *testing.T) {
				check := portScan(1, 10)
				check.Network = tt.network
				if tt.valid {
					require.NoError(t, check.Validate())
				} else {
					require.Error(t, check.Validate())
				}
			},
		)
	}
}

func TestPlugin_DefaultNetworkIsValid(t *testing.T) {
	connect := Plugin{}.Check("connect").(ConnectCheck)
	connect.Address = "127.0.0.1:80"
	require.NoError(t, connect.Validate())

	scan := Plugin{}.Check("portscan").(PortScanCheck)
	scan.Address = "127.0.0.1"
	require.NoError(t, scan.Validate())
}
