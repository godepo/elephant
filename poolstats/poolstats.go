package poolstats

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type (
	Informer interface {
		AcquireCount() int64
		AcquireDuration() time.Duration
		AcquiredConns() int32
		CanceledAcquireCount() int64
		ConstructingConns() int32
		EmptyAcquireCount() int64
		IdleConns() int32
		MaxConns() int32
		MaxLifetimeDestroyCount() int64
		NewConnsCount() int64
		TotalConns() int32
		EmptyAcquireWaitTime() time.Duration
		MaxIdleDestroyCount() int64
	}

	InformerProvider func() Informer

	instance struct {
		provider InformerProvider

		acquireCountDesc         *Describer[int64]
		acquireDurationDesc      *Describer[time.Duration]
		acquiredConnsDesc        *Describer[int32]
		canceledAcquireCountDesc *Describer[int64]
		constructingConnsDesc    *Describer[int32]
		emptyAcquireCountDesc    *Describer[int64]
		emptyAcquireWaitTimeDesc *Describer[time.Duration]
		idleConnsDesc            *Describer[int32]
		maxConnsDesc             *Describer[int32]
		totalConnsDesc           *Describer[int32]
		newConnsCount            *Describer[int64]
		maxLifetimeDestroyCount  *Describer[int64]
		maxIdleDestroyCount      *Describer[int64]
		labels                   []string
	}

	Collector struct {
		instances    []*instance
		staticLabels prometheus.Labels
		labels       []string
	}

	Option func(*Collector)

	InstanceOptions func(informer *instance)
)

func InformerProviderFrom(inf Informer) InformerProvider {
	return func() Informer {
		return inf
	}
}

func WithInformerProvider(
	f InformerProvider,
	labels ...string,
) Option {
	return func(c *Collector) {
		inst := instance{
			provider: f,
			labels:   labels,
		}

		c.instances = append(c.instances, &inst)
	}
}

func WithLabels(labels prometheus.Labels) Option {
	return func(c *Collector) {
		c.staticLabels = labels
	}
}

func WithLabelsNames(labels ...string) Option {
	return func(c *Collector) {
		c.labels = labels
	}
}

func New(options ...Option) *Collector {
	col := Collector{}

	for _, opt := range options {
		opt(&col)
	}

	desc := newDescribers(col.staticLabels, col.labels...)

	for _, inst := range col.instances {
		inst.init(desc)
	}

	return &col
}

// Describe implements the prometheus.Collector interface.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, ch)
}

func (c *Collector) Collect(metrics chan<- prometheus.Metric) {
	for _, inst := range c.instances {
		inst.Collect(metrics)
	}
}
