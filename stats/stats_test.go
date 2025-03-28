package stats

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/khulnasoft/go-kit/config"
	"github.com/khulnasoft/go-kit/logger"
	svcMetric "github.com/khulnasoft/go-kit/stats/metric"
)

func TestTagsType(t *testing.T) {
	tags := Tags{
		"b": "value1",
		"a": "value2",
	}

	t.Run("strings method", func(t *testing.T) {
		for i := 0; i < 100; i++ { // just making sure we are not just lucky with the order
			require.Equal(t, []string{"a", "value2", "b", "value1"}, tags.Strings())
		}
	})

	t.Run("string method", func(t *testing.T) {
		require.Equal(t, "a,value2,b,value1", tags.String())
	})

	t.Run("special character replacement", func(t *testing.T) {
		specialTags := Tags{
			"b:1": "value1:1",
			"a:1": "value2:2",
		}
		require.Equal(t, []string{"a-1", "value2-2", "b-1", "value1-1"}, specialTags.Strings())
	})

	t.Run("empty tags", func(t *testing.T) {
		emptyTags := Tags{}
		require.Nil(t, emptyTags.Strings())
		require.Equal(t, "", emptyTags.String())
	})
}

func TestUnstartedShouldNotPanicWhileTracing(t *testing.T) {
	require.NotPanics(t, func() {
		tr := Default.NewTracer("test")
		_, span := tr.Start(context.Background(), "span-name", SpanKindInternal)
		span.End()
	})
	require.NotPanics(t, func() {
		d := NewStats(config.New(), logger.Default, svcMetric.Instance)
		tr := d.NewTracer("test")
		_, span := tr.Start(context.Background(), "span-name", SpanKindInternal)
		span.End()
	})
}
