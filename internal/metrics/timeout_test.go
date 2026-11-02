package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

type timeoutPool struct {
	Pool
	query    func(context.Context) (pgx.Rows, error)
	queryRow func(context.Context) pgx.Row
	exec     func(context.Context) (pgconn.CommandTag, error)
}

func (p timeoutPool) Query(ctx context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return p.query(ctx)
}
func (p timeoutPool) QueryRow(ctx context.Context, _ string, _ ...any) pgx.Row {
	return p.queryRow(ctx)
}
func (p timeoutPool) Exec(ctx context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return p.exec(ctx)
}

type timeoutRow func(...any) error

func (r timeoutRow) Scan(dest ...any) error { return r(dest...) }

type timeoutRows struct {
	pgx.Rows
	closed int
	next   func() bool
	err    error
}

func (r *timeoutRows) Close()     { r.closed++ }
func (r *timeoutRows) Next() bool { return r.next() }
func (r *timeoutRows) Err() error { return r.err }

type timeoutCollector struct {
	calls  int
	err    error
	ctxErr error
}

func (c *timeoutCollector) TrackQueryMetrics(ctx context.Context, _ time.Time, err error) {
	c.calls++
	c.err = err
	c.ctxErr = ctx.Err()
}

func TestTimeoutStopsUnderlyingOperations(t *testing.T) {
	t.Run("should stop Query with a positive timeout", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(20*time.Millisecond))
		collector := &timeoutCollector{}
		wait := func(actual context.Context) error {
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.WithinDuration(t, time.Now().Add(20*time.Millisecond), deadline, 100*time.Millisecond)
			<-actual.Done()
			require.NoError(t, parent.Err(), "the operation must receive its own timeout")
			return actual.Err()
		}
		pool := timeoutPool{query: func(actual context.Context) (pgx.Rows, error) { return nil, wait(actual) }}
		_, err := New(pool, collector).Query(ctx, "select 1")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, context.DeadlineExceeded)
	})
	t.Run("should stop Query with a zero timeout", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(0))
		collector := &timeoutCollector{}
		wait := func(actual context.Context) error {
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.WithinDuration(t, time.Now().Add(0), deadline, 100*time.Millisecond)
			<-actual.Done()
			require.NoError(t, parent.Err(), "the operation must receive its own timeout")
			return actual.Err()
		}
		pool := timeoutPool{query: func(actual context.Context) (pgx.Rows, error) { return nil, wait(actual) }}
		_, err := New(pool, collector).Query(ctx, "select 1")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, context.DeadlineExceeded)
	})
	t.Run("should stop Query with a negative timeout", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(-time.Second))
		collector := &timeoutCollector{}
		wait := func(actual context.Context) error {
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.WithinDuration(t, time.Now().Add(-time.Second), deadline, 100*time.Millisecond)
			<-actual.Done()
			require.NoError(t, parent.Err(), "the operation must receive its own timeout")
			return actual.Err()
		}
		pool := timeoutPool{query: func(actual context.Context) (pgx.Rows, error) { return nil, wait(actual) }}
		_, err := New(pool, collector).Query(ctx, "select 1")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, context.DeadlineExceeded)
	})
	t.Run("should stop QueryRow with a positive timeout", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(20*time.Millisecond))
		collector := &timeoutCollector{}
		wait := func(actual context.Context) error {
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.WithinDuration(t, time.Now().Add(20*time.Millisecond), deadline, 100*time.Millisecond)
			<-actual.Done()
			require.NoError(t, parent.Err(), "the operation must receive its own timeout")
			return actual.Err()
		}
		pool := timeoutPool{queryRow: func(actual context.Context) pgx.Row { return timeoutRow(func(...any) error { return wait(actual) }) }}
		err := New(pool, collector).QueryRow(ctx, "select 1").Scan()
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, context.DeadlineExceeded)
	})
	t.Run("should stop QueryRow with a zero timeout", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(0))
		collector := &timeoutCollector{}
		wait := func(actual context.Context) error {
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.WithinDuration(t, time.Now().Add(0), deadline, 100*time.Millisecond)
			<-actual.Done()
			require.NoError(t, parent.Err(), "the operation must receive its own timeout")
			return actual.Err()
		}
		pool := timeoutPool{queryRow: func(actual context.Context) pgx.Row { return timeoutRow(func(...any) error { return wait(actual) }) }}
		err := New(pool, collector).QueryRow(ctx, "select 1").Scan()
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, context.DeadlineExceeded)
	})
	t.Run("should stop QueryRow with a negative timeout", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(-time.Second))
		collector := &timeoutCollector{}
		wait := func(actual context.Context) error {
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.WithinDuration(t, time.Now().Add(-time.Second), deadline, 100*time.Millisecond)
			<-actual.Done()
			require.NoError(t, parent.Err(), "the operation must receive its own timeout")
			return actual.Err()
		}
		pool := timeoutPool{queryRow: func(actual context.Context) pgx.Row { return timeoutRow(func(...any) error { return wait(actual) }) }}
		err := New(pool, collector).QueryRow(ctx, "select 1").Scan()
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, context.DeadlineExceeded)
	})
	t.Run("should stop QueryRow dispatch with a positive timeout", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(20*time.Millisecond))
		collector := &timeoutCollector{}
		wait := func(actual context.Context) error {
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.WithinDuration(t, time.Now().Add(20*time.Millisecond), deadline, 100*time.Millisecond)
			<-actual.Done()
			require.NoError(t, parent.Err(), "the operation must receive its own timeout")
			return actual.Err()
		}
		pool := timeoutPool{queryRow: func(actual context.Context) pgx.Row {
			err := wait(actual)
			return timeoutRow(func(...any) error { return err })
		}}
		err := New(pool, collector).QueryRow(ctx, "select 1").Scan()
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, context.DeadlineExceeded)
	})
	t.Run("should stop QueryRow dispatch with a zero timeout", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(0))
		collector := &timeoutCollector{}
		wait := func(actual context.Context) error {
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.WithinDuration(t, time.Now().Add(0), deadline, 100*time.Millisecond)
			<-actual.Done()
			require.NoError(t, parent.Err(), "the operation must receive its own timeout")
			return actual.Err()
		}
		pool := timeoutPool{queryRow: func(actual context.Context) pgx.Row {
			err := wait(actual)
			return timeoutRow(func(...any) error { return err })
		}}
		err := New(pool, collector).QueryRow(ctx, "select 1").Scan()
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, context.DeadlineExceeded)
	})
	t.Run("should stop QueryRow dispatch with a negative timeout", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(-time.Second))
		collector := &timeoutCollector{}
		wait := func(actual context.Context) error {
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.WithinDuration(t, time.Now().Add(-time.Second), deadline, 100*time.Millisecond)
			<-actual.Done()
			require.NoError(t, parent.Err(), "the operation must receive its own timeout")
			return actual.Err()
		}
		pool := timeoutPool{queryRow: func(actual context.Context) pgx.Row {
			err := wait(actual)
			return timeoutRow(func(...any) error { return err })
		}}
		err := New(pool, collector).QueryRow(ctx, "select 1").Scan()
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, context.DeadlineExceeded)
	})
	t.Run("should stop Exec with a positive timeout", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(20*time.Millisecond))
		collector := &timeoutCollector{}
		wait := func(actual context.Context) error {
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.WithinDuration(t, time.Now().Add(20*time.Millisecond), deadline, 100*time.Millisecond)
			<-actual.Done()
			require.NoError(t, parent.Err(), "the operation must receive its own timeout")
			return actual.Err()
		}
		pool := timeoutPool{exec: func(actual context.Context) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, wait(actual) }}
		_, err := New(pool, collector).Exec(ctx, "select 1")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, context.DeadlineExceeded)
	})
	t.Run("should stop Exec with a zero timeout", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(0))
		collector := &timeoutCollector{}
		wait := func(actual context.Context) error {
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.WithinDuration(t, time.Now().Add(0), deadline, 100*time.Millisecond)
			<-actual.Done()
			require.NoError(t, parent.Err(), "the operation must receive its own timeout")
			return actual.Err()
		}
		pool := timeoutPool{exec: func(actual context.Context) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, wait(actual) }}
		_, err := New(pool, collector).Exec(ctx, "select 1")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, context.DeadlineExceeded)
	})
	t.Run("should stop Exec with a negative timeout", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(-time.Second))
		collector := &timeoutCollector{}
		wait := func(actual context.Context) error {
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.WithinDuration(t, time.Now().Add(-time.Second), deadline, 100*time.Millisecond)
			<-actual.Done()
			require.NoError(t, parent.Err(), "the operation must receive its own timeout")
			return actual.Err()
		}
		pool := timeoutPool{exec: func(actual context.Context) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, wait(actual) }}
		_, err := New(pool, collector).Exec(ctx, "select 1")
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 1, collector.calls)
		require.ErrorIs(t, collector.err, context.DeadlineExceeded)
	})
}

