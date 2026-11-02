package dxf

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/apantanella/dxf/tags"
)

// t9Handles returns the numeric value of every 5 and 105 tag of ts, and the
// value of $HANDSEED.
func t9Handles(t testing.TB, ts []tags.Tag) (handles []uint64, seed uint64) {
	t.Helper()
	for i, tg := range ts {
		if tg.Code == 5 || tg.Code == 105 {
			v, err := strconv.ParseUint(tg.Value, 16, 64)
			if err != nil {
				t.Fatalf("bad handle %q", tg.Value)
			}
			if i > 0 && ts[i-1].Code == 9 && ts[i-1].Value == "$HANDSEED" {
				seed = v
				continue
			}
			handles = append(handles, v)
		}
	}
	return handles, seed
}

// t9StripHandles removes the tags tagdiff.py --ignore-handles ignores.
func t9StripHandles(in []tags.Tag) []tags.Tag {
	var out []tags.Tag
	for _, tg := range in {
		c := tg.Code
		switch {
		case c == 5, c == 105, c == 320, c == 330, c == 340, c == 350, c == 360, c == 390, c == 1005:
			continue
		case c >= 331 && c <= 339, c >= 341 && c <= 349, c >= 361 && c <= 369, c >= 380 && c <= 389, c >= 391 && c <= 399:
			continue
		}
		out = append(out, tg)
	}
	return out
}

// v00WithoutEntities returns the tags of the reference file without its LINE,
// CIRCLE and TEXT.
func t9V00WithoutEntities(t testing.TB) []tags.Tag {
	ts := t9Tags(t, t9ReadFile(t, "testdata/t1/v00-baseline.dxf"))
	var out []tags.Tag
	in := false
	for i, tg := range ts {
		if tg.Code == 2 && tg.Value == "ENTITIES" {
			in = true
			out = append(out, tg)
			continue
		}
		if in {
			if tg.Code == 0 && tg.Value == "ENDSEC" {
				in = false
				out = append(out, tg)
			}
			continue
		}
		_ = i
		out = append(out, tg)
	}
	return out
}

// TestR11EmptyDocument: NewDocumentVersion(R2013) written equals v00 without
// its three entities, apart from the handles.
func TestR11EmptyDocument(t *testing.T) {
	t.Parallel()
	got := t9StripHandles(t9Tags(t, t9Encode(t, NewDocumentVersion(VersionR2013))))
	want := t9StripHandles(t9V00WithoutEntities(t))
	if d := t9DiffTags(want, got); d != "" {
		t.Errorf("empty document differs from v00: %s", d)
	}
}

// TestR11V00Reproduced: the three entities of v00 give the whole of v00, apart
// from the handles and the $EXTMAX that v00 fixes to (100,100,0).
func TestR11V00Reproduced(t *testing.T) {
	t.Parallel()
	doc := NewDocumentVersion(VersionR2013)
	t9V00Entities(doc)
	doc.Header.Variables["$EXTMAX"] = Variable{Name: "$EXTMAX", Groups: []Group{{Code: 10, Value: "100.0"}, {Code: 20, Value: "100.0"}, {Code: 30, Value: "0.0"}}}
	out := t9Encode(t, doc)
	got := t9StripHandles(t9Tags(t, out))
	want := t9StripHandles(t9Tags(t, t9ReadFile(t, "testdata/t1/v00-baseline.dxf")))
	if d := t9DiffTags(want, got); d != "" {
		t.Errorf("document differs from v00: %s", d)
	}
	if !bytes.Contains(out, []byte("\r\n")) || bytes.Contains(bytes.ReplaceAll(out, []byte("\r\n"), nil), []byte("\n")) {
		t.Error("lines must end with CR LF")
	}
	if !bytes.HasPrefix(out, []byte("  0\r\nSECTION\r\n  2\r\nHEADER\r\n")) {
		t.Errorf("codes must be right-aligned on three columns: %q", out[:40])
	}
}

// TestR11ExtentsComputed: without $EXTMIN/$EXTMAX in the document the writer
// computes them from the entities.
func TestR11ExtentsComputed(t *testing.T) {
	t.Parallel()
	doc := NewDocumentVersion(VersionR2013)
	doc.Entities = []Entity{&Line{EntityCommon: EntityCommon{LayerName: "0"}, Start: Point3D{X: -5, Y: 2}, End: Point3D{X: 1000, Y: 30}}}
	got := t9Decode(t, t9Encode(t, doc))
	min, _ := got.Header.Variables["$EXTMIN"]
	max, _ := got.Header.Variables["$EXTMAX"]
	if min.Groups[0].Value != "-5.0" || max.Groups[0].Value != "1000.0" || max.Groups[1].Value != "30.0" {
		t.Errorf("extents = %v %v", min.Groups, max.Groups)
	}
	if _, ok := doc.Header.Get("$EXTMIN"); ok {
		t.Error("Encode modified the header of the document")
	}
}

