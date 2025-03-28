package stats_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"github.com/khulnasoft/go-kit/config"
	"github.com/khulnasoft/go-kit/logger"
	"github.com/khulnasoft/go-kit/stats"
	"github.com/khulnasoft/go-kit/stats/collectors"
	"github.com/khulnasoft/go-kit/stats/metric"
	"github.com/khulnasoft/go-kit/testhelper"
)

func TestStatsdMeasurementInvalidOperations(t *testing.T) {
	c := config.New()
	l := logger.NewFactory(c)
	m := metric.NewManager()
	s := stats.NewStats(c, l, m)

	t.Run("counter invalid operations", func(t *testing.T) {
		require.Panics(t, func() {
			s.NewStat("test", stats.CountType).Gauge(1)
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.CountType).Observe(1.2)
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.CountType).RecordDuration()
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.CountType).SendTiming(1)
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.CountType).Since(time.Now())
		})
	})

	t.Run("gauge invalid operations", func(t *testing.T) {
		require.Panics(t, func() {
			s.NewStat("test", stats.GaugeType).Increment()
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.GaugeType).Count(1)
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.GaugeType).Observe(1.2)
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.GaugeType).RecordDuration()
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.GaugeType).SendTiming(1)
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.GaugeType).Since(time.Now())
		})
	})

	t.Run("histogram invalid operations", func(t *testing.T) {
		require.Panics(t, func() {
			s.NewStat("test", stats.HistogramType).Increment()
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.HistogramType).Count(1)
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.HistogramType).Gauge(1)
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.HistogramType).RecordDuration()
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.HistogramType).SendTiming(1)
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.HistogramType).Since(time.Now())
		})
	})

	t.Run("timer invalid operations", func(t *testing.T) {
		require.Panics(t, func() {
			s.NewStat("test", stats.TimerType).Increment()
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.TimerType).Count(1)
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.TimerType).Gauge(1)
		})
		require.Panics(t, func() {
			s.NewStat("test", stats.TimerType).Observe(1.2)
		})
	})
}

