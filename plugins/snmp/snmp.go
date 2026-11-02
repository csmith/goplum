package snmp

import (
	"context"
	"errors"
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
			Community:       "public",
			Port:            161,
			ContentExpected: true,
		}
	case "int":
		return IntCheck{
			Community: "public",
			Port:      161,
			AtLeast:   math.MinInt64,
			AtMost:    math.MaxInt64,
		}
	default:
		return nil
	}
}

type BaseCheck struct {
	Agent     string
	Port      int
	Community string `redact:"true"`
	Oid       []string
}

func (b BaseCheck) Validate() error {
	if len(b.Agent) == 0 {
		return errors.New("missing required argument: agent")
	}

	if b.Port <= 0 || b.Port > 65535 {
		return errors.New("invalid argument: port")
	}

	if len(b.Community) == 0 {
		return errors.New("missing required argument: community")
	}

	if len(b.Oid) == 0 {
		return errors.New("missing required argument: oid")
	}

	return nil
}

func (b BaseCheck) retrieve(ctx context.Context) (*gosnmp.SnmpPacket, error) {
	client := &gosnmp.GoSNMP{
		Context:            ctx,
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

	if err := client.Connect(); err != nil {
		return nil, err
	}
	defer client.Conn.Close()

	return client.Get(b.Oid)
}

type StringCheck struct {
	BaseCheck       `config:",squash"`
	Content         string
	ContentExpected bool `config:"content_expected"`
}

func (s StringCheck) Execute(ctx context.Context) goplum.Result {
	packet, err := s.BaseCheck.retrieve(ctx)
	if err != nil {
		return goplum.FailingResult("SNMP failed: %v", err)
	}

	for i := range packet.Variables {
		variable := packet.Variables[i]
		value, ok := octetString(variable)
		if ok {
			found := strings.Contains(value, s.Content)
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

func octetString(variable gosnmp.SnmpPDU) (string, bool) {
	if variable.Type != gosnmp.OctetString {
		return "", false
	}

	switch value := variable.Value.(type) {
	case []byte:
		return string(value), true
	case string:
		return value, true
	default:
		return "", false
	}
}

type IntCheck struct {
	BaseCheck `config:",squash"`
	AtLeast   int64 `config:"at_least"`
	AtMost    int64 `config:"at_most"`
}

func (c IntCheck) Execute(ctx context.Context) goplum.Result {
	packet, err := c.BaseCheck.retrieve(ctx)
	if err != nil {
		return goplum.FailingResult("SNMP failed: %v", err)
	}

	for i := range packet.Variables {
		variable := packet.Variables[i]
		if !isNumeric(variable.Type) {
			return goplum.FailingResult("OID did not return a number: %s (%s)", variable.Name, variable.Type)
		}

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

func isNumeric(t gosnmp.Asn1BER) bool {
	switch t {
	case gosnmp.Integer, gosnmp.Counter32, gosnmp.Counter64, gosnmp.Gauge32, gosnmp.TimeTicks, gosnmp.Uinteger32:
		return true
	default:
		return false
	}
}
