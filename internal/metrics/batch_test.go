package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type batchStub struct {
	pgx.BatchResults
	execErr, queryErr, closeErr error
	row                         pgx.Row
	rows                        pgx.Rows
	closes                      int
	closeFn                     func()
}

func (b *batchStub) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, b.execErr }
func (b *batchStub) Query() (pgx.Rows, error)         { return b.rows, b.queryErr }
func (b *batchStub) QueryRow() pgx.Row                { return b.row }
func (b *batchStub) Close() error {
	b.closes++
	if b.closeFn != nil {
		b.closeFn()
	}
	return b.closeErr
}

type batchCollector struct {
	calls int
	err   error
	ctx   context.Context
}

func (c *batchCollector) TrackQueryMetrics(ctx context.Context, _ time.Time, err error) {
	c.calls++
	c.err = err
	c.ctx = ctx
}

func TestBatchMetricsErrorsAndClose(t *testing.T) {
	t.Run("should be able to record a successful batch once after closing", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}

		stub := &batchStub{}
		batch := &pgx.Batch{}
		var sentCtx context.Context
		pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(ctx context.Context, _ *pgx.Batch) pgx.BatchResults {
			sentCtx = ctx
			return stub
		}).Once()
		ctx := pgcontext.With(context.Background(), pgcontext.WithTimeout(time.Second), pgcontext.WithMetricsLabel("batch"))

		results := New(pool, collector).SendBatch(ctx, batch)
		require.NotNil(t, sentCtx)
		_, deadline := sentCtx.Deadline()
		require.True(t, deadline)
		_, err := results.Exec()
		require.NoError(t, err)
		require.Zero(t, collector.calls)
		require.NoError(t, sentCtx.Err(), "reading a result must not cancel the batch")
		stub.closeFn = func() {
			require.NoError(t, sentCtx.Err(), "Close must drain before cancellation")
		}
		require.NoError(t, results.Close())
		require.NoError(t, results.Close())
		require.Equal(t, 1, stub.closes)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, sentCtx.Err(), context.Canceled)
		require.NoError(t, collector.err)
		labels, ok := pgcontext.MetricsLabelsFrom(collector.ctx)
		require.True(t, ok)
		require.Equal(t, []string{"batch"}, labels)
	})

	t.Run("should be able to retain an Exec error until the batch is closed", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		failure := errors.New("batch execution failed")
		stub := &batchStub{execErr: failure}
		batch := &pgx.Batch{}
		var sentCtx context.Context
		pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(ctx context.Context, _ *pgx.Batch) pgx.BatchResults {
			sentCtx = ctx
			return stub
		}).Once()
		ctx := pgcontext.With(context.Background(), pgcontext.WithTimeout(time.Second), pgcontext.WithMetricsLabel("batch"))

		results := New(pool, collector).SendBatch(ctx, batch)
		require.NotNil(t, sentCtx)
		_, deadline := sentCtx.Deadline()
		require.True(t, deadline)
		_, err := results.Exec()
		require.ErrorIs(t, err, failure)
		require.Zero(t, collector.calls)
		require.NoError(t, sentCtx.Err(), "reading a result must not cancel the batch")
		stub.closeFn = func() {
			require.NoError(t, sentCtx.Err(), "Close must drain before cancellation")
		}
		require.NoError(t, results.Close())
		require.NoError(t, results.Close())
		require.Equal(t, 1, stub.closes)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, sentCtx.Err(), context.Canceled)
		require.ErrorIs(t, collector.err, failure)
		labels, ok := pgcontext.MetricsLabelsFrom(collector.ctx)
		require.True(t, ok)
		require.Equal(t, []string{"batch"}, labels)
	})

	t.Run("should be able to retain a Query error and preserve its returned rows", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		failure := errors.New("batch query failed")
		rows := NewMockRows(t)
		stub := &batchStub{rows: rows, queryErr: failure}
		batch := &pgx.Batch{}
		var sentCtx context.Context
		pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(ctx context.Context, _ *pgx.Batch) pgx.BatchResults {
			sentCtx = ctx
			return stub
		}).Once()
		ctx := pgcontext.With(context.Background(), pgcontext.WithTimeout(time.Second), pgcontext.WithMetricsLabel("batch"))

		results := New(pool, collector).SendBatch(ctx, batch)
		require.NotNil(t, sentCtx)
		_, deadline := sentCtx.Deadline()
		require.True(t, deadline)
		actual, err := results.Query()
		require.ErrorIs(t, err, failure)
		require.Same(t, rows, actual)
		require.Zero(t, collector.calls)
		require.NoError(t, sentCtx.Err(), "reading a result must not cancel the batch")
		stub.closeFn = func() {
			require.NoError(t, sentCtx.Err(), "Close must drain before cancellation")
		}
		require.NoError(t, results.Close())
		require.NoError(t, results.Close())
		require.Equal(t, 1, stub.closes)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, sentCtx.Err(), context.Canceled)
		require.ErrorIs(t, collector.err, failure)
		labels, ok := pgcontext.MetricsLabelsFrom(collector.ctx)
		require.True(t, ok)
		require.Equal(t, []string{"batch"}, labels)
	})

	t.Run("should be able to retain a QueryRow Scan error until the batch is closed", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		failure := errors.New("batch row scan failed")
		row := NewMockRow(t)
		row.EXPECT().Scan(mock.Anything).Return(failure).Once()
		stub := &batchStub{row: row}
		batch := &pgx.Batch{}
		var sentCtx context.Context
		pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(ctx context.Context, _ *pgx.Batch) pgx.BatchResults {
			sentCtx = ctx
			return stub
		}).Once()
		ctx := pgcontext.With(context.Background(), pgcontext.WithTimeout(time.Second), pgcontext.WithMetricsLabel("batch"))

		results := New(pool, collector).SendBatch(ctx, batch)
		require.NotNil(t, sentCtx)
		_, deadline := sentCtx.Deadline()
		require.True(t, deadline)
		var value int
		require.ErrorIs(t, results.QueryRow().Scan(&value), failure)
		require.Zero(t, collector.calls)
		require.NoError(t, sentCtx.Err(), "reading a result must not cancel the batch")
		stub.closeFn = func() {
			require.NoError(t, sentCtx.Err(), "Close must drain before cancellation")
		}
		require.NoError(t, results.Close())
		require.NoError(t, results.Close())
		require.Equal(t, 1, stub.closes)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, sentCtx.Err(), context.Canceled)
		require.ErrorIs(t, collector.err, failure)
		labels, ok := pgcontext.MetricsLabelsFrom(collector.ctx)
		require.True(t, ok)
		require.Equal(t, []string{"batch"}, labels)
	})

	t.Run("should be able to retain a Rows Scan error until the batch is closed", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		failure := errors.New("batch rows scan failed")
		rows := NewMockRows(t)
		rows.EXPECT().Scan(mock.Anything).Return(failure).Once()
		stub := &batchStub{rows: rows}
		batch := &pgx.Batch{}
		var sentCtx context.Context
		pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(ctx context.Context, _ *pgx.Batch) pgx.BatchResults {
			sentCtx = ctx
			return stub
		}).Once()
		ctx := pgcontext.With(context.Background(), pgcontext.WithTimeout(time.Second), pgcontext.WithMetricsLabel("batch"))

		results := New(pool, collector).SendBatch(ctx, batch)
		require.NotNil(t, sentCtx)
		_, deadline := sentCtx.Deadline()
		require.True(t, deadline)
		wrapped, err := results.Query()
		require.NoError(t, err)
		var value int
		require.ErrorIs(t, wrapped.Scan(&value), failure)
		require.Zero(t, collector.calls)
		require.NoError(t, sentCtx.Err(), "reading a result must not cancel the batch")
		stub.closeFn = func() {
			require.NoError(t, sentCtx.Err(), "Close must drain before cancellation")
		}
		require.NoError(t, results.Close())
		require.NoError(t, results.Close())
		require.Equal(t, 1, stub.closes)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, sentCtx.Err(), context.Canceled)
		require.ErrorIs(t, collector.err, failure)
		labels, ok := pgcontext.MetricsLabelsFrom(collector.ctx)
		require.True(t, ok)
		require.Equal(t, []string{"batch"}, labels)
	})

	t.Run("should be able to retain a Rows Values error until the batch is closed", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		failure := errors.New("batch rows values failed")
		rows := NewMockRows(t)
		rows.EXPECT().Values().Return(nil, failure).Once()
		stub := &batchStub{rows: rows}
		batch := &pgx.Batch{}
		var sentCtx context.Context
		pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(ctx context.Context, _ *pgx.Batch) pgx.BatchResults {
			sentCtx = ctx
			return stub
		}).Once()
		ctx := pgcontext.With(context.Background(), pgcontext.WithTimeout(time.Second), pgcontext.WithMetricsLabel("batch"))

		results := New(pool, collector).SendBatch(ctx, batch)
		require.NotNil(t, sentCtx)
		_, deadline := sentCtx.Deadline()
		require.True(t, deadline)
		wrapped, err := results.Query()
		require.NoError(t, err)
		_, err = wrapped.Values()
		require.ErrorIs(t, err, failure)
		require.Zero(t, collector.calls)
		require.NoError(t, sentCtx.Err(), "reading a result must not cancel the batch")
		stub.closeFn = func() {
			require.NoError(t, sentCtx.Err(), "Close must drain before cancellation")
		}
		require.NoError(t, results.Close())
		require.NoError(t, results.Close())
		require.Equal(t, 1, stub.closes)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, sentCtx.Err(), context.Canceled)
		require.ErrorIs(t, collector.err, failure)
		labels, ok := pgcontext.MetricsLabelsFrom(collector.ctx)
		require.True(t, ok)
		require.Equal(t, []string{"batch"}, labels)
	})

	t.Run("should be able to retain the Rows error when iteration stops", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		failure := errors.New("batch rows iteration failed")
		rows := NewMockRows(t)
		rows.EXPECT().Next().Return(false).Once()
		rows.EXPECT().Err().Return(failure).Once()
		stub := &batchStub{rows: rows}
		batch := &pgx.Batch{}
		var sentCtx context.Context
		pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(ctx context.Context, _ *pgx.Batch) pgx.BatchResults {
			sentCtx = ctx
			return stub
		}).Once()
		ctx := pgcontext.With(context.Background(), pgcontext.WithTimeout(time.Second), pgcontext.WithMetricsLabel("batch"))

		results := New(pool, collector).SendBatch(ctx, batch)
		require.NotNil(t, sentCtx)
		_, deadline := sentCtx.Deadline()
		require.True(t, deadline)
		wrapped, err := results.Query()
		require.NoError(t, err)
		require.False(t, wrapped.Next())
		require.Zero(t, collector.calls)
		require.NoError(t, sentCtx.Err(), "reading a result must not cancel the batch")
		stub.closeFn = func() {
			require.NoError(t, sentCtx.Err(), "Close must drain before cancellation")
		}
		require.NoError(t, results.Close())
		require.NoError(t, results.Close())
		require.Equal(t, 1, stub.closes)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, sentCtx.Err(), context.Canceled)
		require.ErrorIs(t, collector.err, failure)
		labels, ok := pgcontext.MetricsLabelsFrom(collector.ctx)
		require.True(t, ok)
		require.Equal(t, []string{"batch"}, labels)
	})

	t.Run("should be able to retain the Rows error when rows are closed", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		failure := errors.New("batch rows close failed")
		rows := NewMockRows(t)
		rows.EXPECT().Close().Once()
		rows.EXPECT().Err().Return(failure).Once()
		stub := &batchStub{rows: rows}
		batch := &pgx.Batch{}
		var sentCtx context.Context
		pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(ctx context.Context, _ *pgx.Batch) pgx.BatchResults {
			sentCtx = ctx
			return stub
		}).Once()
		ctx := pgcontext.With(context.Background(), pgcontext.WithTimeout(time.Second), pgcontext.WithMetricsLabel("batch"))

		results := New(pool, collector).SendBatch(ctx, batch)
		require.NotNil(t, sentCtx)
		_, deadline := sentCtx.Deadline()
		require.True(t, deadline)
		wrapped, err := results.Query()
		require.NoError(t, err)
		wrapped.Close()
		require.Zero(t, collector.calls)
		require.NoError(t, sentCtx.Err(), "reading a result must not cancel the batch")
		stub.closeFn = func() {
			require.NoError(t, sentCtx.Err(), "Close must drain before cancellation")
		}
		require.NoError(t, results.Close())
		require.NoError(t, results.Close())
		require.Equal(t, 1, stub.closes)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, sentCtx.Err(), context.Canceled)
		require.ErrorIs(t, collector.err, failure)
		labels, ok := pgcontext.MetricsLabelsFrom(collector.ctx)
		require.True(t, ok)
		require.Equal(t, []string{"batch"}, labels)
	})

	t.Run("should be able to retain an explicitly read Rows error", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		failure := errors.New("batch rows failed")
		rows := NewMockRows(t)
		rows.EXPECT().Err().Return(failure).Once()
		stub := &batchStub{rows: rows}
		batch := &pgx.Batch{}
		var sentCtx context.Context
		pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(ctx context.Context, _ *pgx.Batch) pgx.BatchResults {
			sentCtx = ctx
			return stub
		}).Once()
		ctx := pgcontext.With(context.Background(), pgcontext.WithTimeout(time.Second), pgcontext.WithMetricsLabel("batch"))

		results := New(pool, collector).SendBatch(ctx, batch)
		require.NotNil(t, sentCtx)
		_, deadline := sentCtx.Deadline()
		require.True(t, deadline)
		wrapped, err := results.Query()
		require.NoError(t, err)
		require.ErrorIs(t, wrapped.Err(), failure)
		require.Zero(t, collector.calls)
		require.NoError(t, sentCtx.Err(), "reading a result must not cancel the batch")
		stub.closeFn = func() {
			require.NoError(t, sentCtx.Err(), "Close must drain before cancellation")
		}
		require.NoError(t, results.Close())
		require.NoError(t, results.Close())
		require.Equal(t, 1, stub.closes)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, sentCtx.Err(), context.Canceled)
		require.ErrorIs(t, collector.err, failure)
		labels, ok := pgcontext.MetricsLabelsFrom(collector.ctx)
		require.True(t, ok)
		require.Equal(t, []string{"batch"}, labels)
	})

	t.Run("should be able to record and return the same error on repeated batch Close", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		failure := errors.New("batch close failed")
		stub := &batchStub{closeErr: failure}
		batch := &pgx.Batch{}
		var sentCtx context.Context
		pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(ctx context.Context, _ *pgx.Batch) pgx.BatchResults {
			sentCtx = ctx
			return stub
		}).Once()
		ctx := pgcontext.With(context.Background(), pgcontext.WithTimeout(time.Second), pgcontext.WithMetricsLabel("batch"))

		results := New(pool, collector).SendBatch(ctx, batch)
		require.NotNil(t, sentCtx)
		_, deadline := sentCtx.Deadline()
		require.True(t, deadline)

		require.Zero(t, collector.calls)
		require.NoError(t, sentCtx.Err(), "reading a result must not cancel the batch")
		stub.closeFn = func() {
			require.NoError(t, sentCtx.Err(), "Close must drain before cancellation")
		}
		require.ErrorIs(t, results.Close(), failure)
		require.ErrorIs(t, results.Close(), failure)
		require.Equal(t, 1, stub.closes)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, sentCtx.Err(), context.Canceled)
		require.ErrorIs(t, collector.err, failure)
		labels, ok := pgcontext.MetricsLabelsFrom(collector.ctx)
		require.True(t, ok)
		require.Equal(t, []string{"batch"}, labels)
	})
}

