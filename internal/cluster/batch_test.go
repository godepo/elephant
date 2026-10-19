package cluster

import (
	"context"
	"testing"

	"github.com/godepo/elephant/internal/pkg/failed"
	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestSendBatchRouting(t *testing.T) {
	t.Run("should be able to send a read batch to a follower", func(t *testing.T) {
		leader, follower := NewMockPool(t), NewMockPool(t)
		db := New(leader, []Pool{follower})
		ctx := context.Background()
		batch := &pgx.Batch{}
		batch.Queue("SELECT 1")
		batch.Queue("SELECT 2")
		expected := failed.BatchResults(context.Canceled)
		follower.EXPECT().SendBatch(ctx, batch).Return(expected).Once()

		require.Equal(t, expected, db.SendBatch(ctx, batch))
	})

	t.Run("should be able to send the entire batch to the leader with write permission", func(t *testing.T) {
		leader, follower := NewMockPool(t), NewMockPool(t)
		db := New(leader, []Pool{follower})
		ctx := pgcontext.WithCanWrite(context.Background())
		batch := &pgx.Batch{}
		batch.Queue("INSERT INTO numbers (value) VALUES ($1)", 1)
		batch.Queue("SELECT value FROM numbers")
		expected := failed.BatchResults(context.Canceled)
		leader.EXPECT().SendBatch(ctx, batch).Return(expected).Once()

		require.Equal(t, expected, db.SendBatch(ctx, batch))
	})

	t.Run("should be able to send a batch using the transaction from context", func(t *testing.T) {
		leader, follower, tx := NewMockPool(t), NewMockPool(t), NewMockTx(t)
		db := New(leader, []Pool{follower})
		ctx := pgcontext.With(context.Background(), pgcontext.WithTransaction(tx))
		batch := &pgx.Batch{}
		batch.Queue("SELECT 1")
		batch.Queue("SELECT 2")
		expected := failed.BatchResults(context.Canceled)
		tx.EXPECT().SendBatch(ctx, batch).Return(expected).Once()

		require.Equal(t, expected, db.SendBatch(ctx, batch))
	})
}