func TestStatsdMeasurementOperations(t *testing.T) {
	var lastReceived atomic.Value
	server := newStatsdServer(t, func(s string) { lastReceived.Store(s) })
	defer server.Close()

	c := config.New()
	c.Set("STATSD_SERVER_URL", server.addr)
	c.Set("INSTANCE_ID", "test")
	c.Set("RuntimeStats.enabled", false)
	c.Set("statsSamplingRate", 0.5)

	l := logger.NewFactory(c)
	m := metric.NewManager()
	s := stats.NewStats(c, l, m)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start stats
	require.NoError(t, s.Start(ctx, stats.DefaultGoRoutineFactory))
	defer s.Stop()

	t.Run("counter increment", func(t *testing.T) {
		s.NewStat("test-counter", stats.CountType).Increment()

		require.Eventually(t, func() bool {
			return lastReceived.Load() == "test-counter,instanceName=test:1|c"
		}, 2*time.Second, time.Millisecond)
	})

	t.Run("counter count", func(t *testing.T) {
		s.NewStat("test-counter", stats.CountType).Count(10)

		require.Eventually(t, func() bool {
			return lastReceived.Load() == "test-counter,instanceName=test:10|c"
		}, 2*time.Second, time.Millisecond)
	})

	t.Run("gauge", func(t *testing.T) {
		s.NewStat("test-gauge", stats.GaugeType).Gauge(1234)

		require.Eventually(t, func() bool {
			return lastReceived.Load() == "test-gauge,instanceName=test:1234|g"
		}, 2*time.Second, time.Millisecond)
	})

	t.Run("timer send timing", func(t *testing.T) {
		s.NewStat("test-timer-1", stats.TimerType).SendTiming(10 * time.Second)

		require.Eventually(t, func() bool {
			return lastReceived.Load() == "test-timer-1,instanceName=test:10000|ms"
		}, 2*time.Second, time.Millisecond)
	})

	t.Run("timer since", func(t *testing.T) {
		s.NewStat("test-timer-2", stats.TimerType).Since(time.Now())

		require.Eventually(t, func() bool {
			return lastReceived.Load() == "test-timer-2,instanceName=test:0|ms"
		}, 2*time.Second, time.Millisecond)
	})

	t.Run("timer RecordDuration", func(t *testing.T) {
		func() {
			defer s.NewStat("test-timer-4", stats.TimerType).RecordDuration()()
		}()

		require.Eventually(t, func() bool {
			return lastReceived.Load() == "test-timer-4,instanceName=test:0|ms"
		}, 2*time.Second, time.Millisecond)
	})

	t.Run("histogram", func(t *testing.T) {
		s.NewStat("test-hist-1", stats.HistogramType).Observe(1.2)
		require.Eventually(t, func() bool {
			return lastReceived.Load() == "test-hist-1,instanceName=test:1.2|h"
		}, 2*time.Second, time.Millisecond)
	})

	t.Run("tagged stats", func(t *testing.T) {
		s.NewTaggedStat("test-tagged", stats.CountType, stats.Tags{"key": "value"}).Increment()
		require.Eventually(t, func() bool {
			return lastReceived.Load() == "test-tagged,instanceName=test,key=value:1|c"
		}, 2*time.Second, time.Millisecond)

		// same measurement name, different measurement type
		s.NewTaggedStat("test-tagged", stats.GaugeType, stats.Tags{"key": "value"}).Gauge(22)
		require.Eventually(t, func() bool {
			return lastReceived.Load() == "test-tagged,instanceName=test,key=value:22|g"
		}, 2*time.Second, time.Millisecond)
	})

	t.Run("sampled stats", func(t *testing.T) {
		lastReceived.Store("")
		// use the same, non-sampled counter first to make sure we don't get it from cache when we request the sampled one
		counter := s.NewTaggedStat("test-tagged-sampled", stats.CountType, stats.Tags{"key": "value"})
		counter.Increment()

		require.Eventually(t, func() bool {
			return lastReceived.Load() == "test-tagged-sampled,instanceName=test,key=value:1|c"
		}, 2*time.Second, time.Millisecond)

		counterSampled := s.NewSampledTaggedStat("test-tagged-sampled", stats.CountType, stats.Tags{"key": "value"})
		counterSampled.Increment()
		require.Eventually(t, func() bool {
			if lastReceived.Load() == "test-tagged-sampled,instanceName=test,key=value:1|c|@0.5" {
				return true
			}
			// playing with probabilities, we might or might not get the sample (0.5 -> 50% chance)
			counterSampled.Increment()
			return false
		}, 2*time.Second, time.Millisecond)
	})

	t.Run("measurement with empty name", func(t *testing.T) {
		s.NewStat("", stats.CountType).Increment()

		require.Eventually(t, func() bool {
			return lastReceived.Load() == "novalue,instanceName=test:1|c"
		}, 2*time.Second, time.Millisecond)
	})

	t.Run("measurement with empty name and empty tag key", func(t *testing.T) {
		s.NewTaggedStat(" ", stats.GaugeType, stats.Tags{"key": "value", "": "value2"}).Gauge(22)

		require.Eventually(t, func() bool {
			return lastReceived.Load() == "novalue,instanceName=test,key=value:22|g"
		}, 2*time.Second, time.Millisecond)
	})
}

