package dxf

import (
	"errors"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/apantanella/dxf/tags"
)

// v00Candidates lists where the T1 baseline file may live.
var v00Candidates = []string{
	"testdata/t1/v00-baseline.dxf",
	"../t1/out/v00-baseline.dxf",
}

// readV00Tables reads the TABLES section of the T1 baseline R2013 file as a
// flat list of groups (0 SECTION and 0 ENDSEC excluded).
func readV00Tables(t *testing.T) []Group {
	t.Helper()
	var f *os.File
	for _, p := range v00Candidates {
		var err error
		if f, err = os.Open(p); err == nil {
			break
		}
	}
	if f == nil {
		t.Skip("T1 baseline v00-baseline.dxf not available")
	}
	defer f.Close()
	r := tags.NewReader(f)
	var out []Group
	in := false
	var prev Group
	for {
		tag, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read v00: %v", err)
		}
		if tag.Code == 2 && prev.Code == 0 && prev.Value == "SECTION" {
			in = tag.Value == "TABLES"
			prev = tag
			continue
		}
		if tag.Code == 0 && tag.Value == "ENDSEC" {
			in = false
		}
		if in {
			out = append(out, tag)
		}
		prev = tag
	}
	if len(out) == 0 {
		t.Fatal("v00 has no TABLES section")
	}
	return out
}

// parseTables turns the flat group list of a TABLES section into Tables the way
// the reader does: Table.Raw holds the header groups after the "2 name" pair.
func parseTables(t *testing.T, groups []Group) *Tables {
	t.Helper()
	tabs := &Tables{}
	var cur *Table
	var rec *Record
	headerDone := false
	for _, gr := range groups {
		switch {
		case gr.Code == 0 && gr.Value == "TABLE":
			cur = &Table{}
			tabs.Items = append(tabs.Items, cur)
			rec, headerDone = nil, false
		case gr.Code == 0 && gr.Value == "ENDTAB":
			cur, rec = nil, nil
		case gr.Code == 0:
			cur.Records = append(cur.Records, Record{TypeName: gr.Value})
			rec = &cur.Records[len(cur.Records)-1]
			headerDone = true
		case rec != nil:
			rec.Groups = append(rec.Groups, gr)
		case cur != nil && !headerDone && gr.Code == 2 && cur.Name == "":
			cur.Name = gr.Value
		case cur != nil:
			cur.Raw = append(cur.Raw, gr)
		}
	}
	return tabs
}

// flattenTables writes tables as the TABLES section groups, in Items order.
func flattenTables(tabs *Tables) []Group {
	var out []Group
	for _, tb := range tabs.Items {
		out = append(out, Group{Code: 0, Value: "TABLE"}, Group{Code: 2, Value: tb.Name})
		out = append(out, tb.Raw...)
		for _, rec := range tb.Records {
			out = append(out, Group{Code: 0, Value: rec.TypeName})
			out = append(out, rec.Groups...)
		}
		out = append(out, Group{Code: 0, Value: "ENDTAB"})
	}
	return out
}

// stripHandles removes the groups that hold handle values.
func stripHandles(groups []Group) []Group {
	var out []Group
	for _, gr := range groups {
		switch gr.Code {
		case 5, 105, 330, 340, 390:
			continue
		}
		out = append(out, gr)
	}
	return out
}

func counter(start Handle) func() Handle {
	n := start
	return func() Handle { n++; return n - 1 }
}

func namesOf(tb *Table) []string {
	var out []string
	for _, rec := range tb.Records {
		out = append(out, recordText(rec, 2))
	}
	return out
}

