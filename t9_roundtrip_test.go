package dxf

import (
	"path/filepath"
	"testing"
)

// TestT9FixtureRoundTrip: files written by other CADs (here ezdxf and the T1
// reference file) are decoded and encoded again; the tag sequence read back is
// the original one apart from the normalizations of ADR 0005.
func TestT9FixtureRoundTrip(t *testing.T) {
	t.Parallel()
	for _, path := range t9Fixtures(t) {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			orig := t9ReadFile(t, path)
			doc := t9Decode(t, orig)
			out := t9Encode(t, doc)
			want := t9Normalize(t9Tags(t, orig))
			got := t9Normalize(t9Tags(t, out))
			if d := t9DiffTags(want, got); d != "" {
				t.Errorf("round trip differs: %s", d)
			}
		})
	}
}
