package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apantanella/dxf"
	"github.com/apantanella/dxf/structural"
)

func generate(t *testing.T, args ...string) (string, int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out.dxf")
	args = append([]string{"-out", path}, args...)
	var stdout, stderr strings.Builder
	status := run(args, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("run() status = %d, stderr = %q", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wrote ") {
		t.Errorf("stdout = %q, want a confirmation", stdout.String())
	}
	return path, status
}

func decodeGenerated(t *testing.T, path string) *dxf.Document {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	defer file.Close()
	doc, err := dxf.Decode(file)
	if err != nil {
		t.Fatalf("Decode() = %v", err)
	}
	if len(doc.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", doc.Diagnostics)
	}
	return doc
}

func TestRunFrame(t *testing.T) {
	t.Parallel()

	path, _ := generate(t, "-kind", "frame", "-unit", "m")
	doc := decodeGenerated(t, path)
	if len(doc.Entities) == 0 {
		t.Fatal("generated frame has no entities")
	}

	wantLayers := []string{"STR-NODES", "STR-BEAMS", "STR-COLUMNS", "STR-BRACES", "STR-PANELS", "STR-GRIDS", "STR-LABELS"}
	layers := map[string]bool{}
	for _, layer := range doc.Tables.LayerNames() {
		layers[layer] = true
	}
	for _, layer := range wantLayers {
		if !layers[layer] {
			t.Errorf("layer %q missing from %v", layer, doc.Tables.LayerNames())
		}
	}
}

func TestRunEntities(t *testing.T) {
	t.Parallel()

	path, _ := generate(t, "-kind", "entities")
	doc := decodeGenerated(t, path)

	types := map[string]int{}
	for _, entity := range doc.Entities {
		types[entity.Type()]++
	}
	for _, want := range []string{"LINE", "POINT", "CIRCLE", "ARC", "POLYLINE", "TEXT", "SOLID", "TRACE", "3DFACE", "INSERT"} {
		if types[want] == 0 {
			t.Errorf("entity type %q missing from %v", want, types)
		}
	}
	if len(doc.Blocks) != 1 || doc.Blocks[0].Name != "SAMPLE" {
		t.Errorf("blocks = %+v", doc.Blocks)
	}
}

func TestRunPlan(t *testing.T) {
	t.Parallel()

	path, _ := generate(t, "-kind", "plan", "-unit", "m")
	doc := decodeGenerated(t, path)

	counts := map[string]int{}
	for _, entity := range doc.Entities {
		counts[entity.Type()]++
	}
	want := map[string]int{"LINE": 6, "POLYLINE": 5, "POINT": 2, "TEXT": 4}
	for entityType, count := range want {
		if counts[entityType] != count {
			t.Errorf("%s = %d, want %d (counts = %v)", entityType, counts[entityType], count, counts)
		}
	}
}

func TestRunPlanUnitConversion(t *testing.T) {
	t.Parallel()

	path, _ := generate(t, "-kind", "plan", "-unit", "mm")
	doc := decodeGenerated(t, path)

	maxX := 0.0
	for _, entity := range doc.Entities {
		line, ok := entity.(*dxf.Line)
		if !ok {
			continue
		}
		for _, x := range []float64{line.Start.X, line.End.X} {
			if x > maxX {
				maxX = x
			}
		}
	}
	if maxX < 5900 || maxX > 6100 {
		t.Errorf("max X = %v, want about 6000 (6 m in mm)", maxX)
	}
}

func TestRunFrameLayerColors(t *testing.T) {
	t.Parallel()

	path, _ := generate(t, "-kind", "frame", "-unit", "m")
	doc := decodeGenerated(t, path)

	colors := map[string]int{}
	table := doc.Tables.Find("LAYER")
	if table == nil {
		t.Fatal("LAYER table not found")
	}
	for _, record := range table.Records {
		name := ""
		color := -1
		for _, group := range record.Groups {
			switch group.Code {
			case 2:
				name = strings.TrimSpace(group.Value)
			case 62:
				color = int(parseNumber(t, group.Value))
			}
		}
		if name != "" {
			colors[name] = color
		}
	}

	want := map[string]int{
		structural.LayerNodes:   structural.ColorNodes,
		structural.LayerBeams:   structural.ColorBeams,
		structural.LayerColumns: structural.ColorColumns,
		structural.LayerBraces:  structural.ColorBraces,
		structural.LayerPanels:  structural.ColorPanels,
		structural.LayerGrids:   structural.ColorGrids,
	}
	for layer, color := range want {
		if colors[layer] != color {
			t.Errorf("layer %s color = %d, want %d (colors = %v)", layer, colors[layer], color, colors)
		}
	}
}

func parseNumber(t *testing.T, value string) float64 {
	t.Helper()
	var result float64
	if _, err := fmt.Sscanf(strings.TrimSpace(value), "%g", &result); err != nil {
		t.Fatalf("cannot parse %q: %v", value, err)
	}
	return result
}

func TestRunUnitConversion(t *testing.T) {
	t.Parallel()

	path, _ := generate(t, "-kind", "frame", "-unit", "mm")
	doc := decodeGenerated(t, path)

	maxX := 0.0
	for _, entity := range doc.Entities {
		line, ok := entity.(*dxf.Line)
		if !ok {
			continue
		}
		for _, x := range []float64{line.Start.X, line.End.X} {
			if x > maxX {
				maxX = x
			}
		}
	}
	if maxX < 5900 || maxX > 6100 {
		t.Errorf("max X = %v, want about 6000 (6 m in mm)", maxX)
	}
}

func TestRunPrecision(t *testing.T) {
	t.Parallel()

	path, _ := generate(t, "-kind", "frame", "-precision", "1")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() = %v", err)
	}
	if !strings.Contains(string(content), "5.0") {
		t.Errorf("output does not use one decimal place:\n%s", content)
	}
}

func TestRunInvalidArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown unit", args: []string{"-unit", "furlong"}},
		{name: "unknown kind", args: []string{"-kind", "unknown"}},
		{name: "unexpected argument", args: []string{"extra"}},
		{name: "unknown flag", args: []string{"-unknown"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr strings.Builder
			if status := run(tt.args, &stdout, &stderr); status != 2 {
				t.Fatalf("status = %d, want 2 (stderr = %q)", status, stderr.String())
			}
		})
	}
}

func TestRunWriteError(t *testing.T) {
	t.Parallel()

	var stdout, stderr strings.Builder
	status := run([]string{"-out", filepath.Join("missing-dir", "out.dxf")}, &stdout, &stderr)
	if status != 1 {
		t.Fatalf("status = %d, want 1", status)
	}
	if !strings.Contains(stderr.String(), "dxfgen:") {
		t.Errorf("stderr = %q, want the command prefix", stderr.String())
	}
}