func TestR7EnsureFunctionsUnchanged(t *testing.T) {
	t.Parallel()
	doc := NewDocument()
	doc.Tables.EnsureLayer("WALLS", 3, "DASHED")
	doc.Tables.EnsureLinetype("DASHED")
	doc.Tables.EnsureAppID("MYAPP")

	want := Record{TypeName: "LAYER", Groups: []Group{gp(2, "WALLS"), gp(70, "0"), gp(62, "3"), gp(6, "DASHED")}}
	tb := doc.Tables.Find("LAYER")
	if got := tb.Records[len(tb.Records)-1]; !reflect.DeepEqual(got, want) {
		t.Errorf("EnsureLayer record = %v, want %v", got, want)
	}
	if got := doc.Tables.Find("APPID").Records[0]; !reflect.DeepEqual(got.Groups, []Group{gp(2, "MYAPP"), gp(70, "0")}) {
		t.Errorf("EnsureAppID record = %v", got)
	}
	l, ok := doc.Tables.Layer("0")
	if !ok {
		t.Fatal("Layer(0) not found")
	}
	if l.Color != 7 || !strings.EqualFold(l.Linetype, "Continuous") || l.LineWeight != -3 || !l.Plot || l.Flags != 0 {
		t.Errorf("Layer(0) = %+v", l)
	}
	if _, ok := doc.Tables.Layer("walls"); !ok {
		t.Error("Layer lookup is not case-insensitive")
	}
	if _, ok := doc.Tables.Layer("nope"); ok {
		t.Error("Layer(nope) found")
	}
}

func TestR7LayerFields(t *testing.T) {
	t.Parallel()
	tabs := &Tables{}
	tb := tabs.Ensure("LAYER")
	tb.Records = []Record{{TypeName: "LAYER", Groups: []Group{
		gp(5, "2A"), gp(330, "8"), gp(100, "AcDbSymbolTableRecord"), gp(100, "AcDbLayerTableRecord"),
		gp(2, "OFF-LAYER"), gp(70, "4"), gp(62, "-5"), gp(6, "Hidden"), gp(290, "0"), gp(370, "25"), gp(390, "F"),
		gp(1001, "ACAD"), gp(1070, "7"),
	}}}
	l, ok := tabs.Layer("off-layer")
	if !ok {
		t.Fatal("layer not found")
	}
	want := Layer{Name: "OFF-LAYER", Flags: 4, Color: -5, Linetype: "Hidden", LineWeight: 25, Plot: false, Handle: "2A"}
	if l != want {
		t.Fatalf("Layer = %+v, want %+v", l, want)
	}

	before := cloneGroups(tb.Records[0].Groups)
	tabs.SetLayer(l)
	if !reflect.DeepEqual(tb.Records[0].Groups, before) {
		t.Errorf("SetLayer of an unchanged layer changed the record:\n got %v\nwant %v", tb.Records[0].Groups, before)
	}

	l.Color, l.LineWeight, l.Plot = 3, 50, true
	tabs.SetLayer(l)
	got, _ := tabs.Layer("OFF-LAYER")
	if got.Color != 3 || got.LineWeight != 50 || !got.Plot {
		t.Errorf("after SetLayer: %+v", got)
	}
	// Uninterpreted groups survive and stay in place.
	g := tb.Records[0].Groups
	if len(g) != len(before) || g[10] != gp(390, "F") || g[len(g)-1] != gp(1070, "7") {
		t.Errorf("groups not preserved: %v", g)
	}
	if len(tb.Records) != 1 {
		t.Errorf("records = %d, want 1", len(tb.Records))
	}

	// A new layer: 370 and 290 are written only when needed.
	tabs.SetLayer(Layer{Name: "NEW", Color: 1, Linetype: "Continuous", Plot: true})
	rec := tb.Records[1]
	if topLevelIndex(rec.Groups, 370) >= 0 || topLevelIndex(rec.Groups, 290) >= 0 {
		t.Errorf("default new layer has 370/290: %v", rec.Groups)
	}
	tabs.SetLayer(Layer{Name: "NEW2", Color: 1, Linetype: "Continuous", LineWeight: 35, Plot: false})
	rec = tb.Records[2]
	if recordText(rec, 370) != "35" || recordText(rec, 290) != "0" {
		t.Errorf("new layer groups = %v", rec.Groups)
	}
	// Set on an existing record without 290 inserts it before 370.
	tabs.SetLayer(Layer{Name: "NEW", Color: 1, Linetype: "Continuous", LineWeight: 35, Plot: false})
	idx := func(code int) int { return topLevelIndex(tb.Records[1].Groups, code) }
	if !(idx(290) >= 0 && idx(290) < idx(370)) {
		t.Errorf("290 not inserted before 370: %v", tb.Records[1].Groups)
	}
}

