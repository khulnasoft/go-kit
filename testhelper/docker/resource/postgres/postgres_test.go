package postgres_test

import (
	"database/sql"
	"fmt"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"

	"github.com/khulnasoft/go-kit/testhelper/docker/resource/postgres"
)

func TestPostgres(t *testing.T) {
	pool, err := dockertest.NewPool("")
	require.NoError(t, err)

	for i := 1; i <= 6; i++ {
		t.Run(fmt.Sprintf("iteration %d", i), func(t *testing.T) {
			postgresContainer, err := postgres.Setup(pool, t)
			require.NoError(t, err)
			defer func() { _ = postgresContainer.DB.Close() }()

			db, err := sql.Open("postgres", postgresContainer.DBDsn)
			require.NoError(t, err)
			_, err = db.Exec("CREATE TABLE test (id int)")
			require.NoError(t, err)

			var count int
			err = db.QueryRow("SELECT count(*) FROM test").Scan(&count)
			require.NoError(t, err)
		})
	}

	t.Run("with test failure", func(t *testing.T) {
		cl := &testCleaner{T: t, failed: true}
		r, err := postgres.Setup(pool, cl, postgres.WithPrintLogsOnError(true))
		require.NoError(t, err)
		err = pool.Client.StopContainer(r.ContainerID, 10)
		require.NoError(t, err)
		cl.cleanup()
		require.Contains(t, cl.logs, "postgres container state: {Status:exited")
		require.Contains(t, cl.logs, "postgres container logs:")
	})

	t.Run("only ipv4 bindings", func(t *testing.T) {
		postgresContainer, err := postgres.Setup(pool, t)
		require.NoError(t, err)
		defer func() { _ = postgresContainer.DB.Close() }()

		{
			ipv4DSN := fmt.Sprintf(
				"postgres://%s:%s@%s:%s/%s?sslmode=disable",
				postgresContainer.User,
				postgresContainer.Password,
				"127.0.0.1",
				postgresContainer.Port,
				postgresContainer.Database,
			)
			db, err := sql.Open("postgres", ipv4DSN)
			require.NoError(t, err, "it should be able to open the sql db with an ipv4 address")
			require.NoError(t, db.Ping(), "it should be able to ping the db with an ipv4 address")
			require.NoError(t, db.Close(), "it should be able to close the sql db")
		}

		{
			ipv6DSN := fmt.Sprintf(
				"postgres://%s:%s@%s:%s/%s?sslmode=disable",
				postgresContainer.User,
				postgresContainer.Password,
				"::1",
				postgresContainer.Port,
				postgresContainer.Database,
			)
			db, err := sql.Open("postgres", ipv6DSN)
			require.NoError(t, err, "it should be able to open the sql db even with an ipv6 address")
			require.Error(t, db.Ping(), "it should not be able to ping the db with an ipv6 address")
			require.NoError(t, db.Close(), "it should be able to close the sql db")
		}
	})
}

type testCleaner struct {
	*testing.T
	cleanup func()
	failed  bool
	logs    string
}

func (t *testCleaner) Cleanup(f func()) {
	t.cleanup = f
}

func (t *testCleaner) Failed() bool {
	return t.failed
}

func (t *testCleaner) Log(args ...any) {
	t.logs = t.logs + fmt.Sprint(args...)
}
