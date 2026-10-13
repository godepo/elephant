package cluster

import (
	"context"
	"testing"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestCopyFromRouting(t *testing.T) {
	for _, route := range []string{"follower", "leader", "transaction"} {
		t.Run(route, func(t *testing.T) {
			leader, follower, tx := NewMockPool(t), NewMockPool(t), NewMockTx(t)
			db := New(leader, []Pool{follower})
			ctx := context.Background()
			table, columns, source := pgx.Identifier{"values"}, []string{"value"}, pgx.CopyFromRows([][]any{{1}})
			switch route {
			case "leader":
				ctx = pgcontext.WithCanWrite(ctx)
				leader.EXPECT().CopyFrom(ctx, table, columns, source).Return(1, nil).Once()
			case "transaction":
				ctx = pgcontext.With(ctx, pgcontext.WithTransaction(tx))
				tx.EXPECT().CopyFrom(ctx, table, columns, source).Return(1, nil).Once()
			default:
				// Like Exec, a copy without WithCanWrite goes to a follower.
				follower.EXPECT().CopyFrom(ctx, table, columns, source).Return(1, nil).Once()
			}
			copied, err := db.CopyFrom(ctx, table, columns, source)
			require.NoError(t, err)
			require.EqualValues(t, 1, copied)
		})
	}
}
