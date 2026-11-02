//go:build integration

package dxf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFileRoundTrip writes a document to a real file and reads it back. It
// covers the file system path that the unit tests skip.
func TestFileRoundTrip(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "roundtrip.dxf")

	doc := NewDocument()
	doc.Entities = []Entity{
		&Line{EntityCommon: EntityCommon{LayerName: "STR-BEAMS"}, End: Point3D{X: 5, Z: 3}},
		&Polyline{
			EntityCommon: EntityCommon{LayerName: "STR-PANELS"},
			Flags:        1,
			Vertices: []Vertex{
				{Position: Point3D{}},
				{Position: Point3D{X: 3}},
				{Position: Point3D{X: 3, Y: 2}},
			},
		},
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}
	if err := Encode(file, doc); err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() = %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("generated file is empty")
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() = %v", err)
	}

	read, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	defer read.Close()

	decoded, err := Decode(read)
	if err != nil {
		t.Fatalf("Decode() = %v", err)
	}
	if len(decoded.Entities) != len(doc.Entities) {
		t.Fatalf("entities = %d, want %d", len(decoded.Entities), len(doc.Entities))
	}

	var builder strings.Builder
	if err := Encode(&builder, decoded); err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	if string(written) != builder.String() {
		t.Error("re-encoding the file does not reproduce the original bytes")
	}
}

// TestFixturesAreReadableFromDisk verifies that every fixture is a real file on
// disk and decodes without panics.
func TestFixturesAreReadableFromDisk(t *testing.T) {
	t.Parallel()

	paths, err := filepath.Glob(filepath.Join("testdata", "*.dxf"))
	if err != nil {
		t.Fatalf("Glob() = %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no fixture found in testdata")
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			file, err := os.Open(path)
			if err != nil {
				t.Fatalf("Open() = %v", err)
			}
			defer file.Close()
			if _, err := Decode(file); err != nil {
				t.Fatalf("Decode() = %v", err)
			}
		})
	}
}
