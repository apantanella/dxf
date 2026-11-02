package dxf

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestHandleStringAndParse(t *testing.T) {
	t.Parallel()

	strs := map[Handle]string{0: "0", 1: "1", 0xAB: "AB", 0x1F00: "1F00", 1<<64 - 1: "FFFFFFFFFFFFFFFF"}
	for h, want := range strs {
		if got := h.String(); got != want {
			t.Errorf("Handle(%d).String() = %q, want %q", uint64(h), got, want)
		}
	}
	if !Handle(0).IsZero() || Handle(1).IsZero() {
		t.Error("IsZero is wrong")
	}

	valid := map[string]Handle{"0": 0, "00": 0, "1a": 0x1A, "  1A\t": 0x1A, "0001": 1, "FFFFFFFFFFFFFFFF": 1<<64 - 1}
	for in, want := range valid {
		got, err := ParseHandle(in)
		if err != nil || got != want {
			t.Errorf("ParseHandle(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "  ", "xyz", "0x1A", "-1", "+1", "1 2", "10000000000000000"} {
		_, err := ParseHandle(in)
		var se *SyntaxError
		if !errors.As(err, &se) || !errors.Is(err, ErrSyntax) {
			t.Errorf("ParseHandle(%q) error = %v, want *SyntaxError", in, err)
		}
	}

	var c EntityCommon
	c.SetHandle(0x2F)
	if c.Handle != "2F" || c.HandleValue() != 0x2F {
		t.Errorf("SetHandle/HandleValue: %q %v", c.Handle, c.HandleValue())
	}
	c.SetHandle(0)
	if c.Handle != "" || c.HandleValue() != 0 {
		t.Error("SetHandle(0) must clear the handle")
	}
	c.Handle = "zz"
	if c.HandleValue() != 0 {
		t.Error("an invalid handle has value 0")
	}
}

func handleTestDoc() (*Document, *Line, *Block, *Line) {
	doc := NewDocumentVersion(VersionR12)
	line := &Line{EntityCommon: EntityCommon{Handle: "2A"}, End: Point3D{X: 1}}
	inBlock := &Line{EntityCommon: EntityCommon{Handle: "3B"}, End: Point3D{X: 2}}
	block := &Block{Name: "B", Raw: []Group{{Code: 5, Value: "40"}}, Entities: []Entity{inBlock}}
	doc.Blocks = append(doc.Blocks, block)
	doc.Entities = append(doc.Entities, line,
		&Polyline{
			SeqendHandle: "51",
			Vertices:     []Vertex{{Handle: "50"}, {Position: Point3D{X: 1}}},
		})
	layer := doc.Tables.Find("LAYER")
	layer.Raw = append(layer.Raw, Group{Code: 5, Value: "60"})
	layer.Records[0].Groups = append(layer.Records[0].Groups, Group{Code: 5, Value: "61"})
	return doc, line, block, inBlock
}

func TestLookup(t *testing.T) {
	t.Parallel()

	doc, line, block, inBlock := handleTestDoc()

	if got, ok := doc.Lookup(0x2A); !ok || got != Entity(line) {
		t.Errorf("Lookup(2A) = %v, %v", got, ok)
	}
	if got, ok := doc.Lookup(0x3B); !ok || got != Entity(inBlock) {
		t.Errorf("Lookup(3B) = %v, %v", got, ok)
	}
	if got, ok := doc.Lookup(0x40); !ok || got != any(block) {
		t.Errorf("Lookup(40) = %v, %v", got, ok)
	}
	if got, ok := doc.Lookup(0x50); !ok {
		t.Error("vertex not found")
	} else if v, isVertex := got.(*Vertex); !isVertex || v != &doc.Entities[1].(*Polyline).Vertices[0] {
		t.Errorf("Lookup(50) = %T", got)
	}
	if got, ok := doc.Lookup(0x60); !ok || got != any(doc.Tables.Find("LAYER")) {
		t.Errorf("Lookup(60) = %v, %v", got, ok)
	}
	if got, ok := doc.Lookup(0x61); !ok || got != any(&doc.Tables.Find("LAYER").Records[0]) {
		t.Errorf("Lookup(61) = %v, %v", got, ok)
	}
	// SEQEND carries no data, an unknown handle and zero are not found.
	for _, h := range []Handle{0x51, 0x99, 0} {
		if got, ok := doc.Lookup(h); ok || got != nil {
			t.Errorf("Lookup(%v) = %v, %v; want not found", h, got, ok)
		}
	}
	// Lookup never goes stale: it sees changes made after a previous call.
	extra := &Line{EntityCommon: EntityCommon{Handle: "77"}}
	doc.Entities = append(doc.Entities, extra)
	if got, ok := doc.Lookup(0x77); !ok || got != Entity(extra) {
		t.Error("Lookup must see an entity added after a previous lookup")
	}
	doc.Entities = doc.Entities[:len(doc.Entities)-1]
	if _, ok := doc.Lookup(0x77); ok {
		t.Error("Lookup must not find a removed entity")
	}

	index := doc.HandleIndex()
	if len(index) != 6 || index[0x2A] != Entity(line) {
		t.Errorf("HandleIndex has %d entries", len(index))
	}
	var nilDoc *Document
	if _, ok := nilDoc.Lookup(1); ok {
		t.Error("nil document")
	}
}

const handleDupSource = "0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1009\n0\nENDSEC\n" +
	"0\nSECTION\n2\nENTITIES\n" +
	"0\nLINE\n5\n1A\n8\n0\n10\n0\n20\n0\n11\n1\n21\n0\n" +
	"0\nLINE\n5\n1a\n8\n0\n10\n0\n20\n0\n11\n2\n21\n0\n" +
	"0\nLINE\n5\nZZ\n8\n0\n10\n0\n20\n0\n11\n3\n21\n0\n" +
	"0\nENDSEC\n0\nEOF\n"

func TestDecodeDuplicateHandles(t *testing.T) {
	t.Parallel()

	doc, err := Decode(strings.NewReader(handleDupSource), WithLenient())
	if err != nil {
		t.Fatalf("lenient Decode() = %v", err)
	}
	var dup, invalid int
	for _, d := range doc.Diagnostics {
		switch {
		case d.Severity == SeverityError && strings.Contains(d.Message, "duplicate handle 1A"):
			dup++
		case d.Severity == SeverityWarning && strings.Contains(d.Message, `invalid handle "ZZ"`):
			invalid++
		}
	}
	if dup != 1 || invalid != 1 {
		t.Errorf("diagnostics: dup=%d invalid=%d: %v", dup, invalid, doc.Diagnostics)
	}
	// The handles are never renumbered.
	if h := doc.Entities[1].Common().Handle; h != "1a" {
		t.Errorf("handle was altered: %q", h)
	}

	_, err = Decode(strings.NewReader(handleDupSource))
	var ve *ValidationError
	if !errors.Is(err, ErrValidation) || !errors.As(err, &ve) {
		t.Fatalf("strict Decode() = %v, want ErrValidation", err)
	}
	if ve.Entity != "LINE" || !strings.Contains(ve.Reason, "1A") {
		t.Errorf("ValidationError = %+v", ve)
	}

	// An invalid handle alone is only a warning, also in strict mode.
	noDup := strings.Replace(handleDupSource, "5\n1a\n", "5\n1B\n", 1)
	doc, err = Decode(strings.NewReader(noDup))
	if err != nil {
		t.Fatalf("Decode() = %v", err)
	}
	if len(doc.Diagnostics) != 1 || doc.Diagnostics[0].Severity != SeverityWarning {
		t.Errorf("diagnostics = %v", doc.Diagnostics)
	}
}

func TestValidateHandlesZeroAndClean(t *testing.T) {
	t.Parallel()

	doc, _, _, _ := handleTestDoc()
	if d := doc.ValidateHandles(); len(d) != 0 {
		t.Errorf("clean document: %v", d)
	}
	doc.Entities = append(doc.Entities, &Line{EntityCommon: EntityCommon{Handle: "0"}}, &Line{EntityCommon: EntityCommon{Handle: "00"}})
	d := doc.ValidateHandles()
	if len(d) != 2 || d[0].Severity != SeverityWarning {
		t.Errorf("zero handles: %v", d)
	}
}

func TestEncodeRejectsDuplicateHandles(t *testing.T) {
	t.Parallel()

	doc := NewDocumentVersion(VersionR12)
	doc.Entities = []Entity{
		&Line{EntityCommon: EntityCommon{Handle: "1A"}},
		&Line{EntityCommon: EntityCommon{Handle: "1a"}},
	}
	var buf bytes.Buffer
	err := Encode(&buf, doc)
	var ve *ValidationError
	if !errors.Is(err, ErrValidation) || !errors.As(err, &ve) || !strings.Contains(ve.Reason, "1A") {
		t.Fatalf("Encode() = %v, want ErrValidation naming the handle", err)
	}
	if buf.Len() != 0 {
		t.Error("nothing must be written for an invalid document")
	}

	// A duplicate between a block record (Raw) and an entity is also caught.
	doc = NewDocumentVersion(VersionR12)
	doc.Blocks = []*Block{{Name: "B", Raw: []Group{{Code: 5, Value: "7"}}}}
	doc.Entities = []Entity{&Line{EntityCommon: EntityCommon{Handle: "7"}}}
	if err := Encode(&buf, doc); !errors.Is(err, ErrValidation) {
		t.Errorf("block/entity duplicate: %v", err)
	}
}

// parseAllHandles returns every handle written in the entities/blocks/tables
// part of an encoded document, in order (group code 5 outside the header).
func parseAllHandles(t *testing.T, out string) []Handle {
	t.Helper()
	var hs []Handle
	inHeader := false
	for _, g := range groupsOf(t, out) {
		if g.Code == 2 {
			inHeader = g.Value == "HEADER"
		}
		if g.Code == 5 && !inHeader && !(strings.HasPrefix(g.Value, "$")) {
			h, err := ParseHandle(g.Value)
			if err != nil {
				t.Fatalf("bad handle %q", g.Value)
			}
			hs = append(hs, h)
		}
	}
	return hs
}

func TestEncodeAssignsHandlesWithoutCollisions(t *testing.T) {
	t.Parallel()

	doc := NewDocumentVersion(VersionR12)
	doc.Header.Set("$HANDLING", 70, "1")
	// Sparse handles, one hidden in preserved Raw groups of an entity, a
	// vertex, and an unknown entity, and one in a table record.
	doc.Entities = []Entity{
		&Line{EntityCommon: EntityCommon{Handle: "5"}},
		&Line{},
		&Circle{EntityCommon: EntityCommon{Raw: []Group{{Code: 5, Value: "100"}}}, Radius: 1},
		&Polyline{Vertices: []Vertex{{Raw: []Group{{Code: 5, Value: "200"}}}, {Position: Point3D{X: 1}}}},
		&RawEntity{TypeName: "WIPEOUT", EntityCommon: EntityCommon{Raw: []Group{{Code: 5, Value: "300"}}}},
		&Line{EntityCommon: EntityCommon{Handle: "10"}},
	}
	doc.Tables.Find("LAYER").Records[0].Groups = append(doc.Tables.Find("LAYER").Records[0].Groups, Group{Code: 5, Value: "A00"})
	doc.UnknownSections = []RawSection{{Name: "CLASSES", Groups: []Group{{Code: 5, Value: "B00"}}}}

	out := encodeString(t, doc)
	seen := map[Handle]bool{}
	var maxH Handle
	for _, h := range parseAllHandles(t, out) {
		if h == 0 {
			t.Error("handle 0 written")
		}
		if h > maxH {
			maxH = h
		}
		if seen[h] && h != 0x100 && h != 0x200 && h != 0x300 {
			t.Errorf("handle %v written twice", h)
		}
		seen[h] = true
	}
	// Assigned handles all start above the highest existing one (B00).
	if got := doc.Entities[1].Common().Handle; got != "" {
		t.Errorf("Encode modified the document: %q", got)
	}
	back, err := Decode(strings.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if got := back.Entities[1].Common().Handle; got != "B01" {
		t.Errorf("first assigned handle = %q, want B01", got)
	}

	// $HANDSEED is greater than every handle used.
	seed := headerSeed(t, out)
	if seed <= maxH {
		t.Errorf("$HANDSEED %v is not greater than the highest handle %v", seed, maxH)
	}
}

func headerSeed(t *testing.T, out string) Handle {
	t.Helper()
	all := groupsOf(t, out)
	for i, g := range all {
		if g.Code == 9 && g.Value == "$HANDSEED" {
			h, err := ParseHandle(all[i+1].Value)
			if err != nil {
				t.Fatal(err)
			}
			return h
		}
	}
	t.Fatal("$HANDSEED missing")
	return 0
}

func TestEncodeNoHandlesIsUnchanged(t *testing.T) {
	t.Parallel()

	doc := NewDocumentVersion(VersionR12)
	doc.Entities = []Entity{&Line{End: Point3D{X: 1}}, &Polyline{Vertices: []Vertex{{}, {Position: Point3D{X: 1}}}}}
	out := encodeString(t, doc)
	if strings.Contains(out, "$HANDLING") || strings.Contains(out, "$HANDSEED") || len(parseAllHandles(t, out)) != 0 {
		t.Errorf("a document without handles must not gain any:\n%s", out)
	}
}

func TestEncodeIdempotentWithMixedHandles(t *testing.T) {
	t.Parallel()

	insert := NewInsert("B", Point3D{})
	insert.Attributes = []Attribute{{Value: "x", Tag: "T", Height: 1, Handle: "9"}, {Value: "y", Tag: "U", Height: 1}}
	doc := NewDocumentVersion(VersionR12)
	doc.Blocks = []*Block{{Name: "B", Entities: []Entity{&Line{End: Point3D{X: 1}}}}}
	doc.Entities = []Entity{
		&Line{EntityCommon: EntityCommon{Handle: "20"}},
		&Line{},
		&Polyline{Vertices: []Vertex{{Handle: "30"}, {Position: Point3D{X: 1}}, {Position: Point3D{X: 2}}}, Flags: 0},
		insert,
		&Circle{Radius: 2},
	}
	first := encodeString(t, doc)
	for i := 0; i < 3; i++ {
		decoded, err := Decode(strings.NewReader(first))
		if err != nil {
			t.Fatalf("Decode() = %v", err)
		}
		if d := decoded.ValidateHandles(); len(d) != 0 {
			t.Fatalf("encoded handles are not valid: %v", d)
		}
		second := encodeString(t, decoded)
		if first != second {
			t.Fatalf("round %d: not byte-identical\n%s", i, diffStrings(first, second))
		}
	}
	decoded, _ := Decode(strings.NewReader(first))
	// Every written handle (SEQEND included) is unique and below $HANDSEED.
	seen := map[Handle]bool{}
	for _, h := range parseAllHandles(t, first) {
		if seen[h] {
			t.Errorf("duplicate handle %v in output", h)
		}
		seen[h] = true
		if h >= headerSeed(t, first) {
			t.Errorf("handle %v >= $HANDSEED", h)
		}
	}
	if decoded.Entities[2].(*Polyline).SeqendHandle == "" {
		t.Error("SEQEND handle was not read back")
	}
}

func TestDecodeBlockEndHandle(t *testing.T) {
	t.Parallel()

	src := "0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1009\n9\n$HANDLING\n70\n1\n0\nENDSEC\n" +
		"0\nSECTION\n2\nBLOCKS\n0\nBLOCK\n5\n40\n8\n0\n2\nB\n70\n0\n10\n0\n20\n0\n30\n0\n3\nB\n0\nENDBLK\n5\n41\n0\nENDSEC\n" +
		"0\nSECTION\n2\nENTITIES\n0\nENDSEC\n0\nEOF\n"
	doc, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Decode() = %v", err)
	}
	if doc.Blocks[0].EndHandle != "41" {
		t.Errorf("EndHandle = %q", doc.Blocks[0].EndHandle)
	}
	if got, ok := doc.Lookup(0x40); !ok || got != any(doc.Blocks[0]) {
		t.Error("block not found by handle")
	}
	out := encodeString(t, doc)
	if !strings.Contains(out, "0\nENDBLK\n5\n41\n") {
		t.Errorf("ENDBLK handle lost:\n%s", out)
	}
	bad := strings.Replace(src, "ENDBLK\n5\n41\n", "ENDBLK\n8\n0\n", 1)
	if _, err := Decode(strings.NewReader(bad)); !errors.Is(err, ErrSyntax) {
		t.Errorf("other groups after ENDBLK are still a syntax error, got %v", err)
	}
}

func TestOwnerNotWrittenInR12(t *testing.T) {
	t.Parallel()

	doc := NewDocumentVersion(VersionR12)
	doc.Entities = []Entity{&Line{EntityCommon: EntityCommon{Owner: 0x1F}}}
	if out := encodeString(t, doc); strings.Contains(out, "\n330\n") {
		t.Errorf("R12 writer must not emit group 330:\n%s", out)
	}
	src := "0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1009\n0\nENDSEC\n0\nSECTION\n2\nENTITIES\n0\nLINE\n8\n0\n330\n1F\n10\n0\n20\n0\n11\n1\n21\n0\n0\nENDSEC\n0\nEOF\n"
	decoded, err := Decode(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.Entities[0].Common().Owner.IsZero() {
		t.Error("the R12 reader leaves Owner unset")
	}
	if out := encodeString(t, decoded); !strings.Contains(out, "330\n1F\n") {
		t.Errorf("a 330 group read from a file must be preserved:\n%s", out)
	}
}
