package dxf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var validFixtures = []struct {
	name     string
	entities int
}{
	{name: "minimal.dxf", entities: 0},
	{name: "lines.dxf", entities: 3},
	{name: "polyline-closed.dxf", entities: 1},
	{name: "text-ocs.dxf", entities: 1},
	{name: "blocks-insert.dxf", entities: 1},
	{name: "structural-frame.dxf", entities: 6},
}

func decodeFixture(t *testing.T, name string) *Document {
	t.Helper()
	file, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	defer file.Close()
	doc, err := Decode(file)
	if err != nil {
		t.Fatalf("Decode(%s) = %v", name, err)
	}
	return doc
}

func TestFixturesDecode(t *testing.T) {
	t.Parallel()

	for _, fixture := range validFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			doc := decodeFixture(t, fixture.name)
			if len(doc.Diagnostics) != 0 {
				t.Errorf("unexpected diagnostics: %v", doc.Diagnostics)
			}
			if len(doc.Entities) != fixture.entities {
				t.Errorf("entities = %d, want %d", len(doc.Entities), fixture.entities)
			}
			if doc.Version != VersionR12 {
				t.Errorf("Version = %q, want %q", doc.Version, VersionR12)
			}
		})
	}
}

func TestFixtureEntityTypes(t *testing.T) {
	t.Parallel()

	t.Run("lines", func(t *testing.T) {
		t.Parallel()
		doc := decodeFixture(t, "lines.dxf")
		layers := map[string]bool{}
		for _, entity := range doc.Entities {
			if _, ok := entity.(*Line); !ok {
				t.Fatalf("type = %T, want *Line", entity)
			}
			layers[entity.Layer()] = true
		}
		for _, want := range []string{"0", "STR-BEAMS", "STR-COLUMNS"} {
			if !layers[want] {
				t.Errorf("layer %q not found in %v", want, layers)
			}
		}
	})

	t.Run("polyline", func(t *testing.T) {
		t.Parallel()
		doc := decodeFixture(t, "polyline-closed.dxf")
		polyline, ok := doc.Entities[0].(*Polyline)
		if !ok {
			t.Fatalf("type = %T, want *Polyline", doc.Entities[0])
		}
		if !polyline.Closed() {
			t.Error("polyline should be closed")
		}
		if len(polyline.Vertices) != 3 {
			t.Fatalf("vertices = %d, want 3", len(polyline.Vertices))
		}
		if polyline.Vertices[1].Bulge != 0.5 {
			t.Errorf("bulge = %v, want 0.5", polyline.Vertices[1].Bulge)
		}
	})

	t.Run("text", func(t *testing.T) {
		t.Parallel()
		doc := decodeFixture(t, "text-ocs.dxf")
		text, ok := doc.Entities[0].(*Text)
		if !ok {
			t.Fatalf("type = %T, want *Text", doc.Entities[0])
		}
		if text.Value != "BEAM A" || text.Height != 2.5 || text.Style != "STANDARD" {
			t.Errorf("text = %+v", text)
		}
		if text.HAlign != 1 {
			t.Errorf("HAlign = %d, want 1", text.HAlign)
		}
	})

	t.Run("blocks and insert", func(t *testing.T) {
		t.Parallel()
		doc := decodeFixture(t, "blocks-insert.dxf")
		if len(doc.Blocks) != 1 {
			t.Fatalf("blocks = %d, want 1", len(doc.Blocks))
		}
		if doc.Blocks[0].Name != "COLUMN" || len(doc.Blocks[0].Entities) != 1 {
			t.Errorf("block = %+v", doc.Blocks[0])
		}
		insert, ok := doc.Entities[0].(*Insert)
		if !ok {
			t.Fatalf("type = %T, want *Insert", doc.Entities[0])
		}
		if insert.BlockName != "COLUMN" || len(insert.Attributes) != 1 {
			t.Fatalf("insert = %+v", insert)
		}
		if insert.Attributes[0].Tag != "TAG" || insert.Attributes[0].Value != "C1" {
			t.Errorf("attribute = %+v", insert.Attributes[0])
		}
	})

	t.Run("structural frame", func(t *testing.T) {
		t.Parallel()
		doc := decodeFixture(t, "structural-frame.dxf")
		counts := map[string]int{}
		for _, entity := range doc.Entities {
			counts[entity.Type()]++
		}
		want := map[string]int{"POINT": 1, "LINE": 3, "POLYLINE": 1, "TEXT": 1}
		for entityType, count := range want {
			if counts[entityType] != count {
				t.Errorf("%s = %d, want %d", entityType, counts[entityType], count)
			}
		}
	})
}

func TestFixtureRoundTripIsStable(t *testing.T) {
	t.Parallel()

	for _, fixture := range validFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			doc := decodeFixture(t, fixture.name)

			var first strings.Builder
			if err := Encode(&first, doc); err != nil {
				t.Fatalf("Encode() = %v", err)
			}
			reencoded, err := Decode(strings.NewReader(first.String()))
			if err != nil {
				t.Fatalf("Decode() = %v", err)
			}
			var second strings.Builder
			if err := Encode(&second, reencoded); err != nil {
				t.Fatalf("Encode() = %v", err)
			}
			if first.String() != second.String() {
				t.Errorf("round-trip is not stable\ndiff:\n%s",
					diffStrings(first.String(), second.String()))
			}
		})
	}
}

func TestInvalidFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		wantErr error
	}{
		{name: "truncated.dxf", wantErr: ErrSyntax},
		{name: "bad-float.dxf", wantErr: ErrSyntax},
		{name: "no-eof.dxf", wantErr: ErrSyntax},
		{name: "vertex-orphan.dxf", wantErr: ErrSyntax},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			file, err := os.Open(filepath.Join("testdata", "invalid", tt.name))
			if err != nil {
				t.Fatalf("open %s: %v", tt.name, err)
			}
			defer file.Close()
			_, err = Decode(file)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Decode() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
