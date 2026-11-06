package poolstats

import (
	"github.com/prometheus/client_golang/prometheus"
)

type (
	Number interface {
		~int | ~int8 | ~int16 | ~int32 | ~int64 |
			~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
			~float32 | ~float64
	}
	Describer[T Number] struct {
		describer *prometheus.Desc
		labels    []string
	}
)

func newDescriber[T Number](
	describer *prometheus.Desc, labels ...string,
) *Describer[T] {
	return &Describer[T]{
		describer: describer,
		labels:    labels,
	}
}

func (desc *Describer[T]) CollectCounter(
	metrics chan<- prometheus.Metric,
	value T,
) {
	metrics <- prometheus.MustNewConstMetric(desc.describer, prometheus.CounterValue,
		float64(value),
		desc.labels...,
	)
}

func (desc *Describer[T]) CollectGauge(
	metrics chan<- prometheus.Metric,
	value T,
) {
	metrics <- prometheus.MustNewConstMetric(desc.describer, prometheus.GaugeValue,
		float64(value),
		desc.labels...,
	)
}
