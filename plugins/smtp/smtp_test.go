package smtp

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"chameth.com/goplum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withSendTimeout(t *testing.T, d time.Duration) {
	old := sendTimeout
	sendTimeout = d
	t.Cleanup(func() { sendTimeout = old })
}

func TestSendAlert_TimesOutIfServerNeverResponds(t *testing.T) {
	withSendTimeout(t, 100*time.Millisecond)

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_, _ = conn.Read(make([]byte, 1024))
	}()

	start := time.Now()
	err = SendAlert{
		Server: listener.Addr().String(),
		From:   "a@example.com",
		To:     "b@example.com",
	}.Send(goplum.AlertDetails{LastResult: &goplum.Result{}})
	var netErr net.Error
	require.ErrorAs(t, err, &netErr)
	assert.True(t, netErr.Timeout())
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestSendAlert_DeliversMessage(t *testing.T) {
	withSendTimeout(t, 5*time.Second)

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	commands := make(chan []string, 1)
	go func() {
		var received []string
		defer func() { commands <- received }()

		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		r := bufio.NewReader(conn)
		write := func(s string) { _, _ = conn.Write([]byte(s + "\r\n")) }
		write("220 test")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimSpace(line)
			received = append(received, line)
			switch {
			case strings.HasPrefix(line, "EHLO"):
				write("250-test")
				write("250 AUTH PLAIN")
			case strings.HasPrefix(line, "AUTH"):
				write("235 ok")
			case line == "DATA":
				write("354 go ahead")
				for {
					l, err := r.ReadString('\n')
					if err != nil || l == ".\r\n" {
						break
					}
				}
				write("250 ok")
			case line == "QUIT":
				write("221 bye")
				return
			default:
				write("250 ok")
			}
		}
	}()

	err = SendAlert{
		Server:   listener.Addr().String(),
		Username: "user",
		Password: "pass",
		From:     "a@example.com",
		To:       "b@example.com",
	}.Send(goplum.AlertDetails{LastResult: &goplum.Result{}})
	require.NoError(t, err)
	assert.Equal(
		t,
		[]string{
			"EHLO localhost",
			"AUTH PLAIN AHVzZXIAcGFzcw==",
			"MAIL FROM:<a@example.com>",
			"RCPT TO:<b@example.com>",
			"DATA",
			"QUIT",
		},
		<-commands,
	)
}
