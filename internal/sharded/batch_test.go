package sharded

import (
	"context"
	"testing"

	"github.com/godepo/elephant/internal/pkg/failed"
	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestSendBatchRouting(t *testing.T) {
	t.Run("should be able to send a batch to the shard selected by ID", func(t *testing.T) {
		other, selected := NewMockPool(t), NewMockPool(t)
		db := New([]Pool{other, selected}, nil)
		ctx := pgcontext.With(context.Background(), pgcontext.WithShardID(1))
		batch := &pgx.Batch{}
		batch.Queue("SELECT 1")
		batch.Queue("SELECT 2")
		expected := failed.BatchResults(context.Canceled)
		selected.EXPECT().SendBatch(ctx, batch).Return(expected).Once()

		require.Equal(t, expected, db.SendBatch(ctx, batch))
	})

	t.Run("should be able to send a batch to the shard selected by key", func(t *testing.T) {
		other, selected := NewMockPool(t), NewMockPool(t)
		ctx := pgcontext.With(context.Background(), pgcontext.WithShardingKey("key"))
		db := New([]Pool{other, selected}, func(actual context.Context, key string) uint {
			require.Equal(t, ctx, actual)
			require.Equal(t, "key", key)
			return 1
		})
		batch := &pgx.Batch{}
		batch.Queue("SELECT 1")
		batch.Queue("SELECT 2")
		expected := failed.BatchResults(context.Canceled)
		selected.EXPECT().SendBatch(ctx, batch).Return(expected).Once()

		require.Equal(t, expected, db.SendBatch(ctx, batch))
	})

	t.Run("should be able to pass the transaction context to the selected shard", func(t *testing.T) {
		other, selected := NewMockPool(t), NewMockPool(t)
		db := New([]Pool{other, selected}, nil)
		ctx := pgcontext.With(context.Background(), pgcontext.WithShardID(1), pgcontext.WithTransaction(NewMockTx(t)))
		batch := &pgx.Batch{}
		batch.Queue("SELECT 1")
		batch.Queue("SELECT 2")
		expected := failed.BatchResults(context.Canceled)
		selected.EXPECT().SendBatch(ctx, batch).Return(expected).Once()

		require.Equal(t, expected, db.SendBatch(ctx, batch))
	})
}

func TestSendBatchCouldNotPickShard(t *testing.T) {
	t.Run("should be able to report a missing shard through every batch result method", func(t *testing.T) {
		db := New([]Pool{NewMockPool(t)}, nil)

		results := db.SendBatch(context.Background(), &pgx.Batch{})
		_, err := results.Exec()
		require.ErrorIs(t, err, ErrCouldNotPickShard)
		rows, err := results.Query()
		require.ErrorIs(t, err, ErrCouldNotPickShard)
		require.NotNil(t, rows)
		require.ErrorIs(t, rows.Err(), ErrCouldNotPickShard)
		require.ErrorIs(t, results.QueryRow().Scan(), ErrCouldNotPickShard)
		require.ErrorIs(t, results.Close(), ErrCouldNotPickShard)
		require.ErrorIs(t, results.Close(), ErrCouldNotPickShard)
	})

	t.Run("should be able to report a missing shard even when a transaction is in context", func(t *testing.T) {
		db := New([]Pool{NewMockPool(t)}, nil)
		ctx := pgcontext.With(context.Background(), pgcontext.WithTransaction(NewMockTx(t)))

		results := db.SendBatch(ctx, &pgx.Batch{})
		_, err := results.Exec()
		require.ErrorIs(t, err, ErrCouldNotPickShard)
		rows, err := results.Query()
		require.ErrorIs(t, err, ErrCouldNotPickShard)
		require.NotNil(t, rows)
		require.ErrorIs(t, rows.Err(), ErrCouldNotPickShard)
		require.ErrorIs(t, results.QueryRow().Scan(), ErrCouldNotPickShard)
		require.ErrorIs(t, results.Close(), ErrCouldNotPickShard)
		require.ErrorIs(t, results.Close(), ErrCouldNotPickShard)
	})
}

func TestSendBatchShardOutOfRangePanics(t *testing.T) {
	db := New([]Pool{NewMockPool(t)}, nil)
	ctx := pgcontext.With(context.Background(), pgcontext.WithShardID(1))
	require.Panics(t, func() { db.SendBatch(ctx, &pgx.Batch{}) }, "an invalid shard ID panics, as for other methods")
}
