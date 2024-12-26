package shardedpg

import (
	"context"
	"errors"
	"reflect"

	"github.com/godepo/elephant/internal/sharded"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	ErrWrongShardsPoolSize     = errors.New("sharded pg: wrong shards pool size")
	ErrNoShardPickerProvided   = errors.New("sharded pg: no sharded picker provided")
	ErrNotEnoughShardsProvided = errors.New("sharded pg: provided less shards than pool size")
	ErrNilShardProvided        = errors.New("sharded pg: nil shard provided")
)

type Pool interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error)
	Transactional(ctx context.Context, fn func(ctx context.Context) error) (out error)
}

type Builder interface {
	Picker(pickFn ShardPicker) Builder
	Shard(key uint, shard Pool) Builder
	Go() (*sharded.Hive, error)
}

type ShardPicker func(ctx context.Context, key string) uint

type builder struct {
	size   uint
	shards map[uint]Pool
	picker ShardPicker
}

func New(poolSize uint) Builder {
	return &builder{
		size:   poolSize,
		shards: make(map[uint]Pool, poolSize),
	}
}

func (b *builder) Picker(picker ShardPicker) Builder {
	b.picker = picker
	return b
}

func (b *builder) Shard(key uint, shard Pool) Builder {
	b.shards[key] = shard
	return b
}

func (b *builder) Go() (*sharded.Hive, error) {
	if b.size == 0 {
		return nil, ErrWrongShardsPoolSize
	}
	if b.picker == nil {
		return nil, ErrNoShardPickerProvided
	}
	shards := make([]sharded.Pool, 0, b.size)
	for key := range b.size {
		shard, ok := b.shards[key]
		if !ok {
			return nil, ErrNotEnoughShardsProvided
		}
		if shard == nil || reflect.ValueOf(shard).IsNil() {
			return nil, ErrNilShardProvided
		}
		shards = append(shards, shard)
	}
	return sharded.New(shards, sharded.Picker(b.picker)), nil
}
