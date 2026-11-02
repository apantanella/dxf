package dxf

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

// tagText turns a compact listing into DXF text. Each line is "code value":
// the code ends at the first space and the rest of the line, possibly empty
// or with spaces, is the value.
func tagText(listing string) string {
	var b strings.Builder
	for _, line := range strings.Split(listing, "\n") {
		line = strings.TrimLeft(line, " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		code, value, _ := strings.Cut(line, " ")
		b.WriteString(code)
		b.WriteString("\n")
		b.WriteString(value)
		b.WriteString("\n")
	}
	return b.String()
}

// tagList parses the same listing into groups.
func tagList(t *testing.T, listing string) []Group {
	t.Helper()
	var out []Group
	for _, line := range strings.Split(listing, "\n") {
		line = strings.TrimLeft(line, " \t")
		if strings.TrimSpace(line) == "" {
			continue
		}
		code, value, _ := strings.Cut(line, " ")
		var c int
		for _, r := range code {
			c = c*10 + int(r-'0')
		}
		out = append(out, Group{Code: c, Value: value})
	}
	return out
}

// t6File builds a file with the given $ACADVER and sections. Each section is
// (name, listing).
func t6File(version string, sections ...string) string {
	var b strings.Builder
	b.WriteString(tagText("0 SECTION\n2 HEADER\n9 $ACADVER\n1 " + version + "\n0 ENDSEC"))
	for i := 0; i+1 < len(sections); i += 2 {
		b.WriteString(tagText("0 SECTION\n2 " + sections[i]))
		b.WriteString(tagText(sections[i+1]))
		b.WriteString(tagText("0 ENDSEC"))
	}
	b.WriteString(tagText("0 EOF"))
	return b.String()
}

func decodeT6(t *testing.T, input string, opts ...DecodeOption) *Document {
	t.Helper()
	doc, err := Decode(strings.NewReader(input), opts...)
	if err != nil {
		t.Fatalf("Decode() = %v", err)
	}
	return doc
}

func noWarnings(t *testing.T, doc *Document) {
	t.Helper()
	for _, d := range doc.Diagnostics {
		if d.Severity != SeverityInfo {
			t.Errorf("unexpected diagnostic: %v", d)
		}
	}
}

func TestR2Versions(t *testing.T) {
	t.Parallel()

	versions := []string{
		VersionR12, VersionR13, VersionR14, VersionR2000, VersionR2004,
		VersionR2007, VersionR2010, VersionR2013, VersionR2018,
	}
	want := []string{"AC1009", "AC1012", "AC1014", "AC1015", "AC1018", "AC1021", "AC1024", "AC1027", "AC1032"}
	if !reflect.DeepEqual(versions, want) {
		t.Fatalf("version constants = %v, want %v", versions, want)
	}
	for _, v := range versions {
		t.Run(v, func(t *testing.T) {
			t.Parallel()
			input := t6File(v, "ENTITIES", "0 LINE\n8 0\n10 0.0\n20 0.0\n11 1.0\n21 1.0")
			doc := decodeT6(t, input)
			if doc.Version != v {
				t.Errorf("Version = %q, want %q", doc.Version, v)
			}
			noWarnings(t, doc)
			if len(doc.Entities) != 1 {
				t.Errorf("entities = %d, want 1", len(doc.Entities))
			}
			// Case does not matter in the file; the canonical spelling wins.
			lower := strings.Replace(input, v, strings.ToLower(v), 1)
			if got := decodeT6(t, lower).Version; got != v {
				t.Errorf("Version of lower-case file = %q, want %q", got, v)
			}
		})
	}

	t.Run("unsupported", func(t *testing.T) {
		t.Parallel()
		input := t6File("AC1099", "ENTITIES", "0 LINE\n8 0")
		_, err := Decode(strings.NewReader(input))
		if !errors.Is(err, ErrUnsupportedVersion) {
			t.Fatalf("Decode() = %v, want ErrUnsupportedVersion", err)
		}
		var ve *VersionError
		if !errors.As(err, &ve) || ve.Found != "AC1099" {
			t.Fatalf("error = %#v, want *VersionError for AC1099", err)
		}
		if msg := err.Error(); !strings.Contains(msg, "AC1099") || !strings.Contains(msg, "AC1032") || strings.Contains(msg, "expected AC1009") {
			t.Errorf("message = %q", msg)
		}

		doc := decodeT6(t, input, WithLenient())
		if doc.Version != "AC1099" {
			t.Errorf("lenient Version = %q", doc.Version)
		}
		warned := false
		for _, d := range doc.Diagnostics {
			if d.Severity == SeverityWarning && strings.Contains(d.Message, "AC1099") {
				warned = true
			}
		}
		if !warned {
			t.Errorf("no warning for the unsupported version: %v", doc.Diagnostics)
		}
		if len(doc.Entities) != 1 {
			t.Errorf("entities = %d, want 1", len(doc.Entities))
		}
	})

	t.Run("best effort", func(t *testing.T) {
		t.Parallel()
		doc := decodeT6(t, t6File(VersionR14, "ENTITIES", "0 LINE\n8 0"))
		found := false
		for _, d := range doc.Diagnostics {
			if d.Severity == SeverityInfo && strings.Contains(d.Message, "best-effort") {
				found = true
			}
		}
		if !found {
			t.Errorf("no best-effort note for R14: %v", doc.Diagnostics)
		}
	})
}

// emitLine rebuilds the tag sequence of a LINE record from the document model,
// the way a writer for R13 and later is expected to place things: the handle,
// the groups kept before the first marker, the owner, then every subclass with
// its data followed by the groups kept for it.
func emitLine(line *Line) []Group {
	out := []Group{{Code: 0, Value: "LINE"}, {Code: 5, Value: line.Handle}}
	raw := func(ord int) {
		for i, g := range line.Raw {
			if line.RawSubclass[i] == ord {
				out = append(out, g)
			}
		}
	}
	raw(0)
	out = append(out, Group{Code: 330, Value: line.Owner.String()})
	for k, name := range line.Subclasses {
		out = append(out, Group{Code: 100, Value: name})
		switch name {
		case "AcDbEntity":
			out = append(out, Group{Code: 8, Value: line.LayerName})
		case "AcDbLine":
			out = append(out,
				Group{Code: 10, Value: "0.0"}, Group{Code: 20, Value: "0.0"}, Group{Code: 30, Value: "0.0"},
				Group{Code: 11, Value: "100.0"}, Group{Code: 21, Value: "50.0"}, Group{Code: 31, Value: "0.0"})
		}
		raw(k + 1)
	}
	return out
}

func TestR3ReactorsAndXDict(t *testing.T) {
	t.Parallel()

	const lineListing = `0 LINE
5 1F
102 {ACAD_XDICTIONARY
360 4A
102 }
102 {ACAD_REACTORS
330 4B
102 }
330 17
100 AcDbEntity
8 0
100 AcDbLine
10 0.0
20 0.0
30 0.0
11 100.0
21 50.0
31 0.0
1001 ACAD
1000 note
1070 5`
	const lwListing = `0 LWPOLYLINE
5 22
102 {ACAD_REACTORS
330 4B
102 }
330 17
100 AcDbEntity
8 WALLS
62 3
100 AcDbPolyline
90 2
70 1
10 0.0
20 0.0
10 5.0
20 0.0
1001 ACAD
1000 xdata`
	input := t6File(VersionR2018, "ENTITIES", lineListing+"\n"+lwListing)
	doc := decodeT6(t, input)
	noWarnings(t, doc)
	if len(doc.Entities) != 2 {
		t.Fatalf("entities = %d, want 2", len(doc.Entities))
	}

	line, ok := doc.Entities[0].(*Line)
	if !ok {
		t.Fatalf("entity 0 = %T, want *Line", doc.Entities[0])
	}
	if line.Handle != "1F" || line.Owner != 0x17 {
		t.Errorf("handle/owner = %q/%v, want 1F/17 (the 330 inside ACAD_REACTORS is not the owner)", line.Handle, line.Owner)
	}
	wantRaw := tagList(t, `102 {ACAD_XDICTIONARY
360 4A
102 }
102 {ACAD_REACTORS
330 4B
102 }
1001 ACAD
1000 note
1070 5`)
	if !reflect.DeepEqual(line.Raw, wantRaw) {
		t.Errorf("Raw = %v, want %v", line.Raw, wantRaw)
	}
	if want := []int{0, 0, 0, 0, 0, 0, 2, 2, 2}; !reflect.DeepEqual(line.RawSubclass, want) {
		t.Errorf("RawSubclass = %v, want %v", line.RawSubclass, want)
	}
	// The model holds every tag: rebuilding the record gives the original
	// sequence.
	if got, want := emitLine(line), tagList(t, lineListing); !reflect.DeepEqual(got, want) {
		t.Errorf("rebuilt LINE:\n got %v\nwant %v", got, want)
	}

	raw, ok := doc.Entities[1].(*RawEntity)
	if !ok {
		t.Fatalf("entity 1 = %T, want *RawEntity", doc.Entities[1])
	}
	if !raw.Verbatim || raw.TypeName != "LWPOLYLINE" {
		t.Errorf("raw = %+v", raw)
	}
	lwTags := tagList(t, lwListing)
	if want := lwTags[1:]; !reflect.DeepEqual(raw.Groups, want) {
		t.Errorf("Groups = %v, want every tag after the type record %v", raw.Groups, want)
	}
	if raw.Handle != "22" || raw.Owner != 0x17 || raw.LayerName != "WALLS" || raw.Color != 3 || !raw.HasColor {
		t.Errorf("view = handle %q owner %v layer %q color %d", raw.Handle, raw.Owner, raw.LayerName, raw.Color)
	}
	if want := []string{"AcDbEntity", "AcDbPolyline"}; !reflect.DeepEqual(raw.Subclasses, want) {
		t.Errorf("Subclasses = %v, want %v", raw.Subclasses, want)
	}
}

func TestR3SubclassesRecorded(t *testing.T) {
	t.Parallel()

	const entities = `0 LINE
5 30
330 17
100 AcDbEntity
8 0
100 AcDbLine
10 0.0
20 0.0
30 0.0
11 1.0
21 1.0
31 0.0
0 ARC
5 31
330 17
100 AcDbEntity
8 0
100 AcDbCircle
10 1.0
20 2.0
30 0.0
40 5.0
100 AcDbArc
50 30.0
51 90.0
0 TEXT
5 32
330 17
100 AcDbEntity
8 0
100 AcDbText
10 1.0
20 2.0
30 0.0
40 2.5
1 Hello àè
72 1
11 3.0
21 4.0
31 0.0
100 AcDbText
73 2
0 POLYLINE
5 40
330 17
100 AcDbEntity
8 0
100 AcDb2dPolyline
66 1
10 0.0
20 0.0
30 0.0
70 1
0 VERTEX
5 41
330 40
100 AcDbEntity
8 0
100 AcDbVertex
100 AcDb2dVertex
10 1.0
20 2.0
30 0.0
42 0.5
70 0
0 VERTEX
5 42
330 40
100 AcDbEntity
8 0
100 AcDbVertex
100 AcDb2dVertex
10 3.0
20 2.0
30 0.0
0 SEQEND
5 43
330 40
100 AcDbEntity
8 0
0 INSERT
5 50
330 17
100 AcDbEntity
8 0
100 AcDbBlockReference
66 1
2 BLK
10 1.0
20 2.0
30 0.0
41 2.0
0 ATTRIB
5 51
330 50
100 AcDbEntity
8 0
100 AcDbText
10 1.0
20 2.0
30 0.0
40 2.5
1 VAL
71 4
72 1
11 1.0
21 2.0
31 0.0
100 AcDbAttribute
280 0
2 TAG
70 1
73 8
74 2
71 1
280 0
0 SEQEND
5 52
330 50
100 AcDbEntity
8 0`
	doc := decodeT6(t, t6File(VersionR2013, "ENTITIES", entities))
	noWarnings(t, doc)
	if len(doc.Entities) != 5 {
		t.Fatalf("entities = %d, want 5", len(doc.Entities))
	}
	sub := func(names ...string) []string { return names }

	line := doc.Entities[0].(*Line)
	if want := sub("AcDbEntity", "AcDbLine"); !reflect.DeepEqual(line.Subclasses, want) {
		t.Errorf("LINE subclasses = %v", line.Subclasses)
	}
	if line.End != (Point3D{X: 1, Y: 1}) || len(line.Raw) != 0 {
		t.Errorf("LINE = %+v", line)
	}

	arc := doc.Entities[1].(*Arc)
	if want := sub("AcDbEntity", "AcDbCircle", "AcDbArc"); !reflect.DeepEqual(arc.Subclasses, want) {
		t.Errorf("ARC subclasses = %v", arc.Subclasses)
	}
	if arc.Radius != 5 || arc.StartAngle != 30 || arc.EndAngle != 90 || arc.Center != (Point3D{X: 1, Y: 2}) {
		t.Errorf("ARC = %+v", arc)
	}

	text := doc.Entities[2].(*Text)
	if want := sub("AcDbEntity", "AcDbText", "AcDbText"); !reflect.DeepEqual(text.Subclasses, want) {
		t.Errorf("TEXT subclasses = %v", text.Subclasses)
	}
	if text.Value != "Hello àè" || text.HAlign != 1 || text.VAlign != 2 || text.Alignment != (Point3D{X: 3, Y: 4}) {
		t.Errorf("TEXT = %+v", text)
	}

	poly := doc.Entities[3].(*Polyline)
	if want := sub("AcDbEntity", "AcDb2dPolyline"); !reflect.DeepEqual(poly.Subclasses, want) {
		t.Errorf("POLYLINE subclasses = %v", poly.Subclasses)
	}
	if !poly.Closed() || len(poly.Vertices) != 2 || poly.SeqendHandle != "43" {
		t.Fatalf("POLYLINE = %+v", poly)
	}
	vertex := poly.Vertices[0]
	if want := sub("AcDbEntity", "AcDbVertex", "AcDb2dVertex"); !reflect.DeepEqual(vertex.Subclasses, want) {
		t.Errorf("VERTEX subclasses = %v", vertex.Subclasses)
	}
	if vertex.Position != (Point3D{X: 1, Y: 2}) || vertex.Bulge != 0.5 || vertex.Handle != "41" || vertex.Owner != 0x40 {
		t.Errorf("VERTEX = %+v", vertex)
	}
	// The layer of a vertex is not a decoded field: it stays in Raw, in the
	// AcDbEntity subclass (first marker).
	if want := []Group{{Code: 8, Value: "0"}}; !reflect.DeepEqual(vertex.Raw, want) || !reflect.DeepEqual(vertex.RawSubclass, []int{1}) {
		t.Errorf("VERTEX raw = %v at %v", vertex.Raw, vertex.RawSubclass)
	}
	wantSeqend := tagList(t, "330 40\n100 AcDbEntity\n8 0")
	if !reflect.DeepEqual(poly.SeqendGroups, wantSeqend) {
		t.Errorf("SeqendGroups = %v, want %v", poly.SeqendGroups, wantSeqend)
	}

	insert := doc.Entities[4].(*Insert)
	if want := sub("AcDbEntity", "AcDbBlockReference"); !reflect.DeepEqual(insert.Subclasses, want) {
		t.Errorf("INSERT subclasses = %v", insert.Subclasses)
	}
	if insert.BlockName != "BLK" || insert.XScale != 2 || insert.Handle != "50" || insert.Owner != 0x17 || insert.SeqendHandle != "52" {
		t.Errorf("INSERT = %+v", insert)
	}
	if len(insert.Attributes) != 1 {
		t.Fatalf("attributes = %d, want 1", len(insert.Attributes))
	}
	attr := insert.Attributes[0]
	if want := sub("AcDbEntity", "AcDbText", "AcDbAttribute"); !reflect.DeepEqual(attr.Subclasses, want) {
		t.Errorf("ATTRIB subclasses = %v", attr.Subclasses)
	}
	if attr.Value != "VAL" || attr.Tag != "TAG" || attr.Flags != 1 || attr.FieldLength != 8 || attr.VAlign != 2 || attr.HAlign != 1 || attr.Owner != 0x50 {
		t.Errorf("ATTRIB = %+v", attr)
	}
	// The layer, the alignment point, the two 71 and the two 280 are not
	// decoded fields. Each stays in Raw with the subclass it came from: the
	// first 71 belongs to AcDbText, the second to AcDbAttribute.
	wantRaw := tagList(t, "8 0\n71 4\n11 1.0\n21 2.0\n31 0.0\n280 0\n71 1\n280 0")
	if !reflect.DeepEqual(attr.Raw, wantRaw) || !reflect.DeepEqual(attr.RawSubclass, []int{1, 2, 2, 2, 2, 3, 3, 3}) {
		t.Errorf("ATTRIB raw = %v at %v", attr.Raw, attr.RawSubclass)
	}
}

func TestT6SubclassCodesAreDistinguished(t *testing.T) {
	t.Parallel()

	// A TEXT whose 73 sits in the wrong place (before the second marker) and a
	// LINE whose subclasses are in an odd order still decode: the preferred
	// subclass is tried first, then the rest of the record.
	doc := decodeT6(t, t6File(VersionR2013, "ENTITIES", `0 ARC
5 1
100 AcDbEntity
8 0
100 AcDbArc
50 10.0
51 20.0
100 AcDbCircle
10 1.0
20 1.0
30 0.0
40 2.0
0 LINE
5 2
100 AcDbEntity
8 0
10 1.0
20 2.0
30 3.0
11 4.0
21 5.0
31 6.0`))
	arc := doc.Entities[0].(*Arc)
	if arc.StartAngle != 10 || arc.EndAngle != 20 || arc.Radius != 2 {
		t.Errorf("ARC = %+v", arc)
	}
	line := doc.Entities[1].(*Line)
	if line.Start != (Point3D{X: 1, Y: 2, Z: 3}) || line.End != (Point3D{X: 4, Y: 5, Z: 6}) {
		t.Errorf("LINE = %+v", line)
	}
}

func TestT6ModernVersionWithoutMarkers(t *testing.T) {
	t.Parallel()

	// A file that declares R2013 but carries no 100 markers is still read with
	// owners: the version decides.
	doc := decodeT6(t, t6File(VersionR2013, "ENTITIES", "0 LINE\n5 A\n330 B\n8 L\n10 0.0\n20 0.0\n11 1.0\n21 0.0"))
	line := doc.Entities[0].(*Line)
	if line.Owner != 0xB || line.Handle != "A" || len(line.Raw) != 0 || len(line.Subclasses) != 0 {
		t.Errorf("LINE = %+v", line.EntityCommon)
	}
}

func TestT6R12KeepsOwnerAsRaw(t *testing.T) {
	t.Parallel()

	doc := decodeT6(t, t6File(VersionR12, "ENTITIES", "0 LINE\n5 A\n330 B\n8 L\n10 0.0\n20 0.0\n11 1.0\n21 0.0"))
	line := doc.Entities[0].(*Line)
	if !line.Owner.IsZero() || line.RawSubclass != nil || line.Subclasses != nil {
		t.Errorf("R12 line = %+v", line.EntityCommon)
	}
	if want := []Group{{Code: 330, Value: "B"}}; !reflect.DeepEqual(line.Raw, want) {
		t.Errorf("Raw = %v, want %v", line.Raw, want)
	}
}

func TestT6UnknownEntitiesAreVerbatim(t *testing.T) {
	t.Parallel()

	listings := map[string]string{
		"LWPOLYLINE": `0 LWPOLYLINE
5 60
330 17
100 AcDbEntity
8 0
100 AcDbPolyline
90 3
70 0
43 0.0
10 0.0
20 0.0
10 1.0
20 0.0
10 1.0
20 1.0`,
		"MTEXT": `0 MTEXT
5 61
102 {ACAD_REACTORS
330 62
102 }
330 17
100 AcDbEntity
8 NOTES
100 AcDbMText
10 0.0
20 0.0
30 0.0
40 2.5
41 50.0
71 1
1 Line one\P  Line two
7 Standard`,
		"HATCH": `0 HATCH
5 63
330 17
100 AcDbEntity
8 0
62 1
100 AcDbHatch
10 0.0
20 0.0
30 0.0
210 0.0
220 0.0
230 1.0
2 SOLID
70 1
71 0
91 1
92 7
72 0
73 1
93 2
10 0.0
20 0.0
10 1.0
20 0.0
97 0
75 1
76 1
98 0`,
	}
	var all strings.Builder
	order := []string{"LWPOLYLINE", "MTEXT", "HATCH"}
	for _, name := range order {
		all.WriteString(listings[name])
		all.WriteString("\n")
	}
	doc := decodeT6(t, t6File(VersionR2013, "ENTITIES", all.String()))
	noWarnings(t, doc)
	if len(doc.Entities) != len(order) {
		t.Fatalf("entities = %d, want %d", len(doc.Entities), len(order))
	}
	for i, name := range order {
		raw, ok := doc.Entities[i].(*RawEntity)
		if !ok {
			t.Fatalf("%s: %T, want *RawEntity", name, doc.Entities[i])
		}
		tl := tagList(t, listings[name])
		if raw.TypeName != name || !raw.Verbatim {
			t.Errorf("%s: raw = %q verbatim %v", name, raw.TypeName, raw.Verbatim)
		}
		if !reflect.DeepEqual(raw.Groups, tl[1:]) {
			t.Errorf("%s: tags differ:\n got %v\nwant %v", name, raw.Groups, tl[1:])
		}
		if wantHandle := map[string]string{"LWPOLYLINE": "60", "MTEXT": "61", "HATCH": "63"}[name]; raw.Owner != 0x17 || raw.Handle != wantHandle {
			t.Errorf("%s: owner %v handle %q, want 17 %s", name, raw.Owner, raw.Handle, wantHandle)
		}
	}
	// The view skips the 330 inside the reactors group and the 62 of a hatch
	// is the entity color.
	if mt := doc.Entities[1].(*RawEntity); mt.Owner != 0x17 || mt.LayerName != "NOTES" || mt.Handle != "61" {
		t.Errorf("MTEXT view = %+v", mt.EntityCommon)
	}
	if h := doc.Entities[2].(*RawEntity); h.Color != 1 || !h.HasColor || h.Thickness != 0 || h.ExtrusionSet {
		t.Errorf("HATCH view = %+v", h.EntityCommon)
	}
	// Handles of unknown entities are part of the index.
	if _, ok := doc.Lookup(0x61); !ok {
		t.Error("Lookup(61) did not find the MTEXT")
	}
}

func TestT6UnknownObjectsAreRawObjects(t *testing.T) {
	t.Parallel()

	listings := []struct {
		name, listing string
		handle        string
		owner         Handle
	}{
		{"XRECORD", `0 XRECORD
5 70
102 {ACAD_REACTORS
330 71
102 }
330 72
100 AcDbXrecord
280 1
1 data
330 99`, "70", 0x72},
		{"MLINESTYLE", `0 MLINESTYLE
5 73
330 74
100 AcDbMlineStyle
2 STANDARD
70 0
3
62 256
51 90.0
52 90.0
71 2
49 0.5
62 256
6 BYLAYER
49 -0.5
62 256
6 BYLAYER`, "73", 0x74},
		{"SCALELIST", `0 SCALELIST
5 75
102 {ACAD_XDICTIONARY
360 76
102 }
330 77
100 AcDbScaleList`, "75", 0x77},
	}
	var all strings.Builder
	for _, l := range listings {
		all.WriteString(l.listing + "\n")
	}
	doc := decodeT6(t, t6File(VersionR2013, "OBJECTS", all.String()))
	noWarnings(t, doc)
	if len(doc.Objects) != len(listings) {
		t.Fatalf("objects = %d, want %d", len(doc.Objects), len(listings))
	}
	for i, l := range listings {
		raw, ok := doc.Objects[i].(*RawObject)
		if !ok {
			t.Fatalf("%s: %T, want *RawObject", l.name, doc.Objects[i])
		}
		tl := tagList(t, l.listing)
		if raw.Type() != l.name || raw.TypeName != l.name {
			t.Errorf("%s: type = %q", l.name, raw.Type())
		}
		if !reflect.DeepEqual(raw.Groups, tl[1:]) {
			t.Errorf("%s: tags differ:\n got %v\nwant %v", l.name, raw.Groups, tl[1:])
		}
		if raw.Handle != l.handle || raw.Owner != l.owner || raw.ObjectOwner() != l.owner {
			t.Errorf("%s: handle %q owner %v, want %q %v", l.name, raw.Handle, raw.Owner, l.handle, l.owner)
		}
		if got := raw.ObjectHandle().String(); got != l.handle {
			t.Errorf("%s: ObjectHandle = %s", l.name, got)
		}
	}
	// Objects are in the handle index and take part in the duplicate check.
	if obj, ok := doc.Lookup(0x73); !ok || obj != doc.Objects[1] {
		t.Errorf("Lookup(73) = %v, %v", obj, ok)
	}
	dup := t6File(VersionR2013, "OBJECTS", "0 XRECORD\n5 70\n330 1\n0 SCALELIST\n5 70\n330 1")
	if _, err := Decode(strings.NewReader(dup)); !errors.Is(err, ErrValidation) {
		t.Errorf("duplicate object handle: Decode() = %v, want ErrValidation", err)
	}
	if _, err := Decode(strings.NewReader(dup), WithLenient()); err != nil {
		t.Errorf("duplicate object handle, lenient: %v", err)
	}
}

func TestT6ClassesAreInterpreted(t *testing.T) {
	t.Parallel()

	listing := `0 CLASS
1 ACDBDICTIONARYWDFLT
2 AcDbDictionaryWithDefault
3 ObjectDBX Classes
90 0
91 0
280 0
281 0`
	doc := decodeT6(t, t6File(VersionR2013, "CLASSES", listing))
	if len(doc.Diagnostics) != 0 {
		t.Errorf("diagnostics = %v", doc.Diagnostics)
	}
	if len(doc.UnknownSections) != 0 || len(doc.Classes) != 1 ||
		!reflect.DeepEqual(doc.Classes[0].Raw, tagList(t, listing)[1:]) ||
		doc.Classes[0].DXFName != "ACDBDICTIONARYWDFLT" || doc.Classes[0].CppName != "AcDbDictionaryWithDefault" ||
		doc.Classes[0].AppName != "ObjectDBX Classes" {
		t.Errorf("CLASSES = %+v, unknown %+v", doc.Classes, doc.UnknownSections)
	}
}

func TestT6BlockWithSubclasses(t *testing.T) {
	t.Parallel()

	doc := decodeT6(t, t6File(VersionR2013, "BLOCKS", `0 BLOCK
5 19
330 17
100 AcDbEntity
67 1
8 LAY
100 AcDbBlockBegin
2 B1
70 2
10 1.0
20 2.0
30 3.0
3 B1
1 xref.dxf
0 LINE
5 1A
330 17
100 AcDbEntity
8 0
100 AcDbLine
10 0.0
20 0.0
11 1.0
21 1.0
0 ENDBLK
5 1B
330 17
100 AcDbEntity
8 LAY
100 AcDbBlockEnd`))
	noWarnings(t, doc)
	if len(doc.Blocks) != 1 {
		t.Fatalf("blocks = %d", len(doc.Blocks))
	}
	b := doc.Blocks[0]
	if b.Name != "B1" || b.LayerName != "LAY" || b.Flags != 2 || b.BasePoint != (Point3D{X: 1, Y: 2, Z: 3}) {
		t.Errorf("block = %+v", b)
	}
	if b.Owner != 0x17 || b.EndHandle != "1B" || !reflect.DeepEqual(b.Subclasses, []string{"AcDbEntity", "AcDbBlockBegin"}) {
		t.Errorf("block owner/subclasses = %v %q %v", b.Owner, b.EndHandle, b.Subclasses)
	}
	if want := tagList(t, "5 19\n67 1\n1 xref.dxf"); !reflect.DeepEqual(b.Raw, want) || !reflect.DeepEqual(b.RawSubclass, []int{0, 1, 2}) {
		t.Errorf("block raw = %v at %v", b.Raw, b.RawSubclass)
	}
	if want := tagList(t, "330 17\n100 AcDbEntity\n8 LAY\n100 AcDbBlockEnd"); !reflect.DeepEqual(b.EndGroups, want) {
		t.Errorf("EndGroups = %v", b.EndGroups)
	}
	if len(b.Entities) != 1 {
		t.Errorf("block entities = %d", len(b.Entities))
	}
}

func TestT6BaselineFixture(t *testing.T) {
	t.Parallel()

	f, err := os.Open("testdata/t1/v00-baseline.dxf")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := Decode(f)
	if err != nil {
		t.Fatalf("Decode() = %v", err)
	}
	noWarnings(t, doc)
	if doc.Version != VersionR2013 {
		t.Errorf("Version = %q", doc.Version)
	}
	if len(doc.Entities) != 3 {
		t.Fatalf("entities = %d, want 3", len(doc.Entities))
	}
	text, ok := doc.Entities[2].(*Text)
	if !ok || text.Value != "Prova àèìòù ±°" || text.Owner != 0x17 || text.Handle != "21" {
		t.Errorf("TEXT = %+v", doc.Entities[2])
	}
	if len(doc.Objects) != 7 {
		t.Errorf("objects = %d, want 7", len(doc.Objects))
	}
	if len(doc.Blocks) != 2 {
		t.Errorf("blocks = %d, want 2", len(doc.Blocks))
	}
	if len(doc.UnknownSections) != 0 || len(doc.Classes) != 3 {
		t.Errorf("unknown sections = %+v, classes = %d", doc.UnknownSections, len(doc.Classes))
	}
	// Every handle of every object is found.
	for _, h := range []Handle{1, 2, 3, 4, 5, 0x1D, 0x1E, 0x1F, 0x20, 0x21, 0x17, 0x18, 0x19} {
		if _, ok := doc.Lookup(h); !ok {
			t.Errorf("Lookup(%s) failed", h)
		}
	}
	// In strict mode a duplicate handle is an error, here there is none.
	if errs := doc.ValidateHandles(); len(errs) != 0 {
		t.Errorf("ValidateHandles() = %v", errs)
	}
}

func TestD8BinaryDXF(t *testing.T) {
	t.Parallel()

	inputs := map[string]string{
		"signature":      "AutoCAD Binary DXF\r\n\x1a\x00\x00SECTION\x00\x02HEADER\x00",
		"text only":      "AutoCAD Binary DXF",
		"after nothing":  "AutoCAD Binary DXF\r\n",
		"long and noisy": "AutoCAD Binary DXF\r\n\x1a\x00" + strings.Repeat("\xff\x00\x01", 5000),
	}
	for name, input := range inputs {
		for _, lenient := range []bool{false, true} {
			var opts []DecodeOption
			if lenient {
				opts = append(opts, WithLenient())
			}
			_, err := Decode(strings.NewReader(input), opts...)
			if !errors.Is(err, ErrNotDXF) {
				t.Errorf("%s (lenient %v): Decode() = %v, want ErrNotDXF", name, lenient, err)
				continue
			}
			if msg := err.Error(); !strings.Contains(msg, "binary DXF") || !strings.Contains(msg, "not supported") {
				t.Errorf("%s: message %q does not name the binary DXF", name, msg)
			}
		}
	}
	// Text that merely looks similar is not a binary DXF, and an ordinary
	// non-DXF text keeps the plain error.
	_, err := Decode(strings.NewReader("AutoCAD Text DXF\n"))
	if !errors.Is(err, ErrNotDXF) || strings.Contains(err.Error(), "binary") {
		t.Errorf("Decode(similar text) = %v", err)
	}
	// The detection must not consume anything of a normal file.
	doc := decodeT6(t, minimalDXF)
	if len(doc.Entities) != 1 {
		t.Errorf("entities = %d", len(doc.Entities))
	}
}

func TestT6NonASCIIWarningOnlyBeforeR2007(t *testing.T) {
	t.Parallel()

	body := "0 TEXT\n8 0\n10 0.0\n20 0.0\n40 1.0\n1 àè"
	for version, wantWarning := range map[string]bool{
		VersionR12: true, VersionR2004: true, VersionR2007: false, VersionR2013: false, VersionR2018: false,
	} {
		doc := decodeT6(t, t6File(version, "ENTITIES", body))
		got := false
		for _, d := range doc.Diagnostics {
			if d.Severity == SeverityWarning {
				got = true
			}
		}
		if got != wantWarning {
			t.Errorf("%s: warning = %v, want %v (%v)", version, got, wantWarning, doc.Diagnostics)
		}
	}
}

func TestT6R2018Document(t *testing.T) {
	t.Parallel()

	doc := decodeT6(t, t6File(VersionR2018,
		"TABLES", `0 TABLE
2 LAYER
5 2
330 0
100 AcDbSymbolTable
70 1
0 LAYER
5 10
330 2
100 AcDbSymbolTableRecord
100 AcDbLayerTableRecord
2 0
70 0
62 7
6 Continuous
0 ENDTAB`,
		"ENTITIES", `0 CIRCLE
5 20
330 1F
100 AcDbEntity
8 0
440 -1
100 AcDbCircle
10 0.0
20 0.0
30 0.0
40 3.0`,
		"OBJECTS", `0 DICTIONARY
5 C
330 0
100 AcDbDictionary
281 1`))
	noWarnings(t, doc)
	circle := doc.Entities[0].(*Circle)
	if circle.Radius != 3 || circle.Owner != 0x1F {
		t.Errorf("CIRCLE = %+v", circle)
	}
	// 440 (transparency) is not decoded: it stays where it was, in AcDbEntity.
	if want := []Group{{Code: 440, Value: "-1"}}; !reflect.DeepEqual(circle.Raw, want) || !reflect.DeepEqual(circle.RawSubclass, []int{1}) {
		t.Errorf("CIRCLE raw = %v at %v", circle.Raw, circle.RawSubclass)
	}
	if names := doc.Tables.LayerNames(); len(names) != 1 || names[0] != "0" {
		t.Errorf("layers = %v", names)
	}
	if len(doc.Objects) != 1 || doc.Objects[0].Type() != "DICTIONARY" {
		t.Errorf("objects = %v", doc.Objects)
	}
}

func TestT6LimitOnObjects(t *testing.T) {
	t.Parallel()

	input := t6File(VersionR2013, "OBJECTS", "0 XRECORD\n5 1\n0 XRECORD\n5 2\n0 XRECORD\n5 3")
	limits := Limits{MaxEntities: 2}
	if _, err := Decode(strings.NewReader(input), WithLimits(limits)); !errors.Is(err, ErrLimit) {
		t.Fatalf("Decode() = %v, want ErrLimit", err)
	}
}

func TestT6MalformedObjects(t *testing.T) {
	t.Parallel()

	// A section that ends without ENDSEC, or a group before the first type
	// record, is a syntax error, not a panic.
	cases := []string{
		tagText("0 SECTION\n2 HEADER\n9 $ACADVER\n1 AC1027\n0 ENDSEC\n0 SECTION\n2 OBJECTS\n0 XRECORD\n5 1"),
		tagText("0 SECTION\n2 HEADER\n9 $ACADVER\n1 AC1027\n0 ENDSEC\n0 SECTION\n2 OBJECTS\n5 1\n0 ENDSEC\n0 EOF"),
	}
	for i, input := range cases {
		if _, err := Decode(strings.NewReader(input)); !errors.Is(err, ErrSyntax) {
			t.Errorf("case %d: Decode() = %v, want ErrSyntax", i, err)
		}
	}
}
