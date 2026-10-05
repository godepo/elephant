package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// SendBatch measures the entire batch as one operation, completed by Close.
// The timeout covers sending and reading results, including unread results drained by Close.
func (m DB) SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults {
	begin := time.Now()
	cancel := func() {}
	if timeout, ok := pgcontext.QueryTimeoutFrom(ctx); ok {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}
	return &metricBatchResults{BatchResults: m.db.SendBatch(ctx, batch), ctx: ctx,
		begin: begin, cancel: cancel, collector: m.defaultMetricsCollector}
}

type metricBatchResults struct {
	pgx.BatchResults
	ctx       context.Context
	begin     time.Time
	cancel    context.CancelFunc
	collector Collector
	once      sync.Once
	err       error
	closeErr  error
}

func (b *metricBatchResults) record(err error) {
	if b.err == nil && err != nil {
		b.err = err
	}
}

func (b *metricBatchResults) Exec() (pgconn.CommandTag, error) {
	tag, err := b.BatchResults.Exec()
	b.record(err)
	return tag, err
}

func (b *metricBatchResults) Query() (pgx.Rows, error) {
	rows, err := b.BatchResults.Query()
	b.record(err)
	if err != nil {
		return rows, err
	}
	return &metricBatchRows{Rows: rows, batch: b}, nil
}

func (b *metricBatchResults) QueryRow() pgx.Row {
	return metricBatchRow{Row: b.BatchResults.QueryRow(), batch: b}
}

func (b *metricBatchResults) Close() error {
	b.once.Do(func() {
		defer b.cancel()
		b.closeErr = b.BatchResults.Close()
		b.record(b.closeErr)
		b.collector.TrackQueryMetrics(b.ctx, b.begin, b.err)
	})
	return b.closeErr
}

type metricBatchRow struct {
	pgx.Row
	batch *metricBatchResults
}

func (r metricBatchRow) Scan(dest ...any) error {
	err := r.Row.Scan(dest...)
	r.batch.record(err)
	return err
}

type metricBatchRows struct {
	pgx.Rows
	batch *metricBatchResults
}

func (r *metricBatchRows) Close() {
	r.Rows.Close()
	r.batch.record(r.Rows.Err())
}

func (r *metricBatchRows) Next() bool {
	next := r.Rows.Next()
	if !next {
		r.batch.record(r.Rows.Err())
	}
	return next
}

func (r *metricBatchRows) Scan(dest ...any) error {
	err := r.Rows.Scan(dest...)
	r.batch.record(err)
	return err
}

func (r *metricBatchRows) Values() ([]any, error) {
	values, err := r.Rows.Values()
	r.batch.record(err)
	return values, err
}

func (r *metricBatchRows) Err() error {
	err := r.Rows.Err()
	r.batch.record(err)
	return err
}
