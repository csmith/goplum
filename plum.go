package goplum

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"dario.cat/mergo"
	"github.com/csmith/goplum/config"
	"github.com/csmith/goplum/internal"
)

var (
	quietLogging = flag.Bool("quiet", false, "Reduce logging output from normal operations")
	runners      = flag.Int("runners", 4, "Number of runners to use to execute checks concurrently")
)

type CheckSettings struct {
	Alerts           []string
	Groups           []string
	Interval         time.Duration
	Timeout          time.Duration
	GoodThreshold    int `config:"good_threshold"`
	FailingThreshold int `config:"failing_threshold"`
}

type Group struct {
	Name        string
	AlertLimit  int           `config:"alert_limit"`
	AlertWindow time.Duration `config:"alert_window"`
	Defaults    *CheckSettings

	// Alert state tracking for limiting
	alertHistory []time.Time
	mu           sync.RWMutex
}

// canSendAlert checks if an alert can be sent for this group within the alert window.
// Returns (canSend, isLastAlert) where isLastAlert indicates this would be the final alert before throttling.
func (g *Group) canSendAlert() (bool, bool) {
	if g.AlertLimit <= 0 {
		return true, false // No limit configured
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-g.AlertWindow)

	// Remove alerts outside the window
	validAlerts := g.alertHistory[:0]
	for _, alertTime := range g.alertHistory {
		if alertTime.After(windowStart) {
			validAlerts = append(validAlerts, alertTime)
		}
	}
	g.alertHistory = validAlerts

	currentCount := len(g.alertHistory)
	if currentCount >= g.AlertLimit {
		return false, false // Already at limit
	}

	// Record this alert
	g.alertHistory = append(g.alertHistory, now)

	// Check if this is the last alert we'll send before throttling
	isLastAlert := currentCount+1 == g.AlertLimit

	return true, isLastAlert
}

// shouldSendAlert checks all groups for a check and determines if an alert should be sent.
// Returns (shouldSend, suppressionWarning, suppressingGroup) where suppressionWarning contains text to append if this is the last alert.
func (p *Plum) shouldSendAlert(groupNames []string) (bool, string, string) {
	if len(groupNames) == 0 {
		return true, "", "" // No groups, no limits
	}

	var warnings []string

	// Check each group - if ANY group forbids sending, we don't send
	for _, groupName := range groupNames {
		group, exists := p.Groups[groupName]
		if !exists {
			continue // Group doesn't exist, skip (this would be caught in validation)
		}

		canSend, isLast := group.canSendAlert()
		if !canSend {
			return false, "", groupName // Suppressed by this group
		}

		if isLast && group.AlertLimit > 0 {
			warnings = append(warnings, fmt.Sprintf("[GROUP ALERT LIMIT REACHED: %s]", groupName))
		}
	}

	// Combine all suppression warnings
	var warning string
	if len(warnings) > 0 {
		warning = " " + strings.Join(warnings, " ")
	}

	return true, warning, ""
}

func (c CheckSettings) Copy() CheckSettings {
	alerts := make([]string, len(c.Alerts))
	copy(alerts, c.Alerts)

	groups := make([]string, len(c.Groups))
	copy(groups, c.Groups)

	return CheckSettings{
		Alerts:           alerts,
		Groups:           groups,
		Interval:         c.Interval,
		Timeout:          c.Timeout,
		GoodThreshold:    c.GoodThreshold,
		FailingThreshold: c.FailingThreshold,
	}
}

var DefaultSettings = CheckSettings{
	Alerts:           []string{"*"},
	Interval:         time.Second * 30,
	Timeout:          time.Second * 20,
	GoodThreshold:    2,
	FailingThreshold: 2,
}

type PluginLoader func() (Plugin, error)
type CheckListener func(*ScheduledCheck, Result)

type Plum struct {
	Alerts           map[string]Alert
	Checks           map[string]*ScheduledCheck
	Groups           map[string]*Group
	availablePlugins map[string]PluginLoader
	loadedPlugins    map[string]Plugin
	checkDefaults    CheckSettings
	scheduled        chan *ScheduledCheck
	checkListeners   map[reflect.Value]CheckListener
}

