package debug

import (
	"encoding/json"
	"github.com/csmith/goplum"
	"log"
	"math/rand"
)

type Plugin struct{}

func (h Plugin) Name() string {
	return "debug"
}

func (h Plugin) Checks() []goplum.CheckType {
	return []goplum.CheckType{RandomCheckType{}}
}

func (h Plugin) Alerts() []goplum.AlertType {
	return []goplum.AlertType{SysOutAlertType{}}
}

type RandomParameters struct {
	PercentGood float32 `json:"percent_good"`
}

type RandomCheckType struct{}

func (c RandomCheckType) Name() string {
	return "random"
}

func (c RandomCheckType) Create(config json.RawMessage) (goplum.Check, error) {
	p := RandomParameters{}
	err := json.Unmarshal(config, &p)
	if err != nil {
		return nil, err
	}

	if p.PercentGood == 0 {
		p.PercentGood = 0.5
	}

	return RandomCheck{p}, nil
}

type RandomCheck struct {
	params RandomParameters
}

func (t RandomCheck) Execute() goplum.Result {
	r := rand.Float32()
	if r <= t.params.PercentGood {
		return goplum.GoodResult()
	} else {
		return goplum.FailingResult("Random value %f greater than percent_good %f", r, t.params.PercentGood)
	}
}

type SysOutAlertType struct{}

func (s SysOutAlertType) Name() string {
	return "sysout"
}

func (s SysOutAlertType) Create(_ json.RawMessage) (goplum.Alert, error) {
	return SysOutAlert{}, nil
}

type SysOutAlert struct {
}

func (s SysOutAlert) Send(details goplum.AlertDetails) error {
	log.Printf("DEBUG ALERT - %s\n", details.Text)
	return nil
}
