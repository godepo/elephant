package sharded

import (
	"context"
	"testing"

	"github.com/godepo/elephant/internal/pkg/batchresults"
	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestSendBatchRouting(t *testing.T) {
	for _, route := range []string{"id", "key", "transaction", "missing", "out of range"} {
		t.Run(route, func(t *testing.T) {
			pool, tx := NewMockPool(t), NewMockTx(t)
			ctx := context.Background()
			batch := &pgx.Batch{}
			expected := batchresults.Failed(context.Canceled)
			db := New([]Pool{pool}, func(ctx context.Context, key string) uint { require.Equal(t, "key", key); return 0 })
			switch route {
			case "id":
				ctx = pgcontext.With(ctx, pgcontext.WithShardID(0))
			case "key":
				ctx = pgcontext.With(ctx, pgcontext.WithShardingKey("key"))
			case "transaction":
				ctx = pgcontext.With(ctx, pgcontext.WithTransaction(tx))
			case "out of range":
				ctx = pgcontext.With(ctx, pgcontext.WithShardID(1))
			}
			if route == "transaction" {
				tx.EXPECT().SendBatch(ctx, batch).Return(expected).Once()
			} else if route == "id" || route == "key" {
				pool.EXPECT().SendBatch(ctx, batch).Return(expected).Once()
			}
			results := db.SendBatch(ctx, batch)
			if route == "missing" || route == "out of range" {
				_, err := results.Exec()
				require.ErrorIs(t, err, ErrCouldNotPickShard)
				_, err = results.Query()
				require.ErrorIs(t, err, ErrCouldNotPickShard)
				require.ErrorIs(t, results.QueryRow().Scan(), ErrCouldNotPickShard)
				require.ErrorIs(t, results.Close(), ErrCouldNotPickShard)
				require.ErrorIs(t, results.Close(), ErrCouldNotPickShard)
			} else {
				require.Equal(t, expected, results)
			}
		})
	}
}
