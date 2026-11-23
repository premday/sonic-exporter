package collector

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestInterfaceCollectorFEC(t *testing.T) {
	const (
		correctable = `
# HELP sonic_interface_fec_correctable_frames_total Number of received FEC frames with correctable errors
# TYPE sonic_interface_fec_correctable_frames_total counter
sonic_interface_fec_correctable_frames_total{device="Ethernet0"} 42
`
		uncorrectable = `
# HELP sonic_interface_fec_uncorrectable_frames_total Number of received FEC frames with uncorrectable errors
# TYPE sonic_interface_fec_uncorrectable_frames_total counter
sonic_interface_fec_uncorrectable_frames_total{device="Ethernet0"} 0
sonic_interface_fec_uncorrectable_frames_total{device="Ethernet39"} 9
`
		symbolErrors = `
# HELP sonic_interface_fec_symbol_errors_total Number of received FEC symbol errors
# TYPE sonic_interface_fec_symbol_errors_total counter
sonic_interface_fec_symbol_errors_total{device="Ethernet0"} 17
`
		success = `
# HELP sonic_interface_collector_success Whether interface collector succeeded
# TYPE sonic_interface_collector_success gauge
sonic_interface_collector_success 1
`
	)
	for _, tc := range []struct {
		name, enabled, disabledMetrics, want string
		warnings                             int
	}{
		{name: "default"},
		{name: "disabled", enabled: "false"},
		{name: "invalid config uses default", enabled: "invalid"},
		{name: "enabled", enabled: "true", want: correctable + uncorrectable + symbolErrors, warnings: 3},
		{name: "exact filter", enabled: "true", disabledMetrics: "sonic_interface_fec_correctable_frames_total", want: uncorrectable + symbolErrors, warnings: 2},
		{name: "wildcard filter", enabled: "true", disabledMetrics: "sonic_interface_fec_*"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FEC_ENABLED", tc.enabled)
			t.Setenv("SONIC_DISABLED_METRICS", tc.disabledMetrics)
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			collector := NewInterfaceCollector(logger, NewMetricFilter(logger))

			if err := testutil.CollectAndCompare(collector, strings.NewReader(success+tc.want),
				"sonic_interface_collector_success",
				"sonic_interface_fec_correctable_frames_total",
				"sonic_interface_fec_uncorrectable_frames_total",
				"sonic_interface_fec_symbol_errors_total",
			); err != nil {
				t.Fatalf("unexpected interface metrics:\n%s", err)
			}
			assertMetricFamilyPresence(t, collector, "sonic_interface_receive_bytes_total", true)
			problems, err := testutil.CollectAndLint(collector)
			if err != nil || len(problems) != 0 {
				t.Errorf("Prometheus lint: problems=%v, error=%v", problems, err)
			}
			if got := strings.Count(logs.String(), "Invalid interface FEC counter, omitting sample"); got != tc.warnings {
				t.Errorf("got %d malformed FEC counter warnings, want %d; logs:\n%s", got, tc.warnings, logs.String())
			}
		})
	}
}
