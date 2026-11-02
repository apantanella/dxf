package main

import (
	"strings"
	"testing"
)

func TestRunWithoutArguments(t *testing.T) {
	t.Parallel()

	var stdout, stderr strings.Builder
	if status := run(nil, &stdout, &stderr); status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
	if !strings.Contains(stderr.String(), "usage: dxfinspect") {
		t.Errorf("stderr = %q, want the usage message", stderr.String())
	}
}

func TestRunWithUnknownFlag(t *testing.T) {
	t.Parallel()

	var stdout, stderr strings.Builder
	if status := run([]string{"-unknown"}, &stdout, &stderr); status != 2 {
		t.Fatalf("status = %d, want 2", status)
	}
}

func TestRunInspectsFixture(t *testing.T) {
	t.Parallel()

	var stdout, stderr strings.Builder
	status := run([]string{"../../testdata/structural-frame.dxf"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{
		"version: AC1009",
		"blocks: 0",
		"layers: 1",
		"entities: 6",
		"LINE",
		"POLYLINE",
		"POINT",
		"TEXT",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q:\n%s", want, output)
		}
	}
}

func TestRunOnMissingFile(t *testing.T) {
	t.Parallel()

	var stdout, stderr strings.Builder
	if status := run([]string{"does-not-exist.dxf"}, &stdout, &stderr); status != 1 {
		t.Fatalf("status = %d, want 1", status)
	}
	if !strings.Contains(stderr.String(), "does-not-exist.dxf") {
		t.Errorf("stderr = %q, want the file name", stderr.String())
	}
}

func TestRunOnInvalidFixture(t *testing.T) {
	t.Parallel()

	var stdout, stderr strings.Builder
	if status := run([]string{"../../testdata/invalid/no-eof.dxf"}, &stdout, &stderr); status != 1 {
		t.Fatalf("status = %d, want 1", status)
	}
	if !strings.Contains(stderr.String(), "missing EOF terminator") {
		t.Errorf("stderr = %q, want the decode error", stderr.String())
	}
}

func TestRunLenient(t *testing.T) {
	t.Parallel()

	var stdout, stderr strings.Builder
	status := run([]string{"-lenient", "../../testdata/invalid/no-eof.dxf"}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "diagnostics: 1") {
		t.Errorf("output = %q, want one diagnostic", stdout.String())
	}
}

func TestRunMultipleFiles(t *testing.T) {
	t.Parallel()

	var stdout, stderr strings.Builder
	status := run([]string{
		"../../testdata/minimal.dxf",
		"../../testdata/invalid/no-eof.dxf",
		"../../testdata/lines.dxf",
	}, &stdout, &stderr)
	if status != 1 {
		t.Fatalf("status = %d, want 1", status)
	}
	output := stdout.String()
	if !strings.Contains(output, "minimal.dxf") || !strings.Contains(output, "lines.dxf") {
		t.Errorf("output = %q, want both valid files", output)
	}
	if strings.Contains(output, "no-eof.dxf") {
		t.Errorf("output = %q, want no summary for the invalid file", output)
	}
}
