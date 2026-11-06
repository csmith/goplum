package smtp

import (
	"bufio"
	"io"
	"net"
	"net/mail"
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

type smtpSession struct {
	commands []string
	data     string
}

func startServer(t *testing.T) (string, <-chan smtpSession) {
	t.Helper()

	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	sessions := make(chan smtpSession, 1)
	go func() {
		var session smtpSession
		defer func() { sessions <- session }()

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
			session.commands = append(session.commands, line)
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
					session.data += l
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

	return listener.Addr().String(), sessions
}

func TestSendAlert_DeliversMessage(t *testing.T) {
	withSendTimeout(t, 5*time.Second)
	addr, sessions := startServer(t)

	err := SendAlert{
		Server:   addr,
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
		(<-sessions).commands,
	)
}

func sendAndParse(t *testing.T, details goplum.AlertDetails) *mail.Message {
	t.Helper()
	withSendTimeout(t, 5*time.Second)
	addr, sessions := startServer(t)

	require.NoError(
		t,
		SendAlert{
			Server:        addr,
			Username:      "user",
			Password:      "pass",
			SubjectPrefix: "Alert: ",
			From:          "a@example.com",
			To:            "b@example.com",
		}.Send(details),
	)

	message, err := mail.ReadMessage(strings.NewReader((<-sessions).data))
	require.NoError(t, err)
	return message
}

func TestSendAlert_KeepsSubjectOnOneLine(t *testing.T) {
	tests := map[string]struct {
		text     string
		expected string
	}{
		"CRLF":      {"one\r\nX-Extra: two\r\n\r\nthree", "Alert: one X-Extra: two  three"},
		"bare LF":   {"one\nX-Extra: two", "Alert: one X-Extra: two"},
		"bare CR":   {"one\rX-Extra: two", "Alert: one X-Extra: two"},
		"no breaks": {"one two", "Alert: one two"},
	}

	for name, tt := range tests {
		t.Run(
			name,
			func(t *testing.T) {
				message := sendAndParse(
					t,
					goplum.AlertDetails{
						Text:       tt.text,
						LastResult: &goplum.Result{Detail: tt.text},
					},
				)

				assert.Equal(t, tt.expected, message.Header.Get("Subject"))
				assert.Empty(t, message.Header.Get("X-Extra"))
				assert.Equal(t, "a@example.com", message.Header.Get("From"))
				assert.Equal(t, "b@example.com", message.Header.Get("To"))
			},
		)
	}
}

func TestSendAlert_ShortensLongSubjectButNotBody(t *testing.T) {
	detail := strings.Repeat("line of output\n", 100)

	message := sendAndParse(
		t,
		goplum.AlertDetails{
			Text:       detail,
			LastResult: &goplum.Result{Detail: detail},
		},
	)

	subject := message.Header.Get("Subject")
	assert.Len(t, []rune(subject), 200)
	assert.True(t, strings.HasPrefix(subject, "Alert: line of output line of output"), subject)

	body, err := io.ReadAll(message.Body)
	require.NoError(t, err)
	assert.Equal(t, 100, strings.Count(string(body), "line of output"))
}
