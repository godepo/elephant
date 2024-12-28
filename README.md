# Elephant
[![codecov](https://codecov.io/gh/godepo/elephant/graph/badge.svg?token=I5M6SN6ZNI)](https://codecov.io/gh/godepo/elephant)
[![Go Report Card](https://goreportcard.com/badge/godepo/elephant)](https://goreportcard.com/report/godepo/elephant)
[![License](https://img.shields.io/badge/License-MIT%202.0-blue.svg)](https://github.com/godepo/elephant/blob/main/LICENSE)

Lightweight toolkit for using transactional queries through pgx driver and write clean and compact code. 

## Solving problems

1. Write code with support nested transactions.
2. Control postgresql node when run query.
3. Automatic transactions commiting and rollback.
4. Write code with compact method signatures.
5. Using queries separation when using postgresql cluster (user replicas when no need write access and leader for others).
6. Hide boilerplate inside common library.
7. Control transactions through context.
8. Hiding sharded postgresql inside and using custom sharding functions

## Guide


### Create repository

Create abstraction layer for postgresql storing logic. For example using Greeting example from core pgx library:

```go
package repository
import (
    "context"

    "github.com/jackc/pgx/v5"
)

type DB interface {
	QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row
}

type Repository struct {
	db DB
}

func New(db DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Greeting(ctx context.Context) (string, error) {
	var greeting string
	err := r.db.QueryRow(context.Background(), "select 'Hello, world!'").Scan(&greeting)
	if err != nil {
		return "", fmt.Errorf("failed to query: %w", err)
	}
	return greeting, nil
}
```

### Service layer

```go
package service

import (
	"context"
)

type DB interface {
	Transactional(ctx context.Context, fn func(ctx context.Context) error) (out error)
}

type GreetingRepository interface {
	Greeting(ctx context.Context) (string, error)
}

type Service struct {
	db DB
	greetingRepo GreetingRepository
}

func New(db DB, greetingRepo GreetingRepository) *Service {
	return &Service{greetingRepo: greetingRepo, db: db}
}

func (srv *Service) Greeting(ctx context.Context) (result string, err error) {
	if err = srv.db.Transactional(ctx, func(ctx context.Context) error {
		res, err := srv.greetingRepo.Greeting(ctx)
		if err != nil {
			return err
		}
		result = res
		return nil
	}); err != nil {
		return "", err
    }
	return result, nil
}
```

### Construct connection

There support multiple version of postgresql installations.

#### Single node postgresql

Basic version of installation. On This way, need construct pgx connection pool and pass it to constructor from
"github.com/godepo/elephant/singlepg":

```go
package main
import (
	"context"
	"fmt"
	"os"
	
	"github.com/jackc/pgx/v5"
	"github.com/godepo/elephant/singlepg"
	greetRepo "somecompany.com/somesrv/repositories/greeting"
	greetSrv "somecompany.com/somesrv/services/greeting"
)

func main() {
	p, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer p.Close()

	db := singlepg.New(p)
	srv := greetSrv.New(db, greetRepo.New(db))

	greeting, err := srv.Greeting(context.Background())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to greeing: %v\n", err)
		os.Exit(1)
    }
	
	fmt.Println(greeting)
}
```

#### Cluster postgres

When have one leader postgres and one or more replicas can use DSL builder from "github.com/godepo/elephant/clusterpg":

```go
package main
import (
	"context"
	"fmt"
	"os"
	
	"github.com/jackc/pgx/v5"
	"github.com/godepo/elephant/singlepg"
	greetRepo "somecompany.com/somesrv/repositories/greeting"
	greetSrv "somecompany.com/somesrv/services/greeting"
)

func main() {
	leader, err := pgxpool.New(context.Background(), os.Getenv("LEADER_URL"))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer leader.Close()

	replica, err := pgxpool.New(context.Background(), os.Getenv("REPLICA_URL"))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer replica.Close()

	db, err := clusterpg.New().
		Leader(func() (clusterpg.Pool, error) {
			return singlepg.New(leader), nil
		}).
		Follower(func() (clusterpg.Pool, error) {
			return singlepg.New(replica), nil
		}).
		Go()

	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to construct cluster pool: %v\n", err)
		os.Exit(1)
	}
	
	srv := greetSrv.New(db, greetRepo.New(db))

	greeting, err := srv.Greeting(context.Background())
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to greeing: %v\n", err)
		os.Exit(1)
    }
	
	fmt.Println(greeting)
}
```

#### Sharded postgres

When have sharded postgresql can use DSL builder from "github.com/godepo/elephant/shardedpg":

```go
type PG interface {
	sharded.Pool
}

type Repository struct {
	pool PG
}

func NewRepo(pool PG) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetUserNameByPhoneNumber(ctx context.Context, number string) (string, error) {
	var name string
	err := r.pool.
		QueryRow(ctx, `SELECT username FROM users WHERE phone = $1`, number).
		Scan(&name)
	if err != nil {
		return "", err
	}
	return name, nil
}

type TxManager interface {
	Transactional(ctx context.Context, fn func(ctx context.Context) error) (out error)
}

type UserRepository interface {
	GetUserNameByPhoneNumber(ctx context.Context, number string) (string, error)
}

type Service struct {
	txManager TxManager
	userRepo  UserRepository
}

func NewService(txManager TxManager, userRepo UserRepository) *Service {
	return &Service{txManager: txManager, userRepo: userRepo}
}

func (srv *Service) GetUserNameByPhoneNumber(ctx context.Context, number string) (result string, err error) {
	// Shard ID will be found by your Picker function implementation
	ctx = elephant.With(ctx, elephant.WithShardingKey(number))
	// or you can set it directly using this:
	// ctx = pgcontext.With(ctx, pgcontext.WithShardID(0))

	if err = srv.txManager.Transactional(ctx, func(ctx context.Context) error {
		res, err := srv.userRepo.GetUserNameByPhoneNumber(ctx, number)
		if err != nil {
			return err
		}
		result = res
		return nil
	}); err != nil {
		return "", err
	}
	return result, nil
}

func main() {
	shard0, err := pgxpool.New(context.Background(), os.Getenv("SHARD_URL_0"))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer shard0.Close()

	shard1, err := pgxpool.New(context.Background(), os.Getenv("SHARD_URL_1"))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer shard0.Close()

	shard2, err := pgxpool.New(context.Background(), os.Getenv("SHARD_URL_2"))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer shard0.Close()

	const poolSize = 3
	// Create your sharded pool with Sharding function
	pool, err :=
		shardedpg.New(poolSize).
			Picker(func(ctx context.Context, key string) uint {
				hash := md5.Sum([]byte(key))
				hashInt := new(big.Int).SetBytes(hash[:]).Int64()
				return uint(hashInt) % poolSize
			}).
			Shard(0, singlepg.New(shard0)). // You can use any implementation of sharded.Pool interface like clusterpg, singlepg etc...
			Shard(1, singlepg.New(shard1)).
			Shard(2, singlepg.New(shard2)).
			Go()

	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to construct shards pool: %v\n", err)
		os.Exit(1)
	}

	srv := NewService(pool, NewRepo(pool))

	name, err := srv.GetUserNameByPhoneNumber(context.Background(), "1-202-456-1111")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to greeing: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(name)
}

```

### Metrics and timeouts wrapper

Construct regular node and add create collector for it. By example: use prometheus like there:

```go
func main() {
    ctx := context.Background()
    pool, err := pgxpool.New(ctx, "user=elephant password=slonjara port=5432 sslmode=disable host=localhost")
    if err != nil {
        panic(err)
    }
    db := regular.New(pool)
    
    latencies := prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
        Name: "srv_0_queries_latency",
        Help: "latency metrics for postgresql queries",
        },
        []string{"success", "user_find_by_id"},
    )
    qps := prometheus.NewCounterVec(prometheus.CounterOpts{
        Name: "srv_0_queries_per_second",
        Help: "queries per seconds for postgresql queries",
        }, []string{"success", "user_find_by_id"},
	)
    
    clt, err := metrics.Collector().
        Latency(func(labels ...string) (collector.Histogram, error) {
        return latencies.GetMetricWithLabelValues(labels...)
    }).
    QueryPerSecond(func(labels ...string) (collector.Counter, error) {
        return qps.GetMetricWithLabelValues(labels...)
    }).Build()
    if err != nil {
        panic(err)
    }
    
    mtrDB := metrics.New(db, clt)
    usrRepo := pgusers.New(mtrDB)
    
    if err = prometheus.Register(qps); err != nil {
        panic(err)
    }
    if err = prometheus.Register(latencies); err != nil {
        panic(err)
    }
    
    http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
        _, err := usrRepo.FindByID(r.Context(), uuid.New())
        if err != nil {
            fmt.Println(err)
            http.Error(w, err.Error(), http.StatusInternalServerError)
	    }
	})
    http.Handle("/metrics", promhttp.Handler())
    http.ListenAndServe(":8080", nil)
    }
```

In repository method make this changes:

```go
const queryFindByID = `
SELECT * FROM users WHERE id = $1;
`
const labelFindByID = `users_find_by_id`

func (repo *Repository) FindByID(ctx context.Context, userID uuid.UUID) (user.Info, error) {
	ctx = elephant.With(ctx,
		elephant.WithMetricsLabel(labelFindByID),
		elephant.WithTimeout(time.Second),
	)
	rows, err := repo.db.Query(ctx, queryFindByID, userID)
	if err != nil {
		return user.Info{}, err
	}
	res, err := pgx.CollectOneRow[user.Info](rows, pgx.RowToStructByName)
	if err != nil {
		return user.Info{}, err
	}
	return res, nil
}
```

Annotation **elephant.WithMetricsLabel(labelFindByID)** specify query. By default, we add 
"success" and "failure" for separate queries results. In the future, we have plans create auto-generation tool
for generating elephants collectors like in these samples.

Annotation **elephant.WithTimeout(time.Second)** present timeout for execution this query and cancel it 
when timeout exceeded.
 
### Control execution flow

#### Separate read/write queries

When we are have separate query execution to leader node (at clusters db variation), we must specify in context one 
annotation:

```go
ctx = elephant.With(ctx, elephant.CanWrite)
```

There made new context and database facade transfer current query to leader node. By default, all other queries executing 
at replica node. 

This annotation using before transactional method call or before any method of repository. But, when we have started 
transaction - all queries executed with their context database node. 

