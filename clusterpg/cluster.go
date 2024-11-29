//go:generate mockery
package clusterpg

import (
	"context"
	"errors"
	"fmt"

	"github.com/godepo/elephant/internal/cluster"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var ErrInvalidClusterConfiguration = errors.New("invalid cluster configuration")

type Pool interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error)
	Transactional(ctx context.Context, fn func(ctx context.Context) error) (out error)
}

type ConstructDB func() (Pool, error)

type Builder interface {
	Leader(fn ConstructDB) Builder
	Follower(fns ...ConstructDB) Builder
	Go() (Pool, error)
}

func New() Builder {
	return builder{
		leaderConstructor: func() (Pool, error) {
			return nil, ErrInvalidClusterConfiguration
		},
	}
}

type builder struct {
	leaderConstructor     ConstructDB
	followersConstructors []ConstructDB
}

func (b builder) Leader(fn ConstructDB) Builder {
	b.leaderConstructor = fn
	return b
}

func (b builder) Follower(fns ...ConstructDB) Builder {
	cloned := make([]ConstructDB, len(b.followersConstructors), len(b.followersConstructors)+len(fns))
	copy(cloned, b.followersConstructors)
	cloned = append(cloned, fns...)
	b.followersConstructors = cloned
	return b
}

func (b builder) Go() (Pool, error) {
	if len(b.followersConstructors) == 0 {
		return nil, fmt.Errorf("%w: at least one folower constructor is required", ErrInvalidClusterConfiguration)
	}
	leader, err := b.leaderConstructor()
	if err != nil {
		return nil, fmt.Errorf("leader constructor failed: %w", err)
	}
	fellows := make([]cluster.Pool, 0, len(b.followersConstructors))

	for i, fn := range b.followersConstructors {
		follower, err := fn()
		if err != nil {
			return nil, fmt.Errorf("follower constructor [%d] failed: %w", i, err)
		}
		fellows = append(fellows, follower)
	}

	return cluster.New(leader, fellows), nil
}
