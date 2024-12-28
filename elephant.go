//go:generate mockery
package elephant

import (
	"context"

	"github.com/godepo/elephant/internal/metrics"
	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
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

func WithMetricsLabel(metricsLabel string) pgcontext.OptionContext {
	return pgcontext.WithMetricsLabel(metricsLabel)
}

func WithMetricsCollector(collector metrics.Collector) pgcontext.OptionContext {
	return pgcontext.WithMetricsCollector(collector)
}

func MetricsLabelFrom(ctx context.Context) (string, bool) {
	return pgcontext.MetricsLabelFrom(ctx)
}
