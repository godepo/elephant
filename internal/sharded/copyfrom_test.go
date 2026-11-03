package sharded

import (
	"context"
	"testing"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestCopyFromRouting(t *testing.T) {
	t.Run("should be able to copy rows to the shard selected by ID", func(t *testing.T) {
		other, selected := NewMockPool(t), NewMockPool(t)
		db := New([]Pool{other, selected}, nil)
		ctx := pgcontext.With(context.Background(), pgcontext.WithShardID(1))
		table, columns, source := pgx.Identifier{"values"}, []string{"value"}, pgx.CopyFromRows([][]any{{1}})
		selected.EXPECT().CopyFrom(ctx, table, columns, source).Return(1, nil).Once()

		copied, err := db.CopyFrom(ctx, table, columns, source)
		require.NoError(t, err)
		require.EqualValues(t, 1, copied)
	})

	t.Run("should be able to copy rows to the shard selected by key", func(t *testing.T) {
		other, selected := NewMockPool(t), NewMockPool(t)
		ctx := pgcontext.With(context.Background(), pgcontext.WithShardingKey("key"))
		db := New([]Pool{other, selected}, func(actual context.Context, key string) uint {
			require.Equal(t, ctx, actual)
			require.Equal(t, "key", key)
			return 1
		})
		table, columns, source := pgx.Identifier{"values"}, []string{"value"}, pgx.CopyFromRows([][]any{{1}})
		selected.EXPECT().CopyFrom(ctx, table, columns, source).Return(1, nil).Once()

		copied, err := db.CopyFrom(ctx, table, columns, source)
		require.NoError(t, err)
		require.EqualValues(t, 1, copied)
	})

	t.Run("should be able to pass the transaction context to the selected shard", func(t *testing.T) {
		other, selected := NewMockPool(t), NewMockPool(t)
		db := New([]Pool{other, selected}, nil)
		ctx := pgcontext.With(context.Background(), pgcontext.WithShardID(1), pgcontext.WithTransaction(NewMockTx(t)))
		table, columns, source := pgx.Identifier{"values"}, []string{"value"}, pgx.CopyFromRows([][]any{{1}})
		selected.EXPECT().CopyFrom(ctx, table, columns, source).Return(1, nil).Once()

		copied, err := db.CopyFrom(ctx, table, columns, source)
		require.NoError(t, err)
		require.EqualValues(t, 1, copied)
	})
}

func TestCopyFromCouldNotPickShard(t *testing.T) {
	t.Run("should report no shard selected", func(t *testing.T) {
		ctx := pgcontext.With(context.Background())
		copied, err := New([]Pool{NewMockPool(t)}, nil).CopyFrom(ctx, pgx.Identifier{"values"}, nil, pgx.CopyFromRows(nil))
		require.ErrorIs(t, err, ErrCouldNotPickShard)
		require.Zero(t, copied)
	})
	t.Run("should report transaction without a shard", func(t *testing.T) {
		ctx := pgcontext.With(context.Background(), pgcontext.WithTransaction(NewMockTx(t)))
		copied, err := New([]Pool{NewMockPool(t)}, nil).CopyFrom(ctx, pgx.Identifier{"values"}, nil, pgx.CopyFromRows(nil))
		require.ErrorIs(t, err, ErrCouldNotPickShard)
		require.Zero(t, copied)
	})
}

func TestCopyFromShardOutOfRangePanics(t *testing.T) {
	db := New([]Pool{NewMockPool(t)}, nil)
	ctx := pgcontext.With(context.Background(), pgcontext.WithShardID(1))
	require.Panics(t, func() {
		_, _ = db.CopyFrom(ctx, pgx.Identifier{"values"}, nil, pgx.CopyFromRows(nil))
	}, "an invalid shard ID panics, as for other methods")
}
