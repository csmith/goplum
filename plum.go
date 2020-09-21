package goplum

import (
	"fmt"
	"github.com/csmith/goplum/config"
	"github.com/mitchellh/mapstructure"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

type CheckSettings struct {
	Alerts           []string
	Interval         time.Duration
	GoodThreshold    int `config:"good_threshold"`
	FailingThreshold int `config:"failing_threshold"`
}

func (c CheckSettings) Copy() CheckSettings {
	alerts := make([]string, len(c.Alerts))
	copy(alerts, c.Alerts)

	return CheckSettings{
		Alerts:           alerts,
		Interval:         c.Interval,
		GoodThreshold:    c.GoodThreshold,
		FailingThreshold: c.FailingThreshold,
	}
}

var DefaultSettings = CheckSettings{
	Alerts:           []string{"*"},
	Interval:         time.Second * 30,
	GoodThreshold:    2,
	FailingThreshold: 2,
}

type PluginLoader func() (Plugin, error)

type Plum struct {
	availablePlugins map[string]PluginLoader
	loadedPlugins    map[string]Plugin
	checkDefaults    CheckSettings
	Alerts           map[string]Alert
	Checks           []*ScheduledCheck
}

func NewPlum() *Plum {
	return &Plum{
		availablePlugins: make(map[string]PluginLoader),
		loadedPlugins:    make(map[string]Plugin),
		Alerts:           make(map[string]Alert),
		checkDefaults:    DefaultSettings,
	}
}

func (p *Plum) RegisterPlugins(plugins map[string]PluginLoader) {
	for n := range plugins {
		p.RegisterPlugin(n, plugins[n])
	}
}

func (p *Plum) RegisterPlugin(name string, loader PluginLoader) {
	p.availablePlugins[name] = loader
}

func (p *Plum) ReadConfig(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	parser := config.NewParser(f)
	if err := parser.Parse(); err != nil {
		return fmt.Errorf("unable to parse config file %s: %v", path, err)
	}

	if err := p.decodeSettings(&parser.DefaultSettings, &p.checkDefaults); err != nil {
		return fmt.Errorf("unable to merge default settings from %s: %v", path, err)
	}

	if err := p.addChecks(parser.CheckBlocks); err != nil {
		return err
	}

	if err := p.addAlerts(parser.AlertBlocks); err != nil {
		return err
	}

	return nil
}

func (p *Plum) addAlerts(alerts []*config.Block) error {
	for i := range alerts {
		parts := strings.SplitN(alerts[i].Type, ".", 2)
		plugin, err := p.plugin(parts[0])
		if err != nil {
			return err
		}

		alert := plugin.Alert(parts[1])
		if alert == nil {
			return fmt.Errorf("invalid alert %s in plugin %s", parts[1], parts[0])
		}

		if err := p.decodeSettings(&alerts[i].Settings, &alert); err != nil {
			return fmt.Errorf("error merging alert config: %v", err)
		}

		for k := range alerts[i].Settings {
			return fmt.Errorf("invalid configuration key in alert %s: %s", alerts[i].Name, k)
		}

		if err := alert.Validate(); err != nil {
			return fmt.Errorf("error configuring alert %s: %v", alerts[i].Name, err)
		}

		p.Alerts[alerts[i].Name] = alert
	}

	return nil
}

func (p *Plum) addChecks(checks []*config.Block) error {
	for i := range checks {
		parts := strings.SplitN(checks[i].Type, ".", 2)
		plugin, err := p.plugin(parts[0])
		if err != nil {
			return err
		}

		check := plugin.Check(parts[1])
		if check == nil {
			fmt.Printf("%#v\n", plugin)
			return fmt.Errorf("invalid check %s in plugin %s", parts[1], parts[0])
		}

		if err := p.decodeSettings(&checks[i].Settings, &check); err != nil {
			return fmt.Errorf("error configuring check %s: %v", checks[i].Name, err)
		}

		settings := p.checkDefaults.Copy()
		if err := p.decodeSettings(&checks[i].Settings, &settings); err != nil {
			return fmt.Errorf("error configuring check %s: %v", checks[i].Name, err)
		}

		for k := range checks[i].Settings {
			return fmt.Errorf("invalid configuration key in check %s: %s", checks[i].Name, k)
		}

		if err := check.Validate(); err != nil {
			return fmt.Errorf("error configuring check %s: %v", checks[i].Name, err)
		}

		p.Checks = append(p.Checks, &ScheduledCheck{
			Name:   checks[i].Name,
			Type:   checks[i].Type,
			Config: &settings,
			Check:  check,
		})
	}

	return nil
}

func (p *Plum) decodeSettings(src *map[string]interface{}, dst interface{}) error {
	metadata := mapstructure.Metadata{}
	dec, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		TagName:          "config",
		Metadata:         &metadata,
		Result:           dst,
	})

	if err != nil {
		return err
	}

	if err := dec.Decode(*src); err != nil {
		return err
	}

	for key := range *src {
		unused := false
		for _, val := range metadata.Unused {
			if val == key {
				unused = true
				break
			}
		}
		if !unused {
			delete(*src, key)
		}
	}

	return nil
}

