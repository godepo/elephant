package trunk

import (
	"fmt"

	"github.com/godepo/elephant/poolstats"
	"github.com/prometheus/client_golang/prometheus"
)

func (settings *Settings) validate() error {
	if len(settings.leaders) == 0 {
		return ErrDSNNotPresent
	}

	if len(settings.followers) != len(settings.leaders) {
		return ErrInvalidClusterConfiguration
	}

	if len(settings.leaders) > 1 && settings.picker == nil {
		return ErrRequirePicker
	}

	return nil
}

func (settings *Settings) With(opts ...Option) *Settings {
	for _, opt := range opts {
		opt(settings)
	}

	return settings
}

func (settings *Settings) getLabelsNames() []string {
	labels := []string{"query", "success"}

	if len(settings.leaders) > 1 {
		labels = append(labels, "shard_id")
	}

	if settings.followersCount > 0 {
		labels = append(labels, "role", "node_id")
	}

	return labels
}

func (settings *Settings) registerMetricsCollector() error {
	err := settings.registry.Register(settings.qps)
	if err != nil {
		return fmt.Errorf("register query per seconds stats collector: %w", err)
	}

	err = settings.registry.Register(settings.latencies)
	if err != nil {
		return fmt.Errorf("register latencies seconds stats collector: %w", err)
	}

	collector := poolstats.New(
		settings.poolStatsOptions...,
	)

	err = settings.registry.Register(collector)
	if err != nil {
		return fmt.Errorf("register pool stats collector: %w", err)
	}

	return err
}

func (settings *Settings) init(labelsNames ...string) {
	settings.latencies = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "pg_query_latencies",
			Help: "latency metrics for postgresql queries",
		},
		labelsNames,
	)
	settings.qps = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "pg_query_total",
		Help: "queries per seconds for postgresql queries",
	}, labelsNames,
	)
}
