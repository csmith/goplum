package goplum

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
)

type Plum struct {
	checkTypes map[string]CheckType
	alertTypes map[string]AlertType
	alerts     map[string]Alert
	checks     []*ScheduledCheck
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

		p.checks = append(p.checks, &ScheduledCheck{
			Config: cc,
			Check:  t,
		})
	}

	p.alerts = make(map[string]Alert)
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

		p.alerts[a.Name] = n
	}
}

func (p *Plum) Run() {
	for {
		min := time.Hour
		for i := range p.checks {
			c := p.checks[i]
			remaining := c.Remaining()
			if remaining <= 0 {
				p.RunCheck(c)
				remaining = c.Remaining()
			}

			if remaining < min {
				min = remaining
			}
		}

		log.Printf("Sleeping for %s\n", min)
		time.Sleep(min)
	}
}

func (p *Plum) RunCheck(c *ScheduledCheck) {
	result := c.Check.Execute()
	log.Printf("Check '%s' executed: %d\n", c.Config.Name, result.State)
	c.AddResult(&result)
	c.LastRun = time.Now()

	oldState := c.State
	newState := c.History().State(map[CheckState]int{
		StateFailing: c.Config.FailingThreshold,
		StateGood:    c.Config.GoodThreshold,
	})
	if newState != oldState {
		c.State = newState
		if c.Settled {
			p.RaiseAlerts(c, oldState)
		} else {
			c.Settled = true
		}
	}
}

func (p *Plum) RaiseAlerts(c *ScheduledCheck, previousState CheckState) {
	details := AlertDetails{
		Name:          c.Config.Name,
		Type:          c.Config.Type,
		Config:        c.Config.Params,
		LastResult:    c.LastResult(),
		PreviousState: previousState,
		NewState:      c.State,
	}

	if len(details.LastResult.Detail) > 0 {
		details.Text = fmt.Sprintf("Check '%s' is now %s (%s), was %s .", details.Name, details.NewState, details.LastResult.Detail, details.PreviousState)
	} else {
		details.Text = fmt.Sprintf("Check '%s' is now %s, was %s.", details.Name, details.NewState, details.PreviousState)
	}

	alerts := p.AlertsMatching(c.Config.Alerts)
	for n := range alerts {
		if err := alerts[n].Send(details); err != nil {
			log.Printf("Error sending alert: %v\n", err)
		}
	}
}

func (p *Plum) AlertsMatching(names []string) []Alert {
	var res []Alert
	re := regexpForWildcards(names)
	for j := range p.alerts {
		if re.MatchString(j) {
			res = append(res, p.alerts[j])
		}
	}
	return res
}

// regexpForWildcards converts a set of names containing '*' characters as wildcards into a single regex that will
// match any of them.
//
// e.g. ["foo_*", "*+bar"] becomes /^(foo_.*)|(.*\+bar)$/
func regexpForWildcards(names []string) *regexp.Regexp {
	pattern := strings.Builder{}
	pattern.WriteString("^")

	for i := range names {
		if i > 0 {
			pattern.WriteString("|")
		}
		parts := strings.Split(names[i], "*")
		for n := range parts {
			if n > 0 {
				pattern.WriteString(".*")
			}
			pattern.WriteString(regexp.QuoteMeta(parts[n]))
		}
	}

	pattern.WriteString("$")
	re, _ := regexp.Compile(pattern.String())
	return re
}

type ScheduledCheck struct {
	Config  ConfiguredCheck
	Check   Check
	LastRun time.Time
	Settled bool
	State   CheckState
	history ResultHistory
}

func (c *ScheduledCheck) Remaining() time.Duration {
	return c.LastRun.Add(time.Duration(c.Config.Interval)).Sub(time.Now())
}

func (c *ScheduledCheck) AddResult(result *Result) ResultHistory {
	copy(c.history[1:9], c.history[0:8])
	c.history[0] = result
	return c.history
}

func (c *ScheduledCheck) LastResult() *Result {
	return c.history[0]
}

func (c *ScheduledCheck) History() ResultHistory {
	return c.history
}

type ResultHistory [10]*Result

func (h ResultHistory) State(thresholds map[CheckState]int) CheckState {
	var (
		count = 0
		last  = StateIndeterminate
	)

	for i := range h {
		r := h[i]
		if r != nil {
			if r.State != last {
				count = 0
				last = r.State
			}

			count++
			if count == thresholds[last] {
				return last
			}
		}
	}

	return StateIndeterminate
}
