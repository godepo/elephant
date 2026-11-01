package failed

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBatchResults(t *testing.T) {
	failure := errors.New("failure")
	results := BatchResults(failure)

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
}

func TestRow(t *testing.T) {
	failure := errors.New("failure")
	require.ErrorIs(t, Row(failure).Scan(), failure)
}
