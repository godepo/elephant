// Package failed provides pgx results for errors that occur before a query is dispatched.
package failed

import (
	"context"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type batchResults struct {
	ctx context.Context
	err error
}

func (b batchResults) Tx() pgx.Tx {
	tx, ok := pgcontext.TransactionFrom(b.ctx)
	if !ok {
		return nil
	}

	return tx
}

type rows struct{ err error }

func (r rows) TypeMap() *pgtype.Map {
	return nil
}

type row struct{ err error }

// BatchResults returns batch results that consistently report err without holding a connection.
func BatchResults(ctx context.Context, err error) pgx.BatchResults {
	return batchResults{
		err: err,
		ctx: ctx,
	}
}

// Rows returns closed rows that report err.
func Rows(err error) pgx.Rows { return rows{err: err} }

// Row returns a row that reports err on Scan.
func Row(err error) pgx.Row { return row{err: err} }

func (b batchResults) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, b.err }
func (b batchResults) Query() (pgx.Rows, error)         { return Rows(b.err), b.err }
func (b batchResults) QueryRow() pgx.Row                { return Row(b.err) }
func (b batchResults) Close() error                     { return b.err }

func (rows) Close()                                       {}
func (r rows) Err() error                                 { return r.err }
func (rows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (rows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (rows) Next() bool                                   { return false }
func (r rows) Scan(...any) error                          { return r.err }
func (r rows) Values() ([]any, error)                     { return nil, r.err }
func (rows) RawValues() [][]byte                          { return nil }
func (rows) Conn() *pgx.Conn                              { return nil }

func (r row) Scan(...any) error { return r.err }
