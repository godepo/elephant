package collector

import (
	"context"
)

const (
	InterceptAsFailure = "failure"
	InterceptAsSuccess = "success"
)

type Builder interface {
	QueryPerSecond(collector CounterCollector) Builder
	Latency(collector HistogramCollector) Builder
	ErrorsLogInterceptor(interceptor ErrorsLogInterceptor) Builder
	ResultsInterceptor(interceptor Interceptor) Builder
	Build() (*Collector, error)
}

type Interceptor func(ctx context.Context, err error) string

type Counter interface {
	Inc()
}

type Histogram interface {
	Observe(since float64)
}

type HistogramCollector func(labels ...string) (Histogram, error)
type CounterCollector func(labels ...string) (Counter, error)