func TestStatsdPeriodicStats(t *testing.T) {
	runTest := func(t *testing.T, prepareFunc func(c *config.Config, m metric.Manager), expected []string) {
		var received []string
		var receivedMu sync.RWMutex
		server := newStatsdServer(t, func(s string) {
			if i := strings.Index(s, ":"); i > 0 {
				s = s[:i]
			}
			receivedMu.Lock()
			received = append(received, s)
			receivedMu.Unlock()
		})
		defer server.Close()

		c := config.New()
		m := metric.NewManager()
		t.Setenv("KUBE_NAMESPACE", "my-namespace")
		c.Set("STATSD_SERVER_URL", server.addr)
		c.Set("INSTANCE_ID", "test")
		c.Set("RuntimeStats.enabled", true)
		c.Set("RuntimeStats.statsCollectionInterval", 60)
		prepareFunc(c, m)

		l := logger.NewFactory(c)
		s := stats.NewStats(c, l, m)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// start stats
		require.NoError(t, s.Start(ctx, stats.DefaultGoRoutineFactory))
		defer s.Stop()

		require.Eventually(t, func() bool {
			receivedMu.RLock()
			defer receivedMu.RUnlock()

			if len(received) != len(expected) {
				return false
			}
			return reflect.DeepEqual(received, expected)
		}, 10*time.Second, time.Millisecond)
	}

	t.Run("CPU stats", func(t *testing.T) {
		runTest(t, func(c *config.Config, m metric.Manager) {
			c.Set("RuntimeStats.enableCPUStats", true)
			c.Set("RuntimeStats.enabledMemStats", false)
			c.Set("RuntimeStats.enableGCStats", false)
		}, []string{
			"runtime_cpu.goroutines,instanceName=test,namespace=my-namespace",
			"runtime_cpu.cgo_calls,instanceName=test,namespace=my-namespace",
		})
	})

	t.Run("Mem stats", func(t *testing.T) {
		runTest(t, func(c *config.Config, m metric.Manager) {
			c.Set("RuntimeStats.enableCPUStats", false)
			c.Set("RuntimeStats.enabledMemStats", true)
			c.Set("RuntimeStats.enableGCStats", false)
		}, []string{
			"runtime_mem.alloc,instanceName=test,namespace=my-namespace",
			"runtime_mem.total,instanceName=test,namespace=my-namespace",
			"runtime_mem.sys,instanceName=test,namespace=my-namespace",
			"runtime_mem.lookups,instanceName=test,namespace=my-namespace",
			"runtime_mem.malloc,instanceName=test,namespace=my-namespace",
			"runtime_mem.frees,instanceName=test,namespace=my-namespace",
			"runtime_mem.heap.alloc,instanceName=test,namespace=my-namespace",
			"runtime_mem.heap.sys,instanceName=test,namespace=my-namespace",
			"runtime_mem.heap.idle,instanceName=test,namespace=my-namespace",
			"runtime_mem.heap.inuse,instanceName=test,namespace=my-namespace",
			"runtime_mem.heap.released,instanceName=test,namespace=my-namespace",
			"runtime_mem.heap.objects,instanceName=test,namespace=my-namespace",
			"runtime_mem.stack.inuse,instanceName=test,namespace=my-namespace",
			"runtime_mem.stack.sys,instanceName=test,namespace=my-namespace",
			"runtime_mem.stack.mspan_inuse,instanceName=test,namespace=my-namespace",
			"runtime_mem.stack.mspan_sys,instanceName=test,namespace=my-namespace",
			"runtime_mem.stack.mcache_inuse,instanceName=test,namespace=my-namespace",
			"runtime_mem.stack.mcache_sys,instanceName=test,namespace=my-namespace",
			"runtime_mem.othersys,instanceName=test,namespace=my-namespace",
		})
	})

	t.Run("MemGC stats", func(t *testing.T) {
		runTest(t, func(c *config.Config, m metric.Manager) {
			c.Set("RuntimeStats.enableCPUStats", false)
			c.Set("RuntimeStats.enabledMemStats", true)
			c.Set("RuntimeStats.enableGCStats", true)
		}, []string{
			"runtime_mem.alloc,instanceName=test,namespace=my-namespace",
			"runtime_mem.total,instanceName=test,namespace=my-namespace",
			"runtime_mem.sys,instanceName=test,namespace=my-namespace",
			"runtime_mem.lookups,instanceName=test,namespace=my-namespace",
			"runtime_mem.malloc,instanceName=test,namespace=my-namespace",
			"runtime_mem.frees,instanceName=test,namespace=my-namespace",
			"runtime_mem.heap.alloc,instanceName=test,namespace=my-namespace",
			"runtime_mem.heap.sys,instanceName=test,namespace=my-namespace",
			"runtime_mem.heap.idle,instanceName=test,namespace=my-namespace",
			"runtime_mem.heap.inuse,instanceName=test,namespace=my-namespace",
			"runtime_mem.heap.released,instanceName=test,namespace=my-namespace",
			"runtime_mem.heap.objects,instanceName=test,namespace=my-namespace",
			"runtime_mem.stack.inuse,instanceName=test,namespace=my-namespace",
			"runtime_mem.stack.sys,instanceName=test,namespace=my-namespace",
			"runtime_mem.stack.mspan_inuse,instanceName=test,namespace=my-namespace",
			"runtime_mem.stack.mspan_sys,instanceName=test,namespace=my-namespace",
			"runtime_mem.stack.mcache_inuse,instanceName=test,namespace=my-namespace",
			"runtime_mem.stack.mcache_sys,instanceName=test,namespace=my-namespace",
			"runtime_mem.othersys,instanceName=test,namespace=my-namespace",
			"runtime_mem.gc.sys,instanceName=test,namespace=my-namespace",
			"runtime_mem.gc.next,instanceName=test,namespace=my-namespace",
			"runtime_mem.gc.last,instanceName=test,namespace=my-namespace",
			"runtime_mem.gc.pause_total,instanceName=test,namespace=my-namespace",
			"runtime_mem.gc.pause,instanceName=test,namespace=my-namespace",
			"runtime_mem.gc.count,instanceName=test,namespace=my-namespace",
			"runtime_mem.gc.cpu_percent,instanceName=test,namespace=my-namespace",
		})
	})

	t.Run("Pending events", func(t *testing.T) {
		runTest(t, func(c *config.Config, m metric.Manager) {
			c.Set("RuntimeStats.enableCPUStats", false)
			c.Set("RuntimeStats.enabledMemStats", false)
			c.Set("RuntimeStats.enableGCStats", false)
			m.GetRegistry(metric.PublishedMetrics).MustGetGauge(TestMeasurement{tablePrefix: "table", workspace: "workspace", destType: "destType"}).Set(1.0)
		}, []string{
			"test_measurement_table,instanceName=test,namespace=my-namespace,destType=destType,workspaceId=workspace",
		})
	})
}

