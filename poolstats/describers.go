package poolstats

import (
	"github.com/prometheus/client_golang/prometheus"
)

type describers struct {
	acquireCountDesc         *prometheus.Desc
	acquireDurationDesc      *prometheus.Desc
	acquiredConnsDesc        *prometheus.Desc
	canceledAcquireCountDesc *prometheus.Desc
	constructingConnsDesc    *prometheus.Desc
	emptyAcquireCountDesc    *prometheus.Desc
	emptyAcquireWaitTimeDesc *prometheus.Desc
	idleConnsDesc            *prometheus.Desc
	maxConnsDesc             *prometheus.Desc
	totalConnsDesc           *prometheus.Desc
	newConnsCount            *prometheus.Desc
	maxLifetimeDestroyCount  *prometheus.Desc
	maxIdleDestroyCount      *prometheus.Desc
}

func newDescribers(staticLabels prometheus.Labels, labels ...string) describers {
	return describers{
		acquireCountDesc: prometheus.NewDesc(
			"pgxpool_acquire_count",
			"Cumulative count of successful acquires from the pool.",
			labels, staticLabels),
		acquireDurationDesc: prometheus.NewDesc(
			"pgxpool_acquire_duration_ns",
			"Total duration of all successful acquires from the pool in nanoseconds.",
			labels, staticLabels),
		acquiredConnsDesc: prometheus.NewDesc(
			"pgxpool_acquired_conns",
			"Number of currently acquired connections in the pool.",
			labels, staticLabels),
		canceledAcquireCountDesc: prometheus.NewDesc(
			"pgxpool_canceled_acquire_count",
			"Cumulative count of acquires from the pool that were canceled by a context.",
			labels, staticLabels),
		constructingConnsDesc: prometheus.NewDesc(
			"pgxpool_constructing_conns",
			"Number of conns with construction in progress in the pool.",
			labels, staticLabels),
		emptyAcquireCountDesc: prometheus.NewDesc(
			"pgxpool_empty_acquire",
			"Cumulative count of successful acquires from the pool that waited for a resource to be released or constructed because the pool was empty.",
			labels, staticLabels),
		emptyAcquireWaitTimeDesc: prometheus.NewDesc(
			"pgxpool_empty_acquire_wait_time_ns",
			"Cumulative time in nanoseconds waited for successful acquires from the pool for a resource to be released or constructed because the pool was empty.",
			labels, staticLabels),
		idleConnsDesc: prometheus.NewDesc(
			"pgxpool_idle_conns",
			"Number of currently idle conns in the pool.",
			labels, staticLabels),
		maxConnsDesc: prometheus.NewDesc(
			"pgxpool_max_conns",
			"Maximum size of the pool.",
			labels, staticLabels),
		totalConnsDesc: prometheus.NewDesc(
			"pgxpool_total_conns",
			"Total number of resources currently in the pool. The value is the sum of ConstructingConns, AcquiredConns, and IdleConns.",
			labels, staticLabels),
		newConnsCount: prometheus.NewDesc(
			"pgxpool_new_conns_count",
			"Cumulative count of new connections opened.",
			labels, staticLabels),
		maxLifetimeDestroyCount: prometheus.NewDesc(
			"pgxpool_max_lifetime_destroy_count",
			"Cumulative count of connections destroyed because they exceeded MaxConnLifetime.",
			labels, staticLabels),
		maxIdleDestroyCount: prometheus.NewDesc(
			"pgxpool_max_idle_destroy_count",
			"Cumulative count of connections destroyed because they exceeded MaxConnIdleTime.",
			labels, staticLabels),
	}
}