// TestR11HandleSeed: $HANDSEED is greater than every handle written, whether
// the document had handles or not, and DIMSTYLE 105 counts.
func TestR11HandleSeed(t *testing.T) {
	t.Parallel()
	docs := map[string]*Document{
		"empty":  NewDocumentVersion(VersionR2013),
		"sample": sampleDocumentR2013(),
	}
	// A document that already has handles, one of them large.
	withHandles := sampleDocumentR2013()
	withHandles.Entities[0].Common().Handle = "ABCDE"
	docs["large"] = withHandles
	// A document with a $HANDSEED above all its handles.
	seeded := NewDocumentVersion(VersionR2013)
	t9V00Entities(seeded)
	seeded.Header.Set("$HANDSEED", 5, "5000")
	docs["seeded"] = seeded
	for name, doc := range docs {
		name, doc := name, doc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ts := t9Tags(t, t9Encode(t, doc))
			handles, seed := t9Handles(t, ts)
			var max uint64
			seen := map[uint64]bool{}
			for _, h := range handles {
				if h > max {
					max = h
				}
				if seen[h] {
					t.Errorf("handle %X used twice", h)
				}
				seen[h] = true
			}
			if seed <= max {
				t.Errorf("$HANDSEED %X <= highest handle %X", seed, max)
			}
			if name != "seeded" && seed != max+1 {
				t.Errorf("$HANDSEED %X, want %X (the first free handle)", seed, max+1)
			}
			if name == "seeded" && (seed < 0x5000 || max < 0x5000) {
				t.Errorf("seed %X max %X: new handles must start from the document seed", seed, max)
			}
		})
	}
}

// TestR12WriterIdempotent (ADR 0005, criterion 1): from the second write on
// the output does not change, for documents made by the library, in every
// writable version, and for the fixtures.
func TestR12WriterIdempotent(t *testing.T) {
	t.Parallel()
	for _, version := range writableVersions {
		version := version
		t.Run(version, func(t *testing.T) {
			t.Parallel()
			doc := sampleDocument()
			doc.Version = version
			first := t9Encode(t, doc)
			second := t9Encode(t, t9Decode(t, first))
			if !bytes.Equal(first, second) {
				t.Fatalf("second write differs: %s", t9DiffTags(t9Tags(t, first), t9Tags(t, second)))
			}
			third := t9Encode(t, t9Decode(t, second))
			if !bytes.Equal(second, third) {
				t.Errorf("third write differs")
			}
			// Determinism: the same document twice gives the same bytes.
			if again := t9Encode(t, doc); !bytes.Equal(first, again) {
				t.Error("two Encode calls on the same document differ")
			}
		})
	}
	for _, path := range t9Fixtures(t) {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			first := t9Encode(t, t9Decode(t, t9ReadFile(t, path)))
			second := t9Encode(t, t9Decode(t, first))
			if !bytes.Equal(first, second) {
				t.Errorf("second write differs: %s", t9DiffTags(t9Tags(t, first), t9Tags(t, second)))
			}
		})
	}
}

// TestR12EncodeDoesNotModifyDocument: Encode leaves the document as it was,
// for a file with handles and for one without.
func TestR12EncodeDoesNotModifyDocument(t *testing.T) {
	t.Parallel()
	for _, path := range t9Fixtures(t) {
		data := t9ReadFile(t, path)
		doc, ref := t9Decode(t, data), t9Decode(t, data)
		t9Encode(t, doc)
		t9Encode(t, doc, WithVersion(VersionR2000))
		if !reflect.DeepEqual(doc, ref) {
			t.Errorf("%s: Encode modified the document", path)
		}
	}
	doc := sampleDocumentR2013()
	ref := sampleDocumentR2013()
	t9Encode(t, doc)
	if !reflect.DeepEqual(doc, ref) {
		t.Error("Encode modified the sample document")
	}
}

