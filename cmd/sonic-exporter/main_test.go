package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	commonversion "github.com/prometheus/common/version"
)

func buildCLI(t *testing.T) string {
	t.Helper()
	return buildCLIWithLdflags(t, "")
}

func buildCLIWithLdflags(t *testing.T, linkerFlags string) string {
	t.Helper()

	// Given
	binaryPath := filepath.Join(t.TempDir(), "sonic-exporter")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// When
	buildArguments := []string{"build", "-o", binaryPath}
	if linkerFlags != "" {
		buildArguments = append(buildArguments, "-ldflags", linkerFlags)
	}
	buildArguments = append(buildArguments, ".")
	output, err := exec.CommandContext(ctx, "go", buildArguments...).CombinedOutput()

	// Then
	if ctx.Err() != nil {
		t.Fatalf("build CLI binary: %v", ctx.Err())
	}
	if err != nil {
		t.Fatalf("build CLI binary: %v\n%s", err, output)
	}

	return binaryPath
}

func runCLI(t *testing.T, binaryPath string, arguments ...string) (string, error) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	output, err := exec.CommandContext(ctx, binaryPath, arguments...).CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("run CLI %q: %v\n%s", arguments, ctx.Err(), output)
	}

	return string(output), err
}

func TestCLI_help_includesExistingWebVRFFlag(t *testing.T) {
	// Given
	binaryPath := buildCLI(t)

	// When
	output, err := runCLI(t, binaryPath, "--help")

	// Then
	if err != nil {
		t.Fatalf("run CLI help: %v\n%s", err, output)
	}
	if !strings.Contains(output, "--web.vrf") {
		t.Fatalf("CLI help did not include --web.vrf:\n%s", output)
	}
}

func TestCLI_help_includesRootfs(t *testing.T) {
	// Given
	binaryPath := buildCLI(t)

	// When
	output, err := runCLI(t, binaryPath, "--help")

	// Then
	if err != nil {
		t.Fatalf("run CLI help: %v\n%s", err, output)
	}
	if !strings.Contains(output, "--path.rootfs") {
		t.Fatalf("CLI help did not include --path.rootfs:\n%s", output)
	}
}

func TestCLI_exitsNonZero_when_rootfsValueMissing(t *testing.T) {
	// Given
	binaryPath := buildCLI(t)

	// When
	output, err := runCLI(t, binaryPath, "--path.rootfs")

	// Then
	if err == nil {
		t.Fatalf("missing rootfs value exited zero:\n%s", output)
	}
	if !strings.Contains(output, "expected argument for flag '--path.rootfs'") {
		t.Fatalf("missing rootfs value did not report a parse error:\n%s", output)
	}
}

func TestCLI_rejectsImportedNodeExporterFlag(t *testing.T) {
	// Given
	binaryPath := buildCLI(t)

	// When
	output, err := runCLI(t, binaryPath, "--collector.filesystem.mount-points-exclude=^/tmp$")

	// Then
	if err == nil {
		t.Fatalf("imported node_exporter flag was accepted:\n%s", output)
	}
	if !strings.Contains(output, "unknown long flag '--collector.filesystem.mount-points-exclude'") {
		t.Fatalf("imported node_exporter flag did not report a parse error:\n%s", output)
	}
}

func TestNodeCollector_usesExistingCuratedCollectors(t *testing.T) {
	wantCollectors := []string{
		"loadavg",
		"cpu",
		"diskstats",
		"filesystem",
		"meminfo",
		"time",
		"stat",
	}

	gotCollectors := nodeCollectorNames[:]
	if !slices.Equal(gotCollectors, wantCollectors) {
		t.Errorf("node collector names = %q, want %q", gotCollectors, wantCollectors)
	}
}

