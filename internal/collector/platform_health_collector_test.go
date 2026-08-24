package collector

import (
	"context"
	"maps"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/prometheus/common/promslog"
	"github.com/vinted/sonic-exporter/pkg/redis"
)

func TestPlatformHealthCollectorProcessPercentFieldCompatibility(t *testing.T) {
	tests := []struct {
		name       string
		fields     map[string]string
		wantCPU    float64
		wantMemory float64
		wantCPUSet bool
		wantMemSet bool
	}{
		{
			name: "current percent fields take precedence",
			fields: map[string]string{
				"%CPU": "12.5",
				"CPU":  "99",
				"%MEM": "3.25",
				"MEM":  "88",
			},
			wantCPU: 12.5, wantMemory: 3.25, wantCPUSet: true, wantMemSet: true,
		},
		{
			name: "legacy fields remain supported",
			fields: map[string]string{
				"CPU": "7.5",
				"MEM": "2.25",
			},
			wantCPU: 7.5, wantMemory: 2.25, wantCPUSet: true, wantMemSet: true,
		},
		{
			name: "empty current fields fall back",
			fields: map[string]string{
				"%CPU": " ",
				"CPU":  "6",
				"%MEM": "",
				"MEM":  "1.5",
			},
			wantCPU: 6, wantMemory: 1.5, wantCPUSet: true, wantMemSet: true,
		},
		{
			name: "invalid current fields fall back",
			fields: map[string]string{
				"%CPU": "not-a-number",
				"CPU":  "5",
				"%MEM": "NaNx",
				"MEM":  "1",
			},
			wantCPU: 5, wantMemory: 1, wantCPUSet: true, wantMemSet: true,
		},
		{
			name: "absent current fields fall back",
			fields: map[string]string{
				"CPU": "4",
				"MEM": "0.75",
			},
			wantCPU: 4, wantMemory: 0.75, wantCPUSet: true, wantMemSet: true,
		},
		{
			name: "no numeric candidate emits no percent metrics",
			fields: map[string]string{
				"%CPU": "invalid",
				"CPU":  "",
				"%MEM": " ",
				"MEM":  "also-invalid",
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			server := miniredis.RunT(t)
			t.Setenv("REDIS_ADDRESS", server.Addr())
			t.Setenv("PLATFORM_HEALTH_ENABLED", "false")

			redisClient, err := redis.NewClient()
			if err != nil {
				t.Fatalf("failed to create Redis client: %v", err)
			}
			t.Cleanup(redisClient.Close)
			fields := map[string]string{"CMD": "/usr/bin/example", "UID": "1000"}
			maps.Copy(fields, testCase.fields)
			if err := redisClient.HsetToDb(context.Background(), "STATE_DB", "PROCESS_STATS|123", fields); err != nil {
				t.Fatalf("failed to write process data: %v", err)
			}

			logger := promslog.New(&promslog.Config{})
			collector := NewPlatformHealthCollector(logger, NewMetricFilter(logger))
			metrics, _, _, err := collector.scrapeMetrics(context.Background())
			if err != nil {
				t.Fatalf("scrapeMetrics returned an error: %v", err)
			}
			collector.cachedMetrics = metrics
			collector.config.enabled = true

			cpuFamily := getMetricFamily(t, collector, "sonic_platform_process_cpu_percent")
			memoryFamily := getMetricFamily(t, collector, "sonic_platform_process_memory_percent")
			labels := map[string]string{"pid": "123", "process": "example"}
			if got := metricWithLabelsExists(cpuFamily, labels, testCase.wantCPU); got != testCase.wantCPUSet {
				t.Fatalf("CPU metric presence=%v, want=%v", got, testCase.wantCPUSet)
			}
			if got := metricWithLabelsExists(memoryFamily, labels, testCase.wantMemory); got != testCase.wantMemSet {
				t.Fatalf("memory metric presence=%v, want=%v", got, testCase.wantMemSet)
			}
		})
	}
}
