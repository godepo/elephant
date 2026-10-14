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
	for _, route := range []string{"id", "key", "transaction"} {
		t.Run(route, func(t *testing.T) {
			pool := NewMockPool(t)
			ctx := context.Background()
			batch := &pgx.Batch{}
			expected := failed.BatchResults(context.Canceled)
			db := New([]Pool{pool}, func(_ context.Context, key string) uint { require.Equal(t, "key", key); return 0 })
			switch route {
			case "id":
				ctx = pgcontext.With(ctx, pgcontext.WithShardID(0))
			case "key":
				ctx = pgcontext.With(ctx, pgcontext.WithShardingKey("key"))
			case "transaction":
				// The selected shard resolves the transaction from ctx, as for other methods.
				ctx = pgcontext.With(ctx, pgcontext.WithShardID(0), pgcontext.WithTransaction(NewMockTx(t)))
			}
			pool.EXPECT().SendBatch(ctx, batch).Return(expected).Once()
			require.Equal(t, expected, db.SendBatch(ctx, batch))
		})
	}
}

func TestSendBatchCouldNotPickShard(t *testing.T) {
	for _, name := range []string{"empty context", "transaction without shard"} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if name == "transaction without shard" {
				ctx = pgcontext.With(ctx, pgcontext.WithTransaction(NewMockTx(t)))
			}
			results := New([]Pool{NewMockPool(t)}, nil).SendBatch(ctx, &pgx.Batch{})
			_, err := results.Exec()
			require.ErrorIs(t, err, ErrCouldNotPickShard)
			rows, err := results.Query()
			require.ErrorIs(t, err, ErrCouldNotPickShard)
			require.ErrorIs(t, rows.Err(), ErrCouldNotPickShard)
			require.ErrorIs(t, results.QueryRow().Scan(), ErrCouldNotPickShard)
			require.ErrorIs(t, results.Close(), ErrCouldNotPickShard)
			require.ErrorIs(t, results.Close(), ErrCouldNotPickShard)
		})
	}
}

func TestSendBatchShardOutOfRangePanics(t *testing.T) {
	db := New([]Pool{NewMockPool(t)}, nil)
	ctx := pgcontext.With(context.Background(), pgcontext.WithShardID(1))
	require.Panics(t, func() { db.SendBatch(ctx, &pgx.Batch{}) }, "an invalid shard ID panics, as for other methods")
}
