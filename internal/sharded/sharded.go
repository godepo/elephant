package sharded

import (
	"context"
	"errors"

	"github.com/godepo/elephant/internal/pkg/pgcontext"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrCouldNotPickShard = errors.New("could not get shardID or shardingKey from context")

type failedRow struct {
	err error
}

func (r failedRow) Scan(_ ...any) error {
	return r.err
}

type Pool interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error)
	Transactional(ctx context.Context, fn func(ctx context.Context) error) (out error)
}

type Picker func(ctx context.Context, key string) uint

type Hive struct {
	shards      []Pool
	shardPicker Picker
}

func New(shards []Pool, shardPicker Picker) *Hive {
	return &Hive{
		shards:      shards,
		shardPicker: shardPicker,
	}
}

func (s *Hive) pickShardID(ctx context.Context) (uint, error) {
	if id, ok := pgcontext.ShardIDFrom(ctx); ok {
		return id, nil
	}
	if key, ok := pgcontext.ShardingKeyFrom(ctx); ok {
		return s.shardPicker(ctx, key), nil
	}
	return 0, ErrCouldNotPickShard
}

func (s *Hive) getShard(ctx context.Context) (Pool, error) {
	shardID, err := s.pickShardID(ctx)
	if err != nil {
		return nil, err
	}

	return s.shards[shardID], nil
}

func (s *Hive) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	shard, err := s.getShard(ctx)
	if err != nil {
		return nil, err
	}
	return shard.BeginTx(ctx, opts)
}

func (s *Hive) Begin(ctx context.Context) (pgx.Tx, error) {
	shard, err := s.getShard(ctx)
	if err != nil {
		return nil, err
	}
	return shard.Begin(ctx)
}

func (s *Hive) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	shard, err := s.getShard(ctx)
	if err != nil {
		return nil, err
	}
	return shard.Query(ctx, query, args...)
}

func (s *Hive) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	shard, err := s.getShard(ctx)
	if err != nil {
		return failedRow{err: err}
	}
	return shard.QueryRow(ctx, query, args...)
}

func (s *Hive) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	shard, err := s.getShard(ctx)
	if err != nil {
		return pgconn.CommandTag{}, err
	}
	return shard.Exec(ctx, query, args...)
}

func (s *Hive) Transactional(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	shard, err := s.getShard(ctx)
	if err != nil {
		return err
	}
	return shard.Transactional(ctx, fn)
}
