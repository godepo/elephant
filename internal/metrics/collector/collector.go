//go:generate mockery
package collector

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
)

var (
	ErrCantGetQueryPerSecondCollector = errors.New("can't get query per second collector")
	ErrCantQetQueryLatencyCollector   = errors.New("can't query latency collector")
)

type ErrorsLogInterceptor func(err error)

type Collector struct {
	interceptor             Interceptor
	queryPerSecondCollector CounterCollector
	queryResultsCollector   HistogramCollector
	logInterceptor          ErrorsLogInterceptor
}

func (clt *Collector) TrackQueryMetrics(ctx context.Context, begin time.Time, err error) {
	labels, ok := pgcontext.MetricsLabelsFrom(ctx)
	if !ok {
		return
	}

	interceptor := clt.interceptor
	resultLabel := interceptor(ctx, err)

	since := float64(time.Since(begin).Milliseconds())
	labels = append(labels, resultLabel)

	if qps, err := clt.queryPerSecondCollector(labels...); err != nil {
		clt.logInterceptor(
			fmt.Errorf(
				"%w: %w: %v",
				ErrCantGetQueryPerSecondCollector,
				err,
				labels,
			),
		)
	} else {
		qps.Inc()
	}

	if col, err := clt.queryResultsCollector(labels...); err != nil {
		clt.logInterceptor(
			fmt.Errorf(
				"%w: %w: %v",
				ErrCantQetQueryLatencyCollector,
				err,
				labels,
			),
		)
	} else {
		col.Observe(since)
	}
}
