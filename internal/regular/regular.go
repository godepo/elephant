//go:generate go tool mockery
package regular

import (
	"context"
	"errors"
	"fmt"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Pool interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error)
}

type DB interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error)
}

type Instance struct {
	db               Pool
	selector         func(ctx context.Context) DB
	txErrPassMatcher func(context.Context, error) bool
}

func New(db Pool) *Instance {
	ins := &Instance{
		db: db,
	}

	ins.selector = func(ctx context.Context) DB {
		tx, ok := pgcontext.TransactionFrom(ctx)
		if ok {
			return tx
		}
		return ins.db
	}
	ins.txErrPassMatcher = func(ctx context.Context, err error) bool {
		if fn, ok := pgcontext.TxPassMatcherFrom(ctx); ok {
			return fn(ctx, err)
		}
		return false
	}
	return ins
}

func (ins *Instance) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := ins.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	return tx, nil
}

func (ins *Instance) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	tx, err := ins.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("can't begin tx at regular instance: %w", err)
	}
	return tx, nil
}

func (ins *Instance) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	rows, err := ins.selector(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("can't query regular instance: %w", err)
	}
	return rows, nil
}

func (ins *Instance) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	return ins.selector(ctx).QueryRow(ctx, query, args...)
}

func (ins *Instance) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	tag, err := ins.selector(ctx).Exec(ctx, query, args...)
	if err != nil {
		return pgconn.CommandTag{}, fmt.Errorf("can't query regular instance: %w", err)
	}
	return tag, nil
}

func (ins *Instance) nestedTx(ctx context.Context, tx pgx.Tx, fn func(ctx context.Context) error) (out error) {
	nested, err := tx.Begin(ctx)
	if err != nil {
		return fmt.Errorf("can't begin nested transaction: %w", err)
	}
	defer func() {
		rollbackErr := nested.Rollback(ctx)
		if rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			out = rollbackErr
		}
	}()

	nestedCtx := pgcontext.With(ctx, pgcontext.WithTransaction(nested))
	err = fn(nestedCtx)
	if err != nil {
		if !ins.txErrPassMatcher(ctx, err) {
			return err
		}
		out = err
	}
	if err = nested.Commit(ctx); err != nil {
		return fmt.Errorf("can't commit nested transaction: %w", err)
	}
	return out
}

func (ins *Instance) Transactional(ctx context.Context, fn func(ctx context.Context) error) (out error) {
	tx, ok := pgcontext.TransactionFrom(ctx)
	if ok {
		return ins.nestedTx(ctx, tx, fn)
	}

	var opts pgx.TxOptions
	if mod, ok := pgcontext.TxOptionsFrom(ctx); ok {
		opts = mod
	}

	err := pgx.BeginTxFunc(ctx, ins, opts, func(tx pgx.Tx) error {
		txCtx := pgcontext.With(ctx, pgcontext.WithTransaction(tx))
		err := fn(txCtx)
		if err != nil {
			if ins.txErrPassMatcher(ctx, err) {
				out = err
				return nil
			}
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("can't run transaction regular instance: %w", err)
	}
	return out
}