func (p *Plum) plugin(name string) (Plugin, error) {
	loaded, ok := p.loadedPlugins[name]
	if ok {
		return loaded, nil
	}

	available, ok := p.availablePlugins[name]
	if ok {
		plugin, err := available()
		if err != nil {
			return nil, fmt.Errorf("unable to load plugin %s: %v", name, err)
		}
		p.loadedPlugins[name] = plugin
		return plugin, nil
	}

	return nil, fmt.Errorf("no plugin found with name %s", name)
}

func (p *Plum) Run() {
	for {
		min := time.Now().Add(time.Hour)
		for i := range p.Checks {
			c := p.Checks[i]
			remaining := c.Remaining()
			if remaining <= 0 {
				p.RunCheck(c)
				remaining = c.Remaining()
			}

			next := time.Now().Add(remaining)
			if next.Before(min) {
				min = next
			}
		}

		log.Printf("Sleeping until %s (%s)\n", min, min.Sub(time.Now()))
		time.Sleep(min.Sub(time.Now()))
	}
}

func (p *Plum) RunCheck(c *ScheduledCheck) {
	result := c.Check.Execute()
	log.Printf("Check '%s' executed: %s (%s)\n", c.Name, result.State, result.Detail)
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
		Name:          c.Name,
		Type:          c.Type,
		LastResult:    c.LastResult(),
		PreviousState: previousState,
		NewState:      c.State,
	}

	if len(details.LastResult.Detail) > 0 {
		details.Text = fmt.Sprintf("Check '%s' is now %s (%s), was %s.", details.Name, details.NewState, details.LastResult.Detail, details.PreviousState)
	} else {
		details.Text = fmt.Sprintf("Check '%s' is now %s, was %s.", details.Name, details.NewState, details.PreviousState)
	}

	alerts := p.AlertsMatching(c.Config.Alerts)
	log.Printf("Raising alerts for %s: %d alerts match config %v\n", c.Name, len(alerts), c.Config.Alerts)
	for n := range alerts {
		if err := alerts[n].Send(details); err != nil {
			log.Printf("Error sending alert: %v\n", err)
		}
	}
}

func (p *Plum) AlertsMatching(names []string) []Alert {
	var res []Alert
	re := regexpForWildcards(names)
	for j := range p.Alerts {
		if re.MatchString(j) {
			res = append(res, p.Alerts[j])
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
	Name    string
	Type    string
	Config  *CheckSettings
	Check   Check
	LastRun time.Time
	Settled bool
	State   CheckState
	history ResultHistory
}

func (c *ScheduledCheck) Remaining() time.Duration {
	return c.LastRun.Add(c.Config.Interval).Sub(time.Now())
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
