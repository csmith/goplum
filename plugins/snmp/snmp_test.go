package snmp

import (
	"context"
	"net"
	"os"
	"runtime"
	"testing"
	"time"

	"chameth.com/goplum"
	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntCheck_DoesNotLeakSockets(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("counting file descriptors requires /proc/self/fd")
	}

	check := IntCheck{
		Agent:     "127.0.0.1",
		Port:      startAgent(t, true),
		Community: "public",
		Oid:       []string{".1.3.6.1.2.1.1.3.0"},
		AtLeast:   0,
		AtMost:    100,
	}

	before := openFds(t)
	for range 20 {
		result := check.Execute(context.Background())
		require.Equal(t, goplum.StateGood, result.State, result.Detail)
	}
	assert.Equal(t, before, openFds(t))
}

func TestIntCheck_HonoursContext(t *testing.T) {
	check := IntCheck{
		Agent:     "127.0.0.1",
		Port:      startAgent(t, false),
		Community: "public",
		Oid:       []string{".1.3.6.1.2.1.1.3.0"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	result := check.Execute(ctx)
	assert.Equal(t, goplum.StateFailing, result.State)
	assert.Less(t, time.Since(start), 2*time.Second)
}

// startAgent listens on a local UDP port and, if respond is set, answers every Get with an integer value of 42.
func startAgent(t *testing.T, respond bool) int {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	go func() {
		buf := make([]byte, 65535)
		decoder := &gosnmp.GoSNMP{Version: gosnmp.Version2c}
		for {
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if !respond {
				continue
			}

			request, err := decoder.SnmpDecodePacket(buf[:n])
			if err != nil {
				continue
			}

			request.PDUType = gosnmp.GetResponse
			for i := range request.Variables {
				request.Variables[i].Type = gosnmp.Integer
				request.Variables[i].Value = 42
			}

			out, err := request.MarshalMsg()
			if err != nil {
				continue
			}
			_, _ = conn.WriteToUDP(out, addr)
		}
	}()

	return conn.LocalAddr().(*net.UDPAddr).Port
}

func openFds(t *testing.T) int {
	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)
	return len(entries)
}
