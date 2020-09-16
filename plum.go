package goplum

import (
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

type ScheduledChecks []*ScheduledCheck

func Initialise(plugins []Plugin, configPath string) {
	config, err := LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Unable to read config: %v", err)
	}

	checks := make(map[string]CheckType)
	notifications := make(map[string]AlertType)
	for i := range plugins {
		cs := plugins[i].Checks()
		for j := range cs {
			checks[cs[j].Name()] = cs[j]
		}

		ns := plugins[i].Alerts()
		for j := range ns {
			notifications[ns[j].Name()] = ns[j]
		}
	}

	log.Printf("Found %d checks and %d notifiers from %d plugins\n", len(checks), len(notifications), len(plugins))

	items := make(ScheduledChecks, 0)
	for i := range config.Checks {
		cc := config.Checks[i]
		check, ok := checks[cc.Check]
		if !ok {
			log.Fatalf("Invalid check name in config: %s", cc.Check)
		}

		t, err := check.Create(cc.Params)
		if err != nil {
			log.Fatalf("Unable to create check '%s': %v", cc.Name, err)
		}

		if cc.Interval == 0 {
			cc.Interval = time.Second * 30
		}

		items = append(items, &ScheduledCheck{
			Config:  cc,
			Task:    t,
		})
	}

	alerters := make([]Alert, 0)
	for i := range config.Alerts {
		a := config.Alerts[i]
		alert, ok := notifications[a.Notifier]
		if !ok {
			log.Fatalf("Invalid notifier name in config: %s", a.Notifier)
		}

		n, err := alert.Create(a.Params)
		if err != nil {
			log.Fatalf("Unable to create notifier '%s': %v", a.Notifier, err)
		}

		alerters = append(alerters, n)
	}

	items.Schedule(alerters)
}

func (c *ScheduledCheck) Remaining() time.Duration {
	return c.LastRun.Add(c.Config.Interval).Sub(time.Now())
}

func (s ScheduledChecks) Schedule(notifiers []Alert) {
	for {
		min := time.Hour
		for i := range s {
			remaining := s[i].Remaining()
			if remaining <= 0 {
				result := s[i].Task.Execute()
				log.Printf("Check '%s' executed: %t\n", s[i].Config.Name, result.Good)
				lastResult := s[i].History[s[i].HistoryTop]
				s[i].HistoryTop = (s[i].HistoryTop + 1) % 10
				s[i].History[s[i].HistoryTop] = &result
				s[i].LastRun = time.Now()
				remaining = s[i].Remaining()

				if lastResult != nil && result.Good != lastResult.Good {
					for n := range notifiers {
						if err := notifiers[n].Send(s[i]); err != nil {
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
