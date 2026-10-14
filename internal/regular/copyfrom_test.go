package regular

import (
	"context"
	"errors"
	"testing"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestCopyFromRouting(t *testing.T) {
	for _, transaction := range []bool{false, true} {
		t.Run(map[bool]string{false: "pool", true: "transaction"}[transaction], func(t *testing.T) {
			pool, tx := NewMockPool(t), NewMockTx(t)
			ctx := context.Background()
			table, columns, source := pgx.Identifier{"values"}, []string{"value"}, pgx.CopyFromRows([][]any{{1}})
			if transaction {
				ctx = pgcontext.With(ctx, pgcontext.WithTransaction(tx))
				tx.EXPECT().CopyFrom(ctx, table, columns, source).Return(1, nil).Once()
			} else {
				pool.EXPECT().CopyFrom(ctx, table, columns, source).Return(1, nil).Once()
			}
			copied, err := New(pool).CopyFrom(ctx, table, columns, source)
			require.NoError(t, err)
			require.EqualValues(t, 1, copied)
		})
	}
}

func TestCopyFromError(t *testing.T) {
	pool := NewMockPool(t)
	failure := errors.New("copy failed")
	pool.EXPECT().CopyFrom(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(1, failure).Once()
	copied, err := New(pool).CopyFrom(context.Background(), pgx.Identifier{"values"}, nil, pgx.CopyFromRows(nil))
	require.ErrorIs(t, err, failure)
	require.Zero(t, copied)
}
