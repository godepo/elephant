//go:generate mockery
package pgcontext

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type option int8

const (
	optTransactional option = iota + 1
	optCanWrite
	optTxOptions
	optTxPassMatcher
	optShardID
	optShardingKey
)

type OptionContext func(ctx context.Context) context.Context
type TxPassMatcher func(context.Context, error) bool

func With(ctx context.Context, opts ...OptionContext) context.Context {
	for _, opt := range opts {
		ctx = opt(ctx) //nolint:fatcontext
	}
	return ctx
}

func WithCanWrite(ctx context.Context) context.Context {
	return context.WithValue(ctx, optCanWrite, true)
}

func WithTransaction(tx pgx.Tx) OptionContext {
	return func(ctx context.Context) context.Context {
		return context.WithValue(ctx, optTransactional, tx)
	}
}

func TransactionFrom(ctx context.Context) (pgx.Tx, bool) {
	res, ok := ctx.Value(optTransactional).(pgx.Tx)
	if !ok {
		return nil, false
	}
	return res, true
}

func CanWriteFrom(ctx context.Context) bool {
	res, ok := ctx.Value(optCanWrite).(bool)
	if !ok {
		return false
	}
	return res
}

func WithTxOptions(opt pgx.TxOptions) OptionContext {
	return func(ctx context.Context) context.Context {
		return context.WithValue(ctx, optTxOptions, opt)
	}
}

func TxOptionsFrom(ctx context.Context) (pgx.TxOptions, bool) {
	res, ok := ctx.Value(optTxOptions).(pgx.TxOptions)
	return res, ok
}

func WithFnTxPassMatcher(fn TxPassMatcher) OptionContext {
	return func(ctx context.Context) context.Context {
		return context.WithValue(ctx, optTxPassMatcher, fn)
	}
}

func TxPassMatcherFrom(ctx context.Context) (TxPassMatcher, bool) {
	res, ok := ctx.Value(optTxPassMatcher).(TxPassMatcher)
	return res, ok
}

func WithShardID(shardID uint) OptionContext {
	return func(ctx context.Context) context.Context {
		return context.WithValue(ctx, optShardID, shardID)
	}
}

func ShardIDFrom(ctx context.Context) (uint, bool) {
	res, ok := ctx.Value(optShardID).(uint)
	return res, ok
}

func WithShardingKey(shardingKey string) OptionContext {
	return func(ctx context.Context) context.Context {
		return context.WithValue(ctx, optShardingKey, shardingKey)
	}
}

func ShardingKeyFrom(ctx context.Context) (string, bool) {
	res, ok := ctx.Value(optShardingKey).(string)
	return res, ok
}
