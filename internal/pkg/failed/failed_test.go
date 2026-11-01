//go:generate go tool mockery
package failed

import (
	"errors"
	"testing"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/stretchr/testify/require"
)

func TestBatchResults(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		t.Run("without transaction", func(t *testing.T) {
			failure := errors.New("failure")
			results := BatchResults(t.Context(), failure)

			tag, err := results.Exec()
			require.ErrorIs(t, err, failure)
			require.Zero(t, tag.RowsAffected())

			rows, err := results.Query()
			require.ErrorIs(t, err, failure)
			require.NotNil(t, rows, "callers may close rows before checking the error, as with pgx")
			require.ErrorIs(t, rows.Err(), failure)

			require.ErrorIs(t, results.QueryRow().Scan(), failure)
			require.ErrorIs(t, results.Close(), failure)
			require.ErrorIs(t, results.Close(), failure)

			require.Nil(t, results.Tx())
		})

		t.Run("with transaction", func(t *testing.T) {
			tx := NewMockTx(t)

			tx.EXPECT().Commit(t.Context()).Return(nil)

			ctx := pgcontext.With(t.Context(), pgcontext.WithTransaction(tx))

			results := BatchResults(ctx, errors.New("failure"))
			require.NoError(t, results.Tx().Commit(t.Context()))
		})
	})

}

func TestRows(t *testing.T) {
	failure := errors.New("failure")
	rows := Rows(failure)

	require.False(t, rows.Next())
	require.ErrorIs(t, rows.Scan(), failure)
	values, err := rows.Values()
	require.ErrorIs(t, err, failure)
	require.Nil(t, values)
	require.Nil(t, rows.RawValues())
	require.Nil(t, rows.FieldDescriptions())
	require.Zero(t, rows.CommandTag().RowsAffected())
	require.Nil(t, rows.Conn())
	rows.Close()
	require.ErrorIs(t, rows.Err(), failure)
	require.Nil(t, rows.TypeMap())
}

func TestRow(t *testing.T) {
	failure := errors.New("failure")
	require.ErrorIs(t, Row(failure).Scan(), failure)
}
