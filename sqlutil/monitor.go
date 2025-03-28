package sqlutil

import (
	"context"
	"database/sql"
	"time"

	"github.com/khulnasoft/go-kit/config"
	"github.com/khulnasoft/go-kit/stats"
)

// MonitorDatabase collects database connection pool metrics at regular intervals synchronously until the context is canceled.
func MonitorDatabase(
	ctx context.Context,
	conf *config.Config,
	statsFactory stats.Stats,
	db *sql.DB,
	identifier string,
) {
	statsReportInterval := conf.GetDurationVar(10, time.Second, "Database.statsReportInterval")

	tags := stats.Tags{
		"identifier": identifier,
	}

	maxOpenConnectionsStat := statsFactory.NewTaggedStat("db_max_open_connections", stats.GaugeType, tags)
	openConnectionsStat := statsFactory.NewTaggedStat("db_open_connections", stats.GaugeType, tags)
	inUseStat := statsFactory.NewTaggedStat("db_in_use", stats.GaugeType, tags)
	idleStat := statsFactory.NewTaggedStat("db_idle", stats.GaugeType, tags)
	waitCountStat := statsFactory.NewTaggedStat("db_wait_count", stats.GaugeType, tags)
	waitDurationStat := statsFactory.NewTaggedStat("db_wait_duration", stats.TimerType, tags)
	maxIdleClosedStat := statsFactory.NewTaggedStat("db_max_idle_closed", stats.GaugeType, tags)
	maxIdleTimeClosedStat := statsFactory.NewTaggedStat("db_max_idle_time_closed", stats.GaugeType, tags)
	maxLifetimeClosedStat := statsFactory.NewTaggedStat("db_max_lifetime_closed", stats.GaugeType, tags)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(statsReportInterval):
			dbStats := db.Stats()

			maxOpenConnectionsStat.Gauge(dbStats.MaxOpenConnections)
			openConnectionsStat.Gauge(dbStats.OpenConnections)
			inUseStat.Gauge(dbStats.InUse)
			idleStat.Gauge(dbStats.Idle)
			waitCountStat.Gauge(int(dbStats.WaitCount))
			waitDurationStat.SendTiming(dbStats.WaitDuration)
			maxIdleClosedStat.Gauge(int(dbStats.MaxIdleClosed))
			maxIdleTimeClosedStat.Gauge(int(dbStats.MaxIdleTimeClosed))
			maxLifetimeClosedStat.Gauge(int(dbStats.MaxLifetimeClosed))
		}
	}
}
