package dxf

import (
	"os"
	"path/filepath"
	"testing"
)

// TestG1Files checks that the hand-test files of testdata/g1 are the ones the
// writer produces now. Set DXF_UPDATE_GOLDEN=1 to regenerate them.
func TestG1Files(t *testing.T) {
	t.Parallel()

	empty := NewDocumentVersion(VersionR2013)

	entities := sampleDocumentR2013()
	entities.Blocks = nil
	var keep []Entity
	for _, e := range entities.Entities {
		if _, isInsert := e.(*Insert); !isInsert {
			keep = append(keep, e)
		}
	}
	entities.Entities = keep

	block := sampleDocumentR2013()
	block.Entities = block.Entities[len(block.Entities)-1:]

	frame, err := os.Open(filepath.Join("testdata", "structural-frame.dxf"))
	if err != nil {
		t.Fatal(err)
	}
	defer frame.Close()
	converted, err := Decode(frame)
	if err != nil {
		t.Fatal(err)
	}
	converted.Version = VersionR2013

	for name, doc := range map[string]*Document{
		"01-vuoto-r2013.dxf":            empty,
		"02-entita-supportate.dxf":      entities,
		"03-blocco-insert-attrib.dxf":   block,
		"04-structural-frame-r2013.dxf": converted,
	} {
		got := t9Encode(t, doc)
		path := filepath.Join("testdata", "g1", name)
		if os.Getenv("DXF_UPDATE_GOLDEN") == "1" {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if want := t9ReadFile(t, path); string(want) != string(got) {
			t.Errorf("%s differs from the writer output (DXF_UPDATE_GOLDEN=1 regenerates it)", name)
		}
	}
}
