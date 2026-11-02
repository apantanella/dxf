package dxf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/apantanella/dxf/tags"
)

// Helpers shared by the T9 tests.

func t9Encode(t testing.TB, doc *Document, opts ...EncodeOption) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := Encode(&b, doc, opts...); err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	return b.Bytes()
}

func t9Decode(t testing.TB, data []byte, opts ...DecodeOption) *Document {
	t.Helper()
	doc, err := Decode(bytes.NewReader(data), opts...)
	if err != nil {
		t.Fatalf("Decode() = %v", err)
	}
	return doc
}

func t9Tags(t testing.TB, data []byte) []tags.Tag {
	t.Helper()
	r := tags.NewReader(bytes.NewReader(data))
	var out []tags.Tag
	for {
		tag, err := r.Next()
		if err != nil {
			break
		}
		out = append(out, tag)
	}
	return out
}

func t9ReadFile(t testing.TB, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// t9Normalize applies the closed list of normalizations of ADR 0005 to a tag
// sequence: the text of real and integer values (same number, different text)
// and the order of the header variables. The value of $HANDSEED is replaced
// by "-" (the writer recomputes it). Nothing else is changed.
func t9Normalize(in []tags.Tag) []tags.Tag {
	out := make([]tags.Tag, len(in))
	for i, tg := range in {
		switch tg.Kind() {
		case tags.KindFloat:
			if f, err := tg.Float(); err == nil {
				tg.Value = strconv.FormatFloat(f, 'g', -1, 64)
			}
		case tags.KindInt:
			if v, err := tg.Int(); err == nil {
				tg.Value = strconv.Itoa(v)
			}
		}
		out[i] = tg
	}
	// Header: sort the variables by name.
	start, end := -1, -1
	for i := 0; i+1 < len(out); i++ {
		if out[i].Code == 2 && out[i].Value == "HEADER" && i > 0 && out[i-1].Code == 0 && out[i-1].Value == "SECTION" {
			start = i + 1
		}
		if start >= 0 && i >= start && out[i].Code == 0 && out[i].Value == "ENDSEC" {
			end = i
			break
		}
	}
	if start >= 0 && end > start {
		type variable struct {
			name string
			tags []tags.Tag
		}
		var vars []variable
		for _, tg := range out[start:end] {
			if tg.Code == 9 {
				vars = append(vars, variable{name: tg.Value})
			}
			vars[len(vars)-1].tags = append(vars[len(vars)-1].tags, tg)
		}
		sort.SliceStable(vars, func(i, j int) bool { return vars[i].name < vars[j].name })
		var sorted []tags.Tag
		for _, v := range vars {
			if v.name == "$HANDSEED" {
				v.tags[1].Value = "-"
			}
			sorted = append(sorted, v.tags...)
		}
		out = append(append(append([]tags.Tag{}, out[:start]...), sorted...), out[end:]...)
	}
	return out
}

// t9DiffTags returns "" when a and b are the same sequence, else a description
// of the first difference with some context.
func t9DiffTags(a, b []tags.Tag) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			lo := i - 6
			if lo < 0 {
				lo = 0
			}
			var ctx []string
			for _, tg := range a[lo:i] {
				ctx = append(ctx, fmt.Sprintf("%d=%q", tg.Code, tg.Value))
			}
			return fmt.Sprintf("tag #%d: want %d=%q got %d=%q (context: %s)", i, a[i].Code, a[i].Value, b[i].Code, b[i].Value, strings.Join(ctx, " "))
		}
	}
	if len(a) != len(b) {
		return fmt.Sprintf("lengths differ: want %d tags, got %d (first extra: %v)", len(a), len(b), func() tags.Tag {
			if len(a) > n {
				return a[n]
			}
			return b[n]
		}())
	}
	return ""
}

// t9Fixtures returns the DXF files of R13+ the T9 round trips run on.
func t9Fixtures(t testing.TB) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("testdata", "oracle", "*.dxf"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, filepath.Join("testdata", "t1", "v00-baseline.dxf"))
	return files
}

// t9V00Entities adds to doc the three entities of testdata/t1/v00-baseline.dxf.
func t9V00Entities(doc *Document) {
	doc.Entities = []Entity{
		&Line{EntityCommon: EntityCommon{LayerName: "0"}, End: Point3D{X: 100}},
		&Circle{EntityCommon: EntityCommon{LayerName: "0"}, Center: Point3D{X: 50, Y: 50}, Radius: 25},
		&Text{EntityCommon: EntityCommon{LayerName: "0"}, Value: "Prova àèìòù ±°", Position: Point3D{Y: 10}, Height: 5, XScale: 1},
	}
}