func TestLinetypeAndTextStyleViews(t *testing.T) {
	t.Parallel()
	doc := NewDocument()
	doc.Tables.EnsureLinetype("DASHED")
	lt, ok := doc.Tables.Linetype("dashed")
	if !ok || lt.Name != "DASHED" || !reflect.DeepEqual(lt.Pattern, []float64{0.5, -0.25}) || !strings.HasPrefix(lt.Description, "Dashed") {
		t.Errorf("Linetype = %+v ok=%v", lt, ok)
	}
	c, ok := doc.Tables.Linetype("Continuous")
	if !ok || len(c.Pattern) != 0 || c.Description != "Solid line" {
		t.Errorf("Continuous = %+v ok=%v", c, ok)
	}
	if _, ok := doc.Tables.Linetype("x"); ok {
		t.Error("unknown linetype found")
	}
	st, ok := doc.Tables.TextStyle("Standard")
	want := TextStyle{Name: "STANDARD", Font: "txt", WidthFactor: 1}
	if !ok || st != want {
		t.Errorf("TextStyle = %+v ok=%v, want %+v", st, ok, want)
	}
	if _, ok := doc.Tables.TextStyle("x"); ok {
		t.Error("unknown style found")
	}
	// A linetype and a style with complete groups.
	tabs := &Tables{}
	tabs.Ensure("STYLE").Records = []Record{{TypeName: "STYLE", Groups: []Group{
		gp(5, "14"), gp(2, "Arial"), gp(70, "1"), gp(40, "3.5"), gp(41, "0.8"), gp(50, "15"), gp(71, "2"), gp(42, "2.5"), gp(3, "arial.ttf"), gp(4, "big.shx"),
	}}}
	st, _ = tabs.TextStyle("ARIAL")
	want = TextStyle{Name: "Arial", Font: "arial.ttf", BigFont: "big.shx", Height: 3.5, WidthFactor: 0.8, Oblique: 15, Flags: 1, GenFlags: 2, Handle: "14"}
	if st != want {
		t.Errorf("TextStyle = %+v, want %+v", st, want)
	}
}

func TestLoadBlockRecords(t *testing.T) {
	t.Parallel()
	tabs := &Tables{}
	tabs.Ensure("BLOCK_RECORD").Records = []Record{
		{TypeName: "BLOCK_RECORD", Groups: []Group{
			gp(5, "17"), gp(330, "E"), gp(100, "AcDbSymbolTableRecord"), gp(100, "AcDbBlockTableRecord"),
			gp(2, "*Model_Space"), gp(340, "1D"), gp(70, "4"), gp(280, "1"), gp(281, "1"),
		}},
		{TypeName: "BLOCK_RECORD", Groups: []Group{
			gp(5, "30"), gp(102, "{ACAD_XDICTIONARY"), gp(360, "31"), gp(102, "}"),
			gp(102, "{ACAD_REACTORS"), gp(330, "99"), gp(102, "}"),
			gp(330, "E"), gp(100, "AcDbSymbolTableRecord"), gp(100, "AcDbBlockTableRecord"),
			gp(2, "DOOR"), gp(70, "0"), gp(280, "0"), gp(1001, "ACAD"), gp(1000, "x"), gp(1070, "5"),
		}},
	}
	tabs.LoadBlockRecords()
	if len(tabs.BlockRecords) != 2 {
		t.Fatalf("BlockRecords = %d", len(tabs.BlockRecords))
	}
	m := tabs.BlockRecords[0]
	if m.Name != "*Model_Space" || m.Handle != "17" || m.Layout != 0x1D || m.Units != 4 || !m.Explodable || !m.Scalable || len(m.Raw) != 0 {
		t.Errorf("model = %+v", m)
	}
	d := tabs.BlockRecords[1]
	if d.Name != "DOOR" || d.Layout != 0 || d.Explodable || d.Scalable {
		t.Errorf("door = %+v", d)
	}
	wantRaw := []Group{gp(102, "{ACAD_XDICTIONARY"), gp(360, "31"), gp(102, "}"), gp(102, "{ACAD_REACTORS"), gp(330, "99"), gp(102, "}"), gp(1001, "ACAD"), gp(1000, "x"), gp(1070, "5")}
	if !reflect.DeepEqual(d.Raw, wantRaw) {
		t.Errorf("door.Raw = %v\nwant %v", d.Raw, wantRaw)
	}

	// Idempotent, and entries not in the table are kept.
	tabs.BlockRecords = append(tabs.BlockRecords, BlockRecord{Name: "ADDED", Explodable: true})
	tabs.LoadBlockRecords()
	first := append([]BlockRecord(nil), tabs.BlockRecords...)
	tabs.LoadBlockRecords()
	if !reflect.DeepEqual(first, tabs.BlockRecords) || len(first) != 3 || first[2].Name != "ADDED" {
		t.Errorf("not idempotent: %+v", tabs.BlockRecords)
	}
	var empty *Tables
	empty.LoadBlockRecords() // must not panic
	(&Tables{}).LoadBlockRecords()
}

