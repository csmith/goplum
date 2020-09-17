package goplum

import (
	"fmt"
	"log"
	"time"
)

type ScheduledCheck struct {
	Config     ConfiguredCheck
	Task       Check
	LastRun    time.Time
	History    [10]*Result
	HistoryTop int
}

type Plum struct {
	checkTypes map[string]CheckType
	alertTypes map[string]AlertType
	checks []*ScheduledCheck
	alerts []Alert
}

func (p *Plum) AddPlugins(plugins []Plugin) {
	p.checkTypes = make(map[string]CheckType)
	p.alertTypes = make(map[string]AlertType)

	for i := range plugins {
		cs := plugins[i].Checks()
		for j := range cs {
			p.checkTypes[fmt.Sprintf("%s.%s", plugins[i].Name(), cs[j].Name())] = cs[j]
		}

		ns := plugins[i].Alerts()
		for j := range ns {
			p.alertTypes[fmt.Sprintf("%s.%s", plugins[i].Name(), ns[j].Name())] = ns[j]
		}
	}

	log.Printf("Found %d check types and %d alert types from %d plugins\n", len(p.checkTypes), len(p.alertTypes), len(plugins))
}

func (p *Plum) LoadConfig(configPath string) {
	config, err := LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Unable to read config: %v", err)
	}

	for i := range config.Checks {
		cc := config.Checks[i]
		check, ok := p.checkTypes[cc.Type]
		if !ok {
			log.Fatalf("Invalid check type in config: %s", cc.Type)
		}

		t, err := check.Create(cc.Params)
		if err != nil {
			log.Fatalf("Unable to create check '%s': %v", cc.Name, err)
		}

		if cc.Interval == 0 {
			cc.Interval = time.Second * 30
		}

		p.checks = append(p.checks, &ScheduledCheck{
			Config:  cc,
			Task:    t,
		})
	}

	for i := range config.Alerts {
		a := config.Alerts[i]
		alert, ok := p.alertTypes[a.Type]
		if !ok {
			log.Fatalf("Invalid alert type in config: %s", a.Type)
		}

		n, err := alert.Create(a.Params)
		if err != nil {
			log.Fatalf("Unable to create notifier '%s': %v", a.Type, err)
		}

		p.alerts = append(p.alerts, n)
	}
}

func (c *ScheduledCheck) Remaining() time.Duration {
	return c.LastRun.Add(c.Config.Interval).Sub(time.Now())
}

func (p *Plum) Run() {
	for {
		min := time.Hour
		for i := range p.checks {
			c := p.checks[i]
			remaining := c.Remaining()
			if remaining <= 0 {
				result := c.Task.Execute()
				log.Printf("Check '%s' executed: %t\n", c.Config.Name, result.Good)
				lastResult := c.History[c.HistoryTop]
				c.HistoryTop = (c.HistoryTop + 1) % 10
				c.History[c.HistoryTop] = &result
				c.LastRun = time.Now()
				remaining = c.Remaining()

				if lastResult != nil && result.Good != lastResult.Good {
					for n := range p.alerts {
						if err := p.alerts[n].Send(c); err != nil {
							log.Printf("Error sending alert: %v\n", err)
						}
					}
				}
			}

			if remaining < min {
				min = remaining
			}
		}

		log.Printf("Sleeping for %s\n", min)
		time.Sleep(min)
	}
}
