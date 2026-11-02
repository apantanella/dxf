package dxf

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// t9EntityCases builds, per supported entity, a document of one entity with
// the optional groups set, so that each marker and each group position is
// exercised. The expected files are in testdata/r2013.
func t9EntityCases() []struct {
	name string
	doc  func() *Document
} {
	common := func(layer string) EntityCommon {
		return EntityCommon{LayerName: layer, Linetype: "DASHED", Color: 3, HasColor: true}
	}
	base := func(entities ...Entity) func() *Document {
		return func() *Document {
			doc := NewDocumentVersion(VersionR2013)
			doc.Tables.EnsureLinetype("DASHED")
			doc.Entities = entities
			return doc
		}
	}
	return []struct {
		name string
		doc  func() *Document
	}{
		{"line", base(&Line{EntityCommon: common("MURI"), Start: Point3D{X: 1, Y: 2, Z: 3}, End: Point3D{X: 100.5, Y: 2, Z: 3}})},
		{"point", base(&Point{EntityCommon: EntityCommon{LayerName: "0"}, Position: Point3D{X: 5, Y: 5}, Angle: 30})},
		{"circle", base(&Circle{EntityCommon: EntityCommon{LayerName: "0", Thickness: 2, Extrusion: Point3D{Z: -1}, ExtrusionSet: true}, Center: Point3D{X: 50, Y: 50}, Radius: 25})},
		{"arc", base(&Arc{EntityCommon: common("0"), Center: Point3D{}, Radius: 40, StartAngle: 15, EndAngle: 120})},
		{"text", base(&Text{
			EntityCommon: EntityCommon{LayerName: "0"}, Value: "Prova àèìòù ±°", Position: Point3D{X: 10, Y: 10}, Height: 2.5,
			XScale: 0.8, Rotation: 45, Oblique: 10, Style: "Standard", HAlign: 1, VAlign: 2, Alignment: Point3D{X: 12, Y: 11},
		})},
		{"solid", base(&Solid{EntityCommon: EntityCommon{LayerName: "0"}, Corners: [4]Point3D{{}, {X: 1}, {Y: 1}, {X: 1, Y: 1}}})},
		{"trace", base(&Trace{EntityCommon: EntityCommon{LayerName: "0"}, Corners: [4]Point3D{{}, {X: 1}, {Y: 1}, {X: 1, Y: 1}}})},
		{"face3d", base(&Face3D{EntityCommon: EntityCommon{LayerName: "0"}, Corners: [4]Point3D{{}, {X: 1}, {X: 1, Y: 1, Z: 1}, {Y: 1, Z: 1}}, InvisibleEdges: 3})},
		{"polyline2d", base(&Polyline{
			EntityCommon: EntityCommon{LayerName: "MURI"}, Flags: 1, StartWidth: 0.5, EndWidth: 0.5,
			Vertices: []Vertex{{Position: Point3D{}}, {Position: Point3D{X: 10}, Bulge: 0.5}, {Position: Point3D{X: 10, Y: 10}}},
		})},
		{"polyline3d", base(&Polyline{
			EntityCommon: EntityCommon{LayerName: "0"}, Flags: 8,
			Vertices: []Vertex{{Position: Point3D{}}, {Position: Point3D{X: 10, Z: 5}}, {Position: Point3D{X: 10, Y: 10, Z: 7}}},
		})},
		{"insert-attrib", func() *Document {
			doc := NewDocumentVersion(VersionR2013)
			doc.Blocks = []*Block{{Name: "PORTA", LayerName: "0", Flags: 2, Entities: []Entity{
				&Line{EntityCommon: EntityCommon{LayerName: "0"}, End: Point3D{Y: 90}},
			}}}
			in := NewInsert("PORTA", Point3D{X: 200, Y: 100})
			in.LayerName = "MURI"
			in.XScale, in.Rotation = 2, 90
			in.Attributes = []Attribute{{Tag: "TAG1", Value: "P01", Position: Point3D{X: 200, Y: 200}, Height: 2.5, XScale: 1}}
			doc.Entities = []Entity{in}
			return doc
		}},
		{"minsert", func() *Document {
			doc := NewDocumentVersion(VersionR2013)
			doc.Blocks = []*Block{{Name: "B", LayerName: "0", Entities: []Entity{&Circle{EntityCommon: EntityCommon{LayerName: "0"}, Radius: 1}}}}
			in := NewInsert("B", Point3D{})
			in.LayerName = "0"
			in.ColumnCount, in.RowCount, in.ColumnSpacing, in.RowSpacing = 3, 2, 5, 4
			doc.Entities = []Entity{in}
			return doc
		}},
	}
}

