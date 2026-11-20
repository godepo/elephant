//go:generate go tool mockery
package trunk

import (
	"os"
	"testing"

	"github.com/godepo/groat"
	"github.com/godepo/groat/integration"
	"github.com/godepo/pgrx"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaswdr/faker/v2"
)

type (
	Deps struct {
		Faker  faker.Faker
		DB     *pgxpool.Pool   `groat:"pgxpool"`
		Config *pgxpool.Config `groat:"pgxconfig"`
	}

	State struct {
		Faker faker.Faker
	}

	SUT struct{}
)

var suite *integration.Container[Deps, State, *SUT]

func mainProvider(t *testing.T) *groat.Case[Deps, State, *SUT] {
	tcs := groat.New[Deps, State, *SUT](t, func(t *testing.T, deps Deps) *SUT {
		return &SUT{}
	})
	tcs.Before(func(t *testing.T, deps Deps) Deps {
		deps.Faker = faker.New()
		return deps
	})
	tcs.Given(func(t *testing.T, state State) State {
		state.Faker = tcs.Deps.Faker

		return state
	})
	return tcs
}

func TestMain(m *testing.M) {
	suite = integration.New[Deps, State, *SUT](m, mainProvider,
		pgrx.New[Deps](
			pgrx.WithContainerImage("docker.io/postgres:16"),
			pgrx.WithMigrationsPath("../poolstats/sql"),
			pgrx.WithPoolConfigInjectLabel("pgxconfig"),
		),
	)
	os.Exit(suite.Go())
}
