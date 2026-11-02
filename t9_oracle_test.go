//go:build oracle

package dxf_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/apantanella/dxf"
)

// t9Write writes doc to a temporary file and returns its path.
func t9Write(t *testing.T, doc *dxf.Document, opts ...dxf.EncodeOption) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "out.dxf")
	var b bytes.Buffer
	if err := dxf.Encode(&b, doc, opts...); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func t9Sample() *dxf.Document {
	doc := dxf.NewDocument()
	doc.Tables.EnsureLayer("STR-BEAMS", 1, "CONTINUOUS")
	doc.Blocks = []*dxf.Block{{Name: "COLONNA", LayerName: "0", Entities: []dxf.Entity{
		&dxf.Circle{EntityCommon: dxf.EntityCommon{LayerName: "0"}, Center: dxf.Point3D{X: 1}, Radius: 1},
	}}}
	in := dxf.NewInsert("COLONNA", dxf.Point3D{X: 1, Y: 2})
	in.LayerName = "0"
	in.Attributes = []dxf.Attribute{{Tag: "TAG", Value: "C1", Height: 2.5, XScale: 1}}
	doc.Entities = []dxf.Entity{
		&dxf.Line{EntityCommon: dxf.EntityCommon{LayerName: "STR-BEAMS"}, End: dxf.Point3D{X: 5}},
		&dxf.Point{EntityCommon: dxf.EntityCommon{LayerName: "0"}, Position: dxf.Point3D{X: 1, Y: 1}},
		&dxf.Arc{EntityCommon: dxf.EntityCommon{LayerName: "0"}, Radius: 2, StartAngle: 30, EndAngle: 120},
		&dxf.Text{EntityCommon: dxf.EntityCommon{LayerName: "0"}, Value: "BEAM àè", Position: dxf.Point3D{X: 1}, Height: 2.5, XScale: 1},
		&dxf.Solid{EntityCommon: dxf.EntityCommon{LayerName: "0"}, Corners: [4]dxf.Point3D{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}},
		&dxf.Face3D{EntityCommon: dxf.EntityCommon{LayerName: "0"}, Corners: [4]dxf.Point3D{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}},
		&dxf.Polyline{EntityCommon: dxf.EntityCommon{LayerName: "0"}, Flags: 1, Vertices: []dxf.Vertex{{}, {Position: dxf.Point3D{X: 1}, Bulge: 0.5}, {Position: dxf.Point3D{X: 1, Y: 1}}}},
		&dxf.Polyline{EntityCommon: dxf.EntityCommon{LayerName: "0"}, Flags: 8, Vertices: []dxf.Vertex{{}, {Position: dxf.Point3D{X: 1, Z: 1}}}},
		in,
	}
	return doc
}

// TestOracleWrittenByLibrary: ezdxf reads every version the library writes and
// its audit finds no error.
func TestOracleWrittenByLibrary(t *testing.T) {
	for _, version := range []string{
		dxf.VersionR12, dxf.VersionR2000, dxf.VersionR2004, dxf.VersionR2007,
		dxf.VersionR2010, dxf.VersionR2013, dxf.VersionR2018,
	} {
		version := version
		t.Run(version+"/empty", func(t *testing.T) {
			rep := runOracle(t, t9Write(t, dxf.NewDocumentVersion(version), dxf.WithVersion(version)))
			if rep.Version == "" {
				t.Error("no version in the report")
			}
		})
		t.Run(version+"/entities", func(t *testing.T) {
			rep := runOracle(t, t9Write(t, t9Sample(), dxf.WithVersion(version)))
			for _, typ := range []string{"LINE", "POINT", "ARC", "TEXT", "SOLID", "3DFACE", "POLYLINE", "INSERT"} {
				if rep.Entities[typ] == 0 {
					t.Errorf("ezdxf sees no %s in the model space: %v", typ, rep.Entities)
				}
			}
		})
	}
}

// TestOracleConvertedFixtures: the R12 fixtures converted to R2013 and the
// R2000+ fixtures read and written again.
func TestOracleConvertedFixtures(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("testdata", "*.dxf"))
	more, _ := filepath.Glob(filepath.Join("testdata", "oracle", "*.dxf"))
	for _, path := range append(files, more...) {
		path := path
		t.Run(filepath.ToSlash(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := dxf.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			version := dxf.VersionR2013
			if doc.Version != dxf.VersionR12 {
				version = doc.Version
			}
			runOracle(t, t9Write(t, doc, dxf.WithVersion(version)))
		})
	}
}

// TestOracleGoldenEntities: the files expected by TestR11SubclassMarkers are
// valid for ezdxf.
func TestOracleGoldenEntities(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("testdata", "r2013", "*.dxf"))
	if len(files) == 0 {
		t.Skip("no golden files")
	}
	for _, path := range files {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) { runOracle(t, path) })
	}
}

// TestOracleTagdiffV00: scripts/tagdiff.py --ignore-handles finds the empty
// document equal to v00 up to the entities, and the three entities equal to v00.
func TestOracleTagdiffV00(t *testing.T) {
	if ok, why := oracleAvailable(); !ok {
		t.Skip(why)
	}
	script, _ := filepath.Abs(filepath.Join("scripts", "tagdiff.py"))
	v00, _ := filepath.Abs(filepath.Join("testdata", "t1", "v00-baseline.dxf"))
	doc := dxf.NewDocumentVersion(dxf.VersionR2013)
	doc.Entities = []dxf.Entity{
		&dxf.Line{EntityCommon: dxf.EntityCommon{LayerName: "0"}, End: dxf.Point3D{X: 100}},
		&dxf.Circle{EntityCommon: dxf.EntityCommon{LayerName: "0"}, Center: dxf.Point3D{X: 50, Y: 50}, Radius: 25},
		&dxf.Text{EntityCommon: dxf.EntityCommon{LayerName: "0"}, Value: "Prova àèìòù ±°", Position: dxf.Point3D{Y: 10}, Height: 5, XScale: 1},
	}
	doc.Header.Variables["$EXTMAX"] = dxf.Variable{Name: "$EXTMAX", Groups: []dxf.Group{{Code: 10, Value: "100.0"}, {Code: 20, Value: "100.0"}, {Code: 30, Value: "0.0"}}}
	out := t9Write(t, doc)
	if res, err := exec.Command("python3", script, "--ignore-handles", v00, out).CombinedOutput(); err != nil {
		t.Errorf("tagdiff v00 vs library: %v\n%s", err, res)
	}
	// Round trip of every R13+ fixture, handles included (they are kept).
	files, _ := filepath.Glob(filepath.Join("testdata", "oracle", "*.dxf"))
	for _, f := range append(files, v00) {
		data, _ := os.ReadFile(f)
		d, err := dxf.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		o := t9Write(t, d)
		// Header variable order is not preserved (ADR 0005): ignore the
		// variables by comparing with the header removed is not possible in
		// tagdiff, so only the files whose header order is ours are compared.
		if f == v00 {
			if res, err := exec.Command("python3", script, v00, o).CombinedOutput(); err != nil {
				t.Errorf("tagdiff v00 round trip: %v\n%s", err, res)
			}
		}
	}
}
