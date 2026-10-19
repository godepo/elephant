package regular

import (
	"context"
	"testing"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestInstance_SendBatch(t *testing.T) {
	t.Run("should be able to execute a batch using the connection pool", func(t *testing.T) {
		tcs := suite.Case(t)
		ctx := context.Background()
		record := Record{ID: uuid.New(), Value: "batch through pool"}
		batch := &pgx.Batch{}
		batch.Queue("INSERT INTO regular.instance (id, value) VALUES ($1, $2)", record.ID, record.Value)
		batch.Queue("SELECT id, value FROM regular.instance WHERE id = $1", record.ID)

		results := tcs.SUT.SendBatch(ctx, batch)
		defer results.Close()
		tag, err := results.Exec()
		require.NoError(t, err)
		require.EqualValues(t, 1, tag.RowsAffected())
		var actual Record
		require.NoError(t, results.QueryRow().Scan(&actual.ID, &actual.Value))
		require.Equal(t, record, actual)
		require.NoError(t, results.Close())

		var persisted string
		require.NoError(t, tcs.Deps.DB.QueryRow(ctx,
			"SELECT value FROM regular.instance WHERE id = $1", record.ID).Scan(&persisted))
		require.Equal(t, record.Value, persisted)
	})

	t.Run("should be able to execute a batch using the transaction from context", func(t *testing.T) {
		tcs := suite.Case(t)
		ctx := context.Background()
		tx, err := tcs.SUT.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		txCtx := pgcontext.With(ctx, pgcontext.WithTransaction(tx))
		var txPID int32
		require.NoError(t, tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&txPID))
		record := Record{ID: uuid.New(), Value: "batch through transaction"}
		batch := &pgx.Batch{}
		batch.Queue("SELECT pg_backend_pid()")
		batch.Queue("INSERT INTO regular.instance (id, value) VALUES ($1, $2)", record.ID, record.Value)

		results := tcs.SUT.SendBatch(txCtx, batch)
		defer results.Close()
		var batchPID int32
		require.NoError(t, results.QueryRow().Scan(&batchPID))
		require.Equal(t, txPID, batchPID, "the batch must use the transaction's connection")
		tag, err := results.Exec()
		require.NoError(t, err)
		require.EqualValues(t, 1, tag.RowsAffected())
		require.NoError(t, results.Close())
		require.NoError(t, tx.Commit(ctx))

		var persisted string
		require.NoError(t, tcs.Deps.DB.QueryRow(ctx,
			"SELECT value FROM regular.instance WHERE id = $1", record.ID).Scan(&persisted))
		require.Equal(t, record.Value, persisted)
	})

	t.Run("should be able to roll back batch writes with the transaction", func(t *testing.T) {
		tcs := suite.Case(t)
		ctx := context.Background()
		tx, err := tcs.SUT.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		txCtx := pgcontext.With(ctx, pgcontext.WithTransaction(tx))
		record := Record{ID: uuid.New(), Value: "rolled back batch"}
		batch := &pgx.Batch{}
		batch.Queue("INSERT INTO regular.instance (id, value) VALUES ($1, $2)", record.ID, record.Value)
		batch.Queue("SELECT value FROM regular.instance WHERE id = $1", record.ID)

		results := tcs.SUT.SendBatch(txCtx, batch)
		defer results.Close()
		tag, err := results.Exec()
		require.NoError(t, err)
		require.EqualValues(t, 1, tag.RowsAffected())
		var value string
		require.NoError(t, results.QueryRow().Scan(&value))
		require.Equal(t, record.Value, value)
		require.NoError(t, results.Close())
		require.NoError(t, tx.Rollback(ctx))

		require.ErrorIs(t, tcs.Deps.DB.QueryRow(ctx,
			"SELECT value FROM regular.instance WHERE id = $1", record.ID).Scan(&value), pgx.ErrNoRows)
	})
}
