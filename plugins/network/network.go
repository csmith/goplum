package network

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/csmith/goplum"
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
		return fmt.Errorf("missing required argument: address")
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
	return c.ConnectionTimeout * time.Duration((c.End-c.Start)/c.ConcurrentConnections)
}

func (c PortScanCheck) worker(queue <-chan int, job func(int)) {
	for {
		p, more := <-queue
		if more {
			job(p)
		} else {
			return
		}
	}
}

func (c PortScanCheck) check(port int) bool {
	target := net.JoinHostPort(c.Address, strconv.Itoa(port))
	conn, err := net.DialTimeout(c.Network, target, c.ConnectionTimeout)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}

func (c PortScanCheck) allowed(port int) bool {
	return slices.Contains(c.Allow, port)
}

func (c PortScanCheck) Execute(ctx context.Context) goplum.Result {
	open := make(chan int, c.End-c.Start)
	queue := make(chan int)
	wg := sync.WaitGroup{}

	for i := 0; i < c.ConcurrentConnections; i++ {
		go c.worker(queue, func(port int) {
			if c.check(port) {
				open <- port
			}
			wg.Done()
		})
	}

	for p := c.Start; p <= c.End; p++ {
		wg.Add(1)
		queue <- p
	}
	close(queue)

	wg.Wait()
	close(open)

	var failures []int
	for p := range open {
		if !c.allowed(p) {
			failures = append(failures, p)
		}
	}

	if len(failures) == 0 {
		return goplum.GoodResult()
	} else {
		return goplum.FailingResult("Unexpected open ports: %v", failures)
	}
}

func (c PortScanCheck) Validate() error {
	if len(c.Address) == 0 {
		return fmt.Errorf("missing required argument: address")
	}

	if c.Start < 1 || c.End > 65535 || c.Start > c.End {
		return fmt.Errorf("invalid ports: must satisfy 1 <= start <= end <= 65535")
	}

	if c.ConcurrentConnections <= 0 {
		return fmt.Errorf("invalid number of concurrent connections: must be >0")
	}

	return nil
}
