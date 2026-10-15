package app

import "github.com/michaelmjhhhh/Chaos/internal/core"

type ProcessingStage string

const (
	StageIdle      ProcessingStage = ""
	StageAnalyzing ProcessingStage = "analyzing"
	StageRenaming  ProcessingStage = "renaming"
	StageClipboard ProcessingStage = "clipboard"
	StageSuccess   ProcessingStage = "success"
	StageError     ProcessingStage = "error"
)

type HealthStatus string

const (
	HealthUnknown  HealthStatus = "N/A"
	HealthChecking HealthStatus = "Checking…"
	HealthOK       HealthStatus = "OK"
	HealthFailed   HealthStatus = "FAIL"
)

// Snapshot is a detached, immutable copy suitable for handing to the UI thread.
type Snapshot struct {
	Revision                                  uint64
	Config                                    core.Config
	RecentFiles                               []core.RecentFile
	Insights                                  core.Insights
	Watching                                  bool
	WatcherStatus, ActiveWatchDir             string
	CurrentFile, CurrentPath                  string
	CurrentStage                              ProcessingStage
	APIStatus                                 HealthStatus
	Error, Warning, StorageError, HealthError string
	RecoveryAction                            core.RecoveryAction
	Busy                                      bool
	BatchIndex, BatchTotal, QueueLength       int
	AutoStart, OnboardingComplete             bool
	ConfigPath                                string
	ResolvedWatchDir, ResolvedOutputDir       string
}

func cloneSnapshot(s Snapshot) Snapshot {
	s.Config = cloneConfig(s.Config)
	s.RecentFiles = append([]core.RecentFile(nil), s.RecentFiles...)
	s.Insights.Daily = cloneMap(s.Insights.Daily)
	s.Insights.TopCategories = append([]core.Labeled(nil), s.Insights.TopCategories...)
	s.Insights.FileTypes = append([]core.Labeled(nil), s.Insights.FileTypes...)
	if s.Insights.Fastest != nil {
		v := *s.Insights.Fastest
		s.Insights.Fastest = &v
	}
	if s.Insights.Slowest != nil {
		v := *s.Insights.Slowest
		s.Insights.Slowest = &v
	}
	return s
}

func cloneConfig(c core.Config) core.Config {
	c.ProviderKeys = cloneMap(c.ProviderKeys)
	return c
}

func cloneMap[K comparable, V any](m map[K]V) map[K]V {
	if m == nil {
		return nil
	}
	n := make(map[K]V, len(m))
	for k, v := range m {
		n[k] = v
	}
	return n
}