// TestR11VersionSelection covers the order WithVersion, Document.Version,
// $ACADVER, R12 and the versions that cannot be written.
func TestR11VersionSelection(t *testing.T) {
	t.Parallel()
	acadver := func(data []byte) string {
		ts := t9Tags(t, data)
		for i, tg := range ts {
			if tg.Code == 9 && tg.Value == "$ACADVER" {
				return ts[i+1].Value
			}
		}
		return ""
	}
	// WithVersion wins over Document.Version.
	doc := NewDocumentVersion(VersionR2013)
	if v := acadver(t9Encode(t, doc, WithVersion(VersionR2018))); v != VersionR2018 {
		t.Errorf("WithVersion(R2018) wrote %s", v)
	}
	if v := acadver(t9Encode(t, doc, WithVersion("ac1015"))); v != VersionR2000 {
		t.Errorf("WithVersion(ac1015) wrote %s", v)
	}
	// Empty version: the header, then R12.
	empty := &Document{}
	if v := acadver(t9Encode(t, empty)); v != VersionR12 {
		t.Errorf("empty document wrote %s", v)
	}
	hdr := &Document{}
	hdr.Header.Set("$ACADVER", 1, VersionR2010)
	if v := acadver(t9Encode(t, hdr)); v != VersionR2010 {
		t.Errorf("header version wrote %s", v)
	}
	// The T9 example: read as R2000, written as R2013.
	read := t9Decode(t, t9ReadFile(t, "testdata/oracle/r2000-entities.dxf"))
	read.Version = VersionR2013
	if v := acadver(t9Encode(t, read)); v != VersionR2013 {
		t.Errorf("converted document wrote %s", v)
	}
	// Versions that cannot be written.
	for _, v := range []string{VersionR13, VersionR14, "AC1006", "AC9999", "garbage"} {
		var b bytes.Buffer
		err := Encode(&b, NewDocument(), WithVersion(v))
		var verr *VersionError
		if !errors.Is(err, ErrValidation) || !errors.Is(err, ErrUnsupportedVersion) || !errors.As(err, &verr) {
			t.Errorf("WithVersion(%q): %v, want ErrValidation, ErrUnsupportedVersion and *VersionError", v, err)
		}
		if b.Len() != 0 {
			t.Errorf("WithVersion(%q) wrote %d bytes before failing", v, b.Len())
		}
	}
	bad := NewDocument()
	bad.Version = VersionR14
	if err := Encode(&bytes.Buffer{}, bad); !errors.Is(err, ErrUnsupportedVersion) {
		t.Errorf("Document.Version R14: %v", err)
	}
	if d := NewDocumentVersion(VersionR14); d.Version != VersionR14 || Encode(&bytes.Buffer{}, d) == nil {
		t.Error("NewDocumentVersion(R14) must give a document Encode rejects")
	}
}

// TestR11NonRepresentableContent: errors that name what cannot be written.
func TestR11NonRepresentableContent(t *testing.T) {
	t.Parallel()
	check := func(name string, doc *Document, opts ...EncodeOption) {
		t.Helper()
		var b bytes.Buffer
		err := Encode(&b, doc, opts...)
		if !errors.Is(err, ErrValidation) {
			t.Errorf("%s: %v, want ErrValidation", name, err)
			return
		}
		if b.Len() != 0 {
			t.Errorf("%s: %d bytes written before the error", name, b.Len())
		}
	}
	// An R12 entity the package does not interpret has no subclass markers.
	r12 := NewDocumentVersion(VersionR12)
	r12.Entities = []Entity{&RawEntity{EntityCommon: EntityCommon{LayerName: "0"}, TypeName: "VIEWPORT", Groups: []Group{{Code: 40, Value: "1"}}}}
	check("R12 raw entity as R2013", r12, WithVersion(VersionR2013))
	// Polygon and polyface meshes cannot be created (their counts are not modelled).
	mesh := NewDocumentVersion(VersionR2013)
	mesh.Entities = []Entity{&Polyline{EntityCommon: EntityCommon{LayerName: "0"}, Flags: 64, Vertices: []Vertex{{}, {Position: Point3D{X: 1}}}}}
	check("polyface mesh", mesh)
	// R13+ content cannot be written as R12 (ADR 0007).
	for _, path := range []string{"testdata/oracle/r2013-entities.dxf", "testdata/t1/v00-baseline.dxf"} {
		check(path+" as R12", t9Decode(t, t9ReadFile(t, path)), WithVersion(VersionR12))
	}
	modernEntity := NewDocumentVersion(VersionR12)
	modernEntity.Entities = []Entity{&RawEntity{TypeName: "LWPOLYLINE", Verbatim: true, Groups: []Group{{Code: 5, Value: "A"}, {Code: 100, Value: "AcDbEntity"}}}}
	check("verbatim entity as R12", modernEntity)
	rawObj := NewDocumentVersion(VersionR12)
	rawObj.Objects = []Object{&RawObject{TypeName: "XRECORD", Groups: []Group{{Code: 5, Value: "A"}}}}
	check("RawObject as R12", rawObj)
	// The message names the element.
	err := Encode(&bytes.Buffer{}, rawObj)
	if err == nil || !strings.Contains(err.Error(), "XRECORD") {
		t.Errorf("error does not name the object: %v", err)
	}
	err = Encode(&bytes.Buffer{}, r12, WithVersion(VersionR2013))
	if err == nil || !strings.Contains(err.Error(), "VIEWPORT") {
		t.Errorf("error does not name the entity: %v", err)
	}
	// Objects and classes that are only structure are ignored by the R12 writer (T8).
	ok := NewDocumentVersion(VersionR12)
	ok.Classes = []Class{{DXFName: "X", CppName: "Y", AppName: "Z"}}
	ok.Objects = []Object{&Dictionary{Handle: "1"}}
	t9Encode(t, ok)
}