func TestBatchTimeoutPassedBeforeSending(t *testing.T) {
	t.Run("should be able to apply an expired timeout before sending the batch", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		ctx := pgcontext.With(context.Background(), pgcontext.WithTimeout(-time.Second))
		stub := &batchStub{closeErr: context.DeadlineExceeded}
		batch := &pgx.Batch{}
		pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(sent context.Context, _ *pgx.Batch) pgx.BatchResults {
			require.ErrorIs(t, sent.Err(), context.DeadlineExceeded)
			return stub
		}).Once()

		results := New(pool, collector).SendBatch(ctx, batch)
		require.ErrorIs(t, results.Close(), context.DeadlineExceeded)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, context.DeadlineExceeded)
	})

	t.Run("should be able to preserve parent cancellation when sending the batch", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		stub := &batchStub{closeErr: context.Canceled}
		batch := &pgx.Batch{}
		pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(sent context.Context, _ *pgx.Batch) pgx.BatchResults {
			require.ErrorIs(t, sent.Err(), context.Canceled)
			return stub
		}).Once()

		results := New(pool, collector).SendBatch(ctx, batch)
		require.ErrorIs(t, results.Close(), context.Canceled)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, context.Canceled)
	})
}

func TestBatchRowsCloseKeepsFollowingResultsUsable(t *testing.T) {
	pool, collector := NewMockPool(t), &batchCollector{}
	rows := NewMockRows(t)
	rows.EXPECT().Close().Once()
	rows.EXPECT().Err().Return(nil).Once()
	stub := &batchStub{rows: rows}
	batch := &pgx.Batch{}
	var sentCtx context.Context
	pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(ctx context.Context, _ *pgx.Batch) pgx.BatchResults { sentCtx = ctx; return stub }).Once()
	results := New(pool, collector).SendBatch(pgcontext.With(context.Background(), pgcontext.WithTimeout(time.Second)), batch)
	wrapped, err := results.Query()
	require.NoError(t, err)
	wrapped.Close()
	require.NoError(t, sentCtx.Err())
	_, err = results.Exec()
	require.NoError(t, err)
	require.Zero(t, collector.calls)
	require.NoError(t, results.Close())
	require.Equal(t, 1, collector.calls)
}

