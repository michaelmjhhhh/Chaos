package app

import (
	"path/filepath"
	"reflect"
	"strings"

	"github.com/michaelmjhhhh/Chaos/internal/core"
)

// Each operation clears only its own failure. A settings save cannot resolve a
// pending journal or restore a history entry that failed to persist.
type storageErrors struct {
	config, preferences, historyRead, historyWrite, recovery string
}

func (e storageErrors) message() string {
	var messages []string
	for _, message := range []string{e.config, e.preferences, e.historyRead, e.historyWrite, e.recovery} {
		if message != "" {
			messages = append(messages, message)
		}
	}
	return strings.Join(messages, "\n")
}

func (c *Controller) SaveConfig(cfg core.Config) {
	changedWatch := false
	if !c.change(func(s *Snapshot) bool {
		if c.ctx.Err() != nil || (reflect.DeepEqual(s.Config, cfg) && c.storageErrors.config == "") {
			return false
		}
		old := s.Config.Resolve(c.home)
		s.Config = cloneConfig(cfg)
		resolved := s.Config.Resolve(c.home)
		s.ResolvedWatchDir, s.ResolvedOutputDir = resolved.WatchDir, resolved.OutputDir
		c.configRevision++
		if old.Provider.ID != resolved.Provider.ID || old.APIKey != resolved.APIKey || old.Model != resolved.Model || old.BaseURL != resolved.BaseURL {
			s.APIStatus, s.HealthError = "N/A", ""
			c.healthRevision++
		}
		changedWatch = s.Watching && old.WatchDir != resolved.WatchDir
		return true
	}) {
		return
	}
	signal(c.saveWake)
	if changedWatch {
		signal(c.watchWake)
	}
}

func (c *Controller) SetAutoStart(enabled bool) {
	if c.change(func(s *Snapshot) bool {
		if c.ctx.Err() != nil || (c.prefs.AutoStart == enabled && c.storageErrors.preferences == "") {
			return false
		}
		c.prefs.AutoStart, s.AutoStart = enabled, enabled
		c.preferencesRevision++
		return true
	}) {
		signal(c.saveWake)
	}
}

func (c *Controller) CompleteOnboarding(start bool) {
	if c.change(func(s *Snapshot) bool {
		if c.ctx.Err() != nil || (c.prefs.OnboardingComplete && c.storageErrors.preferences == "") {
			return false
		}
		c.prefs.OnboardingComplete, s.OnboardingComplete = true, true
		c.preferencesRevision++
		return true
	}) {
		signal(c.saveWake)
	}
	if start {
		c.Start()
	}
}

func (c *Controller) ReplayOnboarding() {
	if c.change(func(s *Snapshot) bool {
		if c.ctx.Err() != nil || (!c.prefs.OnboardingComplete && c.storageErrors.preferences == "") {
			return false
		}
		c.prefs.OnboardingComplete, s.OnboardingComplete = false, false
		c.preferencesRevision++
		return true
	}) {
		signal(c.saveWake)
	}
}

func (c *Controller) settingsWorker() {
	defer c.wg.Done()
	for {
		select {
		case <-c.saveWake:
			c.saveSettings()
		case <-c.ctx.Done():
			c.saveSettings()
			return
		}
	}
}

// Only this worker writes settings. Acknowledging the captured revisions leaves
// any edits made during disk I/O dirty for its next wakeup or shutdown flush.
func (c *Controller) saveSettings() {
	c.mu.Lock()
	configRevision, preferencesRevision := c.configRevision, c.preferencesRevision
	writeConfig := configRevision != c.savedConfigRevision
	writePreferences := preferencesRevision != c.savedPreferencesRevision
	var cfg core.Config
	if writeConfig {
		cfg = cloneConfig(c.snapshot.Config)
	}
	prefs := c.prefs
	c.mu.Unlock()
	if !writeConfig && !writePreferences {
		return
	}
	// Opening corrupt configuration never authorizes a preference edit to replace it.
	var configErr, preferencesErr error
	if writeConfig {
		configErr = core.SaveConfig(filepath.Join(c.dataDir, "config.json"), cfg)
	}
	if writePreferences {
		preferencesErr = savePreferences(filepath.Join(c.dataDir, "preferences.json"), prefs)
	}
	c.change(func(*Snapshot) bool {
		previous := c.storageErrors
		if writeConfig {
			if configErr == nil {
				c.savedConfigRevision = configRevision
			}
			if configRevision == c.configRevision {
				c.storageErrors.config = ""
				if configErr != nil {
					c.storageErrors.config = "Save configuration: " + configErr.Error()
				}
			}
		}
		if writePreferences {
			if preferencesErr == nil {
				c.savedPreferencesRevision = preferencesRevision
			}
			if preferencesRevision == c.preferencesRevision {
				c.storageErrors.preferences = ""
				if preferencesErr != nil {
					c.storageErrors.preferences = "Save preferences: " + preferencesErr.Error()
				}
			}
		}
		return previous != c.storageErrors
	})
}
