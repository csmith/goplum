package debug

import (
	"context"
	"github.com/csmith/goplum"
	"log"
	"math/rand"
)

type Plugin struct{}

func (p Plugin) Alert(kind string) goplum.Alert {
	switch kind {
	case "sysout":
		return SysOutAlert{}
	default:
		return nil
	}
}

func (p Plugin) Check(kind string) goplum.Check {
	switch kind {
	case "random":
		return RandomCheck{PercentGood: 0.5}
	default:
		return nil
	}
}

type RandomCheck struct {
	PercentGood float64 `config:"percent_good"`
}

func (t RandomCheck) Execute(_ context.Context) goplum.Result {
	r := rand.Float64()
	if r <= t.PercentGood {
		return goplum.GoodResult()
	} else {
		return goplum.FailingResult("Random value %f greater than percent_good %f", r, t.PercentGood)
	}
}

type SysOutAlert struct {
}

func (s SysOutAlert) Send(details goplum.AlertDetails) error {
	log.Printf("DEBUG ALERT - %s\n", details.Text)
	return nil
}
