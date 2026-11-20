package trunk

import (
	"context"
	"errors"
	"strconv"

	"github.com/godepo/elephant"
	"github.com/godepo/elephant/clusterpg"
	"github.com/godepo/elephant/internal/sharded"
	"github.com/godepo/elephant/poolstats"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultLeadersDSNSize = 4
)

var (
	ErrDSNNotPresent               = errors.New("DSN to connect is not present")
	ErrInvalidClusterConfiguration = errors.New("invalid cluster configuration")
	ErrRequirePicker               = errors.New("picker is required for sharded configuration")
)

type (
	Settings struct {
		leaders          []string
		followers        [][]string
		registry         prometheus.Registerer
		picker           sharded.Picker
		latencies        *prometheus.HistogramVec
		qps              *prometheus.CounterVec
		followersCount   int
		poolStatsOptions []poolstats.Option
	}

	roleRules struct {
		shardID string
		role    string
		id      string

		labelsValues             []string
		metricsCollectorProvider func() elephant.MetricsBuilder
	}

	roleRule func(rules *roleRules)

	Option func(*Settings)

	Instance interface {
		BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
		Begin(ctx context.Context) (pgx.Tx, error)
		Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
		QueryRow(ctx context.Context, query string, args ...any) pgx.Row
		Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
		SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults
		CopyFrom(ctx context.Context,
			tableName pgx.Identifier,
			columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
		Transactional(ctx context.Context, fn func(ctx context.Context) error) (out error)
	}

	dbCloser func()

	DB struct {
		Instance
		closers  []dbCloser
		informer poolstats.InformerProvider
	}
)

func WithNode(leaderDSN string, followersDSNs ...string) Option {
	return func(settings *Settings) {
		followers := make([]string, len(followersDSNs))
		copy(followers, followersDSNs)

		settings.leaders = append(settings.leaders, leaderDSN)
		settings.followers = append(settings.followers, followers)
		settings.followersCount += len(followers)
	}
}

func defaultSettings() *Settings {
	return &Settings{
		leaders:   make([]string, 0, defaultLeadersDSNSize),
		followers: make([][]string, 0, defaultLeadersDSNSize),
	}
}

func WithMetrics(reg prometheus.Registerer) Option {
	return func(settings *Settings) {
		settings.registry = reg
	}
}

func WithPicker(picker sharded.Picker) Option {
	return func(settings *Settings) {
		settings.picker = picker
	}
}

func New(ctx context.Context, options ...Option) (*DB, error) {
	settings := defaultSettings().With(options...)

	err := settings.validate()
	if err != nil {
		return nil, err
	}

	labelsNames := settings.getLabelsNames()

	settings.init(labelsNames...)

	settings.poolStatsOptions = []poolstats.Option{
		poolstats.WithLabelsNames(labelsNames[2:]...),
	}

	if len(settings.leaders) == 1 && len(settings.followers) == 1 && len(settings.followers[0]) == 0 {
		return createSingleNode(ctx, settings)
	}

	dbs, err := createCluster(ctx, settings)
	if err != nil {
		return nil, err
	}

	err = settings.registerMetricsCollector()
	if err != nil {
		return nil, err
	}

	if len(dbs) == 1 {
		return dbs[0], nil
	}

	pools := make([]sharded.Pool, 0, len(dbs))
	closers := make([]dbCloser, 0, len(dbs))

	for _, db := range dbs {
		pools = append(pools, db.Instance)
		closers = append(closers, db.closers...)
	}

	return &DB{
		Instance: sharded.New(pools, settings.picker),
		closers:  closers,
	}, nil
}

func createCluster(ctx context.Context, settings *Settings) ([]*DB, error) {
	dbs := make([]*DB, 0, len(settings.leaders))

	for leaderID := range settings.leaders {
		followers, err := makeFollowers(ctx, settings, leaderID)
		if err != nil {
			return nil, err
		}

		nodeOpts := makeNodeOpts(settings, leaderID)

		leader, err := newNode(ctx, settings.leaders[leaderID], settings, nodeOpts...)
		if err != nil {
			return nil, err
		}

		if len(followers) > 0 {
			createHACluster(leader, followers)
		}

		dbs = append(dbs, leader)
	}
	return dbs, nil
}

func makeFollowers(ctx context.Context, settings *Settings, leaderID int) ([]*DB, error) {
	followers := make([]*DB, 0, len(settings.followers))

	for followerID, dsn := range settings.followers[leaderID] {
		rulesOpts := make([]roleRule, 0, defaultRolesLabelsSize)

		if len(settings.leaders) > 1 {
			rulesOpts = append(rulesOpts,
				withShardID(leaderID),
			)
		}

		rulesOpts = append(rulesOpts,
			withRole("follower"),
			withID(followerID),
		)

		db, err := newNode(ctx, dsn, settings,
			rulesOpts...,
		)
		if err != nil {
			return nil, err
		}

		followers = append(followers, db)
	}

	return followers, nil
}

func makeNodeOpts(settings *Settings, leaderID int) []roleRule {
	nodeOpts := make([]roleRule, 0, defaultRolesLabelsSize)

	if len(settings.leaders) > 1 {
		nodeOpts = append(nodeOpts, withShardID(leaderID))
	}

	nodeOpts = append(nodeOpts, withRole("leader"))
	if settings.followersCount > 0 {
		nodeOpts = append(nodeOpts, withID(0))
	}
	return nodeOpts
}

func createHACluster(leader *DB, followers []*DB) {
	builder := clusterpg.New().
		Leader(func() (clusterpg.Pool, error) {
			return leader.Instance, nil
		})

	followersClosers := make([]dbCloser, 0, len(followers))

	for _, db := range followers {
		builder = builder.Follower(func() (clusterpg.Pool, error) {
			return db.Instance, nil
		})
		followersClosers = append(followersClosers, db.closers...)
	}

	db, _ := builder.Go()

	leader.Instance = db
	leader.closers = append(leader.closers, followersClosers...)
}

func createSingleNode(ctx context.Context, settings *Settings) (*DB, error) {
	node, err := newNode(ctx, settings.leaders[0], settings)
	if err != nil {
		return nil, err
	}
	settings.poolStatsOptions = append(settings.poolStatsOptions, poolstats.WithInformerProvider(node.informer))

	err = settings.registerMetricsCollector()
	if err != nil {
		return nil, err
	}
	return node, nil
}

func withID(id int) roleRule {
	return func(rules *roleRules) {
		rules.id = strconv.Itoa(id)
	}
}

func withRole(role string) roleRule {
	return func(rules *roleRules) {
		rules.role = role
	}
}

func withShardID(id int) roleRule {
	return func(rules *roleRules) {
		rules.shardID = strconv.Itoa(id)
	}
}
