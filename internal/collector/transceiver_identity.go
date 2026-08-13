package collector

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/vinted/sonic-exporter/pkg/redis"
)

func (collector *transceiverCollector) scrapeIdentityMetrics(ctx context.Context, redisClient redis.Client) ([]prometheus.Metric, int, bool, error) {
	identityKeys, err := redisClient.ScanKeysFromDb(ctx, "STATE_DB", "TRANSCEIVER_INFO|*", collector.config.redisScanCount)
	if err != nil {
		return nil, 0, false, fmt.Errorf("failed to scan transceiver identity keys: %w", err)
	}
	sort.Strings(identityKeys)

	metrics := make([]prometheus.Metric, 0, min(len(identityKeys), collector.config.maxPorts))
	skippedEntries := 0
	truncated := false
	for index, identityKey := range identityKeys {
		if index >= collector.config.maxPorts {
			truncated = true
			skippedEntries += len(identityKeys) - index
			break
		}

		device, err := parseKeySuffix(identityKey, "TRANSCEIVER_INFO|")
		if err != nil {
			skippedEntries++
			continue
		}

		identity, err := redisClient.HgetAllFromDb(ctx, "STATE_DB", identityKey)
		if err != nil {
			return nil, 0, false, fmt.Errorf("failed to read transceiver identity entry %s: %w", identityKey, err)
		}
		if identityIsEmpty(identity) {
			skippedEntries++
			continue
		}
		if !collector.metricFilter.Enabled("sonic_transceiver_identity_info") {
			continue
		}

		metrics = append(metrics, prometheus.MustNewConstMetric(
			collector.identityInfo,
			prometheus.GaugeValue,
			1,
			device,
			strings.TrimSpace(identity["manufacturer"]),
			strings.TrimSpace(identity["model"]),
			strings.TrimSpace(identity["serial"]),
			strings.TrimSpace(identity["vendor_rev"]),
			strings.TrimSpace(identity["vendor_oui"]),
			strings.TrimSpace(identity["type"]),
		))
	}

	return metrics, skippedEntries, truncated, nil
}

func identityIsEmpty(identity map[string]string) bool {
	for _, field := range []string{"manufacturer", "model", "serial", "vendor_rev", "vendor_oui", "type"} {
		if strings.TrimSpace(identity[field]) != "" {
			return false
		}
	}
	return true
}
