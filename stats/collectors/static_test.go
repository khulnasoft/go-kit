package collectors_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/khulnasoft/go-kit/stats"
	"github.com/khulnasoft/go-kit/stats/collectors"
	"github.com/khulnasoft/go-kit/stats/memstats"
)

func TestStatic(t *testing.T) {
	testName := "test_sqlite"
	s := collectors.NewStaticMetric(testName, stats.Tags{
		"foo": "bar",
	}, 2)

	m, err := memstats.New()
	require.NoError(t, err)

	err = m.RegisterCollector(s)
	require.NoError(t, err)

	require.Equal(t, []memstats.Metric{
		{
			Name:  testName,
			Tags:  stats.Tags{"foo": "bar"},
			Value: 2,
		},
	}, m.GetAll())
}