func TestR6BlockRecordPerBlock(t *testing.T) {
	t.Parallel()
	doc := NewDocument()
	doc.Blocks = []*Block{{Name: "DOOR"}, {Name: "Window"}}
	res := prepareR2013Tables(doc, counter(0x100))
	tb := res.Tables.Find("BLOCK_RECORD")
	if got, want := namesOf(tb), []string{"*Model_Space", "*Paper_Space", "DOOR", "Window"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("BLOCK_RECORD names = %v, want %v", got, want)
	}
	seen := map[Handle]bool{}
	for _, name := range []string{"*model_space", "*PAPER_SPACE", "door", "WINDOW"} {
		h, ok := res.BlockRecordHandle(name)
		if !ok || h.IsZero() || seen[h] {
			t.Errorf("handle of %s = %v ok=%v (dup=%v)", name, h, ok, seen[h])
		}
		seen[h] = true
	}
	if res.ModelSpace != res.BlockRecord["*MODEL_SPACE"] || res.PaperSpace == 0 {
		t.Errorf("Model/Paper handles = %v %v", res.ModelSpace, res.PaperSpace)
	}
	// Record handle in the group list matches the map.
	for _, rec := range tb.Records {
		h, _ := ParseHandle(recordText(rec, 5))
		if got, _ := res.BlockRecordHandle(recordText(rec, 2)); got != h {
			t.Errorf("record %s: group handle %v, map %v", recordText(rec, 2), h, got)
		}
		if recordText(rec, 330) != res.TableHandle["BLOCK_RECORD"].String() {
			t.Errorf("record %s owner = %s", recordText(rec, 2), recordText(rec, 330))
		}
	}
	// A block that already has a record (any case) gets no second one; a typed
	// entry the table lacks is used instead of the default.
	doc2 := NewDocument()
	doc2.Tables.Ensure("BLOCK_RECORD").Records = []Record{{TypeName: "BLOCK_RECORD", Groups: []Group{
		gp(5, "40"), gp(330, "3F"), gp(100, "AcDbSymbolTableRecord"), gp(100, "AcDbBlockTableRecord"), gp(2, "door"), gp(70, "0"),
	}}}
	doc2.Tables.BlockRecords = []BlockRecord{{Name: "EXTRA", Units: 6, Explodable: true, Scalable: true}}
	doc2.Blocks = []*Block{{Name: "DOOR"}, {Name: "EXTRA"}, {Name: "X"}}
	res2 := prepareR2013Tables(doc2, counter(0x100))
	if got, want := namesOf(res2.Tables.Find("BLOCK_RECORD")), []string{"*Model_Space", "*Paper_Space", "door", "EXTRA", "X"}; !reflect.DeepEqual(got, want) {
		t.Errorf("names = %v, want %v", got, want)
	}
	if h, _ := res2.BlockRecordHandle("DOOR"); h != 0x40 {
		t.Errorf("existing record handle = %v, want 40", h)
	}
	extra := res2.Tables.Find("BLOCK_RECORD").Records[3]
	if recordText(extra, 70) != "6" || recordText(extra, 281) != "1" {
		t.Errorf("EXTRA record = %v", extra.Groups)
	}
}

