// Package batchresults provides batch results for errors that occur before dispatch.
package batchresults

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type failed struct{ err error }
type failedRow struct{ err error }

// Failed returns results that consistently report err without holding a connection.
func Failed(err error) pgx.BatchResults           { return failed{err: err} }
func (b failed) Exec() (pgconn.CommandTag, error) { return pgconn.CommandTag{}, b.err }
func (b failed) Query() (pgx.Rows, error)         { return nil, b.err }
func (b failed) QueryRow() pgx.Row                { return failedRow(b) }
func (b failed) Close() error                     { return b.err }
func (r failedRow) Scan(...any) error             { return r.err }
