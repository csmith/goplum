package network

import (
	"fmt"
	"github.com/csmith/goplum"
	"net"
	"time"
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
		return ConnectCheck{}
	default:
		return nil
	}
}

type ConnectCheck struct {
	Network string
	Address string
}

func (c ConnectCheck) Execute() goplum.Result {
	conn , err := net.DialTimeout(c.Network, c.Address, 10 * time.Second)
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

	if len(c.Network) == 0 {
		c.Network = "tcp"
	}

	return nil
}
