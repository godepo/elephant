//go:generate go tool mockery
package cluster

import (
	"context"

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
	Transactional(ctx context.Context, fn func(ctx context.Context) error) (out error)
}

type DB interface {
	Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row
	Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error)
}

type LoadBalancer func(fellows []Pool) Pool

type Config struct {
	loadBalancer LoadBalancer
}

type Option func(opt *Config)

func WithLoadBalancer(balancer LoadBalancer) Option {
	return func(opt *Config) {
		opt.loadBalancer = balancer
	}
}

func New(leader Pool, fellows []Pool, opts ...Option) *Cluster {
	cfg := Config{
		loadBalancer: DefaultLoadBalancer(),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return &Cluster{
		leader:  leader,
		fellows: fellows,
		cfg:     cfg,
	}
}

type Cluster struct {
	leader  Pool
	fellows []Pool
	cfg     Config
}

func (cls *Cluster) selector(ctx context.Context) DB {
	if tx, ok := pgcontext.TransactionFrom(ctx); ok {
		return tx
	}
	if pgcontext.CanWriteFrom(ctx) {
		return cls.leader
	}
	return cls.cfg.loadBalancer(cls.fellows)
}

func (cls *Cluster) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	tx, ok := pgcontext.TransactionFrom(ctx)
	if ok {
		return tx.Begin(ctx)
	}
	return cls.leader.BeginTx(ctx, opts)
}

func (cls *Cluster) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, ok := pgcontext.TransactionFrom(ctx)
	if ok {
		return tx.Begin(ctx)
	}
	return cls.leader.Begin(ctx)
}

func (cls *Cluster) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	return cls.selector(ctx).Query(ctx, query, args...)
}

func (cls *Cluster) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	return cls.selector(ctx).QueryRow(ctx, query, args...)
}

func (cls *Cluster) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	return cls.selector(ctx).Exec(ctx, query, args...)
}

func (cls *Cluster) Transactional(ctx context.Context, fn func(ctx context.Context) error) (out error) {
	_, ok := pgcontext.TransactionFrom(ctx)
	if ok || pgcontext.CanWriteFrom(ctx) {
		return cls.leader.Transactional(ctx, fn)
	}
	return cls.cfg.loadBalancer(cls.fellows).Transactional(ctx, fn)
}
