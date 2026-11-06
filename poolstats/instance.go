package poolstats

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func (inst *instance) Collect(metrics chan<- prometheus.Metric) {
	stats := inst.provider()

	inst.acquireCountDesc.CollectCounter(metrics, stats.AcquireCount())
	inst.acquireDurationDesc.CollectCounter(metrics, stats.AcquireDuration())
	inst.acquiredConnsDesc.CollectGauge(metrics, stats.AcquiredConns())
	inst.canceledAcquireCountDesc.CollectCounter(metrics, stats.CanceledAcquireCount())
	inst.constructingConnsDesc.CollectGauge(metrics, stats.ConstructingConns())
	inst.emptyAcquireCountDesc.CollectCounter(metrics, stats.EmptyAcquireCount())
	inst.emptyAcquireWaitTimeDesc.CollectCounter(metrics, stats.EmptyAcquireWaitTime())
	inst.idleConnsDesc.CollectGauge(metrics, stats.IdleConns())
	inst.maxConnsDesc.CollectGauge(metrics, stats.MaxConns())
	inst.totalConnsDesc.CollectGauge(metrics, stats.TotalConns())
	inst.newConnsCount.CollectCounter(metrics, stats.NewConnsCount())
	inst.maxLifetimeDestroyCount.CollectCounter(metrics, stats.MaxLifetimeDestroyCount())
	inst.maxIdleDestroyCount.CollectCounter(metrics, stats.MaxIdleDestroyCount())
}

// newCollector is an internal only constructor for a Collecter. It accepts
// a staterFunc which provides a closure for requesting pgxpool.Stat metrics.
// Labels to each metric and may be nil. A label is recommended when an
// application uses more than one pgxpool.Pool to enable differentiation between them.
func (inst *instance) init(desc describers) {
	inst.acquireCountDesc = newDescriber[int64](
		desc.acquireCountDesc,
		inst.labels...,
	)
	inst.acquireDurationDesc = newDescriber[time.Duration](
		desc.acquireDurationDesc,
		inst.labels...)
	inst.acquiredConnsDesc = newDescriber[int32](
		desc.acquiredConnsDesc,
		inst.labels...)
	inst.canceledAcquireCountDesc = newDescriber[int64](
		desc.canceledAcquireCountDesc,
		inst.labels...)
	inst.constructingConnsDesc = newDescriber[int32](
		desc.constructingConnsDesc,
		inst.labels...)
	inst.emptyAcquireCountDesc = newDescriber[int64](
		desc.emptyAcquireCountDesc,
		inst.labels...)
	inst.emptyAcquireWaitTimeDesc = newDescriber[time.Duration](
		desc.emptyAcquireWaitTimeDesc,
		inst.labels...)
	inst.idleConnsDesc = newDescriber[int32](
		desc.idleConnsDesc,
		inst.labels...)
	inst.maxConnsDesc = newDescriber[int32](
		desc.maxConnsDesc,
		inst.labels...)
	inst.totalConnsDesc = newDescriber[int32](
		desc.totalConnsDesc,
		inst.labels...)
	inst.newConnsCount = newDescriber[int64](
		desc.newConnsCount,
		inst.labels...)
	inst.maxLifetimeDestroyCount = newDescriber[int64](
		desc.maxLifetimeDestroyCount,
		inst.labels...)
	inst.maxIdleDestroyCount = newDescriber[int64](
		desc.maxIdleDestroyCount,
		inst.labels...)
}