func NewPlum() *Plum {
	plum := &Plum{
		availablePlugins: make(map[string]PluginLoader),
		loadedPlugins:    make(map[string]Plugin),
		Alerts:           make(map[string]Alert),
		Checks:           make(map[string]*ScheduledCheck),
		Groups:           make(map[string]*Group),
		checkDefaults:    DefaultSettings.Copy(),
		scheduled:        make(chan *ScheduledCheck, 100),
		checkListeners:   make(map[reflect.Value]CheckListener),
	}

	plum.AddCheckListener(plum.updateStatus)
	plum.AddCheckListener(plum.logCheck)

	return plum
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

	if err := internal.DecodeSettings(&parser.DefaultSettings, &p.checkDefaults); err != nil {
		return fmt.Errorf("unable to merge default settings from %s: %v", path, err)
	}

	if err := p.addAlerts(parser.AlertBlocks); err != nil {
		return err
	}

	if err := p.addGroups(parser.GroupBlocks); err != nil {
		return err
	}

	if err := p.addChecks(parser.CheckBlocks); err != nil {
		return err
	}

	if err := p.configurePlugins(parser.PluginSettings); err != nil {
		return err
	}

	return nil
}

func (p *Plum) RestoreState() error {
	ts, err := LoadTombStone()
	if err != nil {
		return err
	}

	return ts.Restore(p.Checks)
}

func (p *Plum) SaveState() error {
	return NewTombStone(p.Checks).Save()
}

func (p *Plum) addAlerts(alerts []*config.Block) error {
	for i := range alerts {
		if _, ok := p.Alerts[alerts[i].Name]; ok {
			return fmt.Errorf("alert defined multiple times: %s", alerts[i].Name)
		}

		parts := strings.SplitN(alerts[i].Type, ".", 2)
		plugin, err := p.plugin(parts[0])
		if err != nil {
			return err
		}

		alert := plugin.Alert(parts[1])
		if alert == nil {
			return fmt.Errorf("invalid alert %s in plugin %s", parts[1], parts[0])
		}

		if err := internal.DecodeSettings(&alerts[i].Settings, &alert); err != nil {
			return fmt.Errorf("error configuring alert %s: %v", alerts[i].Name, err)
		}

		if v, ok := alert.(Validator); ok {
			if err := v.Validate(); err != nil {
				return fmt.Errorf("error configuring alert %s: %v", alerts[i].Name, err)
			}
		}

		p.Alerts[alerts[i].Name] = alert
	}

	return nil
}

func (p *Plum) addGroups(groups []*config.Block) error {
	for i := range groups {
		if _, ok := p.Groups[groups[i].Name]; ok {
			return fmt.Errorf("group defined multiple times: %s", groups[i].Name)
		}

		group := &Group{
			Name: groups[i].Name,
		}

		// Extract defaults from settings if present
		if defaults, ok := groups[i].Settings["defaults"].(map[string]any); ok {
			group.Defaults = &CheckSettings{}
			if err := internal.DecodeSettings(&defaults, group.Defaults); err != nil {
				return fmt.Errorf("error configuring defaults for group %s: %v", groups[i].Name, err)
			}
			// Remove defaults from settings so it doesn't interfere with decoding other fields
			delete(groups[i].Settings, "defaults")
		}

		// Decode the rest of the group settings
		if err := internal.DecodeSettings(&groups[i].Settings, group); err != nil {
			return fmt.Errorf("error configuring group %s: %v", groups[i].Name, err)
		}

		p.Groups[groups[i].Name] = group
	}

	return nil
}

// createCheck constructs a Check and CheckSettings by merging global defaults, group defaults, and check-specific settings
func (p *Plum) createCheck(checkBlock *config.Block, groups []string) (Check, CheckSettings, error) {
	// Create the check object
	parts := strings.SplitN(checkBlock.Type, ".", 2)
	plugin, err := p.plugin(parts[0])
	if err != nil {
		return nil, CheckSettings{}, err
	}

	check := plugin.Check(parts[1])
	if check == nil {
		return nil, CheckSettings{}, fmt.Errorf("invalid check %s in plugin %s", parts[1], parts[0])
	}

	// Start with global defaults
	settings := p.checkDefaults.Copy()

	// Apply group defaults in order
	for _, groupName := range groups {
		if group := p.Groups[groupName]; group != nil && group.Defaults != nil {
			if err := mergo.Merge(&settings, group.Defaults, mergo.WithOverride); err != nil {
				return nil, CheckSettings{}, fmt.Errorf("error applying group defaults from %s: %v", groupName, err)
			}
		}
	}

	// Apply check-specific settings to both check and settings
	if err := internal.DecodeSettings(&checkBlock.Settings, &check, &settings); err != nil {
		return nil, CheckSettings{}, fmt.Errorf("error decoding check settings: %v", err)
	}

	return check, settings, nil
}

