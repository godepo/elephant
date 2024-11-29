package regular

import (
	"os"
	"testing"

	"github.com/godepo/groat"
	"github.com/godepo/groat/integration"
	"github.com/godepo/pgrx"
	"github.com/jaswdr/faker/v2"
)

func mainProvider(t *testing.T) *groat.Case[Deps, State, *Instance] {
	tcs := groat.New[Deps, State, *Instance](t, func(t *testing.T, deps Deps) *Instance {
		return New(deps.DB)
	})
	tcs.Before(func(t *testing.T, deps Deps) Deps {
		deps.Faker = faker.New()
		deps.MockDB = NewMockDB(t)
		deps.MockPool = NewMockPool(t)
		deps.MockRows = NewMockRows(t)
		return deps
	})
	tcs.Given(func(t *testing.T, state State) State {
		state.Faker = tcs.Deps.Faker
		return state
	})
	return tcs
}

func TestMain(m *testing.M) {
	suite = integration.New[Deps, State, *Instance](m, mainProvider,
		pgrx.New[Deps](
			pgrx.WithContainerImage("docker.io/postgres:16"),
			pgrx.WithMigrationsPath("./sql"),
		),
	)
	os.Exit(suite.Go())
}
