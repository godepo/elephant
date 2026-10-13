package sharded

import (
	"context"
	"testing"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestCopyFromRouting(t *testing.T) {
	for _, route := range []string{"id", "key", "transaction"} {
		t.Run(route, func(t *testing.T) {
			pool := NewMockPool(t)
			ctx := context.Background()
			table, columns, source := pgx.Identifier{"values"}, []string{"value"}, pgx.CopyFromRows([][]any{{1}})
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
			pool.EXPECT().CopyFrom(ctx, table, columns, source).Return(1, nil).Once()
			copied, err := db.CopyFrom(ctx, table, columns, source)
			require.NoError(t, err)
			require.EqualValues(t, 1, copied)
		})
	}
}

func TestCopyFromCouldNotPickShard(t *testing.T) {
	for _, name := range []string{"empty context", "transaction without shard"} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			if name == "transaction without shard" {
				ctx = pgcontext.With(ctx, pgcontext.WithTransaction(NewMockTx(t)))
			}
			copied, err := New([]Pool{NewMockPool(t)}, nil).CopyFrom(ctx, pgx.Identifier{"values"}, nil, pgx.CopyFromRows(nil))
			require.ErrorIs(t, err, ErrCouldNotPickShard)
			require.Zero(t, copied)
		})
	}
}

func TestCopyFromShardOutOfRangePanics(t *testing.T) {
	db := New([]Pool{NewMockPool(t)}, nil)
	ctx := pgcontext.With(context.Background(), pgcontext.WithShardID(1))
	require.Panics(t, func() {
		_, _ = db.CopyFrom(ctx, pgx.Identifier{"values"}, nil, pgx.CopyFromRows(nil))
	}, "an invalid shard ID panics, as for other methods")
}
