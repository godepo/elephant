package acceptance

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/godepo/elephant/internal/metrics"
	"github.com/godepo/elephant/singlepg"
	"github.com/godepo/groat"
	"github.com/godepo/groat/integration"
	"github.com/godepo/pgrx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Deps struct {
	Pool      *pgxpool.Pool `groat:"pgxpool"`
	Collector *collector
}

type State struct{}

var suite *integration.Container[Deps, State, *metrics.DB]

// collector counts tracked operations and keeps the error of the last one.
type collector struct {
	calls int
	err   error
}

func (c *collector) TrackQueryMetrics(_ context.Context, _ time.Time, err error) {
	c.calls++
	c.err = err
}

func mainProvider(t *testing.T) *groat.Case[Deps, State, *metrics.DB] {
	tcs := groat.New[Deps, State, *metrics.DB](t, func(t *testing.T, deps Deps) *metrics.DB {
		return metrics.New(singlepg.New(deps.Pool), deps.Collector)
	})
	tcs.Before(func(t *testing.T, deps Deps) Deps {
		deps.Collector = &collector{}
		return deps
	})
	return tcs
}

func TestMain(m *testing.M) {
	suite = integration.New[Deps, State, *metrics.DB](m, mainProvider,
		pgrx.New[Deps](
			pgrx.WithContainerImage("docker.io/postgres:16"),
			pgrx.WithMigrationsPath("./sql"),
		),
	)
	os.Exit(suite.Go())
}
