package structural_test

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apantanella/dxf"
	"github.com/apantanella/dxf/structural"
)

func TestUnitConvert(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		value   float64
		from    structural.Unit
		to      structural.Unit
		want    float64
		wantErr bool
	}{
		{name: "meters to millimeters", value: 1.5, from: structural.UnitMeter, to: structural.UnitMillimeter, want: 1500},
		{name: "millimeters to meters", value: 1500, from: structural.UnitMillimeter, to: structural.UnitMeter, want: 1.5},
		{name: "inches to millimeters", value: 1, from: structural.UnitInch, to: structural.UnitMillimeter, want: 25.4},
		{name: "same unit", value: 3, from: structural.UnitMeter, to: structural.UnitMeter, want: 3},
		{name: "unknown unit", value: 1, from: structural.UnitUnknown, to: structural.UnitMeter, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := structural.Convert(tt.value, tt.from, tt.to)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Convert() = %v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Convert() = %v", err)
			}
			if diff := got - tt.want; diff > 1e-9 || diff < -1e-9 {
				t.Errorf("Convert() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestModelValidate(t *testing.T) {
	t.Parallel()

	valid := &structural.Model{
		Unit: structural.UnitMeter,
		Nodes: []structural.Node{
			{ID: "N1", Position: dxf.Point3D{}},
			{ID: "N2", Position: dxf.Point3D{X: 1}},
		},
		Members: []structural.Member{{ID: "M1", Start: "N1", End: "N2", Kind: structural.MemberBeam}},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}

	tests := []struct {
		name  string
		model *structural.Model
	}{
		{name: "duplicate node", model: &structural.Model{Nodes: []structural.Node{{ID: "N1"}, {ID: "N1"}}}},
		{name: "node without id", model: &structural.Model{Nodes: []structural.Node{{}}}},
		{name: "member without nodes", model: &structural.Model{Members: []structural.Member{{ID: "M1", Start: "N1", End: "N2"}}}},
		{name: "member with coincident ends", model: &structural.Model{
			Nodes:   []structural.Node{{ID: "N1"}},
			Members: []structural.Member{{ID: "M1", Start: "N1", End: "N1"}},
		}},
		{name: "panel with two points", model: &structural.Model{
			Panels: []structural.Panel{{ID: "P1", Boundary: []dxf.Point3D{{}, {X: 1}}}},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := tt.model.Validate(); err == nil {
				t.Fatal("Validate() = nil, want error")
			}
		})
	}
}

func TestImportConvertsOCSToWCS(t *testing.T) {
	t.Parallel()

	mapping := structural.DefaultMapping()
	doc := dxf.NewDocument()
	doc.Entities = []dxf.Entity{
		&dxf.Polyline{
			EntityCommon: dxf.EntityCommon{
				LayerName:    structural.LayerPanels,
				Extrusion:    dxf.Point3D{Z: -1},
				ExtrusionSet: true,
			},
			Flags:     1,
			Elevation: dxf.Point3D{Z: 5},
			Vertices: []dxf.Vertex{
				{Position: dxf.Point3D{}},
				{Position: dxf.Point3D{X: 1}},
				{Position: dxf.Point3D{X: 1, Y: 1}},
			},
		},
	}

	model, report, err := structural.Import(doc, mapping)
	if err != nil {
		t.Fatalf("Import() = %v", err)
	}
	if report.Skipped != 0 {
		t.Fatalf("skipped = %d: %v", report.Skipped, report.Diagnostics)
	}
	if len(model.Panels) != 1 {
		t.Fatalf("panels = %d, want 1", len(model.Panels))
	}
	want := []dxf.Point3D{{Z: -5}, {X: -1, Z: -5}, {X: -1, Y: 1, Z: -5}}
	got := model.Panels[0].Boundary
	if len(got) != len(want) {
		t.Fatalf("boundary points = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if !pointClose(got[i], want[i]) {
			t.Errorf("boundary[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestImportConvertsTextOCSToWCS(t *testing.T) {
	t.Parallel()

	mapping := structural.DefaultMapping()
	doc := dxf.NewDocument()
	doc.Entities = []dxf.Entity{
		&dxf.Point{EntityCommon: dxf.EntityCommon{LayerName: structural.LayerNodes}, Position: dxf.Point3D{X: -1, Y: 2}},
		&dxf.Text{
			EntityCommon: dxf.EntityCommon{
				LayerName:    structural.LayerLabels,
				Extrusion:    dxf.Point3D{Z: -1},
				ExtrusionSet: true,
			},
			Value:    "N1",
			Position: dxf.Point3D{X: 1, Y: 2},
			Height:   1,
			XScale:   1,
		},
	}

	model, _, err := structural.Import(doc, mapping)
	if err != nil {
		t.Fatalf("Import() = %v", err)
	}
	if len(model.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(model.Nodes))
	}
	if model.Nodes[0].Label != "N1" {
		t.Errorf("label = %q, want %q", model.Nodes[0].Label, "N1")
	}
}

func pointClose(a, b dxf.Point3D) bool {
	const epsilon = 1e-9
	return math.Abs(a.X-b.X) < epsilon && math.Abs(a.Y-b.Y) < epsilon && math.Abs(a.Z-b.Z) < epsilon
}

func TestImportStructuralFrameFixture(t *testing.T) {
	t.Parallel()

	file, err := os.Open(filepath.Join("..", "testdata", "structural-frame.dxf"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer file.Close()

	doc, err := dxf.Decode(file)
	if err != nil {
		t.Fatalf("Decode() = %v", err)
	}
	if len(doc.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", doc.Diagnostics)
	}

	model, report, err := structural.Import(doc, structural.DefaultMapping())
	if err != nil {
		t.Fatalf("Import() = %v", err)
	}
	if len(model.Nodes) != 3 {
		t.Errorf("nodes = %d, want 3", len(model.Nodes))
	}
	if len(model.Members) != 2 {
		t.Fatalf("members = %d, want 2", len(model.Members))
	}
	if model.Members[0].Kind != structural.MemberColumn || model.Members[1].Kind != structural.MemberBeam {
		t.Errorf("kinds = %v, %v", model.Members[0].Kind, model.Members[1].Kind)
	}
	if len(model.Panels) != 1 {
		t.Errorf("panels = %d, want 1", len(model.Panels))
	}
	if len(model.Grids) != 1 {
		t.Fatalf("grids = %d, want 1", len(model.Grids))
	}
	if model.Grids[0].Label != "A" {
		t.Errorf("grid label = %q, want %q", model.Grids[0].Label, "A")
	}
	if report.Skipped != 0 {
		t.Errorf("skipped = %d: %v", report.Skipped, report.Diagnostics)
	}
}

func TestImportStructuralEntities(t *testing.T) {
	t.Parallel()

	doc := dxf.NewDocument()
	doc.Entities = []dxf.Entity{
		&dxf.Point{EntityCommon: dxf.EntityCommon{LayerName: structural.LayerNodes}, Position: dxf.Point3D{}},
		&dxf.Line{EntityCommon: dxf.EntityCommon{LayerName: structural.LayerColumns}, Start: dxf.Point3D{}, End: dxf.Point3D{Z: 3}},
		&dxf.Line{
			EntityCommon: dxf.EntityCommon{LayerName: structural.LayerBeams},
			Start:        dxf.Point3D{Z: 3},
			End:          dxf.Point3D{X: 5, Z: 3},
		},
		&dxf.Polyline{
			EntityCommon: dxf.EntityCommon{LayerName: structural.LayerPanels},
			Flags:        1,
			Vertices: []dxf.Vertex{
				{Position: dxf.Point3D{}},
				{Position: dxf.Point3D{X: 1}},
				{Position: dxf.Point3D{X: 1, Y: 1}},
			},
		},
		&dxf.Line{
			EntityCommon: dxf.EntityCommon{LayerName: structural.LayerGrids},
			Start:        dxf.Point3D{X: -1},
			End:          dxf.Point3D{X: 6},
		},
	}

	model, report, err := structural.Import(doc, structural.DefaultMapping())
	if err != nil {
		t.Fatalf("Import() = %v", err)
	}
	if len(model.Nodes) != 3 {
		t.Errorf("nodes = %d, want 3", len(model.Nodes))
	}
	if len(model.Members) != 2 {
		t.Fatalf("members = %d, want 2", len(model.Members))
	}
	if model.Members[0].Kind != structural.MemberColumn || model.Members[1].Kind != structural.MemberBeam {
		t.Errorf("kinds = %v, %v", model.Members[0].Kind, model.Members[1].Kind)
	}
	if len(model.Panels) != 1 || len(model.Panels[0].Boundary) != 3 {
		t.Errorf("panels = %+v", model.Panels)
	}
	if len(model.Grids) != 1 {
		t.Errorf("grids = %+v", model.Grids)
	}
	if report.Skipped != 0 {
		t.Errorf("skipped = %d: %v", report.Skipped, report.Diagnostics)
	}
}

func TestImportSkipsUnmappedEntities(t *testing.T) {
	t.Parallel()

	doc := dxf.NewDocument()
	doc.Entities = []dxf.Entity{
		&dxf.Line{EntityCommon: dxf.EntityCommon{LayerName: "DIM"}, End: dxf.Point3D{X: 1}},
		&dxf.Circle{EntityCommon: dxf.EntityCommon{LayerName: structural.LayerBeams}, Radius: 1},
		&dxf.Polyline{
			EntityCommon: dxf.EntityCommon{LayerName: structural.LayerPanels},
			Vertices:     []dxf.Vertex{{Position: dxf.Point3D{}}, {Position: dxf.Point3D{X: 1}}},
		},
	}

	model, report, err := structural.Import(doc, structural.DefaultMapping())
	if err != nil {
		t.Fatalf("Import() = %v", err)
	}
	if len(model.Members) != 0 || len(model.Panels) != 0 {
		t.Errorf("model = %+v", model)
	}
	if report.Skipped != 3 {
		t.Fatalf("skipped = %d, want 3: %v", report.Skipped, report.Diagnostics)
	}
}

func TestImportClassifyByOrientation(t *testing.T) {
	t.Parallel()

	mapping := structural.DefaultMapping()
	mapping.ClassifyByOrientation = true

	doc := dxf.NewDocument()
	doc.Entities = []dxf.Entity{
		&dxf.Line{EntityCommon: dxf.EntityCommon{LayerName: structural.LayerMembers}, Start: dxf.Point3D{}, End: dxf.Point3D{Z: 3}},
		&dxf.Line{EntityCommon: dxf.EntityCommon{LayerName: structural.LayerMembers}, Start: dxf.Point3D{}, End: dxf.Point3D{X: 3}},
		&dxf.Line{EntityCommon: dxf.EntityCommon{LayerName: structural.LayerMembers}, Start: dxf.Point3D{}, End: dxf.Point3D{X: 3, Z: 3}},
	}

	model, _, err := structural.Import(doc, mapping)
	if err != nil {
		t.Fatalf("Import() = %v", err)
	}
	if len(model.Members) != 3 {
		t.Fatalf("members = %d", len(model.Members))
	}
	want := []structural.MemberKind{structural.MemberColumn, structural.MemberBeam, structural.MemberBrace}
	for i := range want {
		if model.Members[i].Kind != want[i] {
			t.Errorf("member %d = %v, want %v", i, model.Members[i].Kind, want[i])
		}
	}
}

func TestExportUnitConversion(t *testing.T) {
	t.Parallel()

	model := &structural.Model{Unit: structural.UnitMeter}
	start := model.AddNode(dxf.Point3D{})
	end := model.AddNode(dxf.Point3D{X: 5})
	if _, err := model.AddMember(start, end, structural.MemberBeam); err != nil {
		t.Fatalf("AddMember() = %v", err)
	}

	mapping := structural.DefaultMapping()
	mapping.Unit = structural.UnitMillimeter
	doc, err := structural.Export(model, mapping)
	if err != nil {
		t.Fatalf("Export() = %v", err)
	}
	var line *dxf.Line
	for _, entity := range doc.Entities {
		if candidate, ok := entity.(*dxf.Line); ok {
			line = candidate
			break
		}
	}
	if line == nil {
		t.Fatalf("no LINE exported: %+v", doc.Entities)
	}
	if line.End.X != 5000 {
		t.Errorf("X = %v, want 5000", line.End.X)
	}
}

func TestExportValidation(t *testing.T) {
	t.Parallel()

	model := &structural.Model{Members: []structural.Member{{ID: "M1", Start: "N1", End: "N2"}}}
	if _, err := structural.Export(model, structural.DefaultMapping()); err == nil {
		t.Fatal("Export() = nil, want error")
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	t.Parallel()

	model := &structural.Model{Unit: structural.UnitMeter}
	base := model.AddNode(dxf.Point3D{})
	top := model.AddNode(dxf.Point3D{Z: 3})
	tip := model.AddNode(dxf.Point3D{X: 5, Z: 3})
	if _, err := model.AddMember(base, top, structural.MemberColumn); err != nil {
		t.Fatalf("AddMember() = %v", err)
	}
	if _, err := model.AddMember(top, tip, structural.MemberBeam); err != nil {
		t.Fatalf("AddMember() = %v", err)
	}
	if _, err := model.AddPanel([]dxf.Point3D{{}, {X: 1}, {X: 1, Y: 1}}, structural.PanelSlab); err != nil {
		t.Fatalf("AddPanel() = %v", err)
	}
	model.Grids = append(model.Grids, structural.Grid{ID: "G1", Start: dxf.Point3D{X: -1}, End: dxf.Point3D{X: 6}, Label: "A"})

	mapping := structural.DefaultMapping()
	mapping.Unit = structural.UnitMeter
	doc, err := structural.Export(model, mapping)
	if err != nil {
		t.Fatalf("Export() = %v", err)
	}

	var builder strings.Builder
	if err := dxf.Encode(&builder, doc); err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	decoded, err := dxf.Decode(strings.NewReader(builder.String()))
	if err != nil {
		t.Fatalf("Decode() = %v", err)
	}

	imported, report, err := structural.Import(decoded, mapping)
	if err != nil {
		t.Fatalf("Import() = %v", err)
	}
	if len(imported.Nodes) != len(model.Nodes) {
		t.Errorf("nodes = %d, want %d", len(imported.Nodes), len(model.Nodes))
	}
	if len(imported.Members) != len(model.Members) {
		t.Errorf("members = %d, want %d", len(imported.Members), len(model.Members))
	}
	if len(imported.Panels) != len(model.Panels) {
		t.Errorf("panels = %d, want %d", len(imported.Panels), len(model.Panels))
	}
	if len(imported.Grids) != len(model.Grids) {
		t.Errorf("grids = %d, want %d", len(imported.Grids), len(model.Grids))
	}
	if imported.Members[0].Kind != structural.MemberColumn || imported.Members[1].Kind != structural.MemberBeam {
		t.Errorf("imported kinds = %v, %v", imported.Members[0].Kind, imported.Members[1].Kind)
	}
	if report.Skipped != 0 {
		t.Errorf("skipped = %d: %v", report.Skipped, report.Diagnostics)
	}
}
