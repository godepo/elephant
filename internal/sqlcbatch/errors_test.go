package sqlcbatch_test

import (
	"context"
	"errors"
	"testing"

	"github.com/godepo/elephant/internal/pkg/failed"
	"github.com/godepo/elephant/internal/sqlcbatch"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

type errorDB struct {
	sqlcbatch.DBTX
	results pgx.BatchResults
}

func (db errorDB) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return db.results }

type scanErrorRows struct {
	pgx.Rows
	err error
}

func (r scanErrorRows) Next() bool        { return true }
func (r scanErrorRows) Scan(...any) error { return r.err }
func (r scanErrorRows) Close()            {}

type scanErrorBatch struct {
	pgx.BatchResults
	err error
}

func (b scanErrorBatch) Query() (pgx.Rows, error) { return scanErrorRows{err: b.err}, nil }
func (b scanErrorBatch) Close() error             { return nil }

func TestGeneratedBatchErrorHandling(t *testing.T) {
	failure := errors.New("dispatch failed")
	queries := sqlcbatch.New(errorDB{results: failed.BatchResults(failure)})
	ctx := context.Background()
	t.Run("exec", func(t *testing.T) {
		result := queries.ExecuteValues(ctx, []int32{1})
		calls := 0
		result.Exec(func(index int, err error) { require.Zero(t, index); require.ErrorIs(t, err, failure); calls++ })
		require.Equal(t, 1, calls)
		require.ErrorIs(t, result.Close(), failure)
	})
	t.Run("one", func(t *testing.T) {
		result := queries.GetValues(ctx, []int32{1})
		calls := 0
		result.QueryRow(func(index int, _ int32, err error) { require.Zero(t, index); require.ErrorIs(t, err, failure); calls++ })
		require.Equal(t, 1, calls)
		require.ErrorIs(t, result.Close(), failure)
	})
	t.Run("many", func(t *testing.T) {
		result := queries.ListValues(ctx, []int32{1})
		calls := 0
		result.Query(func(index int, items []int32, err error) {
			require.Zero(t, index)
			require.Empty(t, items)
			require.ErrorIs(t, err, failure)
			calls++
		})
		require.Equal(t, 1, calls)
		require.ErrorIs(t, result.Close(), failure)
	})
	t.Run("many scan failure", func(t *testing.T) {
		result := sqlcbatch.New(errorDB{results: scanErrorBatch{err: failure}}).ListValues(ctx, []int32{1})
		calls := 0
		result.Query(func(_ int, items []int32, err error) {
			require.Empty(t, items)
			require.ErrorIs(t, err, failure)
			calls++
		})
		require.Equal(t, 1, calls)
		require.NoError(t, result.Close())
	})
}

func TestGeneratedBatchEarlyClose(t *testing.T) {
	ctx := context.Background()
	queries := sqlcbatch.New(errorDB{results: failed.BatchResults(context.Canceled)})
	t.Run("exec", func(t *testing.T) {
		result := queries.ExecuteValues(ctx, []int32{1})
		require.ErrorIs(t, result.Close(), context.Canceled)
		calls := 0
		result.Exec(func(index int, err error) {
			require.Zero(t, index)
			require.ErrorIs(t, err, sqlcbatch.ErrBatchAlreadyClosed)
			calls++
		})
		require.Equal(t, 1, calls)
		result.Exec(nil)
	})
	t.Run("one", func(t *testing.T) {
		result := queries.GetValues(ctx, []int32{1})
		require.ErrorIs(t, result.Close(), context.Canceled)
		calls := 0
		result.QueryRow(func(index int, _ int32, err error) {
			require.Zero(t, index)
			require.ErrorIs(t, err, sqlcbatch.ErrBatchAlreadyClosed)
			calls++
		})
		require.Equal(t, 1, calls)
		result.QueryRow(nil)
	})
	t.Run("many", func(t *testing.T) {
		result := queries.ListValues(ctx, []int32{1})
		require.ErrorIs(t, result.Close(), context.Canceled)
		calls := 0
		result.Query(func(index int, items []int32, err error) {
			require.Zero(t, index)
			require.Empty(t, items)
			require.ErrorIs(t, err, sqlcbatch.ErrBatchAlreadyClosed)
			calls++
		})
		require.Equal(t, 1, calls)
		result.Query(nil)
	})
}
