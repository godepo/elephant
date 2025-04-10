//go:generate go tool mockery
package elephant

import (
	"context"
	"time"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
)

type (
	Interceptor func(ctx context.Context, err error) string

	Counter interface {
		Inc()
	}

	Histogram interface {
		Observe(since float64)
	}

	HistogramCollector func(labels ...string) (Histogram, error)
	CounterCollector   func(labels ...string) (Counter, error)

	ErrorsLogInterceptor func(err error)

	MetricsCollector interface {
		TrackQueryMetrics(ctx context.Context, begin time.Time, err error)
	}

	MetricsBuilder interface {
		QueryPerSecond(collector CounterCollector) MetricsBuilder
		Latency(collector HistogramCollector) MetricsBuilder
		ErrorsLogInterceptor(interceptor ErrorsLogInterceptor) MetricsBuilder
		ResultsInterceptor(interceptor Interceptor) MetricsBuilder
		Build() (MetricsCollector, error)
	}
)

func With(ctx context.Context, opts ...pgcontext.OptionContext) context.Context {
	return pgcontext.With(ctx, opts...)
}

func WithCanWrite(ctx context.Context) context.Context {
	return pgcontext.WithCanWrite(ctx)
}

func WithTransaction(tx pgx.Tx) pgcontext.OptionContext {
	return pgcontext.WithTransaction(tx)
}

func WithMetricsLabel(metricsLabels ...string) pgcontext.OptionContext {
	return pgcontext.WithMetricsLabel(metricsLabels...)
}

func WithShardID(id uint) pgcontext.OptionContext {
	return pgcontext.WithShardID(id)
}

func WithShardingKey(key string) pgcontext.OptionContext {
	return pgcontext.WithShardingKey(key)
}

func WithTimeout(timeout time.Duration) pgcontext.OptionContext {
	return pgcontext.WithTimeout(timeout)
}

func WithTxOptions(opt pgx.TxOptions) pgcontext.OptionContext {
	return pgcontext.WithTxOptions(opt)
}

func WithFnTxPassMatcher(fn pgcontext.TxPassMatcher) pgcontext.OptionContext {
	return pgcontext.WithFnTxPassMatcher(fn)
}
