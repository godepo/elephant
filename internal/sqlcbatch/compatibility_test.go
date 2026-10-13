package sqlcbatch_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/godepo/elephant"
	"github.com/godepo/elephant/clusterpg"
	"github.com/godepo/elephant/internal/sharded"
	"github.com/godepo/elephant/internal/sqlcbatch"
	"github.com/godepo/elephant/metrics"
	"github.com/godepo/elephant/singlepg"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

var (
	_ sqlcbatch.DBTX = (singlepg.DB)(nil)
	_ sqlcbatch.DBTX = (clusterpg.Pool)(nil)
	_ sqlcbatch.DBTX = (*sharded.Hive)(nil)
	_ sqlcbatch.DBTX = (*pgxpool.Pool)(nil)
)

type collector struct {
	calls int
	err   error
}

func (c *collector) TrackQueryMetrics(_ context.Context, _ time.Time, err error) {
	c.calls++
	c.err = err
}

func TestSQLCBatches(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:16", postgres.WithDatabase("elephant"), postgres.WithUsername("postgres"), postgres.WithPassword("postgres"), postgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	connection, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	config, err := pgxpool.ParseConfig(connection)
	require.NoError(t, err)
	config.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	base := singlepg.New(pool)
	col := &collector{}
	db := metrics.New(base, col)
	queries := sqlcbatch.New(db)

	t.Run("generated annotations", func(t *testing.T) {
		before := col.calls
		executed := 0
		batchExec := queries.ExecuteValues(ctx, []int32{10, 20})
		batchExec.Exec(func(index int, err error) { require.NoError(t, err); require.Equal(t, executed, index); executed++ })
		require.Equal(t, 2, executed)
		require.NoError(t, batchExec.Close())
		values := []int32{}
		batchOne := queries.GetValues(ctx, []int32{10, 20})
		batchOne.QueryRow(func(index int, value int32, err error) {
			require.NoError(t, err)
			require.Equal(t, len(values), index)
			values = append(values, value)
		})
		require.Equal(t, []int32{10, 20}, values)
		require.NoError(t, batchOne.Close())
		groups := [][]int32{}
		batchMany := queries.ListValues(ctx, []int32{2, 3})
		batchMany.Query(func(index int, items []int32, err error) {
			require.NoError(t, err)
			require.Equal(t, len(groups), index)
			groups = append(groups, items)
		})
		require.Equal(t, [][]int32{{1, 2}, {1, 2, 3}}, groups)
		require.NoError(t, batchMany.Close())
		require.Equal(t, before+3, col.calls)
		require.NoError(t, col.err)
		require.Zero(t, pool.Stat().AcquiredConns())
	})

	t.Run("transaction context", func(t *testing.T) {
		err := db.Transactional(ctx, func(txCtx context.Context) error {
			tx, ok := elephant.TransactionFrom(txCtx)
			require.True(t, ok)
			var expectedPID int32
			require.NoError(t, tx.QueryRow(txCtx, "SELECT pg_backend_pid()").Scan(&expectedPID))
			batch := &pgx.Batch{}
			batch.Queue("SELECT pg_backend_pid()")
			batch.Queue("SELECT pg_backend_pid()")
			results := db.SendBatch(txCtx, batch)
			for range 2 {
				var pid int32
				require.NoError(t, results.QueryRow().Scan(&pid))
				require.Equal(t, expectedPID, pid)
			}
			require.NoError(t, results.Close())
			generated := queries.GetValues(txCtx, []int32{7, 8})
			count := 0
			generated.QueryRow(func(_ int, value int32, err error) {
				require.NoError(t, err)
				require.GreaterOrEqual(t, value, int32(7))
				count++
			})
			require.Equal(t, 2, count)
			return generated.Close()
		})
		require.NoError(t, err)
	})

	t.Run("sqlc WithTx", func(t *testing.T) {
		tx, err := db.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		result := queries.WithTx(tx).GetValues(ctx, []int32{9})
		result.QueryRow(func(_ int, value int32, err error) { require.NoError(t, err); require.Equal(t, int32(9), value) })
		require.NoError(t, result.Close())
		require.NoError(t, tx.Commit(ctx))
	})

	t.Run("callbacks and drain", func(t *testing.T) {
		before := col.calls
		batch := &pgx.Batch{}
		calls := 0
		batch.Queue("SELECT 1").Exec(func(tag pgconn.CommandTag) error { calls++; require.EqualValues(t, 1, tag.RowsAffected()); return nil })
		batch.Queue("SELECT 2").Query(func(rows pgx.Rows) error {
			calls++
			require.True(t, rows.Next())
			var value int
			require.NoError(t, rows.Scan(&value))
			require.Equal(t, 2, value)
			return nil
		})
		batch.Queue("SELECT 3").QueryRow(func(row pgx.Row) error {
			calls++
			var value int
			require.NoError(t, row.Scan(&value))
			require.Equal(t, 3, value)
			return nil
		})
		result := db.SendBatch(ctx, batch)
		require.NoError(t, result.Close())
		require.NoError(t, result.Close())
		require.Equal(t, 3, calls)
		require.Equal(t, before+1, col.calls)
		require.NoError(t, col.err)
	})

	t.Run("callback error", func(t *testing.T) {
		failure := errors.New("callback failed")
		batch := &pgx.Batch{}
		batch.Queue("SELECT 1").Exec(func(pgconn.CommandTag) error { return failure })
		batch.Queue("SELECT 2").Exec(func(pgconn.CommandTag) error { t.Error("callback after error"); return nil })
		result := db.SendBatch(ctx, batch)
		require.ErrorIs(t, result.Close(), failure)
		require.ErrorIs(t, result.Close(), failure)
		require.ErrorIs(t, col.err, failure)
	})

	t.Run("unread SQL error", func(t *testing.T) {
		batch := &pgx.Batch{}
		batch.Queue("SELECT 1")
		batch.Queue("SELECT 1 / $1::integer", 0)
		result := db.SendBatch(ctx, batch)
		_, err := result.Exec()
		require.NoError(t, err)
		err = result.Close()
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr)
		require.Equal(t, "22012", pgErr.Code)
		require.ErrorIs(t, col.err, err)
		require.ErrorIs(t, result.Close(), err)
	})

	t.Run("no rows preserved in metrics", func(t *testing.T) {
		batch := &pgx.Batch{}
		batch.Queue("SELECT 1 WHERE false")
		batch.Queue("SELECT 2")
		result := db.SendBatch(ctx, batch)
		var value int
		require.ErrorIs(t, result.QueryRow().Scan(&value), pgx.ErrNoRows)
		require.NoError(t, result.QueryRow().Scan(&value))
		require.Equal(t, 2, value)
		require.NoError(t, result.Close())
		require.ErrorIs(t, col.err, pgx.ErrNoRows)
	})

	t.Run("timeout during query", func(t *testing.T) {
		timeoutCtx := elephant.With(ctx, elephant.WithTimeout(20*time.Millisecond))
		batch := &pgx.Batch{}
		batch.Queue("SELECT pg_sleep(1)")
		result := db.SendBatch(timeoutCtx, batch)
		_, err := result.Exec()
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Error(t, result.Close())
		require.ErrorIs(t, col.err, context.DeadlineExceeded)
	})

	t.Run("rows close leaves batch alive", func(t *testing.T) {
		batch := &pgx.Batch{}
		batch.Queue("SELECT 1")
		batch.Queue("SELECT 2")
		result := db.SendBatch(elephant.With(ctx, elephant.WithTimeout(time.Second)), batch)
		rows, err := result.Query()
		require.NoError(t, err)
		rows.Close()
		var value int
		require.NoError(t, result.QueryRow().Scan(&value))
		require.Equal(t, 2, value)
		require.NoError(t, result.Close())
	})

	t.Run("empty batch", func(t *testing.T) {
		before := col.calls
		result := queries.ExecuteValues(ctx, nil)
		result.Exec(nil)
		require.NoError(t, result.Close())
		require.Equal(t, before+1, col.calls)
	})
	require.Zero(t, pool.Stat().AcquiredConns())
	require.NoError(t, pool.Ping(ctx))
}
