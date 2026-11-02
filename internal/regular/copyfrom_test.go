package regular

import (
	"context"
	"errors"
	"testing"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestInstance_CopyFrom(t *testing.T) {
	t.Run("should copy rows through the pool", func(t *testing.T) {
		tcs := suite.Case(t)
		ctx := context.Background()
		record := Record{ID: uuid.New(), Value: "copy through pool"}
		copied, err := tcs.SUT.CopyFrom(ctx, pgx.Identifier{"regular", "instance"}, []string{"id", "value"},
			pgx.CopyFromRows([][]any{{record.ID, record.Value}}))
		require.NoError(t, err)
		require.EqualValues(t, 1, copied)
		var actual string
		require.NoError(t, tcs.SUT.QueryRow(ctx,
			"SELECT value FROM regular.instance WHERE id = $1", record.ID).Scan(&actual))
		require.Equal(t, record.Value, actual)
		require.NoError(t, tcs.Deps.DB.QueryRow(context.Background(),
			"SELECT value FROM regular.instance WHERE id = $1", record.ID).Scan(&actual))
		require.Equal(t, record.Value, actual)
	})
	t.Run("should copy rows through the committed transaction", func(t *testing.T) {
		tcs := suite.Case(t)
		ctx := context.Background()
		record := Record{ID: uuid.New(), Value: "copy through committed transaction"}
		tx, err := tcs.SUT.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		ctx = pgcontext.With(ctx, pgcontext.WithTransaction(tx))
		copied, err := tcs.SUT.CopyFrom(ctx, pgx.Identifier{"regular", "instance"}, []string{"id", "value"},
			pgx.CopyFromRows([][]any{{record.ID, record.Value}}))
		require.NoError(t, err)
		require.EqualValues(t, 1, copied)
		var actual string
		require.NoError(t, tcs.SUT.QueryRow(ctx,
			"SELECT value FROM regular.instance WHERE id = $1", record.ID).Scan(&actual))
		require.Equal(t, record.Value, actual)
		require.NoError(t, tx.Commit(context.Background()))
		require.NoError(t, tcs.Deps.DB.QueryRow(context.Background(),
			"SELECT value FROM regular.instance WHERE id = $1", record.ID).Scan(&actual))
		require.Equal(t, record.Value, actual)
	})
	t.Run("should copy rows through the rolled back transaction", func(t *testing.T) {
		tcs := suite.Case(t)
		ctx := context.Background()
		record := Record{ID: uuid.New(), Value: "copy through rolled back transaction"}
		tx, err := tcs.SUT.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		ctx = pgcontext.With(ctx, pgcontext.WithTransaction(tx))
		copied, err := tcs.SUT.CopyFrom(ctx, pgx.Identifier{"regular", "instance"}, []string{"id", "value"},
			pgx.CopyFromRows([][]any{{record.ID, record.Value}}))
		require.NoError(t, err)
		require.EqualValues(t, 1, copied)
		var actual string
		require.NoError(t, tcs.SUT.QueryRow(ctx,
			"SELECT value FROM regular.instance WHERE id = $1", record.ID).Scan(&actual))
		require.Equal(t, record.Value, actual)
		require.NoError(t, tx.Rollback(context.Background()))
		require.ErrorIs(t, tcs.Deps.DB.QueryRow(context.Background(),
			"SELECT value FROM regular.instance WHERE id = $1", record.ID).Scan(&actual), pgx.ErrNoRows)
	})
}

func TestCopyFromError(t *testing.T) {
	pool := NewMockPool(t)
	failure := errors.New("copy failed")
	pool.EXPECT().CopyFrom(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(1, failure).Once()
	copied, err := New(pool).CopyFrom(context.Background(), pgx.Identifier{"values"}, nil, pgx.CopyFromRows(nil))
	require.ErrorIs(t, err, failure)
	require.Zero(t, copied)
}
