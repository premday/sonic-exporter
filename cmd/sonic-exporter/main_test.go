package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func buildCLI(t *testing.T) string {
	t.Helper()

	// Given
	binaryPath := filepath.Join(t.TempDir(), "sonic-exporter")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// When
	output, err := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, ".").CombinedOutput()

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
