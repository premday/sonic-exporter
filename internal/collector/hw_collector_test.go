package collector

import (
	"context"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/common/promslog"
	"github.com/vinted/sonic-exporter/pkg/redis"
)

func TestHwCollectorScrapeMetrics_emitsSlotForPsuKeyForms(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		wantSlot    string
		assertsSlot bool
		wantsError  bool
	}{
		{
			name:        "unspaced live key",
			key:         "PSU_INFO|PSU1",
			wantSlot:    "1",
			assertsSlot: true,
		},
		{
			name:        "spaced fixture key",
			key:         "PSU_INFO|PSU 1",
			wantSlot:    "1",
			assertsSlot: true,
		},
		{
			name:       "empty suffix",
			key:        "PSU_INFO|PSU",
			wantsError: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			server := miniredis.RunT(t)
			t.Setenv("REDIS_ADDRESS", server.Addr())
			ctx := context.Background()
			redisClient, err := redis.NewClient()
			if err != nil {
				t.Fatalf("failed to create Redis client: %v", err)
			}
			t.Cleanup(redisClient.Close)

			err = redisClient.HsetToDb(ctx, "STATE_DB", testCase.key, map[string]string{
				"presence": "true",
				"status":   "true",
				"model":    "PSU-AC",
				"serial":   "PSU-SERIAL-1",
				"name":     "PSU Model",
				"voltage":  "12.4",
				"current":  "5",
				"power":    "60",
				"temp":     "35",
			})
			if err != nil {
				t.Fatalf("failed to write PSU data: %v", err)
			}

			logger := promslog.New(&promslog.Config{})
			collector := NewHwCollector(logger, NewMetricFilter(logger))

			err = collector.scrapeMetrics(ctx)

			if testCase.wantsError {
				if err == nil {
					t.Fatalf("scrapeMetrics returned no error for PSU key %q", testCase.key)
				}
				if !strings.Contains(err.Error(), testCase.key) {
					t.Fatalf("scrapeMetrics error %q does not include PSU key %q", err, testCase.key)
				}
				if len(collector.cachedMetrics) != 0 {
					t.Fatalf("scrapeMetrics emitted metrics for malformed PSU key %q", testCase.key)
				}
				return
			}
			if err != nil {
				t.Fatalf("scrapeMetrics returned an error: %v", err)
			}
			if testCase.assertsSlot && !metricWithLabelsExists(
				getMetricFamily(t, collector, hwPsuInfoMetricName),
				map[string]string{"slot": testCase.wantSlot},
				1,
			) {
				t.Fatalf("PSU key %q did not emit slot %q", testCase.key, testCase.wantSlot)
			}
		})
	}
}
