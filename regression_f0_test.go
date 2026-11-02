package dxf

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
)

// groupsOf splits an encoded document into its (code, value) pairs.
func groupsOf(t *testing.T, text string) []Group {
	t.Helper()
	gr := newGroupReader(strings.NewReader(text), DefaultLimits())
	var out []Group
	for {
		g, err := gr.next()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatalf("groupsOf: %v", err)
		}
		out = append(out, g)
	}
}

// entityGroups returns the groups of the first record of the given type in the
// ENTITIES section, up to the next code 0 record.
func entityGroups(t *testing.T, text, typeName string) []Group {
	t.Helper()
	all := groupsOf(t, text)
	inEntities := false
	for i, g := range all {
		if g.Code == 2 && g.Value == "ENTITIES" {
			inEntities = true
		}
		if inEntities && g.Code == 0 && g.Value == typeName {
			var out []Group
			for _, n := range all[i+1:] {
				if n.Code == 0 {
					break
				}
				out = append(out, n)
			}
			return out
		}
	}
	t.Fatalf("entity %s not found", typeName)
	return nil
}

func countCode(groups []Group, code int) int {
	n := 0
	for _, g := range groups {
		if g.Code == code {
			n++
		}
	}
	return n
}

func blockDoc() *Document {
	doc := NewDocumentVersion(VersionR12)
	doc.Blocks = append(doc.Blocks, &Block{Name: "B"})
	return doc
}

// Bug 1: POLYLINE and INSERT must write the common groups (handle, layer) once.
func TestF0CommonGroupsWrittenOnce(t *testing.T) {
	doc := blockDoc()
	doc.Entities = append(doc.Entities,
		&Polyline{
			EntityCommon: EntityCommon{LayerName: "L1"},
			Flags:        1,
			Vertices:     []Vertex{{}, {Position: Point3D{X: 1}}, {Position: Point3D{X: 1, Y: 1}}},
		},
		&Insert{EntityCommon: EntityCommon{LayerName: "L2"}, BlockName: "B", XScale: 1, YScale: 1, ZScale: 1},
	)
	for _, withHandles := range []bool{false, true} {
		if withHandles {
			doc.Header.Set("$HANDLING", 70, "1")
		}
		out := encodeString(t, doc)
		for _, typeName := range []string{"POLYLINE", "INSERT"} {
			groups := entityGroups(t, out, typeName)
			if n := countCode(groups, 8); n != 1 {
				t.Errorf("handles=%v %s: layer written %d times, want 1", withHandles, typeName, n)
			}
			wantHandles := 0
			if withHandles {
				wantHandles = 1
			}
			if n := countCode(groups, 5); n != wantHandles {
				t.Errorf("handles=%v %s: handle written %d times, want %d", withHandles, typeName, n, wantHandles)
			}
		}
	}
}

