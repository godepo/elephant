// Package sqlcbatch contains a reproducible sqlc batch compatibility test fixture.
// It is a separate module, so the root module's go generate, tests and linters skip it.
//
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate
package sqlcbatch