func TestTimeoutCancellationLifecycle(t *testing.T) {
	t.Run("should cancel the derived context after Query error and preserve the parent deadline", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		parentDeadline, _ := parent.Deadline()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(2*time.Hour))
		collector := &timeoutCollector{}
		var received context.Context
		capture := func(actual context.Context) {
			received = actual
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.Equal(t, parentDeadline, deadline)
			require.NoError(t, actual.Err())
		}
		expectedErr := errors.New("database failure")
		pool := timeoutPool{query: func(actual context.Context) (pgx.Rows, error) { capture(actual); return nil, expectedErr }}
		rows, err := New(pool, collector).Query(ctx, "select 1")
		require.Nil(t, rows)
		require.NoError(t, collector.ctxErr, "metrics must be recorded before canceling the derived context")
		require.ErrorIs(t, err, expectedErr)
		require.ErrorIs(t, collector.err, expectedErr)
		require.ErrorIs(t, received.Err(), context.Canceled)
		require.NoError(t, parent.Err())
		require.Equal(t, 1, collector.calls)
	})
	t.Run("should cancel the derived context after Query close and preserve the parent deadline", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		parentDeadline, _ := parent.Deadline()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(2*time.Hour))
		collector := &timeoutCollector{}
		var received context.Context
		capture := func(actual context.Context) {
			received = actual
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.Equal(t, parentDeadline, deadline)
			require.NoError(t, actual.Err())
		}
		remaining := 1
		rawRows := &timeoutRows{next: func() bool { remaining--; return remaining >= 0 }}
		pool := timeoutPool{query: func(actual context.Context) (pgx.Rows, error) { capture(actual); return rawRows, nil }}
		rows, err := New(pool, collector).Query(ctx, "select 1")
		require.NoError(t, err)
		require.NoError(t, received.Err())
		require.Zero(t, collector.calls)
		rows.Close()
		rows.Close()
		require.Equal(t, 1, rawRows.closed)
		require.ErrorIs(t, err, nil)
		require.ErrorIs(t, collector.err, nil)
		require.ErrorIs(t, received.Err(), context.Canceled)
		require.NoError(t, parent.Err())
		require.Equal(t, 1, collector.calls)
	})
	t.Run("should cancel the derived context after Query exhausted and preserve the parent deadline", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		parentDeadline, _ := parent.Deadline()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(2*time.Hour))
		collector := &timeoutCollector{}
		var received context.Context
		capture := func(actual context.Context) {
			received = actual
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.Equal(t, parentDeadline, deadline)
			require.NoError(t, actual.Err())
		}
		remaining := 1
		rawRows := &timeoutRows{next: func() bool { remaining--; return remaining >= 0 }}
		pool := timeoutPool{query: func(actual context.Context) (pgx.Rows, error) { capture(actual); return rawRows, nil }}
		rows, err := New(pool, collector).Query(ctx, "select 1")
		require.NoError(t, err)
		require.NoError(t, received.Err())
		require.Zero(t, collector.calls)
		require.True(t, rows.Next())
		require.NoError(t, received.Err())
		require.Zero(t, collector.calls)
		require.False(t, rows.Next())
		rows.Close()
		require.Equal(t, 1, rawRows.closed)
		require.ErrorIs(t, err, nil)
		require.ErrorIs(t, collector.err, nil)
		require.ErrorIs(t, received.Err(), context.Canceled)
		require.NoError(t, parent.Err())
		require.Equal(t, 1, collector.calls)
	})
	t.Run("should cancel the derived context after QueryRow success and preserve the parent deadline", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		parentDeadline, _ := parent.Deadline()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(2*time.Hour))
		collector := &timeoutCollector{}
		var received context.Context
		capture := func(actual context.Context) {
			received = actual
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.Equal(t, parentDeadline, deadline)
			require.NoError(t, actual.Err())
		}
		pool := timeoutPool{queryRow: func(actual context.Context) pgx.Row {
			capture(actual)
			return timeoutRow(func(...any) error { require.NoError(t, actual.Err()); return nil })
		}}
		row := New(pool, collector).QueryRow(ctx, "select 1")
		require.NoError(t, received.Err())
		require.Zero(t, collector.calls)
		err := row.Scan()
		require.ErrorIs(t, err, nil)
		require.ErrorIs(t, collector.err, nil)
		require.ErrorIs(t, received.Err(), context.Canceled)
		require.NoError(t, parent.Err())
		require.Equal(t, 1, collector.calls)
	})
	t.Run("should cancel the derived context after QueryRow error and preserve the parent deadline", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		parentDeadline, _ := parent.Deadline()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(2*time.Hour))
		collector := &timeoutCollector{}
		var received context.Context
		capture := func(actual context.Context) {
			received = actual
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.Equal(t, parentDeadline, deadline)
			require.NoError(t, actual.Err())
		}
		expectedErr := errors.New("database failure")
		pool := timeoutPool{queryRow: func(actual context.Context) pgx.Row {
			capture(actual)
			return timeoutRow(func(...any) error { require.NoError(t, actual.Err()); return expectedErr })
		}}
		row := New(pool, collector).QueryRow(ctx, "select 1")
		require.NoError(t, received.Err())
		require.Zero(t, collector.calls)
		err := row.Scan()
		require.ErrorIs(t, err, expectedErr)
		require.ErrorIs(t, collector.err, expectedErr)
		require.ErrorIs(t, received.Err(), context.Canceled)
		require.NoError(t, parent.Err())
		require.Equal(t, 1, collector.calls)
	})
	t.Run("should cancel the derived context after Exec success and preserve the parent deadline", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		parentDeadline, _ := parent.Deadline()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(2*time.Hour))
		collector := &timeoutCollector{}
		var received context.Context
		capture := func(actual context.Context) {
			received = actual
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.Equal(t, parentDeadline, deadline)
			require.NoError(t, actual.Err())
		}
		pool := timeoutPool{exec: func(actual context.Context) (pgconn.CommandTag, error) {
			capture(actual)
			return pgconn.NewCommandTag("UPDATE 1"), nil
		}}
		tag, err := New(pool, collector).Exec(ctx, "update items")
		require.EqualValues(t, 1, tag.RowsAffected())
		require.ErrorIs(t, err, nil)
		require.ErrorIs(t, collector.err, nil)
		require.ErrorIs(t, received.Err(), context.Canceled)
		require.NoError(t, parent.Err())
		require.Equal(t, 1, collector.calls)
	})
	t.Run("should cancel the derived context after Exec error and preserve the parent deadline", func(t *testing.T) {
		parent, cancel := context.WithTimeout(context.Background(), time.Hour)
		defer cancel()
		parentDeadline, _ := parent.Deadline()
		ctx := pgcontext.With(parent, pgcontext.WithTimeout(2*time.Hour))
		collector := &timeoutCollector{}
		var received context.Context
		capture := func(actual context.Context) {
			received = actual
			deadline, ok := actual.Deadline()
			require.True(t, ok)
			require.Equal(t, parentDeadline, deadline)
			require.NoError(t, actual.Err())
		}
		expectedErr := errors.New("database failure")
		pool := timeoutPool{exec: func(actual context.Context) (pgconn.CommandTag, error) {
			capture(actual)
			return pgconn.NewCommandTag("UPDATE 1"), expectedErr
		}}
		_, err := New(pool, collector).Exec(ctx, "update items")
		require.ErrorIs(t, err, expectedErr)
		require.ErrorIs(t, collector.err, expectedErr)
		require.ErrorIs(t, received.Err(), context.Canceled)
		require.NoError(t, parent.Err())
		require.Equal(t, 1, collector.calls)
	})
}