func TestStatsdRegisterCollector(t *testing.T) {
	runTest := func(t *testing.T, expected []string, cols ...stats.Collector) {
		var received []string
		var receivedMu sync.RWMutex
		server := newStatsdServer(t, func(s string) {
			if i := strings.Index(s, ":"); i > 0 {
				s = s[:i]
			}
			receivedMu.Lock()
			received = append(received, s)
			receivedMu.Unlock()
		})
		defer server.Close()

		c := config.New()
		m := metric.NewManager()
		t.Setenv("KUBE_NAMESPACE", "my-namespace")
		c.Set("STATSD_SERVER_URL", server.addr)
		c.Set("INSTANCE_ID", "test")
		c.Set("RuntimeStats.enabled", true)
		c.Set("RuntimeStats.statsCollectionInterval", 60)
		c.Set("RuntimeStats.enableCPUStats", false)
		c.Set("RuntimeStats.enabledMemStats", false)
		c.Set("RuntimeStats.enableGCStats", false)

		l := logger.NewFactory(c)
		s := stats.NewStats(c, l, m)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		for _, col := range cols {
			err := s.RegisterCollector(col)
			require.NoError(t, err)
		}

		// start stats
		require.NoError(t, s.Start(ctx, stats.DefaultGoRoutineFactory))
		defer s.Stop()

		defer func() {
			receivedMu.RLock()
			defer receivedMu.RUnlock()

			t.Logf("received: %s \n!=\n expected: %s", received, expected)
		}()

		require.Eventually(t, func() bool {
			receivedMu.RLock()
			defer receivedMu.RUnlock()

			if len(received) != len(expected) {
				return false
			}

			sort.Strings(received)
			sort.Strings(expected)

			return reflect.DeepEqual(received, expected)
		}, 10*time.Second, time.Millisecond)
	}

	t.Run("static stats", func(t *testing.T) {
		runTest(t,
			[]string{"a_custom_metric,instanceName=test,namespace=my-namespace"},
			collectors.NewStaticMetric("a_custom_metric", nil, 1),
		)

		runTest(t,
			[]string{"a_custom_metric,instanceName=test,namespace=my-namespace,foo=bar"},
			collectors.NewStaticMetric("a_custom_metric", stats.Tags{"foo": "bar"}, 1),
		)
	})

	t.Run("multiple collectors", func(t *testing.T) {
		runTest(t,
			[]string{
				"col_1,instanceName=test,namespace=my-namespace",
				"col_2,instanceName=test,namespace=my-namespace",
				"col_3,instanceName=test,namespace=my-namespace",
			},
			collectors.NewStaticMetric("col_1", nil, 1),
			collectors.NewStaticMetric("col_2", nil, 1),
			collectors.NewStaticMetric("col_3", nil, 1),
		)
	})

	t.Run("sql collector", func(t *testing.T) {
		db, _, err := sqlmock.New()
		if err != nil {
			t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
		}
		defer db.Close()

		runTest(t,
			[]string{
				"sql_db_max_open_connections,instanceName=test,namespace=my-namespace,name=test",
				"sql_db_open_connections,instanceName=test,namespace=my-namespace,name=test",
				"sql_db_in_use_connections,instanceName=test,namespace=my-namespace,name=test",
				"sql_db_idle_connections,instanceName=test,namespace=my-namespace,name=test",
				"sql_db_wait_count_total,instanceName=test,namespace=my-namespace,name=test",
				"sql_db_wait_duration_seconds_total,instanceName=test,namespace=my-namespace,name=test",
				"sql_db_max_idle_closed_total,instanceName=test,namespace=my-namespace,name=test",
				"sql_db_max_idle_time_closed_total,instanceName=test,namespace=my-namespace,name=test",
				"sql_db_max_lifetime_closed_total,instanceName=test,namespace=my-namespace,name=test",
			},
			collectors.NewDatabaseSQLStats("test", db),
		)
	})

	t.Run("error on duplicate collector", func(t *testing.T) {
		c := config.New()
		m := metric.NewManager()
		l := logger.NewFactory(c)
		s := stats.NewStats(c, l, m)

		err := s.RegisterCollector(collectors.NewStaticMetric("col_1", nil, 1))
		require.NoError(t, err)

		err = s.RegisterCollector(collectors.NewStaticMetric("col_1", nil, 1))
		require.Error(t, err)
	})
}