// TestR11FormatFloat covers the number format of the R2000+ writer: the one of
// the T1 files (repr of a Python float), which differs from the R12 writer for
// whole numbers and large values.
func TestR11FormatFloat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		v    float64
		prec int
		want string
	}{
		{0, -1, "0.0"}, {math.Copysign(0, -1), -1, "0.0"}, {1, -1, "1.0"}, {100, -1, "100.0"}, {-2.5, -1, "-2.5"},
		{1.34, -1, "1.34"}, {0.1, -1, "0.1"}, {0.0001, -1, "0.0001"}, {0.00001, -1, "1e-05"},
		{1e15, -1, "1000000000000000.0"}, {1e16, -1, "1e+16"}, {1e20, -1, "1e+20"}, {-1e20, -1, "-1e+20"},
		{1.5e20, -1, "1.5e+20"}, {123456789.125, -1, "123456789.125"}, {math.Pi, -1, "3.141592653589793"},
		{2.5, 3, "2.500"}, {1e20, 1, "100000000000000000000.0"},
	}
	for _, tc := range tests {
		if got := formatFloatModern(tc.v, tc.prec); got != tc.want {
			t.Errorf("formatFloatModern(%v, %d) = %q, want %q", tc.v, tc.prec, got, tc.want)
		}
		if tc.prec < 0 {
			if back, err := strconv.ParseFloat(formatFloatModern(tc.v, tc.prec), 64); err != nil || back != tc.v {
				t.Errorf("%v does not read back: %v %v", tc.v, back, err)
			}
		}
	}
	// The R12 writer is unchanged.
	if got := formatFloat(100, -1); got != "100" {
		t.Errorf("R12 formatFloat(100) = %q", got)
	}
	// WithPrecision applies to the typed values of the R2000+ writer too.
	doc := NewDocumentVersion(VersionR2013)
	doc.Entities = []Entity{&Circle{EntityCommon: EntityCommon{LayerName: "0"}, Center: Point3D{X: 1.23456}, Radius: 2}}
	if out := string(t9Encode(t, doc, WithPrecision(2))); !strings.Contains(out, "\r\n1.23\r\n") || !strings.Contains(out, "\r\n2.00\r\n") {
		t.Errorf("WithPrecision(2) not applied:\n%s", out)
	}
}

// TestR11TextEncoding: UTF-8 from R2007, the code page of $DWGCODEPAGE before
// (ADR 0004). Text that is not valid UTF-8 (bytes read from a file) passes unchanged.
func TestR11TextEncoding(t *testing.T) {
	t.Parallel()
	mk := func(version, text string) []byte {
		doc := NewDocumentVersion(version)
		doc.Entities = []Entity{&Text{EntityCommon: EntityCommon{LayerName: "0"}, Value: text, Height: 1, XScale: 1}}
		return t9Encode(t, doc)
	}
	utf8 := []byte("\r\nà°\r\n")
	cp1252 := []byte("\r\n\xe0\xb0\r\n")
	for _, v := range []string{VersionR2007, VersionR2010, VersionR2013, VersionR2018} {
		if out := mk(v, "à°"); !bytes.Contains(out, utf8) || bytes.Contains(out, cp1252) {
			t.Errorf("%s: text must be UTF-8", v)
		}
	}
	for _, v := range []string{VersionR2000, VersionR2004} {
		if out := mk(v, "à°"); !bytes.Contains(out, cp1252) || bytes.Contains(out, utf8) {
			t.Errorf("%s: text must be CP1252", v)
		}
		// A character outside the code page becomes \U+XXXX.
		if out := mk(v, "€ Ж"); !bytes.Contains(out, []byte("\x80 \\U+0416")) {
			t.Errorf("%s: unrepresentable character not escaped", v)
		}
		// Bytes that are not UTF-8 are not touched.
		if out := mk(v, "\xe0\xb0"); !bytes.Contains(out, cp1252) {
			t.Errorf("%s: raw bytes were altered", v)
		}
	}
	if out := mk(VersionR2013, "€"); !bytes.Contains(out, []byte("€")) {
		t.Error("R2013: euro sign must stay UTF-8")
	}
}