func TestSetLayoutHandles(t *testing.T) {
	t.Parallel()
	res := prepareR2013Tables(NewDocument(), counter(1))
	res.SetLayoutHandles(0x1D, 0x1E)
	res.SetLayoutHandles(0, 0) // no-op
	tb := res.Tables.Find("BLOCK_RECORD")
	if recordText(tb.Records[0], 340) != "1D" || recordText(tb.Records[1], 340) != "1E" {
		t.Errorf("340 = %q %q", recordText(tb.Records[0], 340), recordText(tb.Records[1], 340))
	}
	// 340 sits right after the name, as in v00.
	if i := topLevelIndex(tb.Records[0].Groups, 340); i != topLevelIndex(tb.Records[0].Groups, 2)+1 {
		t.Errorf("340 position %d in %v", i, tb.Records[0].Groups)
	}
}

func hasRecord(doc *Tables, table, name string) bool {
	return findRecord(doc.Find(table), table, name) >= 0
}

func TestR8MinimalFileTables(t *testing.T) {
	t.Parallel()
	res := prepareR2013Tables(&Document{}, counter(1))
	for table, names := range map[string][]string{
		"LTYPE":    {"ByBlock", "ByLayer", "Continuous"},
		"APPID":    {"ACAD"},
		"LAYER":    {"0"},
		"STYLE":    {"Standard"},
		"VPORT":    {"*Active"},
		"DIMSTYLE": {"Standard"},
	} {
		got := namesOf(res.Tables.Find(table))
		if !reflect.DeepEqual(got, names) {
			t.Errorf("%s = %v, want %v", table, got, names)
		}
	}
	var order []string
	for _, tb := range res.Tables.Items {
		order = append(order, tb.Name)
	}
	if !reflect.DeepEqual(order, r2013TableOrder) {
		t.Errorf("table order = %v", order)
	}
}

func TestNoDuplicatesByCase(t *testing.T) {
	t.Parallel()
	doc := NewDocument() // CONTINUOUS, STANDARD in capitals
	res := prepareR2013Tables(doc, counter(1))
	if got, want := namesOf(res.Tables.Find("LTYPE")), []string{"ByBlock", "ByLayer", "CONTINUOUS"}; !reflect.DeepEqual(got, want) {
		t.Errorf("LTYPE = %v, want %v", got, want)
	}
	if got, want := namesOf(res.Tables.Find("STYLE")), []string{"STANDARD"}; !reflect.DeepEqual(got, want) {
		t.Errorf("STYLE = %v, want %v", got, want)
	}
	if got := namesOf(res.Tables.Find("LAYER")); !reflect.DeepEqual(got, []string{"0"}) {
		t.Errorf("LAYER = %v", got)
	}
	// Mixed case in the other direction, and a layer using the continuous name.
	doc2 := &Document{}
	doc2.Tables.EnsureAppID("acad")
	doc2.Tables.EnsureLayer("0", 7, "continuous")
	doc2.Tables.EnsureLayer("L1", 1, "continuous")
	res2 := prepareR2013Tables(doc2, counter(1))
	if got := namesOf(res2.Tables.Find("APPID")); !reflect.DeepEqual(got, []string{"acad"}) {
		t.Errorf("APPID = %v", got)
	}
	if got := len(res2.Tables.Find("LAYER").Records); got != 2 {
		t.Errorf("LAYER records = %d", got)
	}
	// dim style 340 points at the existing style handle.
	dim := res.Tables.Find("DIMSTYLE").Records[0]
	style := res.Tables.Find("STYLE").Records[0]
	if recordText(dim, 340) != recordText(style, 5) || recordText(dim, 340) == "" {
		t.Errorf("DIMSTYLE 340 = %q, STYLE handle = %q", recordText(dim, 340), recordText(style, 5))
	}
}

func TestPrepareDoesNotModifyDocument(t *testing.T) {
	t.Parallel()
	doc := NewDocument()
	doc.Blocks = []*Block{{Name: "B"}}
	doc.Tables.EnsureLayer("X", 1, "DASHED")
	snapshot := cloneTables(&doc.Tables)
	prepareR2013Tables(doc, counter(1))
	if !reflect.DeepEqual(snapshot.Items, doc.Tables.Items) || len(doc.Tables.BlockRecords) != 0 {
		t.Error("prepareR2013Tables modified doc.Tables")
	}
}

