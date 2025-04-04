package collectors

import (
	"database/sql"
	"fmt"

	"github.com/khulnasoft/go-kit/stats"
)

const (
	uniqName = "database_sql_%s"
)

type SQLDBStats struct {
	name string
	db   *sql.DB
}

func NewDatabaseSQLStats(name string, db *sql.DB) *SQLDBStats {
	return &SQLDBStats{
		name: name,
		db:   db,
	}
}

func (s *SQLDBStats) Collect(gaugeFunc func(key string, tag stats.Tags, val uint64)) {
	dbStats := s.db.Stats()
	tags := stats.Tags{"name": s.name}

	gaugeFunc("sql_db_max_open_connections", tags, uint64(dbStats.MaxOpenConnections))
	gaugeFunc("sql_db_open_connections", tags, uint64(dbStats.OpenConnections))
	gaugeFunc("sql_db_in_use_connections", tags, uint64(dbStats.InUse))
	gaugeFunc("sql_db_idle_connections", tags, uint64(dbStats.Idle))

	gaugeFunc("sql_db_wait_count_total", tags, uint64(dbStats.WaitCount))
	gaugeFunc("sql_db_wait_duration_seconds_total", tags, uint64(dbStats.WaitDuration.Seconds()))

	gaugeFunc("sql_db_max_idle_closed_total", tags, uint64(dbStats.MaxIdleClosed))
	gaugeFunc("sql_db_max_idle_time_closed_total", tags, uint64(dbStats.MaxIdleTimeClosed))
	gaugeFunc("sql_db_max_lifetime_closed_total", tags, uint64(dbStats.MaxLifetimeClosed))
}

func (s *SQLDBStats) Zero(gaugeFunc func(key string, tag stats.Tags, val uint64)) {
	tags := stats.Tags{"name": s.name}

	gaugeFunc("sql_db_max_open_connections", tags, 0)

	gaugeFunc("sql_db_open_connections", tags, 0)
	gaugeFunc("sql_db_in_use_connections", tags, 0)
	gaugeFunc("sql_db_idle_connections", tags, 0)

	gaugeFunc("sql_db_wait_count_total", tags, 0)
	gaugeFunc("sql_db_wait_duration_seconds_total", tags, 0)

	gaugeFunc("sql_db_max_idle_closed_total", tags, 0)
	gaugeFunc("sql_db_max_idle_time_closed_total", tags, 0)
	gaugeFunc("sql_db_max_lifetime_closed_total", tags, 0)
}

func (s *SQLDBStats) ID() string {
	return fmt.Sprintf(uniqName, s.name)
}
