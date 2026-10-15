package app

import (
	"time"

	"github.com/michaelmjhhhh/Chaos/internal/core"
)

// Snapshot is a detached, immutable copy suitable for handing to the UI thread.
type Snapshot struct {
	Revision                                             uint64
	Config                                               core.Config
	RecentFiles                                          []core.RecentFile
	Insights                                             core.Insights
	Watching                                             bool
	WatcherStatus, ActiveWatchDir                        string
	CurrentFile, CurrentPath, CurrentStage               string
	APIStatus, Error, Warning, StorageError, HealthError string
	RecoveryAction                                       core.RecoveryAction
	Busy                                                 bool
	BatchIndex, BatchTotal, QueueLength                  int
	Processed, Successes, Errors                         int
	AvgLatency, P95Latency                               float64
	Latencies                                            []float64
	HourlyThroughput                                     [24]int
	SuccessWindow                                        []float64
	WatcherStartedAt, StartedAt                          time.Time
	SessionNumber                                        int
	AutoStart, OnboardingComplete                        bool
	HistoryRevision                                      uint64
	DataDir, ConfigPath                                  string
	ResolvedWatchDir, ResolvedOutputDir                  string
}

func cloneSnapshot(s Snapshot) Snapshot {
	s.Config = cloneConfig(s.Config)
	s.RecentFiles = append([]core.RecentFile(nil), s.RecentFiles...)
	s.Latencies = append([]float64(nil), s.Latencies...)
	s.SuccessWindow = append([]float64(nil), s.SuccessWindow...)
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
