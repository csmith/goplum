package snmp

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"chameth.com/goplum"
	"github.com/gosnmp/gosnmp"
)

type Plugin struct{}

func (p Plugin) Alert(kind string) goplum.Alert {
	return nil
}

func (p Plugin) Check(kind string) goplum.Check {
	switch kind {
	case "string":
		return StringCheck{
			BaseCheck: BaseCheck{
				Community: "public",
				Port:      161,
			},
			ContentExpected: true,
		}
	case "int":
		return IntCheck{
			BaseCheck: BaseCheck{
				Community: "public",
				Port:      161,
			},
			AtLeast: math.MinInt64,
			AtMost:  math.MaxInt64,
		}
	default:
		return nil
	}
}

type BaseCheck struct {
	Agent     string
	Port      int
	Community string
	Oid       []string

	client *gosnmp.GoSNMP
}

func (b BaseCheck) Validate() error {
	if len(b.Agent) == 0 {
		return fmt.Errorf("missing required argument: agent")
	}

	if b.Port <= 0 || b.Port > 65535 {
		return fmt.Errorf("invalid argument: port")
	}

	if len(b.Community) == 0 {
		return fmt.Errorf("missing required argument: community")
	}

	if len(b.Oid) == 0 {
		return fmt.Errorf("missing required argument: oid")
	}

	return nil
}

func (b BaseCheck) retrieve() (*gosnmp.SnmpPacket, error) {
	if b.client == nil {
		b.client = &gosnmp.GoSNMP{
			Target:             b.Agent,
			Port:               uint16(b.Port),
			Transport:          "udp",
			Community:          b.Community,
			Version:            gosnmp.Version2c,
			Timeout:            time.Duration(5) * time.Second,
			Retries:            3,
			ExponentialTimeout: true,
			MaxOids:            gosnmp.MaxOids,
		}

		if err := b.client.Connect(); err != nil {
			b.client = nil
			return nil, err
		}
	}

	return b.client.Get(b.Oid)
}

type StringCheck struct {
	BaseCheck       `config:",squash"`
	Content         string
	ContentExpected bool `config:"content_expected"`
}

func (s StringCheck) Execute(ctx context.Context) goplum.Result {
	packet, err := s.BaseCheck.retrieve()
	if err != nil {
		return goplum.FailingResult("SNMP failed: %v", err)
	}

	for i := range packet.Variables {
		variable := packet.Variables[i]
		if variable.Type == gosnmp.OctetString {
			c := variable.Value.(string)
			found := strings.Contains(c, s.Content)
			if found && !s.ContentExpected {
				return goplum.FailingResult("OID %s contained content %s", variable.Name, s.Content)
			} else if !found && s.ContentExpected {
				return goplum.FailingResult("OID %s did not contain content %s", variable.Name, s.Content)
			}
		} else {
			return goplum.FailingResult("OID did not return a string: %s", variable.Name)
		}
	}

	return goplum.GoodResult()
}

func (s StringCheck) Validate() error {
	return s.BaseCheck.Validate()
}

type IntCheck struct {
	BaseCheck `config:",squash"`
	AtLeast   int64 `config:"at_least"`
	AtMost    int64 `config:"at_most"`
}

func (c IntCheck) Execute(ctx context.Context) goplum.Result {
	packet, err := c.BaseCheck.retrieve()
	if err != nil {
		return goplum.FailingResult("SNMP failed: %v", err)
	}

	for i := range packet.Variables {
		variable := packet.Variables[i]
		val := gosnmp.ToBigInt(variable.Value)
		if val.Cmp(big.NewInt(c.AtLeast)) == -1 {
			return goplum.FailingResult("OID %s returned %d, must be at least %d", variable.Name, val, c.AtLeast)
		} else if val.Cmp(big.NewInt(c.AtMost)) == 1 {
			return goplum.FailingResult("OID %s returned %d, must be at most %d", variable.Name, val, c.AtMost)
		}
	}

	return goplum.GoodResult()
}

func (c IntCheck) Validate() error {
	return c.BaseCheck.Validate()
}
