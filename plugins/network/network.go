package network

import (
	"context"
	"fmt"
	"net"

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
