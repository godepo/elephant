package metrics

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/godepo/elephant/internal/pkg/monads"
	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// SendBatch measures the entire batch as one operation, completed by Close.
// The timeout covers sending and reading results, including unread results drained by Close.
func (m DB) SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults {
	begin := time.Now()
	cancel := monads.EmptyOf[context.CancelFunc]()
	if timeout, ok := pgcontext.QueryTimeoutFrom(ctx); ok {
		cancelCtx, cancelFunc := context.WithTimeout(ctx, timeout)
		ctx, cancel = cancelCtx, monads.OptionalOf(cancelFunc)
	}
	return &decoratedMetricBatchResults{BatchResults: m.db.SendBatch(ctx, batch), ctx: ctx,
		begin: begin, cancel: cancel, collector: m.defaultMetricsCollector}
}

type decoratedMetricBatchResults struct {
	pgx.BatchResults
	ctx       context.Context
	begin     time.Time
	cancel    monads.Optional[context.CancelFunc]
	collector Collector
	once      sync.Once
	err       error
	closeErr  error
}

// record retains the first error, except that a later error replaces pgx.ErrNoRows.
func (b *decoratedMetricBatchResults) record(err error) {
	if err == nil {
		return
	}
	if b.err == nil || (errors.Is(b.err, pgx.ErrNoRows) && !errors.Is(err, pgx.ErrNoRows)) {
		b.err = err
	}
}

func (b *decoratedMetricBatchResults) Exec() (pgconn.CommandTag, error) {
	tag, err := b.BatchResults.Exec()
	b.record(err)
	return tag, err
}

func (b *decoratedMetricBatchResults) Query() (pgx.Rows, error) {
	rows, err := b.BatchResults.Query()
	b.record(err)
	if err != nil {
		return rows, err
	}
	return &decoratedMetricBatchRows{Rows: rows, batch: b}, nil
}

func (b *decoratedMetricBatchResults) QueryRow() pgx.Row {
	return decoratedMetricBatchRow{Row: b.BatchResults.QueryRow(), batch: b}
}

func (b *decoratedMetricBatchResults) Close() error {
	b.once.Do(func() {
		b.closeErr = b.BatchResults.Close()
		b.record(b.closeErr)
		b.collector.TrackQueryMetrics(b.ctx, b.begin, b.err)
		if !b.cancel.IsEmpty() {
			b.cancel.Value()
		}
	})
	return b.closeErr
}

type decoratedMetricBatchRow struct {
	pgx.Row
	batch *decoratedMetricBatchResults
}

func (r decoratedMetricBatchRow) Scan(dest ...any) error {
	err := r.Row.Scan(dest...)
	r.batch.record(err)
	return err
}

type decoratedMetricBatchRows struct {
	pgx.Rows
	batch *decoratedMetricBatchResults
}

func (r *decoratedMetricBatchRows) Close() {
	r.Rows.Close()
	r.batch.record(r.Rows.Err())
}

func (r *decoratedMetricBatchRows) Next() bool {
	next := r.Rows.Next()
	if !next {
		r.batch.record(r.Rows.Err())
	}
	return next
}

func (r *decoratedMetricBatchRows) Scan(dest ...any) error {
	err := r.Rows.Scan(dest...)
	r.batch.record(err)
	return err
}

func (r *decoratedMetricBatchRows) Values() ([]any, error) {
	values, err := r.Rows.Values()
	r.batch.record(err)
	return values, err
}

func (r *decoratedMetricBatchRows) Err() error {
	err := r.Rows.Err()
	r.batch.record(err)
	return err
}
