//go:generate go tool mockery
package collector

import (
	"context"
	"errors"

	"github.com/godepo/elephant"
	"github.com/godepo/elephant/internal/pkg/monads"
)

var (
	ErrQueryPerSecondIsRequired = errors.New("collector for query per second is required")
	ErrQueryLatencyIsRequired   = errors.New("collector for query latency is required")
)

type builder struct {
	queryPerSeconds    monads.Optional[elephant.CounterCollector]
	queryLatency       monads.Optional[elephant.HistogramCollector]
	logInterceptor     monads.Optional[elephant.ErrorsLogInterceptor]
	resultsInterceptor monads.Optional[elephant.Interceptor]
}

func (b builder) ResultsInterceptor(interceptor elephant.Interceptor) elephant.MetricsBuilder {
	cln := b.clone()
	cln.resultsInterceptor = monads.OptionalOf(interceptor)
	return cln
}

func (b builder) ErrorsLogInterceptor(interceptor elephant.ErrorsLogInterceptor) elephant.MetricsBuilder {
	cln := b.clone()
	cln.logInterceptor = monads.OptionalOf(interceptor)
	return cln
}

func (b builder) QueryPerSecond(collector elephant.CounterCollector) elephant.MetricsBuilder {
	cln := b.clone()
	cln.queryPerSeconds = monads.OptionalOf(collector)
	return cln
}

func (b builder) Latency(collector elephant.HistogramCollector) elephant.MetricsBuilder {
	cln := b.clone()
	cln.queryLatency = monads.OptionalOf(collector)
	return cln
}

func (b builder) Build() (elephant.MetricsCollector, error) {
	if b.queryPerSeconds.IsEmpty() {
		return nil, ErrQueryPerSecondIsRequired
	}
	if b.queryLatency.IsEmpty() {
		return nil, ErrQueryLatencyIsRequired
	}
	collector := &Collector{
		queryPerSecondCollector: b.queryPerSeconds.Value,
		queryResultsCollector:   b.queryLatency.Value,
		interceptor:             defaultInterceptor,
		logInterceptor:          func(err error) {},
	}
	if !b.logInterceptor.IsEmpty() {
		collector.logInterceptor = b.logInterceptor.Value
	}
	if !b.resultsInterceptor.IsEmpty() {
		collector.interceptor = b.resultsInterceptor.Value
	}
	return collector, nil
}

func (b builder) clone() builder {
	out := builder{
		queryLatency:       b.queryLatency,
		queryPerSeconds:    b.queryPerSeconds,
		logInterceptor:     b.logInterceptor,
		resultsInterceptor: b.resultsInterceptor,
	}

	return out
}

func New() elephant.MetricsBuilder {
	return builder{}
}

func defaultInterceptor(_ context.Context, err error) string {
	if err != nil {
		return InterceptAsFailure
	}
	return InterceptAsSuccess
}