func TestStatsdExcludedTags(t *testing.T) {
	var lastReceived atomic.Value
	server := newStatsdServer(t, func(s string) { lastReceived.Store(s) })
	defer server.Close()

	c := config.New()
	c.Set("STATSD_SERVER_URL", server.addr)
	c.Set("statsExcludedTags", []string{"workspaceId"})
	c.Set("INSTANCE_ID", "test")
	c.Set("RuntimeStats.enabled", false)

	l := logger.NewFactory(c)
	m := metric.NewManager()
	s := stats.NewStats(c, l, m)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// start stats
	require.NoError(t, s.Start(ctx, stats.DefaultGoRoutineFactory))
	defer s.Stop()

	c.Set("statsExcludedTags", []string{"workspaceId"})
	s.NewTaggedStat("test-workspaceId", stats.CountType, stats.Tags{"workspaceId": "value"}).Increment()
	require.Eventually(t, func() bool {
		return lastReceived.Load() == "test-workspaceId,instanceName=test:1|c"
	}, 2*time.Second, time.Millisecond)
}

type statsdServer struct {
	t      *testing.T
	addr   string
	closer io.Closer
	closed chan bool
}

func newStatsdServer(t *testing.T, f func(string)) *statsdServer {
	port, err := testhelper.GetFreePort()
	require.NoError(t, err)
	addr := net.JoinHostPort("localhost", strconv.Itoa(port))
	s := &statsdServer{t: t, closed: make(chan bool)}
	laddr, err := net.ResolveUDPAddr("udp", addr)
	require.NoError(t, err)
	conn, err := net.ListenUDP("udp", laddr)
	require.NoError(t, err)
	s.closer = conn
	s.addr = conn.LocalAddr().String()
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				s.closed <- true
				return
			}
			s := string(buf[:n])
			lines := strings.Split(s, "\n")
			if n > 0 {
				for _, line := range lines {
					f(line)
				}
			}
		}
	}()

	return s
}

func (s *statsdServer) Close() {
	require.NoError(s.t, s.closer.Close())
	<-s.closed
}

type TestMeasurement struct {
	tablePrefix string
	workspace   string
	destType    string
}

func (r TestMeasurement) GetName() string {
	return fmt.Sprintf("test_measurement_%s", r.tablePrefix)
}

func (r TestMeasurement) GetTags() map[string]string {
	return map[string]string{
		"workspaceId": r.workspace,
		"destType":    r.destType,
	}
}
