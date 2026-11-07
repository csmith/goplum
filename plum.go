package goplum

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"chameth.com/goplum/config"
	"chameth.com/goplum/internal"
	"dario.cat/mergo"
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
	Reminder         time.Duration
	GoodThreshold    int `config:"good_threshold"`
	FailingThreshold int `config:"failing_threshold"`
	Link             string
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
	return true, currentCount+1 == g.AlertLimit
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
		Reminder:         c.Reminder,
		GoodThreshold:    c.GoodThreshold,
		FailingThreshold: c.FailingThreshold,
		Link:             c.Link,
	}
}

var DefaultSettings = CheckSettings{
	Alerts:           []string{"*"},
	Interval:         time.Second * 30,
	Timeout:          time.Second * 20,
	GoodThreshold:    2,
	FailingThreshold: 2,
}

type CheckListener func(*ScheduledCheck, Result)

type Plum struct {
	Alerts        map[string]Alert
	Checks        map[string]*ScheduledCheck
	Groups        map[string]*Group
	plugins       map[string]Plugin
	usedPlugins   map[string]bool
	checkDefaults CheckSettings
	scheduled     chan *ScheduledCheck

	listenerMutex  sync.RWMutex
	checkListeners map[reflect.Value]CheckListener
}

func NewPlum() *Plum {
	plum := &Plum{
		plugins:        make(map[string]Plugin),
		usedPlugins:    make(map[string]bool),
		Alerts:         make(map[string]Alert),
		Checks:         make(map[string]*ScheduledCheck),
		Groups:         make(map[string]*Group),
		checkDefaults:  DefaultSettings.Copy(),
		scheduled:      make(chan *ScheduledCheck, 100),
		checkListeners: make(map[reflect.Value]CheckListener),
	}

	plum.AddCheckListener(plum.updateStatus)
	plum.AddCheckListener(plum.logCheck)

	return plum
}

func (p *Plum) RegisterPlugins(plugins map[string]Plugin) {
	for n := range plugins {
		p.RegisterPlugin(n, plugins[n])
	}
}

func (p *Plum) RegisterPlugin(name string, plugin Plugin) {
	p.plugins[name] = plugin
}

