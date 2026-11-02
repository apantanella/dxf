package dxf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// T0 (F2) — debts and decisions of F1 closed before the new entities.
// See docs/f2-guida-implementazione.md, "Debiti di F1 da chiudere prima".

// TestF2NewDocumentIsR2013: since F2 the default document is R2013 (ADR 0002),
// because the entities F2 adds do not exist in R12. NewDocumentVersion(VersionR12)
// still gives an R12 document with the default tables.
func TestF2NewDocumentIsR2013(t *testing.T) {
	t.Parallel()

	doc := NewDocument()
	if doc.Version != VersionR2013 {
		t.Fatalf("NewDocument().Version = %q, want %q", doc.Version, VersionR2013)
	}
	if v, ok := doc.Header.Get("$ACADVER"); !ok || v != VersionR2013 {
		t.Errorf("NewDocument $ACADVER = %q, want %q", v, VersionR2013)
	}
	if out := string(t9Encode(t, doc)); !strings.Contains(out, VersionR2013) {
		t.Errorf("Encode(NewDocument()) does not carry %s:\n%s", VersionR2013, out)
	}

	r12 := NewDocumentVersion(VersionR12)
	if r12.Version != VersionR12 {
		t.Errorf("NewDocumentVersion(R12).Version = %q", r12.Version)
	}
	for _, name := range []string{"LAYER", "LTYPE", "STYLE"} {
		if r12.Tables.Find(name) == nil {
			t.Errorf("NewDocumentVersion(R12) is missing the %s table", name)
		}
	}
}

// TestF2ATTRIBAlignmentSubclass: the horizontal alignment (72) belongs to
// AcDbText. R2018 files also write a 72 inside AcDbAttribute, after the 71: it
// must not be read as the alignment and must stay in Raw, so the writer puts it
// back where it was (the old writer needed a workaround for exactly this).
func TestF2ATTRIBAlignmentSubclass(t *testing.T) {
	t.Parallel()

	// The AcDbAttribute 72 is not the alignment.
	groups := []Group{
		{Code: 100, Value: "AcDbEntity"},
		{Code: 8, Value: "0"},
		{Code: 100, Value: "AcDbText"},
		{Code: 10, Value: "1.0"}, {Code: 20, Value: "2.0"}, {Code: 30, Value: "0.0"},
		{Code: 40, Value: "2.5"},
		{Code: 1, Value: "v"},
		{Code: 100, Value: "AcDbAttribute"},
		{Code: 2, Value: "TAG"},
		{Code: 70, Value: "0"},
		{Code: 71, Value: "1"},
		{Code: 72, Value: "0"},
	}
	a, err := decodeAttribute(groups, true)
	if err != nil {
		t.Fatal(err)
	}
	if a.HAlign != 0 {
		t.Errorf("HAlign = %d, want 0: the AcDbAttribute 72 is not the alignment", a.HAlign)
	}
	if countCode(a.Raw, 71) != 1 || countCode(a.Raw, 72) != 1 {
		t.Errorf("the AcDbAttribute 71/72 must stay in Raw: %v", a.Raw)
	}

	// A 72 inside AcDbText is still the alignment.
	withAlign := []Group{
		{Code: 100, Value: "AcDbEntity"},
		{Code: 8, Value: "0"},
		{Code: 100, Value: "AcDbText"},
		{Code: 10, Value: "1.0"}, {Code: 20, Value: "2.0"}, {Code: 30, Value: "0.0"},
		{Code: 40, Value: "2.5"},
		{Code: 1, Value: "v"},
		{Code: 72, Value: "1"},
		{Code: 100, Value: "AcDbAttribute"},
		{Code: 2, Value: "TAG"},
		{Code: 70, Value: "0"},
	}
	a2, err := decodeAttribute(withAlign, true)
	if err != nil {
		t.Fatal(err)
	}
	if a2.HAlign != 1 {
		t.Errorf("HAlign = %d, want 1 (72 in AcDbText)", a2.HAlign)
	}
	if countCode(a2.Raw, 72) != 0 {
		t.Errorf("the AcDbText 72 must be consumed: %v", a2.Raw)
	}
}

// TestF2ATTRIBFixtureRoundTrip: the R2018 fixture has an ATTRIB whose
// AcDbAttribute carries 71 and 72. Decode and Encode again must keep the same
// tags, with no writer workaround.
func TestF2ATTRIBFixtureRoundTrip(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("testdata", "oracle", "r2018-entities.dxf"))
	if err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	doc := t9Decode(t, data)
	var attrib *Attribute
	for _, e := range doc.Entities {
		if in, ok := e.(*Insert); ok && len(in.Attributes) > 0 {
			attrib = &in.Attributes[0]
			break
		}
	}
	if attrib == nil {
		t.Fatal("the fixture has no INSERT with attributes")
	}
	if attrib.HAlign != 0 {
		t.Errorf("HAlign = %d, want 0", attrib.HAlign)
	}
	if countCode(attrib.Raw, 71) != 1 || countCode(attrib.Raw, 72) != 1 {
		t.Errorf("ATTRIB Raw = %v, want the 71 and 72 kept", attrib.Raw)
	}
	// The whole file must round-trip tag for tag (TestT9FixtureRoundTrip covers
	// every fixture; this pins the ATTRIB case).
	want := t9Normalize(t9Tags(t, data))
	got := t9Normalize(t9Tags(t, t9Encode(t, doc)))
	if d := t9DiffTags(want, got); d != "" {
		t.Errorf("ATTRIB round trip differs: %s", d)
	}
}

// TestF2XDataAppIDAutoCreated: the R2013 writer adds an APPID record for every
// application named by extended data (1001). Dimensions and MLEADERs refer to
// ACAD and ACAD_DSTYLE_* there; a file that uses an undefined application is
// rejected by some CAD applications.
func TestF2XDataAppIDAutoCreated(t *testing.T) {
	t.Parallel()

	doc := NewDocument()
	doc.Entities = []Entity{&Line{
		EntityCommon: EntityCommon{
			LayerName: "0",
			Raw:       []Group{{Code: 1001, Value: "ACME"}, {Code: 1000, Value: "x"}},
		},
		End: Point3D{X: 1},
	}}
	// A DIMSTYLE record refers to ACAD_DSTYLE_DIMJAG, as a real file does.
	doc.Tables.EnsureLayer("0", 7, "Continuous")
	dim := doc.Tables.Ensure("DIMSTYLE")
	dim.Records = append(dim.Records, Record{TypeName: "DIMSTYLE", Groups: []Group{
		{Code: 2, Value: "Standard"},
		{Code: 70, Value: "0"},
		{Code: 1001, Value: "ACAD_DSTYLE_DIMJAG"},
		{Code: 1070, Value: "388"},
	}})

	out := t9Encode(t, doc)
	got := t9Decode(t, out)
	table := got.Tables.Find("APPID")
	if table == nil {
		t.Fatal("no APPID table in the output")
	}
	names := map[string]bool{}
	for _, rec := range table.Records {
		if name, ok := recordValue(rec, 2); ok {
			names[name] = true
		}
	}
	for _, want := range []string{"ACAD", "ACME", "ACAD_DSTYLE_DIMJAG"} {
		if !names[want] {
			t.Errorf("APPID %q missing: %v", want, names)
		}
	}
	// The extended data itself survives the round trip.
	line := got.Entities[0].Common().Raw
	if countCode(line, 1001) != 1 {
		t.Errorf("extended data lost: %v", line)
	}
}