func (p *Plum) addChecks(checks []*config.Block) error {
	for i := range checks {
		if _, ok := p.Checks[checks[i].Name]; ok {
			return fmt.Errorf("check defined multiple times: %s", checks[i].Name)
		}

		// First, get settings to extract groups for validation
		_, tempSettings, err := p.createCheck(checks[i], nil)
		if err != nil {
			return fmt.Errorf("error configuring check %s: %v", checks[i].Name, err)
		}

		// Validate that all referenced groups exist
		for _, groupName := range tempSettings.Groups {
			if _, ok := p.Groups[groupName]; !ok {
				return fmt.Errorf("error configuring check %s: no group named '%s'", checks[i].Name, groupName)
			}
		}

		// Now create final check with validated groups
		check, settings, err := p.createCheck(checks[i], tempSettings.Groups)
		if err != nil {
			return fmt.Errorf("error configuring check %s: %v", checks[i].Name, err)
		}

		if v, ok := check.(Validator); ok {
			if err := v.Validate(); err != nil {
				return fmt.Errorf("error configuring check %s: %v", checks[i].Name, err)
			}
		}

		for a := range settings.Alerts {
			if settings.Alerts[a] != "-" && len(p.AlertsMatching(settings.Alerts[a:a+1])) == 0 {
				return fmt.Errorf("error configuring check %s: no alerts match '%s'", checks[i].Name, settings.Alerts[a])
			}
		}

		p.Checks[checks[i].Name] = &ScheduledCheck{
			Name:   checks[i].Name,
			Type:   checks[i].Type,
			Config: &settings,
			Check:  check,
		}
	}

	return nil
}

