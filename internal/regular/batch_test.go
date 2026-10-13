package regular

import (
	"context"
	"testing"

	"github.com/godepo/elephant/internal/pkg/failed"
	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestSendBatchRouting(t *testing.T) {
	for _, transaction := range []bool{false, true} {
		t.Run(map[bool]string{false: "pool", true: "transaction"}[transaction], func(t *testing.T) {
			pool, tx := NewMockPool(t), NewMockTx(t)
			ctx := context.Background()
			batch := &pgx.Batch{}
			expected := failed.BatchResults(context.Canceled)
			if transaction {
				ctx = pgcontext.With(ctx, pgcontext.WithTransaction(tx))
				tx.EXPECT().SendBatch(ctx, batch).Return(expected).Once()
			} else {
				pool.EXPECT().SendBatch(ctx, batch).Return(expected).Once()
			}
			require.Equal(t, expected, New(pool).SendBatch(ctx, batch))
		})
	}
}
