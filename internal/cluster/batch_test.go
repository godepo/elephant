package cluster

import (
	"context"
	"testing"

	"github.com/godepo/elephant/internal/pkg/batchresults"
	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestSendBatchRouting(t *testing.T) {
	for _, route := range []string{"follower", "leader", "transaction"} {
		t.Run(route, func(t *testing.T) {
			leader, follower, tx := NewMockPool(t), NewMockPool(t), NewMockTx(t)
			db := New(leader, []Pool{follower})
			ctx := context.Background()
			batch := &pgx.Batch{}
			batch.Queue("SELECT 1")
			batch.Queue("SELECT 2")
			expected := batchresults.Failed(context.Canceled)
			switch route {
			case "leader":
				ctx = pgcontext.WithCanWrite(ctx)
				leader.EXPECT().SendBatch(ctx, batch).Return(expected).Once()
			case "transaction":
				ctx = pgcontext.With(ctx, pgcontext.WithTransaction(tx))
				tx.EXPECT().SendBatch(ctx, batch).Return(expected).Once()
			default:
				follower.EXPECT().SendBatch(ctx, batch).Return(expected).Once()
			}
			require.Equal(t, expected, db.SendBatch(ctx, batch))
		})
	}
}
