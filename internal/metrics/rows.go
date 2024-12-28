package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/godepo/elephant/internal/pkg/monads"
	"github.com/jackc/pgx/v5"
)

type decoratedMetricRows struct {
	pgx.Rows
	ctx       context.Context
	once      *sync.Once
	cancel    monads.Optional[context.CancelFunc]
	begin     time.Time
	collector Collector
}

func newDecoratedRows(
	ctx context.Context,
	rows pgx.Rows,
	cancel monads.Optional[context.CancelFunc],
	begin time.Time, collector Collector,
) decoratedMetricRows {
	return decoratedMetricRows{
		Rows:      rows,
		ctx:       ctx,
		once:      &sync.Once{},
		cancel:    cancel,
		begin:     begin,
		collector: collector,
	}
}

func (rows decoratedMetricRows) Close() {
	rows.once.Do(rows.close)
}

func (rows decoratedMetricRows) close() {
	rows.collector.TrackQueryMetrics(rows.ctx, rows.begin, rows.Err())
	rows.Rows.Close()
	if !rows.cancel.IsEmpty() {
		rows.cancel.Value()
	}
}
