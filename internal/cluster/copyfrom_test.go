package cluster

import (
	"context"
	"testing"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestCopyFromRouting(t *testing.T) {
	t.Run("should copy through the follower", func(t *testing.T) {
		leader, follower := NewMockPool(t), NewMockPool(t)
		db := New(leader, []Pool{follower})
		ctx := context.Background()
		table, columns, source := pgx.Identifier{"values"}, []string{"value"}, pgx.CopyFromRows([][]any{{1}})
		follower.EXPECT().CopyFrom(ctx, table, columns, source).Return(1, nil).Once()
		copied, err := db.CopyFrom(ctx, table, columns, source)
		require.NoError(t, err)
		require.EqualValues(t, 1, copied)
	})
	t.Run("should copy through the leader", func(t *testing.T) {
		leader, follower := NewMockPool(t), NewMockPool(t)
		db := New(leader, []Pool{follower})
		ctx := context.Background()
		ctx = pgcontext.WithCanWrite(ctx)
		table, columns, source := pgx.Identifier{"values"}, []string{"value"}, pgx.CopyFromRows([][]any{{1}})
		leader.EXPECT().CopyFrom(ctx, table, columns, source).Return(1, nil).Once()
		copied, err := db.CopyFrom(ctx, table, columns, source)
		require.NoError(t, err)
		require.EqualValues(t, 1, copied)
	})
	t.Run("should copy through the transaction", func(t *testing.T) {
		leader, follower := NewMockPool(t), NewMockPool(t)
		db := New(leader, []Pool{follower})
		ctx := context.Background()
		tx := NewMockTx(t)
		ctx = pgcontext.With(ctx, pgcontext.WithTransaction(tx))
		table, columns, source := pgx.Identifier{"values"}, []string{"value"}, pgx.CopyFromRows([][]any{{1}})
		tx.EXPECT().CopyFrom(ctx, table, columns, source).Return(1, nil).Once()
		copied, err := db.CopyFrom(ctx, table, columns, source)
		require.NoError(t, err)
		require.EqualValues(t, 1, copied)
	})
}