func TestPrepareHeadersAndOwners(t *testing.T) {
	t.Parallel()
	doc := NewDocument()
	doc.Blocks = []*Block{{Name: "B"}}
	doc.Tables.EnsureLayer("X", 1, "DASHED")
	res := prepareR2013Tables(doc, counter(0x50))
	seen := map[Handle]string{}
	for _, tb := range res.Tables.Items {
		th, ok := existingHandle(tb.Raw, 5)
		if !ok || recordText(Record{Groups: tb.Raw}, 330) != "0" {
			t.Errorf("table %s header = %v", tb.Name, tb.Raw)
		}
		if th != res.TableHandle[strings.ToUpper(tb.Name)] {
			t.Errorf("table %s handle mismatch", tb.Name)
		}
		if prev, dup := seen[th]; dup {
			t.Errorf("duplicate handle %v: %s and %s", th, prev, tb.Name)
		}
		seen[th] = tb.Name
		if recordText(Record{Groups: tb.Raw}, 70) != itoa(len(tb.Records)) {
			t.Errorf("table %s count = %q for %d records", tb.Name, recordText(Record{Groups: tb.Raw}, 70), len(tb.Records))
		}
		marker := r2013RecordMarker[strings.ToUpper(tb.Name)]
		code := 5
		if tb.Name == "DIMSTYLE" {
			code = 105
			if recordText(Record{Groups: tb.Raw}, 71) != itoa(len(tb.Records)) || !hasSubclass(tb.Raw, markerDimStylTbl) {
				t.Errorf("DIMSTYLE header = %v", tb.Raw)
			}
		}
		for _, rec := range tb.Records {
			h, ok := existingHandle(rec.Groups, code)
			if !ok {
				t.Fatalf("%s %v has no handle", tb.Name, rec.Groups)
			}
			if prev, dup := seen[h]; dup {
				t.Errorf("duplicate handle %v: %s and %s", h, prev, tb.Name)
			}
			seen[h] = tb.Name
			if recordText(rec, 330) != th.String() {
				t.Errorf("%s record owner = %s, want %v", tb.Name, recordText(rec, 330), th)
			}
			if rec.Groups[0].Code != code || rec.Groups[1].Code != 330 ||
				rec.Groups[2] != gp(100, "AcDbSymbolTableRecord") || rec.Groups[3] != gp(100, marker) {
				t.Errorf("%s record header = %v", tb.Name, rec.Groups[:4])
			}
		}
	}
	if x, ok := res.Tables.Layer("X"); !ok || x.Handle == "" {
		t.Errorf("layer X = %+v ok=%v", x, ok)
	}
	if !hasRecord(res.Tables, "LTYPE", "DASHED") {
		t.Error("standard linetype used by a layer was not added")
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func TestPrepareSkipsUsedHandlesIncludingDimStyle(t *testing.T) {
	t.Parallel()
	doc := &Document{}
	doc.Tables.Ensure("DIMSTYLE").Records = []Record{{TypeName: "DIMSTYLE", Groups: []Group{gp(105, "16"), gp(2, "Standard"), gp(70, "0")}}}
	res := prepareR2013Tables(doc, counter(0x14))
	seen := map[Handle]bool{}
	for _, tb := range res.Tables.Items {
		code := 5
		if tb.Name == "DIMSTYLE" {
			code = 105
		}
		for _, rec := range tb.Records {
			h, _ := existingHandle(rec.Groups, code)
			if seen[h] {
				t.Errorf("duplicate handle %v in %s", h, tb.Name)
			}
			seen[h] = true
		}
	}
	if h, _ := existingHandle(res.Tables.Find("DIMSTYLE").Records[0].Groups, 105); h != 0x16 {
		t.Errorf("DIMSTYLE handle changed to %v", h)
	}
}

// TestR8TablesMatchV00 compares the tables generated for an empty document with
// the TABLES section of T1 v00, ignoring handle values.
func TestR8TablesMatchV00(t *testing.T) {
	t.Parallel()
	want := readV00Tables(t)
	res := prepareR2013Tables(&Document{}, counter(6))
	res.SetLayoutHandles(0x1D, 0x1E)
	got := flattenTables(res.Tables)
	if !reflect.DeepEqual(stripHandles(got), stripHandles(want)) {
		gs, ws := stripHandles(got), stripHandles(want)
		for i := 0; i < len(gs) && i < len(ws); i++ {
			if gs[i] != ws[i] {
				t.Fatalf("first difference at group %d: got %v, want %v", i, gs[i], ws[i])
			}
		}
		t.Fatalf("length differs: got %d groups, want %d", len(gs), len(ws))
	}
	// With the same allocation order the handles are the same too, apart from
	// the 390 (plot style object, T8) and the layouts, which v00 numbers after
	// the tables.
	if !reflect.DeepEqual(dropCodes(got, 390), dropCodes(want, 390)) {
		t.Errorf("handles differ from v00 even with sequential allocation")
	}
}

func dropCodes(groups []Group, codes ...int) []Group {
	var out []Group
next:
	for _, gr := range groups {
		for _, c := range codes {
			if gr.Code == c {
				continue next
			}
		}
		out = append(out, gr)
	}
	return out
}

// TestR7TablesFromRealFileUnchanged reads the v00 tables, prepares them for
// writing and expects the same tags in the same order: VIEW, UCS, DIMSTYLE and
// every other record come back identical.
func TestR7TablesFromRealFileUnchanged(t *testing.T) {
	t.Parallel()
	groups := readV00Tables(t)
	tabs := parseTables(t, groups)
	tabs.LoadBlockRecords()
	if len(tabs.BlockRecords) != 2 || tabs.BlockRecords[0].Layout != 0x1D {
		t.Fatalf("BlockRecords = %+v", tabs.BlockRecords)
	}
	res := prepareR2013Tables(&Document{Tables: *tabs}, counter(0x22))
	if got := flattenTables(res.Tables); !reflect.DeepEqual(got, groups) {
		for i := 0; i < len(got) && i < len(groups); i++ {
			if got[i] != groups[i] {
				t.Fatalf("first difference at group %d: got %v, want %v", i, got[i], groups[i])
			}
		}
		t.Fatalf("length differs: got %d, want %d", len(got), len(groups))
	}
	if res.ModelSpace != 0x17 || res.PaperSpace != 0x18 {
		t.Errorf("Model/Paper = %v %v", res.ModelSpace, res.PaperSpace)
	}
}

// TestR7ViewUCSDimStylePreserved checks records with reactors, extended data
// and many DIMSTYLE variables: they come back byte for byte, with handles kept.
func TestR7ViewUCSDimStylePreserved(t *testing.T) {
	t.Parallel()
	view := Record{TypeName: "VIEW", Groups: []Group{
		gp(5, "60"), gp(102, "{ACAD_XDICTIONARY"), gp(360, "61"), gp(102, "}"), gp(330, "A"),
		gp(100, "AcDbSymbolTableRecord"), gp(100, "AcDbViewTableRecord"),
		gp(2, "V1"), gp(70, "0"), gp(40, "100.5"), gp(41, "2.0"), gp(10, "1.0"), gp(20, "2.0"), gp(11, "0.0"), gp(21, "0.0"), gp(31, "1.0"), gp(1001, "ACAD"), gp(1000, "data"),
	}}
	ucs := Record{TypeName: "UCS", Groups: []Group{
		gp(5, "62"), gp(330, "B"), gp(100, "AcDbSymbolTableRecord"), gp(100, "AcDbUCSTableRecord"),
		gp(2, "U1"), gp(70, "0"), gp(10, "0.0"), gp(20, "0.0"), gp(30, "0.0"), gp(11, "1.0"), gp(21, "0.0"), gp(31, "0.0"), gp(12, "0.0"), gp(22, "1.0"), gp(32, "0.0"),
	}}
	dim := Record{TypeName: "DIMSTYLE", Groups: []Group{
		gp(105, "63"), gp(330, "D"), gp(100, "AcDbSymbolTableRecord"), gp(100, "AcDbDimStyleTableRecord"),
		gp(2, "ISO-25"), gp(70, "0"), gp(3, ""), gp(4, ""), gp(40, "1.0"), gp(41, "2.5"), gp(42, "0.625"), gp(44, "1.25"), gp(73, "0"), gp(74, "0"),
		gp(77, "1"), gp(78, "8"), gp(140, "2.5"), gp(141, "2.5"), gp(271, "2"), gp(340, "14"), gp(371, "-2"), gp(1001, "ACAD_DSTYLE_DIMJAG"), gp(1070, "388"), gp(1040, "1.5"),
	}}
	doc := &Document{}
	doc.Tables.Ensure("VIEW").Records = []Record{view}
	doc.Tables.Ensure("UCS").Records = []Record{ucs}
	doc.Tables.Ensure("DIMSTYLE").Records = []Record{dim}
	doc.Tables.Find("VIEW").Raw = []Group{gp(5, "A"), gp(330, "0"), gp(100, "AcDbSymbolTable"), gp(70, "1")}
	doc.Tables.Find("UCS").Raw = []Group{gp(5, "B"), gp(330, "0"), gp(100, "AcDbSymbolTable"), gp(70, "1")}
	doc.Tables.Find("DIMSTYLE").Raw = []Group{gp(5, "D"), gp(330, "0"), gp(100, "AcDbSymbolTable"), gp(70, "1"), gp(100, "AcDbDimStyleTable"), gp(71, "1")}

	res := prepareR2013Tables(doc, counter(0x70))
	for _, c := range []struct {
		table string
		want  Record
	}{{"VIEW", view}, {"UCS", ucs}, {"DIMSTYLE", dim}} {
		tb := res.Tables.Find(c.table)
		// The generated "Standard" DIMSTYLE comes first; the file's record
		// must follow unchanged.
		var found bool
		for _, rec := range tb.Records {
			if reflect.DeepEqual(rec, c.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s record changed: %v", c.table, tb.Records)
		}
	}
	if raw := res.Tables.Find("VIEW").Raw; !reflect.DeepEqual(raw, doc.Tables.Find("VIEW").Raw) {
		t.Errorf("VIEW header changed: %v", raw)
	}
	if res.TableHandle["DIMSTYLE"] != 0xD || res.TableHandle["VIEW"] != 0xA {
		t.Errorf("table handles = %v", res.TableHandle)
	}
	// DIMSTYLE count: file record + generated Standard.
	if raw := res.Tables.Find("DIMSTYLE").Raw; recordText(Record{Groups: raw}, 70) != "2" || recordText(Record{Groups: raw}, 71) != "2" {
		t.Errorf("DIMSTYLE header = %v", raw)
	}
}

func TestPrepareRecordWithoutHeader(t *testing.T) {
	t.Parallel()
	// An R12-style record (no handle, no markers) and one with only a handle.
	doc := &Document{}
	doc.Tables.Ensure("LAYER").Records = []Record{
		{TypeName: "LAYER", Groups: []Group{gp(2, "A"), gp(70, "0"), gp(62, "1"), gp(6, "Continuous")}},
		{TypeName: "LAYER", Groups: []Group{gp(5, "2F"), gp(2, "B"), gp(70, "0"), gp(62, "2"), gp(6, "Continuous")}},
	}
	res := prepareR2013Tables(doc, counter(0x100))
	layers := res.Tables.Find("LAYER")
	lh := res.TableHandle["LAYER"].String()
	wantB := []Group{gp(5, "2F"), gp(330, lh), gp(100, "AcDbSymbolTableRecord"), gp(100, "AcDbLayerTableRecord"), gp(2, "B"), gp(70, "0"), gp(62, "2"), gp(6, "Continuous")}
	if got := layers.Records[2].Groups; !reflect.DeepEqual(got, wantB) {
		t.Errorf("record B = %v\nwant %v", got, wantB)
	}
	a := layers.Records[1].Groups
	if len(a) != 8 || a[0].Code != 5 || a[0].Value == "" || a[4] != gp(2, "A") {
		t.Errorf("record A = %v", a)
	}
}