// TestR11SubclassMarkers compares the file written for each supported entity
// with testdata/r2013/<name>.dxf byte by byte. Set DXF_UPDATE_GOLDEN=1 to
// rewrite the expected files after a deliberate change (and review them).
func TestR11SubclassMarkers(t *testing.T) {
	for _, tc := range t9EntityCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := t9Encode(t, tc.doc())
			path := filepath.Join("testdata", "r2013", tc.name+".dxf")
			if os.Getenv("DXF_UPDATE_GOLDEN") == "1" {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want := t9ReadFile(t, path)
			if !bytes.Equal(got, want) {
				t.Errorf("%s differs from the expected file: %s", path, t9DiffTags(t9Tags(t, want), t9Tags(t, got)))
			}
			// Whatever the case, the file reads back without warnings.
			doc := t9Decode(t, got)
			for _, d := range doc.Diagnostics {
				if d.Severity >= SeverityWarning {
					t.Errorf("diagnostic on reading: %v", d)
				}
			}
		})
	}
}

// TestR11MarkerSequence checks the 100 markers of each entity record in the
// ENTITIES section independently of the golden files.
func TestR11MarkerSequence(t *testing.T) {
	want := map[string][]string{
		"LINE":     {"AcDbEntity", "AcDbLine"},
		"POINT":    {"AcDbEntity", "AcDbPoint"},
		"CIRCLE":   {"AcDbEntity", "AcDbCircle"},
		"ARC":      {"AcDbEntity", "AcDbCircle", "AcDbArc"},
		"TEXT":     {"AcDbEntity", "AcDbText", "AcDbText"},
		"SOLID":    {"AcDbEntity", "AcDbTrace"},
		"TRACE":    {"AcDbEntity", "AcDbTrace"},
		"3DFACE":   {"AcDbEntity", "AcDbFace"},
		"INSERT":   {"AcDbEntity", "AcDbBlockReference"},
		"ATTRIB":   {"AcDbEntity", "AcDbText", "AcDbAttribute"},
		"VERTEX":   {"AcDbEntity", "AcDbVertex", "AcDb2dVertex"},
		"SEQEND":   {"AcDbEntity"},
		"POLYLINE": {"AcDbEntity", "AcDb2dPolyline"},
	}
	doc := sampleDocumentR2013()
	data := t9Encode(t, doc)
	ts := t9Tags(t, data)
	// Walk the ENTITIES section.
	in := false
	var typ string
	var got []string
	seen := map[string]bool{}
	flush := func() {
		if typ == "" {
			return
		}
		seen[typ] = true
		if w, ok := want[typ]; ok && strings.Join(w, ",") != strings.Join(got, ",") {
			t.Errorf("%s: markers %v, want %v", typ, got, w)
		}
		typ, got = "", nil
	}
	for i, tg := range ts {
		if tg.Code == 2 && tg.Value == "ENTITIES" && i > 0 && ts[i-1].Value == "SECTION" {
			in = true
			continue
		}
		if !in {
			continue
		}
		if tg.Code == 0 {
			flush()
			if tg.Value == "ENDSEC" {
				break
			}
			typ = tg.Value
			continue
		}
		if tg.Code == 100 {
			got = append(got, tg.Value)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("sample document has no %s", name)
		}
	}
}

// sampleDocumentR2013 is sampleDocument converted to R2013.
func sampleDocumentR2013() *Document {
	doc := sampleDocument()
	doc.Version = VersionR2013
	return doc
}
