package collector

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/promslog"
)

func TestTransceiverIdentityCollector_emits_trimmed_identity_without_status(t *testing.T) {
	// Given
	logger := promslog.New(&promslog.Config{})
	collector := NewTransceiverCollector(logger, NewMetricFilter(logger))
	want := `
		# HELP sonic_transceiver_identity_info Transceiver hardware identity metadata, value is always 1
		# TYPE sonic_transceiver_identity_info gauge
		sonic_transceiver_identity_info{device="Ethernet24",manufacturer="Example Optics",model="EX-100G-CR5",serial="EXAMPLE0001",type="QSFP28 or later",vendor_oui="00-00-00",vendor_rev="A"} 1
		sonic_transceiver_identity_info{device="Ethernet248",manufacturer="Example Modules",model="EX-400G-DAC1",serial="EXAMPLE0002",type="QSFP-DD Double Density 8X Pluggable Transceiver",vendor_oui="00-00-01",vendor_rev="20"} 1
		sonic_transceiver_identity_info{device="Ethernet252",manufacturer="Example Modules",model="EX-400G-DAC2",serial="EXAMPLE0003",type="",vendor_oui="",vendor_rev=""} 1
	`

	// When / Then
	if err := testutil.CollectAndCompare(collector, strings.NewReader(want), "sonic_transceiver_identity_info"); err != nil {
		t.Fatalf("unexpected identity metrics:\n%s", err)
	}
}

func TestTransceiverIdentityCollector_respects_metric_filter(t *testing.T) {
	// Given
	t.Setenv("SONIC_DISABLED_METRICS", "sonic_transceiver_identity_info")
	logger := promslog.New(&promslog.Config{})

	// When
	collector := NewTransceiverCollector(logger, NewMetricFilter(logger))

	// Then
	assertMetricFamilyPresence(t, collector, "sonic_transceiver_identity_info", false)
	assertMetricFamilyPresence(t, collector, "sonic_transceiver_module_info", true)
}

func TestTransceiverIdentityCollector_applies_sorted_port_limit(t *testing.T) {
	// Given
	t.Setenv("TRANSCEIVER_MAX_PORTS", "2")
	logger := promslog.New(&promslog.Config{})

	// When
	collector := NewTransceiverCollector(logger, NewMetricFilter(logger))

	// Then
	identityFamily := getMetricFamily(t, collector, "sonic_transceiver_identity_info")
	if len(identityFamily.GetMetric()) != 2 {
		t.Fatalf("identity metric count=%d, want=2", len(identityFamily.GetMetric()))
	}
	if !metricWithLabelsExists(identityFamily, map[string]string{"device": "Ethernet24"}, 1) {
		t.Fatal("expected sorted identity entry for Ethernet24")
	}
	if !metricWithLabelsExists(identityFamily, map[string]string{"device": "Ethernet248"}, 1) {
		t.Fatal("expected sorted identity entry for Ethernet248")
	}

	truncatedFamily := getMetricFamily(t, collector, "sonic_transceiver_entries_truncated")
	if !metricWithLabelsExists(truncatedFamily, nil, 1) {
		t.Fatal("expected truncation signal")
	}
}

func TestIdentityIsEmpty_returns_true_when_selected_fields_are_blank(t *testing.T) {
	// Given
	identity := map[string]string{"manufacturer": "   "}

	// When
	empty := identityIsEmpty(identity)

	// Then
	if !empty {
		t.Fatal("blank selected fields should be empty")
	}
}
