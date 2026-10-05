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
	calls int
	err   error
}

func (c *timeoutCollector) TrackQueryMetrics(_ context.Context, _ time.Time, err error) {
	c.calls++
	c.err = err
}

func TestTimeoutStopsUnderlyingOperations(t *testing.T) {
	for _, operation := range []string{"Query", "QueryRow", "QueryRow dispatch", "Exec"} {
		for _, duration := range []time.Duration{20 * time.Millisecond, 0, -time.Second} {
			t.Run(operation+"/"+duration.String(), func(t *testing.T) {
				ctx := pgcontext.With(context.Background(), pgcontext.WithTimeout(duration))
				collector := &timeoutCollector{}
				wait := func(actual context.Context) error {
					deadline, ok := actual.Deadline()
					require.True(t, ok)
					require.WithinDuration(t, time.Now().Add(duration), deadline, 100*time.Millisecond)
					select {
					case <-actual.Done():
						return actual.Err()
					case <-time.After(time.Second):
						t.Fatal("underlying operation did not receive cancellation")
						return nil
					}
				}
				pool := timeoutPool{
					query: func(actual context.Context) (pgx.Rows, error) { return nil, wait(actual) },
					queryRow: func(actual context.Context) pgx.Row {
						if operation == "QueryRow dispatch" {
							err := wait(actual)
							return timeoutRow(func(...any) error { return err })
						}
						return timeoutRow(func(...any) error { return wait(actual) })
					},
					exec: func(actual context.Context) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, wait(actual) },
				}
				db := New(pool, collector)
				var err error
				switch operation {
				case "Query":
					_, err = db.Query(ctx, "select 1")
				case "QueryRow", "QueryRow dispatch":
					err = db.QueryRow(ctx, "select 1").Scan()
				case "Exec":
					_, err = db.Exec(ctx, "select 1")
				}
				require.ErrorIs(t, err, context.DeadlineExceeded)
				require.Equal(t, 1, collector.calls)
				require.ErrorIs(t, collector.err, context.DeadlineExceeded)
			})
		}
	}
}

func TestTimeoutCancellationLifecycle(t *testing.T) {
	expectedErr := errors.New("database failure")
	for _, operation := range []string{"Query error", "Query close", "Query exhausted", "QueryRow success", "QueryRow error", "Exec success", "Exec error"} {
		t.Run(operation, func(t *testing.T) {
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
			rawRows := &timeoutRows{next: func() bool {
				remaining--
				return remaining >= 0
			}}
			pool := timeoutPool{
				query: func(actual context.Context) (pgx.Rows, error) {
					capture(actual)
					if operation == "Query error" {
						return nil, expectedErr
					}
					return rawRows, nil
				},
				queryRow: func(actual context.Context) pgx.Row {
					capture(actual)
					return timeoutRow(func(...any) error {
						require.NoError(t, actual.Err())
						if operation == "QueryRow error" {
							return expectedErr
						}
						return nil
					})
				},
				exec: func(actual context.Context) (pgconn.CommandTag, error) {
					capture(actual)
					if operation == "Exec error" {
						return pgconn.CommandTag{}, expectedErr
					}
					return pgconn.NewCommandTag("UPDATE 1"), nil
				},
			}
			db := New(pool, collector)
			var err error
			switch operation {
			case "Query error", "Query close", "Query exhausted":
				var rows pgx.Rows
				rows, err = db.Query(ctx, "select 1")
				if err == nil {
					require.NoError(t, received.Err())
					require.Zero(t, collector.calls)
					if operation == "Query exhausted" {
						require.True(t, rows.Next())
						require.NoError(t, received.Err())
						require.Zero(t, collector.calls)
						require.False(t, rows.Next())
					} else {
						rows.Close()
					}
					rows.Close()
					require.Equal(t, 1, rawRows.closed)
				} else {
					require.Nil(t, rows)
				}
			case "QueryRow success", "QueryRow error":
				row := db.QueryRow(ctx, "select 1")
				require.NoError(t, received.Err())
				require.Zero(t, collector.calls)
				err = row.Scan()
			default:
				var tag pgconn.CommandTag
				tag, err = db.Exec(ctx, "update items")
				if err == nil {
					require.EqualValues(t, 1, tag.RowsAffected())
				}
			}
			if operation == "Query error" || operation == "QueryRow error" || operation == "Exec error" {
				require.ErrorIs(t, err, expectedErr)
				require.ErrorIs(t, collector.err, expectedErr)
			} else {
				require.NoError(t, err)
				require.NoError(t, collector.err)
			}
			require.ErrorIs(t, received.Err(), context.Canceled)
			require.NoError(t, parent.Err())
			require.Equal(t, 1, collector.calls)
		})
	}
}
