//go:generate mockery
package metrics

import (
	"github.com/godepo/elephant"
	"github.com/godepo/elephant/internal/metrics"
	"github.com/godepo/elephant/internal/metrics/collector"
)

func Collector() elephant.MetricsBuilder {
	return collector.New()
}

func New(pool metrics.Pool, col metrics.Collector) metrics.Pool {
	return metrics.New(pool, col)
}