func (p *Plum) ReadConfig(path string) error {
	parser, err := config.Load(path)
	if err != nil {
		return err
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

func (p *Plum) RestoreState(tombStonePath string) error {
	ts, err := LoadTombStone(tombStonePath)
	if err != nil {
		return err
	}

	return ts.Restore(p.Checks)
}

func (p *Plum) SaveState(tombStonePath string) error {
	return NewTombStone(p.Checks).Save(tombStonePath)
}

// Run schedules checks until ctx is cancelled, then waits for any running checks and alerts to finish. Cancelling ctx
// also interrupts the running checks, and their results are discarded.
func (p *Plum) Run(ctx context.Context) {
	var workers sync.WaitGroup
	defer workers.Wait()

	for range *runners {
		workers.Go(
			func() {
				p.processScheduledChecks(ctx)
			},
		)
	}

	for {
		earliest := time.Now().Add(time.Hour)
		for i := range p.Checks {
			c := p.Checks[i]

			if c.Suspended {
				// If a check is suspended, don't wait more than a minute before we check again.
				if next := time.Now().Add(time.Minute); next.Before(earliest) {
					earliest = next
				}
				continue
			}

			remaining := c.Remaining()
			if remaining <= 0 {
				c.Scheduled = true
				select {
				case p.scheduled <- c:
				case <-ctx.Done():
					return
				}
				remaining = c.Remaining()
			}

			if next := time.Now().Add(remaining); next.Before(earliest) {
				earliest = next
			}
		}

		select {
		case <-time.After(time.Until(earliest)):
		case <-ctx.Done():
			return
		}
	}
}

func (p *Plum) RunCheck(c *ScheduledCheck) {
	p.runCheck(context.Background(), c)
}

func (p *Plum) RaiseAlerts(c *ScheduledCheck, previousState CheckState) {
	p.raiseAlerts(c, previousState, false)
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
	p.listenerMutex.Lock()
	defer p.listenerMutex.Unlock()
	p.checkListeners[reflect.ValueOf(listener)] = listener
}

func (p *Plum) RemoveCheckListener(listener CheckListener) {
	p.listenerMutex.Lock()
	defer p.listenerMutex.Unlock()
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

// shouldSendAlert checks all groups for a check and determines if an alert should be sent.
// Returns (shouldSend, suppressionWarning, suppressingGroup) where suppressionWarning contains text to append if this
// is the last alert.
func (p *Plum) runCheck(ctx context.Context, c *ScheduledCheck) {
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

		checkCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		res = c.Check.Execute(checkCtx)
		return
	}()

	if ctx.Err() != nil {
		return
	}

	if result.Facts == nil {
		result.Facts = map[Fact]any{}
	}
	result.Facts[CheckTime] = time.Since(start)

	c.AddResult(&result)

	p.listenerMutex.RLock()
	listeners := make([]CheckListener, 0, len(p.checkListeners))
	for _, listener := range p.checkListeners {
		listeners = append(listeners, listener)
	}
	p.listenerMutex.RUnlock()

	for _, listener := range listeners {
		listener(c, result)
	}
}

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

func (p *Plum) addAlerts(alerts []*config.Block) error {
	seen := make(map[string]config.Position)
	for i := range alerts {
		if first, ok := seen[alerts[i].Name]; ok {
			return fmt.Errorf("alert defined multiple times: %s (at %s and %s)", alerts[i].Name, first, alerts[i].Position)
		}
		seen[alerts[i].Name] = alerts[i].Position

		if err := p.addAlert(alerts[i]); err != nil {
			return fmt.Errorf("%s: %v", alerts[i].Position, err)
		}
	}

	return nil
}

func (p *Plum) addAlert(block *config.Block) error {
	pluginName, kind, ok := strings.Cut(block.Type, ".")
	if !ok {
		return fmt.Errorf("invalid alert type %q: expected <plugin>.<type>", block.Type)
	}

	plugin, err := p.plugin(pluginName)
	if err != nil {
		return err
	}

	alert := plugin.Alert(kind)
	if alert == nil {
		return fmt.Errorf("invalid alert %s in plugin %s", kind, pluginName)
	}

	if err := internal.DecodeSettings(&block.Settings, &alert); err != nil {
		return fmt.Errorf("error configuring alert %s: %v", block.Name, err)
	}

	if v, ok := alert.(Validator); ok {
		if err := v.Validate(); err != nil {
			return fmt.Errorf("error configuring alert %s: %v", block.Name, err)
		}
	}

	p.Alerts[block.Name] = alert
	return nil
}

func (p *Plum) addGroups(groups []*config.Block) error {
	seen := make(map[string]config.Position)
	for i := range groups {
		if first, ok := seen[groups[i].Name]; ok {
			return fmt.Errorf("group defined multiple times: %s (at %s and %s)", groups[i].Name, first, groups[i].Position)
		}
		seen[groups[i].Name] = groups[i].Position

		if err := p.addGroup(groups[i]); err != nil {
			return fmt.Errorf("%s: %v", groups[i].Position, err)
		}
	}

	return nil
}

func (p *Plum) addGroup(block *config.Block) error {
	group := &Group{
		Name: block.Name,
	}

	// Extract defaults from settings if present
	if defaults, ok := block.Settings["defaults"].(map[string]any); ok {
		group.Defaults = &CheckSettings{}
		if err := internal.DecodeSettings(&defaults, group.Defaults); err != nil {
			return fmt.Errorf("error configuring defaults for group %s: %v", block.Name, err)
		}
		// Remove defaults from settings so it doesn't interfere with decoding other fields
		delete(block.Settings, "defaults")
	}

	// Decode the rest of the group settings
	if err := internal.DecodeSettings(&block.Settings, group); err != nil {
		return fmt.Errorf("error configuring group %s: %v", block.Name, err)
	}

	if group.AlertLimit < 0 {
		return fmt.Errorf("error configuring group %s: alert_limit can't be negative", block.Name)
	}

	p.Groups[block.Name] = group
	return nil
}

// createCheck constructs a Check and CheckSettings by merging global defaults, group defaults, and check-specific
// settings
func (p *Plum) createCheck(checkBlock *config.Block, groups []string) (Check, CheckSettings, error) {
	// Create the check object
	pluginName, kind, ok := strings.Cut(checkBlock.Type, ".")
	if !ok {
		return nil, CheckSettings{}, fmt.Errorf("invalid check type %q: expected <plugin>.<type>", checkBlock.Type)
	}

	plugin, err := p.plugin(pluginName)
	if err != nil {
		return nil, CheckSettings{}, err
	}

	check := plugin.Check(kind)
	if check == nil {
		return nil, CheckSettings{}, fmt.Errorf("invalid check %s in plugin %s", kind, pluginName)
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
	seen := make(map[string]config.Position)
	for i := range checks {
		if first, ok := seen[checks[i].Name]; ok {
			return fmt.Errorf("check defined multiple times: %s (at %s and %s)", checks[i].Name, first, checks[i].Position)
		}
		seen[checks[i].Name] = checks[i].Position

		if err := p.addCheck(checks[i]); err != nil {
			return fmt.Errorf("%s: error configuring check %s: %v", checks[i].Position, checks[i].Name, err)
		}
	}

	return nil
}

func (p *Plum) addCheck(block *config.Block) error {
	// First, get settings to extract groups for validation
	_, tempSettings, err := p.createCheck(block, nil)
	if err != nil {
		return err
	}

	// Validate that all referenced groups exist
	for _, groupName := range tempSettings.Groups {
		if _, ok := p.Groups[groupName]; !ok {
			return fmt.Errorf("no group named '%s'", groupName)
		}
	}

	// Now create final check with validated groups
	check, settings, err := p.createCheck(block, tempSettings.Groups)
	if err != nil {
		return err
	}

	if err := validateThreshold("good_threshold", settings.GoodThreshold); err != nil {
		return err
	}

	if err := validateThreshold("failing_threshold", settings.FailingThreshold); err != nil {
		return err
	}

	if err := validateLink(settings.Link); err != nil {
		return err
	}

	if v, ok := check.(Validator); ok {
		if err := v.Validate(); err != nil {
			return err
		}
	}

	for a := range settings.Alerts {
		if settings.Alerts[a] != "-" && len(p.AlertsMatching(settings.Alerts[a:a+1])) == 0 {
			return fmt.Errorf("no alerts match '%s'", settings.Alerts[a])
		}
	}

	p.Checks[block.Name] = &ScheduledCheck{
		Name:   block.Name,
		Type:   block.Type,
		Config: &settings,
		Check:  check,
	}
	return nil
}

func validateThreshold(name string, value int) error {
	if limit := len(ResultHistory{}); value < 1 || value > limit {
		return fmt.Errorf("%s must be between 1 and %d, got %d", name, limit, value)
	}
	return nil
}

// validateLink doesn't include the link in its errors, as it may contain credentials.
func validateLink(link string) error {
	if link == "" {
		return nil
	}

	u, err := url.Parse(expandLink(link, "name", "plugin.type"))
	if err != nil {
		return fmt.Errorf("invalid link: %w", internal.WithoutURL(err))
	}

	if u.Scheme == "" || u.Host == "" {
		return errors.New("invalid link: must be an absolute URL")
	}

	return nil
}

func expandLink(link, name, checkType string) string {
	return strings.NewReplacer("{name}", url.PathEscape(name), "{type}", checkType).Replace(link)
}

func (p *Plum) configurePlugins(blocks []*config.Block) error {
	seen := make(map[string]config.Position)
	for i := range blocks {
		name := blocks[i].Type
		if first, ok := seen[name]; ok {
			return fmt.Errorf("plugin defined multiple times: %s (at %s and %s)", name, first, blocks[i].Position)
		}
		seen[name] = blocks[i].Position

		plugin, ok := p.plugins[name]
		if ok && p.usedPlugins[name] {
			if err := internal.DecodeSettings(&blocks[i].Settings, &plugin); err != nil {
				return fmt.Errorf("%s: error configuring plugin %s: %v", blocks[i].Position, name, err)
			}
			continue
		}

		if ok {
			log.Printf("Config for plugin %s ignored as it is not loaded", name)
		} else {
			return fmt.Errorf("%s: unable to configure plugin %s: no such plugin", blocks[i].Position, name)
		}
	}

	for name := range p.usedPlugins {
		if v, ok := p.plugins[name].(Validator); ok {
			if err := v.Validate(); err != nil {
				return fmt.Errorf("error configuring plugin %s: %v", name, err)
			}
		}
	}

	return nil
}

func (p *Plum) plugin(name string) (Plugin, error) {
	plugin, ok := p.plugins[name]
	if ok {
		p.usedPlugins[name] = true
		return plugin, nil
	}

	return nil, fmt.Errorf("no plugin found with name %s", name)
}

func (p *Plum) processScheduledChecks(ctx context.Context) {
	for {
		select {
		case c := <-p.scheduled:
			p.runCheck(ctx, c)
			c.Scheduled = false
		case <-ctx.Done():
			return
		}
	}
}

func (p *Plum) updateStatus(c *ScheduledCheck, _ Result) {
	oldState := c.State
	newState := c.History.State(
		map[CheckState]int{
			StateFailing: c.Config.FailingThreshold,
			StateGood:    c.Config.GoodThreshold,
		},
	)
	if newState != oldState {
		c.State = newState
		if c.Settled || newState == StateFailing {
			p.raiseAlerts(c, oldState, false)
		}
		c.Settled = true
		return
	}

	reminderDue := c.Config.Reminder > 0 && time.Since(c.LastAlertTime) >= c.Config.Reminder
	if newState == StateFailing && c.Settled && reminderDue {
		p.raiseAlerts(c, oldState, true)
	}
}

func (p *Plum) logCheck(c *ScheduledCheck, result Result) {
	if !*quietLogging {
		log.Printf("Check '%s' executed in %s: %s (%s)\n", c.Name, result.Facts[CheckTime], result.State, result.Detail)
	}
}

func (p *Plum) raiseAlerts(c *ScheduledCheck, previousState CheckState, isReminder bool) {
	details := AlertDetails{
		Name:          c.Name,
		Config:        redact(c.Check),
		Type:          c.Type,
		LastResult:    c.LastResult(),
		PreviousState: previousState,
		NewState:      c.State,
		IsReminder:    isReminder,
		Link:          expandLink(c.Config.Link, c.Name, c.Type),
	}

	hasDetail := len(details.LastResult.Detail) > 0
	switch {
	case isReminder && hasDetail:
		details.Text = fmt.Sprintf("Check '%s' is still %s (%s).", details.Name, details.NewState, details.LastResult.Detail)
	case isReminder:
		details.Text = fmt.Sprintf("Check '%s' is still %s.", details.Name, details.NewState)
	case hasDetail:
		details.Text = fmt.Sprintf(
			"Check '%s' is now %s (%s), was %s.",
			details.Name,
			details.NewState,
			details.LastResult.Detail,
			details.PreviousState,
		)
	default:
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

	c.LastAlertTime = time.Now()
}

// regexpForWildcards converts a set of names containing '*' characters as wildcards into a single regex that will
// match any of them.
//
// e.g. ["foo_*", "*+bar"] becomes /^(?:foo_.*|.*\+bar)$/
func regexpForWildcards(names []string) *regexp.Regexp {
	pattern := strings.Builder{}
	pattern.WriteString("^(?:")

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

	pattern.WriteString(")$")
	re, _ := regexp.Compile(pattern.String())
	return re
}

type ScheduledCheck struct {
	Name          string
	Type          string
	Config        *CheckSettings
	Check         Check
	LastRun       time.Time
	LastAlertTime time.Time
	Scheduled     bool
	Settled       bool
	State         CheckState
	Suspended     bool
	History       ResultHistory
}

func (c *ScheduledCheck) Remaining() time.Duration {
	if c.Scheduled {
		return c.Config.Interval
	}
	return time.Until(c.LastRun.Add(c.Config.Interval))
}

func (c *ScheduledCheck) AddResult(result *Result) ResultHistory {
	copy(c.History[1:], c.History[:len(c.History)-1])
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

const shutdownTimeout = 5 * time.Second

// Run creates a new instance of Plum, registers plugins and loads configuration, and starts the main loop. It blocks
// until a value is received on signals, then saves state and cleans up. An error is returned if goplum couldn't start.
// It is expected that flag.Parse has been called prior to calling this method.
func Run(plugins map[string]Plugin, configPath, tombStonePath string, signals <-chan os.Signal) error {
	p := NewPlum()
	p.RegisterPlugins(plugins)

	if err := p.ReadConfig(configPath); err != nil {
		return fmt.Errorf("unable to read config: %w", err)
	}

	if err := p.RestoreState(tombStonePath); err != nil {
		log.Printf("Unable to restore state from tombstone: %v", err)
	}

	api := NewGrpcServer(p)
	if err := api.Start(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		p.Run(ctx)
	}()

	<-signals

	api.Stop()
	cancel()

	select {
	case <-stopped:
	case <-time.After(shutdownTimeout):
		log.Printf("Timed out waiting for running checks and alerts to finish")
	}

	if err := p.SaveState(tombStonePath); err != nil {
		log.Printf("Unable to save state to tombstone: %v", err)
	}

	return nil
}
