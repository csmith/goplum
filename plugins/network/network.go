package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"chameth.com/goplum"
)

type Plugin struct{}

func (p Plugin) Alert(kind string) goplum.Alert {
	switch kind {
	default:
		return nil
	}
}

func (p Plugin) Check(kind string) goplum.Check {
	switch kind {
	case "connect":
		return ConnectCheck{
			Network: "tcp",
		}
	case "portscan":
		return PortScanCheck{
			Network: "tcp",

			Start: 1,
			End:   65535,

			ConcurrentConnections: 100,
			ConnectionTimeout:     5 * time.Second,
		}
	default:
		return nil
	}
}

var validNetworks = []string{"tcp", "tcp4", "tcp6"}

func validateNetwork(network string) error {
	if !slices.Contains(validNetworks, network) {
		return fmt.Errorf("invalid network %q: must be one of %s", network, strings.Join(validNetworks, ", "))
	}
	return nil
}

type ConnectCheck struct {
	Network string
	Address string
}

func (c ConnectCheck) Execute(ctx context.Context) goplum.Result {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, c.Network, c.Address)
	if err != nil {
		return goplum.FailingResult("unable to connect to %s: %v", c.Address, err)
	}
	defer conn.Close()
	return goplum.GoodResult()
}

func (c ConnectCheck) Validate() error {
	if len(c.Address) == 0 {
		return errors.New("missing required argument: address")
	}

	if err := validateNetwork(c.Network); err != nil {
		return err
	}

	_, _, err := net.SplitHostPort(c.Address)
	if err != nil {
		return err
	}

	return nil
}

type PortScanCheck struct {
	Network string
	Address string

	Start int
	End   int
	Allow []int

	ConcurrentConnections int           `config:"concurrent_connections"`
	ConnectionTimeout     time.Duration `config:"connection_timeout"`
}

func (c PortScanCheck) Timeout() time.Duration {
	// Allow an extra batch of slack so the final dials can time out on their own before the check is cancelled.
	return c.ConnectionTimeout * time.Duration((c.End-c.Start+c.ConcurrentConnections)/c.ConcurrentConnections+1)
}

func (c PortScanCheck) Execute(ctx context.Context) goplum.Result {
	var (
		mu   sync.Mutex
		open []int
	)

	queue := make(chan int)
	wg := sync.WaitGroup{}
	for range c.ConcurrentConnections {
		wg.Go(
			func() {
				for port := range queue {
					if c.check(ctx, port) {
						mu.Lock()
						open = append(open, port)
						mu.Unlock()
					}
				}
			},
		)
	}

enqueue:
	for p := c.Start; p <= c.End; p++ {
		select {
		case queue <- p:
		case <-ctx.Done():
			break enqueue
		}
	}
	close(queue)
	wg.Wait()

	slices.Sort(open)
	var failures []int
	for _, p := range open {
		if !c.allowed(p) {
			failures = append(failures, p)
		}
	}

	if len(failures) > 0 {
		return goplum.FailingResult("Unexpected open ports: %v", failures)
	}
	if err := ctx.Err(); err != nil {
		return goplum.IndeterminateResult("Port scan did not complete: %v", err)
	}
	return goplum.GoodResult()
}

func (c PortScanCheck) Validate() error {
	if len(c.Address) == 0 {
		return errors.New("missing required argument: address")
	}

	if _, _, err := net.SplitHostPort(c.Address); err == nil {
		return fmt.Errorf("invalid address %q: must not include a port", c.Address)
	}

	if err := validateNetwork(c.Network); err != nil {
		return err
	}

	if c.Start < 1 || c.End > 65535 || c.Start > c.End {
		return errors.New("invalid ports: must satisfy 1 <= start <= end <= 65535")
	}

	if c.ConcurrentConnections <= 0 {
		return errors.New("invalid number of concurrent connections: must be >0")
	}

	if c.ConnectionTimeout <= 0 {
		return errors.New("invalid connection timeout: must be >0")
	}

	return nil
}

func (c PortScanCheck) check(ctx context.Context, port int) bool {
	d := net.Dialer{Timeout: c.ConnectionTimeout}
	conn, err := d.DialContext(ctx, c.Network, net.JoinHostPort(c.Address, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func (c PortScanCheck) allowed(port int) bool {
	return slices.Contains(c.Allow, port)
}
