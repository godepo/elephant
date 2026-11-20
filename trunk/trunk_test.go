package trunk

import (
	"context"
	"errors"
	"testing"

	"github.com/godepo/elephant"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWithLeader(t *testing.T) {
	exp := uuid.NewString()

	settings := defaultSettings().With(
		WithNode(exp),
	)

	require.Len(t, settings.leaders, 1)
	require.Equal(t, settings.leaders[0], exp)
}

func TestNew(t *testing.T) {
	t.Run("should be able to be able", func(t *testing.T) {
		t.Run("connect to single pg instance", func(t *testing.T) {
			tc := suite.Case(t)

			reg := prometheus.NewRegistry()

			db, err := New(
				t.Context(),
				WithNode(tc.Deps.Config.ConnString()),
				WithMetrics(reg),
			)

			require.NoError(t, err)
			require.NotNil(t, db)

			defer db.Close()

			ctx := elephant.With(
				t.Context(),
				elephant.WithMetricsLabel("query123"),
			)

			row := db.QueryRow(ctx, "select 1")
			var count int
			err = row.Scan(&count)
			require.NoError(t, err)

			require.Equal(t, 1, count)
		})

		t.Run("connect to HA pg cluster", func(t *testing.T) {
			tc := suite.Case(t)

			reg := prometheus.NewRegistry()

			db, err := New(
				t.Context(),
				WithNode(tc.Deps.Config.ConnString(), tc.Deps.Config.ConnString()),
				WithMetrics(reg),
			)

			require.NoError(t, err)
			require.NotNil(t, db)

			defer db.Close()

			ctx := elephant.With(
				t.Context(),
				elephant.WithMetricsLabel("query123"),
			)

			row := db.QueryRow(ctx, "select 1")
			var count int
			err = row.Scan(&count)
			require.NoError(t, err)

			require.Equal(t, 1, count)
		})

		t.Run("connect to multiple pg instances", func(t *testing.T) {
			tc := suite.Case(t)

			reg := prometheus.NewRegistry()

			db, err := New(
				t.Context(),
				WithNode(tc.Deps.Config.ConnString()),
				WithNode(tc.Deps.Config.ConnString(), tc.Deps.Config.ConnString()),
				WithPicker(func(ctx context.Context, key string) uint {
					return 1
				}),
				WithMetrics(reg),
			)
			require.NoError(t, err)

			defer db.Close()

			ctx := elephant.With(
				t.Context(),
				elephant.WithShardID(1),
				elephant.WithMetricsLabel("query123"),
			)

			row := db.QueryRow(ctx, "select 1")
			var count int
			err = row.Scan(&count)
			require.NoError(t, err)
			assert.Equal(t, 1, count)

			ctx = elephant.With(
				t.Context(),
				elephant.WithShardID(0),
				elephant.WithMetricsLabel("query321"),
			)

			row = db.QueryRow(ctx, "select 2")
			err = row.Scan(&count)
			require.NoError(t, err)
			assert.Equal(t, 2, count)

			ctx = elephant.With(
				t.Context(),
				elephant.WithShardID(1),
				elephant.WithCanWrite,
				elephant.WithMetricsLabel("query321"),
			)

			row = db.QueryRow(ctx, "select 3")
			err = row.Scan(&count)
			require.NoError(t, err)
			assert.Equal(t, 3, count)
		})
	})
	t.Run("should be able to not be able", func(t *testing.T) {
		t.Run("when can't have correct DSN", func(t *testing.T) {
			db, err := New(
				t.Context(),
				WithNode(uuid.NewString()),
			)
			require.Error(t, err)
			require.Nil(t, db)
		})
		t.Run("when follower has invalid dsn", func(t *testing.T) {
			tc := suite.Case(t)

			db, err := New(
				t.Context(),
				WithNode(tc.Deps.Config.ConnString(), uuid.NewString()),
			)
			require.Error(t, err)
			require.Nil(t, db)
		})
		t.Run("when follower in cluster has invalid dsn", func(t *testing.T) {
			tc := suite.Case(t)

			db, err := New(
				t.Context(),
				WithNode(uuid.NewString(), tc.Deps.Config.ConnString()),
			)
			require.Error(t, err)
			require.Nil(t, db)
		})

		t.Run("when last follower has invalid dsn", func(t *testing.T) {
			tc := suite.Case(t)

			db, err := New(
				t.Context(),
				WithNode(tc.Deps.Config.ConnString(), tc.Deps.Config.ConnString(), uuid.NewString()),
			)
			require.Error(t, err)
			require.Nil(t, db)
		})

		t.Run("when settings is invalid", func(t *testing.T) {
			t.Run("when dsn is not present", func(t *testing.T) {
				_, err := New(t.Context())
				require.ErrorIs(t, err, ErrDSNNotPresent)
			})

			t.Run("when picker is not present for sharded cluster", func(t *testing.T) {
				_, err := New(t.Context(),
					WithNode(uuid.NewString()),
					WithNode(uuid.NewString()),
				)
				require.ErrorIs(t, err, ErrRequirePicker)
			})

			t.Run("when follower is present for non leaders settings", func(t *testing.T) {
				_, err := New(t.Context(),
					WithNode(uuid.NewString()),
					func(settings *Settings) {
						settings.followers = append(settings.followers, []string{uuid.NewString()})
					})
				require.ErrorIs(t, err, ErrInvalidClusterConfiguration)
			})

			t.Run("when fail register metrics", func(t *testing.T) {
				tc := suite.Case(t)

				exp := errors.New(uuid.NewString())

				reg := NewMockRegisterer(t)
				reg.EXPECT().Register(mock.Anything).Return(exp)

				_, err := New(
					t.Context(),
					WithNode(tc.Deps.Config.ConnString(), tc.Deps.Config.ConnString()),
					WithMetrics(reg),
				)

				require.Error(t, err)
			})
		})
	})
}

func TestSettings(t *testing.T) {
	t.Run("should be able to be broken", func(t *testing.T) {
		t.Run("when qps is not register", func(t *testing.T) {
			var settings Settings
			reg := NewMockRegisterer(t)
			settings.registry = reg
			exp := errors.New(uuid.NewString())

			reg.EXPECT().Register(mock.Anything).Return(exp)

			err := settings.registerMetricsCollector()
			require.ErrorIs(t, err, exp)
		})

		t.Run("when latencies is not register", func(t *testing.T) {
			reg := NewMockRegisterer(t)

			settings := Settings{
				registry:  reg,
				latencies: &prometheus.HistogramVec{},
				qps:       &prometheus.CounterVec{},
			}

			exp := errors.New(uuid.NewString())

			reg.EXPECT().Register(settings.qps).Return(nil)
			reg.EXPECT().Register(settings.latencies).Return(exp)

			err := settings.registerMetricsCollector()
			require.ErrorIs(t, err, exp)
		})

		t.Run("when latencies is not register", func(t *testing.T) {
			reg := NewMockRegisterer(t)

			settings := Settings{
				registry:  reg,
				latencies: &prometheus.HistogramVec{},
				qps:       &prometheus.CounterVec{},
			}

			exp := errors.New(uuid.NewString())

			reg.EXPECT().Register(mock.Anything).RunAndReturn(
				func(collector prometheus.Collector) error {
					switch collector.(type) {
					case *prometheus.HistogramVec:
						return nil
					case *prometheus.CounterVec:
						return nil
					default:
						return exp
					}
				},
			)

			err := settings.registerMetricsCollector()
			require.ErrorIs(t, err, exp)
		})
		t.Run("when register is fail for createSingleNode", func(t *testing.T) {
			tc := suite.Case(t)

			var settings Settings
			reg := NewMockRegisterer(t)

			settings.registry = reg
			settings.leaders = append(settings.leaders, tc.Deps.Config.ConnString())

			exp := errors.New(uuid.NewString())

			reg.EXPECT().Register(mock.Anything).Return(exp)

			_, err := createSingleNode(t.Context(), &settings)
			require.ErrorIs(t, err, exp)
		})
	})
}
