//go:generate mockery
package metrics

import (
	"context"
	"time"

	"github.com/godepo/elephant/internal/pkg/monads"
	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Pool interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error)
	Transactional(ctx context.Context, fn func(ctx context.Context) error) (out error)
}

type Collector interface {
	TrackQueryMetrics(ctx context.Context, begin time.Time, err error)
}

type DB struct {
	defaultMetricsCollector Collector
	db                      Pool
}

func (m DB) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	return m.db.BeginTx(ctx, opts)
}

func (m DB) Begin(ctx context.Context) (pgx.Tx, error) {
	return m.db.Begin(ctx)
}

func New(db Pool, collector Collector) DB {
	return DB{
		db:                      db,
		defaultMetricsCollector: collector,
	}
}

func (m DB) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	begin := time.Now()
	rows, err := m.db.Query(ctx, query, args...)
	if err != nil {
		m.defaultMetricsCollector.TrackQueryMetrics(ctx, begin, err)
		return nil, err
	}
	cancel := monads.EmptyOf[context.CancelFunc]()
	if timeout, ok := pgcontext.QueryTimeoutFrom(ctx); ok {
		cancelCtx, cancelFunc := context.WithTimeout(ctx, timeout)
		ctx, cancel = cancelCtx, monads.OptionalOf(cancelFunc)
	}
	return newDecoratedRows(ctx, rows, cancel, begin, m.defaultMetricsCollector), nil
}

func (m DB) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	row := decoratedMetricRow{
		ctx:       ctx,
		begin:     time.Now(),
		row:       m.db.QueryRow(ctx, query, args...),
		collector: m.defaultMetricsCollector,
	}
	timeout, ok := pgcontext.QueryTimeoutFrom(ctx)
	if ok {
		cancelCtx, cancel := context.WithTimeout(ctx, timeout)
		row.ctx = cancelCtx
		row.cancel = monads.OptionalOf(cancel)
	}
	return row
}

func (m DB) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	begin := time.Now()
	tag, err := m.db.Exec(ctx, query, args...)
	if err != nil {
		m.defaultMetricsCollector.TrackQueryMetrics(ctx, begin, err)
		return tag, err
	}
	m.defaultMetricsCollector.TrackQueryMetrics(ctx, begin, err)
	return tag, nil
}

func (m DB) Transactional(ctx context.Context, fn func(ctx context.Context) error) (out error) {
	return m.db.Transactional(ctx, fn)
}