// Bug 2: unhandled groups (XDATA) belong to the entity, not to SEQEND.
func TestF0RawGroupsBeforeVerticesAndSeqend(t *testing.T) {
	xdata := []Group{{Code: 1001, Value: "APP"}, {Code: 1000, Value: "x"}}
	doc := blockDoc()
	doc.Tables.EnsureAppID("APP")
	doc.Entities = append(doc.Entities,
		&Polyline{
			EntityCommon: EntityCommon{Raw: xdata},
			Flags:        1,
			Vertices:     []Vertex{{}, {Position: Point3D{X: 1}}, {Position: Point3D{X: 1, Y: 1}}},
		},
		&Insert{
			EntityCommon: EntityCommon{Raw: xdata},
			BlockName:    "B", XScale: 1, YScale: 1, ZScale: 1,
			Attributes: []Attribute{{Tag: "T", Value: "v", Height: 1, XScale: 1}},
		},
	)
	out := encodeString(t, doc)
	all := groupsOf(t, out)
	for i, g := range all {
		if g.Code == 0 && g.Value == "SEQEND" {
			for _, n := range all[i+1:] {
				if n.Code == 0 {
					break
				}
				if n.Code >= 1000 {
					t.Fatalf("XDATA group %v written after SEQEND", n)
				}
			}
		}
	}
	// The XDATA must still be present and reach the entity on re-reading.
	doc2, err := Decode(strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range doc2.Entities {
		if countCode(e.Common().Raw, 1001) != 1 {
			t.Errorf("%s lost its XDATA after a round trip", e.Type())
		}
	}
}

// Bug 3: Encode must not modify the document it receives.
func TestF0EncodeDoesNotMutateDocument(t *testing.T) {
	doc := NewDocumentVersion(VersionR12)
	doc.Entities = append(doc.Entities, &Line{EntityCommon: EntityCommon{LayerName: "NEW"}, End: Point3D{X: 1}})
	before := len(doc.Tables.Find("LAYER").Records)
	tables := len(doc.Tables.Items)
	encodeString(t, doc)
	if after := len(doc.Tables.Find("LAYER").Records); after != before {
		t.Errorf("Encode changed the LAYER table: %d -> %d records", before, after)
	}
	if len(doc.Tables.Items) != tables {
		t.Errorf("Encode changed the number of tables: %d -> %d", tables, len(doc.Tables.Items))
	}
}

func TestF0EncodeConcurrent(t *testing.T) {
	doc := NewDocumentVersion(VersionR12)
	doc.Entities = append(doc.Entities, &Line{EntityCommon: EntityCommon{LayerName: "NEW"}, End: Point3D{X: 1}})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var b bytes.Buffer
			if err := Encode(&b, doc); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

// Bug 4: ATTRIB uses 73 for the field length and 74 for the vertical alignment.
func TestF0AttributeAlignmentCodes(t *testing.T) {
	doc := blockDoc()
	doc.Entities = append(doc.Entities, &Insert{
		BlockName: "B", XScale: 1, YScale: 1, ZScale: 1,
		Attributes: []Attribute{{Tag: "T", Value: "v", Height: 1, XScale: 1, FieldLength: 5, VAlign: 2, HAlign: 1}},
	})
	out := encodeString(t, doc)
	all := groupsOf(t, out)
	var attrib []Group
	for i, g := range all {
		if g.Code == 0 && g.Value == "ATTRIB" {
			for _, n := range all[i+1:] {
				if n.Code == 0 {
					break
				}
				attrib = append(attrib, n)
			}
			break
		}
	}
	find := func(code int) string {
		for _, g := range attrib {
			if g.Code == code {
				return g.Value
			}
		}
		return ""
	}
	if got := find(73); got != "5" {
		t.Errorf("code 73 (field length) = %q, want 5", got)
	}
	if got := find(74); got != "2" {
		t.Errorf("code 74 (vertical alignment) = %q, want 2", got)
	}
	doc2, err := Decode(strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	a := doc2.Entities[0].(*Insert).Attributes[0]
	if a.FieldLength != 5 || a.VAlign != 2 || a.HAlign != 1 {
		t.Errorf("round trip: field length %d, valign %d, halign %d", a.FieldLength, a.VAlign, a.HAlign)
	}
}

func tableOrder(t *testing.T, text string) []string {
	t.Helper()
	all := groupsOf(t, text)
	var names []string
	for i, g := range all {
		if g.Code == 0 && g.Value == "TABLE" && i+1 < len(all) && all[i+1].Code == 2 {
			names = append(names, all[i+1].Value)
		}
	}
	return names
}

// Bug 5: LTYPE must precede LAYER, which references it.
func TestF0TableOrder(t *testing.T) {
	doc := NewDocumentVersion(VersionR12)
	doc.Tables.EnsureAppID("APP")
	names := tableOrder(t, encodeString(t, doc))
	index := map[string]int{}
	for i, n := range names {
		index[n] = i
	}
	for _, pair := range [][2]string{{"LTYPE", "LAYER"}, {"LAYER", "STYLE"}, {"STYLE", "APPID"}} {
		a, okA := index[pair[0]]
		b, okB := index[pair[1]]
		if !okA || !okB || a > b {
			t.Errorf("table order %v: %s must come before %s", names, pair[0], pair[1])
		}
	}
}

// Bug 6: a referenced linetype is defined or the document is rejected.
func TestF0LinetypeReferences(t *testing.T) {
	doc := NewDocumentVersion(VersionR12)
	doc.Tables.EnsureLayer("AXES", 1, "CENTER")
	doc.Entities = append(doc.Entities, &Line{EntityCommon: EntityCommon{LayerName: "AXES", Linetype: "DASHED"}, End: Point3D{X: 1}})
	out := encodeString(t, doc)
	for _, name := range []string{"CENTER", "DASHED"} {
		if !strings.Contains(out, "0\nLTYPE\n2\n"+name+"\n") {
			t.Errorf("standard linetype %s was not defined", name)
		}
	}
	doc2, err := Decode(strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := doc2.Tables.Find("LTYPE").FindRecord("LTYPE"); !ok {
		t.Fatal("LTYPE table is empty after a round trip")
	}

	bad := NewDocumentVersion(VersionR12)
	bad.Entities = append(bad.Entities, &Line{EntityCommon: EntityCommon{Linetype: "NOT_STANDARD"}, End: Point3D{X: 1}})
	err = Encode(io.Discard, bad)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("undefined linetype: error = %v, want ErrValidation", err)
	}

	byLayer := NewDocumentVersion(VersionR12)
	byLayer.Entities = append(byLayer.Entities, &Line{EntityCommon: EntityCommon{Linetype: "BYLAYER"}, End: Point3D{X: 1}})
	if err := Encode(io.Discard, byLayer); err != nil {
		t.Errorf("BYLAYER must be accepted: %v", err)
	}
}

// Bug 7: duplicate group codes are preserved instead of silently dropped.
func TestF0DuplicateGroupsPreserved(t *testing.T) {
	groups := []Group{
		{Code: 8, Value: "L"},
		{Code: 10, Value: "1"}, {Code: 20, Value: "2"}, {Code: 30, Value: "3"},
		{Code: 11, Value: "4"}, {Code: 21, Value: "5"}, {Code: 31, Value: "6"},
		{Code: 1000, Value: "a"}, {Code: 1000, Value: "b"},
		{Code: 62, Value: "1"}, {Code: 62, Value: "2"},
	}
	ent, err := decodeEntity("LINE", groups)
	if err != nil {
		t.Fatal(err)
	}
	raw := ent.Common().Raw
	if countCode(raw, 1000) != 2 {
		t.Errorf("repeated XDATA groups lost: %v", raw)
	}
	if countCode(raw, 62) != 1 {
		t.Errorf("second color group must be kept as an unhandled group: %v", raw)
	}
	if got := ent.Common().Color; got != 1 {
		t.Errorf("first occurrence must win: color = %d", got)
	}
}

// Bug 8: an oversized line is rejected without being buffered completely.
type countingReader struct {
	remaining int
	read      int
}

func (c *countingReader) Read(p []byte) (int, error) {
	if c.remaining == 0 {
		return 0, io.EOF
	}
	n := len(p)
	if n > c.remaining {
		n = c.remaining
	}
	for i := 0; i < n; i++ {
		p[i] = 'a'
	}
	c.remaining -= n
	c.read += n
	return n, nil
}

func TestF0LongLineLimit(t *testing.T) {
	cr := &countingReader{remaining: 64 << 20}
	src := io.MultiReader(strings.NewReader("0\nSECTION\n2\nHEADER\n9\n$X\n1\n"), cr)
	limits := DefaultLimits()
	limits.MaxValueBytes = 1024
	_, err := Decode(src, WithLimits(limits))
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("error = %v, want ErrLimit", err)
	}
	if cr.read > 1<<20 {
		t.Errorf("read %d bytes before rejecting the line, want a small bounded amount", cr.read)
	}
}

// Bug 9: BLOCK repeats its name with code 3.
func TestF0BlockNameCode3(t *testing.T) {
	doc := blockDoc()
	out := encodeString(t, doc)
	all := groupsOf(t, out)
	found := false
	for i, g := range all {
		if g.Code == 0 && g.Value == "BLOCK" {
			for _, n := range all[i+1:] {
				if n.Code == 0 {
					break
				}
				if n.Code == 3 && n.Value == "B" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("BLOCK without code 3")
	}
	doc2, err := Decode(strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if n := countCode(doc2.Blocks[0].Raw, 3); n != 0 {
		t.Errorf("code 3 must be consumed by the reader, found %d in Raw", n)
	}
	if out2 := encodeString(t, doc2); out2 != out {
		t.Error("BLOCK round trip is not stable")
	}
}

// Bug 10: the header carries the drawing extents.
func TestF0Extents(t *testing.T) {
	doc := NewDocumentVersion(VersionR12)
	doc.Entities = append(doc.Entities,
		&Line{Start: Point3D{X: -1, Y: 2, Z: 0}, End: Point3D{X: 10, Y: 5, Z: 3}},
		&Circle{Center: Point3D{X: 0, Y: 0}, Radius: 20},
	)
	min, max, ok := doc.Extents()
	if !ok || min != (Point3D{X: -20, Y: -20, Z: 0}) || max != (Point3D{X: 20, Y: 20, Z: 3}) {
		t.Fatalf("Extents = %v %v %v", min, max, ok)
	}
	out := encodeString(t, doc)
	doc2, err := Decode(strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	v, ok := doc2.Header.Variables["$EXTMAX"]
	if !ok || len(v.Groups) != 3 || v.Groups[0].Value != "20" {
		t.Errorf("$EXTMAX = %v", v)
	}
	if _, ok := doc2.Header.Variables["$EXTMIN"]; !ok {
		t.Error("$EXTMIN missing")
	}

	empty := encodeString(t, NewDocumentVersion(VersionR12))
	if strings.Contains(empty, "$EXTMIN") {
		t.Error("an empty drawing must not declare extents")
	}

	doc.UpdateExtents()
	doc.Entities = append(doc.Entities, &Line{End: Point3D{X: 100}})
	doc.UpdateExtents()
	if v := doc.Header.Variables["$EXTMAX"]; v.Groups[0].Value != "100" {
		t.Errorf("UpdateExtents: $EXTMAX = %v", v)
	}
}

// Bug 11: handles stored in table records and blocks reserve the seed too.
func TestF0MaxHandleInTablesAndBlocks(t *testing.T) {
	doc := NewDocumentVersion(VersionR12)
	doc.Tables.Find("LAYER").Records[0].Groups = append(doc.Tables.Find("LAYER").Records[0].Groups, Group{Code: 5, Value: "FF"})
	doc.Blocks = append(doc.Blocks, &Block{Name: "B", Raw: []Group{{Code: 5, Value: "1FF"}}})
	if got := maxHandle(doc); got != 0x1FF {
		t.Errorf("maxHandle = %X, want 1FF", got)
	}
}

// Bug 13: non-ASCII bytes produce one documented diagnostic.
func TestF0NonASCIIDiagnostic(t *testing.T) {
	src := "0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1009\n0\nENDSEC\n" +
		"0\nSECTION\n2\nENTITIES\n0\nTEXT\n8\n0\n1\nQuota \xe8 \xd8 20\n10\n0\n20\n0\n40\n1\n0\nENDSEC\n0\nEOF\n"
	doc, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, d := range doc.Diagnostics {
		if strings.Contains(d.Message, "non-ASCII") {
			n++
			if d.Severity != SeverityWarning {
				t.Errorf("severity = %v, want warning", d.Severity)
			}
			if d.Line != 19 {
				t.Errorf("diagnostic line = %d, want the first occurrence (19)", d.Line)
			}
		}
	}
	if n != 1 {
		t.Errorf("got %d non-ASCII diagnostics, want exactly 1: %v", n, doc.Diagnostics)
	}
	text := doc.Entities[0].(*Text)
	if text.Value != "Quota \xe8 \xd8 20" {
		t.Errorf("text bytes were altered: %q", text.Value)
	}
}