func (p *Plum) configurePlugins(blocks []*config.Block) error {
	for i := range blocks {
		name := blocks[i].Type
		loaded, ok := p.loadedPlugins[name]
		if ok {
			if err := internal.DecodeSettings(&blocks[i].Settings, &loaded); err != nil {
				return fmt.Errorf("error configuring plugin %s: %v", name, err)
			}
			continue
		}

		if _, ok := p.availablePlugins[name]; ok {
			log.Printf("Config for plugin %s ignored as it is not loaded", name)
		} else {
			return fmt.Errorf("unable to configure plugin %s: no such plugin", name)
		}
	}

	for name := range p.loadedPlugins {
		if v, ok := p.loadedPlugins[name].(Validator); ok {
			if err := v.Validate(); err != nil {
				return fmt.Errorf("error configuring plugin %s: %v", name, err)
			}
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
	for i := 0; i < *runners; i++ {
		go p.processScheduledChecks()
	}

	for {
		min := time.Now().Add(time.Hour)
		for i := range p.Checks {
			c := p.Checks[i]

			if c.Suspended {
				// If a check is suspended, don't wait more than a minute before we check again.
				if next := time.Now().Add(time.Minute); next.Before(min) {
					min = next
				}
				continue
			}

			remaining := c.Remaining()
			if remaining <= 0 {
				c.Scheduled = true
				p.scheduled <- c
				remaining = c.Remaining()
			}

			if next := time.Now().Add(remaining); next.Before(min) {
				min = next
			}
		}

		time.Sleep(time.Until(min))
	}
}

func (p *Plum) processScheduledChecks() {
	for c := range p.scheduled {
		p.RunCheck(c)
		c.Scheduled = false
	}
}

func (p *Plum) RunCheck(c *ScheduledCheck) {
	start := time.Now()
	result := func() (res Result) {
		defer func() {
			if r := recover(); r != nil {
				res = FailingResult("PANIC: %v", r)
			}
		}()

		timeout := c.Config.Timeout
		if longRunning, ok := c.Check.(LongRunning); ok {
			timeout = longRunning.Timeout()
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		res = c.Check.Execute(ctx)
		return
	}()

	if result.Facts == nil {
		result.Facts = map[Fact]any{}
	}
	result.Facts[CheckTime] = time.Since(start)

	c.AddResult(&result)

	for _, listener := range p.checkListeners {
		listener(c, result)
	}
}

func (p *Plum) updateStatus(c *ScheduledCheck, _ Result) {
	oldState := c.State
	newState := c.History.State(map[CheckState]int{
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

func (p *Plum) logCheck(c *ScheduledCheck, result Result) {
	if !*quietLogging {
		log.Printf("Check '%s' executed in %s: %s (%s)\n", c.Name, result.Facts[CheckTime], result.State, result.Detail)
	}
}

func (p *Plum) RaiseAlerts(c *ScheduledCheck, previousState CheckState) {
	details := AlertDetails{
		Name:          c.Name,
		Config:        c.Check,
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

	// Check group limits for all groups this check belongs to
	shouldSend, suppressionWarning, suppressingGroup := p.shouldSendAlert(c.Config.Groups)
	if !shouldSend {
		log.Printf("Alert for %s suppressed due to group limit (group: %s)\n", c.Name, suppressingGroup)
		return
	}

	// Add suppression warning if this is the last alert before throttling
	if suppressionWarning != "" {
		details.Text += suppressionWarning
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

func (p *Plum) AddCheckListener(listener CheckListener) {
	p.checkListeners[reflect.ValueOf(listener)] = listener
}

func (p *Plum) RemoveCheckListener(listener CheckListener) {
	delete(p.checkListeners, reflect.ValueOf(listener))
}

// Suspend sets the check with the given name to be suspended (i.e., it won't run until unsuspended).
// Returns the modified check, or nil if the check didn't exist.
func (p *Plum) Suspend(checkName string) *ScheduledCheck {
	if check, ok := p.Checks[checkName]; ok {
		log.Printf("Check %s has been suspended", checkName)
		check.Suspended = true
		return check
	}
	return nil
}

// Unsuspend sets the check with the given name to be resumed (i.e., it will run normally).
// Returns the modified check, or nil if the check didn't exist.
func (p *Plum) Unsuspend(checkName string) *ScheduledCheck {
	if check, ok := p.Checks[checkName]; ok {
		log.Printf("Check %s has been unsuspended", checkName)
		check.Suspended = false
		return check
	}
	return nil
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
	Name      string
	Type      string
	Config    *CheckSettings
	Check     Check
	LastRun   time.Time
	Scheduled bool
	Settled   bool
	State     CheckState
	Suspended bool
	History   ResultHistory
}

func (c *ScheduledCheck) Remaining() time.Duration {
	if c.Scheduled {
		return c.Config.Interval
	}
	return time.Until(c.LastRun.Add(c.Config.Interval))
}

func (c *ScheduledCheck) AddResult(result *Result) ResultHistory {
	copy(c.History[1:9], c.History[0:8])
	c.History[0] = result
	c.LastRun = time.Now()
	return c.History
}

func (c *ScheduledCheck) LastResult() *Result {
	return c.History[0]
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

// Run creates a new instance of Plum, registers plugins and loads configuration, and starts the main loop.
// Listens for interrupt and sigterm signals in order to save state and clean up. It is expected that flag.Parse
// has been called prior to calling this method.
func Run(plugins map[string]PluginLoader, configPath string) {
	p := NewPlum()
	p.RegisterPlugins(plugins)

	if err := p.ReadConfig(configPath); err != nil {
		log.Fatalf("Unable to read config: %v", err)
	}

	if err := p.RestoreState(); err != nil {
		log.Printf("Unable to restore state from tombstone: %v", err)
	}

	api := NewGrpcServer(p)

	go api.Start()
	go p.Run()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, syscall.SIGTERM)

	<-c

	api.Stop()
	if err := p.SaveState(); err != nil {
		log.Printf("Unable to save state to tombstone: %v", err)
	}
}