func TestNodeCollectorArgs_forwardsRootfs(t *testing.T) {
	tests := []struct {
		name   string
		rootfs string
		want   []string
	}{
		{
			name:   "default root filesystem",
			rootfs: "/",
			want: []string{
				"--collector.loadavg",
				"--collector.cpu",
				"--collector.diskstats",
				"--collector.filesystem",
				"--collector.meminfo",
				"--collector.time",
				"--collector.stat",
				"--path.rootfs=/",
			},
		},
		{
			name:   "host filesystem",
			rootfs: "/hostfs",
			want: []string{
				"--collector.loadavg",
				"--collector.cpu",
				"--collector.diskstats",
				"--collector.filesystem",
				"--collector.meminfo",
				"--collector.time",
				"--collector.stat",
				"--path.rootfs=/hostfs",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rootfs := test.rootfs

			got := nodeCollectorArgs(rootfs)

			if !slices.Equal(got, test.want) {
				t.Errorf("node collector arguments = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCLI_version_printsPrometheusVersionAndExitsZero(t *testing.T) {
	// Given
	binaryPath := buildCLI(t)

	// When
	output, err := runCLI(t, binaryPath, "--version")

	// Then
	if err != nil {
		t.Fatalf("run CLI version: %v\n%s", err, output)
	}
	for _, expected := range []string{
		"sonic-exporter",
		"version unknown",
		"branch: unknown",
		"build user:       unknown",
		"build date:       unknown",
		"go version:",
		"platform:",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("CLI version output did not include %q:\n%s", expected, output)
		}
	}
}

func TestCLI_version_usesLinkerMetadata(t *testing.T) {
	// Given
	const (
		version   = "v0.6.0-test"
		revision  = "abcdef123456"
		buildDate = "2026-08-25T00:00:00Z"
	)
	linkerFlags := strings.Join([]string{
		"-X github.com/prometheus/common/version.Version=" + version,
		"-X github.com/prometheus/common/version.Revision=" + revision,
		"-X github.com/prometheus/common/version.BuildDate=" + buildDate,
	}, " ")
	binaryPath := buildCLIWithLdflags(t, linkerFlags)

	// When
	output, err := runCLI(t, binaryPath, "--version")

	// Then
	if err != nil {
		t.Fatalf("run CLI version: %v\n%s", err, output)
	}
	wantFields := strings.Join([]string{
		"sonic-exporter, version " + version + " (branch: unknown, revision: " + revision + ")",
		"  build user:       unknown",
		"  build date:       " + buildDate,
	}, "\n")
	if !strings.HasPrefix(output, wantFields+"\n") {
		t.Fatalf("CLI version output = %q, want fields %q", output, wantFields)
	}
}

func TestVersionCollector_usesUnknownLabels_whenLinkerFlagsMissing(t *testing.T) {
	// Given linker-provided version globals are empty.
	originalVersion := commonversion.Version
	originalBranch := commonversion.Branch
	originalRevision := commonversion.Revision
	originalBuildUser := commonversion.BuildUser
	originalBuildDate := commonversion.BuildDate
	t.Cleanup(func() {
		commonversion.Version = originalVersion
		commonversion.Branch = originalBranch
		commonversion.Revision = originalRevision
		commonversion.BuildUser = originalBuildUser
		commonversion.BuildDate = originalBuildDate
	})
	commonversion.Version = ""
	commonversion.Branch = ""
	commonversion.Revision = ""
	commonversion.BuildUser = ""
	commonversion.BuildDate = ""
	normalizeVersionMetadata()

	collector := newVersionCollector()
	registry := prometheus.NewPedanticRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatalf("failed to register version collector: %v", err)
	}

	// When the isolated registry gathers the collector.
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("failed to gather version collector: %v", err)
	}

	// Then it emits one constant build-info metric with unknown version labels.
	var buildInfo *dto.MetricFamily
	for _, metricFamily := range metricFamilies {
		if metricFamily.GetName() == "sonic_exporter_build_info" {
			if buildInfo != nil {
				t.Fatalf("found duplicate sonic_exporter_build_info metric families")
			}
			buildInfo = metricFamily
		}
	}
	if buildInfo == nil {
		t.Fatal("sonic_exporter_build_info metric family was not gathered")
	}
	if len(buildInfo.GetMetric()) != 1 {
		t.Fatalf("got %d build-info metrics, want 1", len(buildInfo.GetMetric()))
	}
	metric := buildInfo.GetMetric()[0]
	if got := metric.GetGauge().GetValue(); got != 1 {
		t.Fatalf("got build-info value %v, want 1", got)
	}
	labels := make(map[string]string, len(metric.GetLabel()))
	for _, label := range metric.GetLabel() {
		labels[label.GetName()] = label.GetValue()
	}
	if labels["version"] != "unknown" {
		t.Fatalf("got version label %q, want unknown", labels["version"])
	}
	if labels["branch"] != "unknown" {
		t.Fatalf("got branch label %q, want unknown", labels["branch"])
	}
}