func TestBatchParentDeadlineTakesPrecedence(t *testing.T) {
	pool, collector := NewMockPool(t), &batchCollector{}
	parent, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	deadline, _ := parent.Deadline()
	ctx := pgcontext.With(parent, pgcontext.WithTimeout(time.Hour))
	batch := &pgx.Batch{}
	stub := &batchStub{}
	pool.EXPECT().SendBatch(mock.Anything, batch).RunAndReturn(func(sent context.Context, _ *pgx.Batch) pgx.BatchResults {
		actual, ok := sent.Deadline()
		require.True(t, ok)
		require.Equal(t, deadline, actual)
		return stub
	}).Once()
	require.NoError(t, New(pool, collector).SendBatch(ctx, batch).Close())
	require.NoError(t, parent.Err(), "batch Close must not cancel the parent")
}

func TestBatchMetricsRealErrorReplacesNoRows(t *testing.T) {
	t.Run("should be able to retain ErrNoRows when no other error occurs", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		row := NewMockRow(t)
		row.EXPECT().Scan(mock.Anything).Return(pgx.ErrNoRows).Once()
		stub := &batchStub{row: row}
		batch := &pgx.Batch{}
		pool.EXPECT().SendBatch(mock.Anything, batch).Return(stub).Once()

		results := New(pool, collector).SendBatch(context.Background(), batch)
		var value int
		require.ErrorIs(t, results.QueryRow().Scan(&value), pgx.ErrNoRows)
		require.NoError(t, results.Close())
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, pgx.ErrNoRows)
	})

	t.Run("should be able to replace ErrNoRows with a subsequent execution error", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		failure := errors.New("batch execution failed")
		row := NewMockRow(t)
		row.EXPECT().Scan(mock.Anything).Return(pgx.ErrNoRows).Once()
		stub := &batchStub{row: row, execErr: failure}
		batch := &pgx.Batch{}
		pool.EXPECT().SendBatch(mock.Anything, batch).Return(stub).Once()

		results := New(pool, collector).SendBatch(context.Background(), batch)
		var value int
		require.ErrorIs(t, results.QueryRow().Scan(&value), pgx.ErrNoRows)
		_, err := results.Exec()
		require.ErrorIs(t, err, failure)
		require.NoError(t, results.Close())
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, failure)
	})

	t.Run("should be able to retain an execution error after a subsequent ErrNoRows", func(t *testing.T) {
		pool, collector := NewMockPool(t), &batchCollector{}
		failure := errors.New("batch execution failed")
		row := NewMockRow(t)
		row.EXPECT().Scan(mock.Anything).Return(pgx.ErrNoRows).Once()
		stub := &batchStub{row: row, execErr: failure}
		batch := &pgx.Batch{}
		pool.EXPECT().SendBatch(mock.Anything, batch).Return(stub).Once()

		results := New(pool, collector).SendBatch(context.Background(), batch)
		_, err := results.Exec()
		require.ErrorIs(t, err, failure)
		var value int
		require.ErrorIs(t, results.QueryRow().Scan(&value), pgx.ErrNoRows)
		require.NoError(t, results.Close())
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, failure)
	})
}