// TestR11PreservedPositions: groups the model does not interpret (reactors,
// extension dictionary, lineweight, extended data, paper space flag) are
// written where they were read, and an entity of paper space keeps its owner.
func TestR11PreservedPositions(t *testing.T) {
	t.Parallel()
	doc0 := NewDocumentVersion(VersionR2013)
	doc0.Entities = []Entity{&Line{EntityCommon: EntityCommon{LayerName: "0"}, End: Point3D{X: 1}}}
	base := string(t9Encode(t, doc0))
	recs := t9Records(t9Tags(t, []byte(base)))
	var paper string
	for _, r := range recs {
		if r.typ == "BLOCK_RECORD" && r.name == "*Paper_Space" {
			paper = r.handle
		}
	}
	// A LINE as another CAD writes it: reactors and extension dictionary
	// between 5 and 330, paper space flag, lineweight and true colour after the
	// layer, thickness in the LINE part, extended data at the end.
	record := strings.Join([]string{
		"  0", "LINE", "  5", "ABC", "102", "{ACAD_XDICTIONARY", "360", "ABD", "102", "}",
		"102", "{ACAD_REACTORS", "330", "ABE", "102", "}", "330", paper,
		"100", "AcDbEntity", " 67", "1", "  8", "0", "370", "   35", "420", "255",
		"100", "AcDbLine", " 39", "2.0", " 10", "0.0", " 20", "0.0", " 30", "0.0", " 11", "1.0", " 21", "0.0", " 31", "0.0",
		"1001", "ACAD", "1000", "nota", "",
	}, "\r\n")
	start := strings.Index(base, "  0\r\nLINE\r\n")
	end := strings.Index(base[start:], "  0\r\nENDSEC") + start
	src := base[:start] + record + base[end:]
	doc := t9Decode(t, []byte(src))
	out := t9Encode(t, doc)
	want := t9Tags(t, []byte(src))
	got := t9Tags(t, out)
	if d := t9DiffTags(t9Normalize(want), t9Normalize(got)); d != "" {
		t.Errorf("preserved groups moved: %s", d)
	}
	// An entity built by hand with only the paper space flag belongs to *Paper_Space.
	hand := NewDocumentVersion(VersionR2013)
	hand.Entities = []Entity{&Line{EntityCommon: EntityCommon{LayerName: "0", Raw: []Group{{Code: 67, Value: "1"}}}, End: Point3D{X: 1}}}
	var lineOwner, paperH string
	for _, r := range t9Records(t9Tags(t, t9Encode(t, hand))) {
		switch {
		case r.typ == "LINE":
			lineOwner = r.owner
			// 67 sits right after AcDbEntity, before the layer.
			if !strings.Contains(fmt.Sprint(r.tags), "100: AcDbEntity 67: 1 8: 0") {
				t.Errorf("67 not placed after AcDbEntity: %v", r.tags)
			}
		case r.typ == "BLOCK_RECORD" && r.name == "*Paper_Space":
			paperH = r.handle
		}
	}
	if lineOwner == "" || lineOwner != paperH {
		t.Errorf("owner %q, want the *Paper_Space record %q", lineOwner, paperH)
	}
}

func BenchmarkEncodeLinesR2013(b *testing.B) {
	doc, err := Decode(strings.NewReader(buildBenchmarkInput(b, benchmarkEntities)))
	if err != nil {
		b.Fatalf("Decode() = %v", err)
	}
	doc.Version = VersionR2013
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out bytes.Buffer
		if err := Encode(&out, doc); err != nil {
			b.Fatalf("Encode() = %v", err)
		}
	}
}
