# Elephant
[![codecov](https://codecov.io/gh/godepo/elephant/graph/badge.svg?token=I5M6SN6ZNI)](https://codecov.io/gh/godepo/elephant)

Lightweight toolkit for using transactional queries through pgx driver and write clean and compact code. 

## Solving problems

1. Write code with support nested transactions.
2. Control postgresql node, when run query.
3. Automatic transactions commiting and rollback.
4. Write code with compact method signatures.
5. Using queries separation when using postgresql cluster (user replicas when no need write access and leader for others).
6. Hide boilerplate inside common library.
7. Control transactions through context

## Guide


### Create repository

Create abstraction layer, for postgresql storing logic. For example using Greeing example from core pgx library:

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
	p, err := pgxpool.New(context.Background(), context.Background(), os.Getenv("DATABASE_URL"))
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

When have one leader postgres and one or more replicas, can use DSL builder from "github.com/godepo/elephant/clusterpg":

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
	leader, err := pgxpool.New(context.Background(), context.Background(), os.Getenv("LEADER_URL"))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer leader.Close()

	replica, err := pgxpool.New(context.Background(), context.Background(), os.Getenv("REPLICA_URL"))
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

