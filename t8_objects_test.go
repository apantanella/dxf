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

// t8Counter returns a handle counter that starts at 1.
func t8Counter() func() Handle {
	var n Handle
	return func() Handle { n++; return n }
}

// t8Prepare runs prepareR2013Tables and prepareR2013Objects with one counter
// that starts after the highest handle of doc.
func t8Prepare(doc *Document) (*r2013Tables, *r2013Objects) {
	n := Handle(maxHandle(doc))
	next := func() Handle { n++; return n }
	tbl := prepareR2013Tables(doc, next)
	return tbl, prepareR2013Objects(doc, tbl, next)
}

func t8EmptyDoc() *Document {
	doc := &Document{Version: VersionR2013, Header: Header{Variables: map[string]Variable{}}}
	doc.Header.Set("$ACADVER", 1, VersionR2013)
	return doc
}

// t8Sections reads a DXF file with tags.Reader and returns, per section name,
// the tags between the section name and ENDSEC.
func t8Sections(t *testing.T, path string) map[string][]tags.Tag {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := tags.NewReader(f)
	out := map[string][]tags.Tag{}
	var cur string
	for {
		tag, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case tag.Code == 0 && tag.Value == "SECTION":
			name, err := r.Next()
			if err != nil {
				t.Fatal(err)
			}
			cur = name.Value
			out[cur] = nil
		case tag.Code == 0 && tag.Value == "ENDSEC":
			cur = ""
		case cur != "":
			out[cur] = append(out[cur], tag)
		}
	}
	return out
}

// t8Flatten writes classes and objects as the tags of the two sections.
func t8Flatten(res *r2013Objects) (classes, objects []Group) {
	for _, c := range res.Classes {
		classes = append(classes, Group{Code: 0, Value: "CLASS"})
		classes = append(classes, c.Groups()...)
	}
	for _, o := range res.Objects {
		objects = append(objects, Group{Code: 0, Value: o.Type()})
		objects = append(objects, objectGroups(o)...)
	}
	return
}

var t8HandleCodes = map[int]bool{5: true, 105: true, 330: true, 340: true, 350: true, 360: true, 390: true}

// t8SameModuloHandles compares two tag sequences: same codes, same values,
// except for the values of handle codes, which must correspond one to one.
func t8SameModuloHandles(t *testing.T, name string, got, want []Group) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: %d tags, want %d", name, len(got), len(want))
	}
	fwd, back := map[string]string{"0": "0"}, map[string]string{"0": "0"}
	for i := 0; i < len(got) && i < len(want); i++ {
		g, w := got[i], want[i]
		if g.Code != w.Code {
			t.Errorf("%s: tag %d: code %d, want %d (%v / %v)", name, i, g.Code, w.Code, g, w)
			return
		}
		if t8HandleCodes[g.Code] {
			if prev, ok := fwd[g.Value]; ok && prev != w.Value {
				t.Errorf("%s: tag %d: handle %s was %s, now %s", name, i, g.Value, prev, w.Value)
			}
			if prev, ok := back[w.Value]; ok && prev != g.Value {
				t.Errorf("%s: tag %d: v00 handle %s maps from %s and %s", name, i, w.Value, prev, g.Value)
			}
			fwd[g.Value], back[w.Value] = w.Value, g.Value
			continue
		}
		if g.Value != w.Value {
			t.Errorf("%s: tag %d: %v, want %v", name, i, g, w)
			return
		}
	}
}

func TestR10MinimalObjects(t *testing.T) {
	t.Parallel()

	doc := t8EmptyDoc()
	_, res := t8Prepare(doc)

	if len(doc.Objects) != 0 || len(doc.Classes) != 0 {
		t.Fatal("prepareR2013Objects modified the document")
	}
	var types []string
	for _, o := range res.Objects {
		types = append(types, o.Type())
	}
	want := []string{"DICTIONARY", "DICTIONARY", "DICTIONARY", "ACDBDICTIONARYWDFLT", "ACDBPLACEHOLDER", "LAYOUT", "LAYOUT"}
	if !reflect.DeepEqual(types, want) {
		t.Fatalf("types = %v, want %v", types, want)
	}
	root := res.Objects[0].(*Dictionary)
	if !root.Owner.IsZero() || root.ObjectHandle() != res.RootDictionary {
		t.Errorf("root = %+v", root)
	}
	d := &Document{Objects: res.Objects}
	if d.RootDictionary() != root {
		t.Error("RootDictionary() is not the first object")
	}
	for _, name := range []string{"ACAD_GROUP", "ACAD_LAYOUT", "ACAD_PLOTSTYLENAME"} {
		if _, ok := root.Lookup(name); !ok {
			t.Errorf("root has no %s", name)
		}
	}
	layoutH, _ := root.Lookup("ACAD_LAYOUT")
	layouts := res.Objects[2].(*Dictionary)
	if layouts.ObjectHandle() != layoutH {
		t.Fatalf("object 2 is not ACAD_LAYOUT")
	}
	if m, ok := layouts.Lookup("Model"); !ok || m != res.ModelLayout {
		t.Errorf("Model = %v %v", m, ok)
	}
	if p, ok := layouts.Lookup("Layout1"); !ok || p != res.PaperLayout {
		t.Errorf("Layout1 = %v %v", p, ok)
	}
	plot := res.Objects[3].(*Dictionary)
	if !plot.WithDefault || plot.Default != res.PlotStyleNormal {
		t.Errorf("plot style dictionary = %+v", plot)
	}
	if n, ok := plot.Lookup("Normal"); !ok || n != res.PlotStyleNormal {
		t.Errorf("Normal = %v %v", n, ok)
	}
	model, paper := res.Objects[5].(*Layout), res.Objects[6].(*Layout)
	if model.Name != "Model" || model.TabOrder != 0 || paper.Name != "Layout1" || paper.TabOrder != 1 {
		t.Errorf("layouts = %+v %+v", model, paper)
	}
	if model.Owner != layoutH || paper.Owner != layoutH {
		t.Errorf("layout owners = %v %v, want %v", model.Owner, paper.Owner, layoutH)
	}
}

func TestT8MatchesV00(t *testing.T) {
	t.Parallel()

	sections := t8Sections(t, "testdata/t1/v00-baseline.dxf")
	_, res := t8Prepare(t8EmptyDoc())
	classes, objects := t8Flatten(res)
	t8SameModuloHandles(t, "CLASSES", classes, sections["CLASSES"])
	t8SameModuloHandles(t, "OBJECTS", objects, sections["OBJECTS"])
}

func TestT8PreparedTablesMatchLayouts(t *testing.T) {
	t.Parallel()

	tbl, res := t8Prepare(t8EmptyDoc())
	br := tbl.Tables.Find("BLOCK_RECORD")
	for _, c := range []struct {
		name   string
		layout Handle
	}{{"*Model_Space", res.ModelLayout}, {"*Paper_Space", res.PaperLayout}} {
		i := findRecord(br, "BLOCK_RECORD", c.name)
		if i < 0 {
			t.Fatalf("no BLOCK_RECORD %s", c.name)
		}
		if got := handleOf(recordText(br.Records[i], 340)); got != c.layout {
			t.Errorf("%s: 340 = %s, want %s", c.name, got, c.layout)
		}
	}
	model, paper := res.Objects[5].(*Layout), res.Objects[6].(*Layout)
	if model.BlockRecord != tbl.ModelSpace || paper.BlockRecord != tbl.PaperSpace {
		t.Errorf("layout block records = %s %s, want %s %s", model.BlockRecord, paper.BlockRecord, tbl.ModelSpace, tbl.PaperSpace)
	}
	// The layer 0 refers to the generated "Normal".
	layer := tbl.Tables.Find("LAYER")
	if h := handleOf(recordText(layer.Records[0], 390)); h != res.PlotStyleNormal || h.IsZero() {
		t.Errorf("layer 0: 390 = %s, want %s", h, res.PlotStyleNormal)
	}
}

// t8References checks that every 330, 340, 350, 360 and 390 of the tables and
// of the objects points to a handle that exists (0 is allowed as an owner).
func t8References(t *testing.T, tbl *r2013Tables, res *r2013Objects) {
	t.Helper()
	exists := map[Handle]bool{0: true}
	var all [][]Group
	for _, tb := range tbl.Tables.Items {
		all = append(all, tb.Raw)
		for _, rec := range tb.Records {
			all = append(all, rec.Groups)
		}
	}
	for _, o := range res.Objects {
		all = append(all, objectGroups(o))
	}
	for _, g := range all {
		for _, grp := range g {
			if grp.Code == 5 || grp.Code == 105 {
				exists[handleOf(grp.Value)] = true
			}
		}
	}
	for _, g := range all {
		for _, grp := range g {
			switch grp.Code {
			case 330, 340, 350, 360, 390:
				if !exists[handleOf(grp.Value)] {
					t.Errorf("%d %s points to no object", grp.Code, grp.Value)
				}
			}
		}
	}
}

func TestT8ReferencesAreConsistent(t *testing.T) {
	t.Parallel()

	doc := t8EmptyDoc()
	doc.Tables.EnsureLayer("WALLS", 3, "Continuous")
	doc.Tables.EnsureLayer("BEAMS", 5, "Continuous")
	tbl, res := t8Prepare(doc)
	t8References(t, tbl, res)

	check := &Document{Version: VersionR2013, Tables: *tbl.Tables, Objects: res.Objects}
	if diags := check.ValidateHandles(); len(diags) != 0 {
		t.Errorf("ValidateHandles = %v", diags)
	}
	layer := tbl.Tables.Find("LAYER")
	for _, rec := range layer.Records {
		if h := handleOf(recordText(rec, 390)); h != res.PlotStyleNormal {
			t.Errorf("layer %s: 390 = %s", recordText(rec, 2), h)
		}
	}
	// Every object has an owner that exists and a distinct handle.
	if _, ok := check.Lookup(res.RootDictionary); !ok {
		t.Error("root dictionary not found by Lookup")
	}
}

const t8V00 = "testdata/t1/v00-baseline.dxf"

func TestT8V00ReadAndRewriteIsIdentical(t *testing.T) {
	t.Parallel()

	f, err := os.Open(t8V00)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, o := range doc.Objects {
		types = append(types, o.Type())
	}
	if want := []string{"DICTIONARY", "DICTIONARY", "DICTIONARY", "ACDBDICTIONARYWDFLT", "ACDBPLACEHOLDER", "LAYOUT", "LAYOUT"}; !reflect.DeepEqual(types, want) {
		t.Fatalf("types = %v", types)
	}
	if doc.RootDictionary() == nil || doc.RootDictionary().Handle != "1" {
		t.Fatalf("RootDictionary = %+v", doc.RootDictionary())
	}
	if len(doc.Classes) != 3 {
		t.Fatalf("classes = %d", len(doc.Classes))
	}

	sections := t8Sections(t, t8V00)
	// Read back exactly (no handle mapping needed): the document is not changed.
	tbl, res := t8Prepare(doc)
	classes, objects := t8Flatten(res)
	if !reflect.DeepEqual(classes, sections["CLASSES"]) {
		t.Errorf("CLASSES differ:\n got %v\nwant %v", classes, sections["CLASSES"])
	}
	if !reflect.DeepEqual(objects, sections["OBJECTS"]) {
		t.Errorf("OBJECTS differ:\n got %v\nwant %v", objects, sections["OBJECTS"])
	}
	// No second root dictionary, no second layout, nothing generated.
	if len(res.Objects) != 7 {
		t.Errorf("objects = %d, want 7", len(res.Objects))
	}
	roots := 0
	for _, o := range res.Objects {
		if d, ok := o.(*Dictionary); ok && d.Owner.IsZero() && !d.WithDefault {
			roots++
		}
	}
	if roots != 1 {
		t.Errorf("root dictionaries = %d", roots)
	}
	if res.ModelLayout != 0x1D || res.PaperLayout != 0x1E || res.PlotStyleNormal != 5 {
		t.Errorf("handles = %s %s %s", res.ModelLayout, res.PaperLayout, res.PlotStyleNormal)
	}
	t8References(t, tbl, res)
}

func TestR9NonStandardClassesPreserved(t *testing.T) {
	t.Parallel()

	listing := `0 CLASS
1 MYOBJECT
2 CMyObject
3 Acme Tools|v2
90 4095
91 17
280 1
281 1
0 CLASS
1 WEIRD
2 CWeird
3 ObjectDBX Classes
90 0
999 comment kept
91 0
0 CLASS
1 SHORT
2 CShort
3 App`
	doc := decodeT6(t, t6File(VersionR2013, "CLASSES", listing))
	if len(doc.Classes) != 3 || len(doc.UnknownSections) != 0 {
		t.Fatalf("classes = %+v, unknown = %v", doc.Classes, doc.UnknownSections)
	}
	c := doc.Classes[0]
	if c.DXFName != "MYOBJECT" || c.CppName != "CMyObject" || c.AppName != "Acme Tools|v2" ||
		c.Flags != 4095 || c.Instances != 17 || !c.WasProxy || !c.IsEntity {
		t.Errorf("class 0 = %+v", c)
	}
	// Each class is rewritten with exactly the groups it was read with.
	var want []Group
	for _, tag := range tagList(t, listing) {
		want = append(want, tag)
	}
	var got []Group
	for _, c := range doc.Classes {
		got = append(got, Group{Code: 0, Value: "CLASS"})
		got = append(got, c.Groups()...)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("classes rewritten differently:\n got %v\nwant %v", got, want)
	}

	// The same through the R2013 preparation: the invented classes come first
	// and unchanged, then the generated ones.
	_, res := t8Prepare(doc)
	classes, _ := t8Flatten(res)
	if len(classes) < len(want) || !reflect.DeepEqual(classes[:len(want)], want) {
		t.Errorf("prepared classes do not start with the file's classes: %v", classes)
	}
	var names []string
	for _, c := range res.Classes {
		names = append(names, c.DXFName)
	}
	if wantNames := []string{"MYOBJECT", "WEIRD", "SHORT", "ACDBDICTIONARYWDFLT", "ACDBPLACEHOLDER", "LAYOUT"}; !reflect.DeepEqual(names, wantNames) {
		t.Errorf("class names = %v, want %v", names, wantNames)
	}
}

func TestT8ClassFieldsWinOverRaw(t *testing.T) {
	t.Parallel()

	doc := decodeT6(t, t6File(VersionR2013, "CLASSES", "0 CLASS\n1 A\n2 B\n3 C\n90 007\n91 1"))
	c := doc.Classes[0]
	c.Flags = 7 // same value, other text: the original text is kept
	c.Instances = 2
	c.IsEntity = true // absent in the file: added
	got := c.Groups()
	want := tagList(t, "1 A\n2 B\n3 C\n90 007\n91 2\n281 1")
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Groups() = %v, want %v", got, want)
	}
	// A class built by hand writes all seven groups.
	hand := Class{DXFName: "X", CppName: "CX", AppName: "App"}
	if g := hand.Groups(); len(g) != 7 || g[0].Code != 1 || g[6].Code != 281 {
		t.Errorf("hand-built class groups = %v", g)
	}
	// Groups returns a copy.
	g1 := doc.Classes[0].Groups()
	g1[0].Value = "CHANGED"
	if doc.Classes[0].DXFName != "A" || doc.Classes[0].Raw[0].Value != "A" {
		t.Error("Groups() shares memory with the class")
	}
}

func TestT8ClassesSectionNotPlainStaysRaw(t *testing.T) {
	t.Parallel()

	doc := decodeT6(t, t6File(VersionR2013, "CLASSES", "0 CLASS\n1 A\n0 JUNK\n1 B"))
	if len(doc.Classes) != 0 || len(doc.UnknownSections) != 1 || doc.UnknownSections[0].Name != "CLASSES" {
		t.Errorf("classes = %v, unknown = %v", doc.Classes, doc.UnknownSections)
	}
}

func TestR10RawObjectsPreserved(t *testing.T) {
	t.Parallel()

	objectListing := `0 XRECORD
5 70
102 {ACAD_REACTORS
330 71
102 }
330 72
100 AcDbXrecord
280 1
1 data
0 ACAD_MLINESTYLE_DICT_PLACEHOLDER
5 73
330 74
0 MLINESTYLE
5 75
330 76
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
0 SCALELIST
5 77
102 {ACAD_XDICTIONARY
360 78
102 }
330 79
100 AcDbScaleList
0 DICTIONARY
5 7A
330 7B
100 AcDbDictionary
281 1
3 ACAD_MLINESTYLE
350 75
0 ACDBPLACEHOLDER
5 7C
330 7D`
	doc := decodeT6(t, t6File(VersionR2013, "OBJECTS", objectListing))
	if len(doc.Objects) != 6 {
		t.Fatalf("objects = %d", len(doc.Objects))
	}
	_, res := t8Prepare(doc)
	// The file's objects are in the output, in their order, as the same RawObject
	// pointers or with the same groups; the generated ones come first (root) or
	// after.
	var kept []Object
	for _, o := range res.Objects {
		for _, orig := range doc.Objects {
			if o == orig {
				kept = append(kept, o)
			}
		}
	}
	rawKept := 0
	for _, o := range kept {
		if _, ok := o.(*RawObject); ok {
			rawKept++
		}
	}
	if rawKept != 4 {
		t.Fatalf("RawObjects kept as the same pointers = %d, want 4", rawKept)
	}
	// The root has been generated in front of the file's objects (the file's
	// DICTIONARY 7A has an owner).
	if _, ok := res.Objects[0].(*Dictionary); !ok || !res.Objects[0].ObjectOwner().IsZero() {
		t.Fatalf("first object = %#v", res.Objects[0])
	}
	got := res.Objects[1 : 1+len(doc.Objects)]
	for i, o := range got {
		if o.Type() != doc.Objects[i].Type() || !reflect.DeepEqual(objectGroups(o), objectGroups(doc.Objects[i])) {
			t.Errorf("object %d (%s) changed:\n got %v\nwant %v", i, doc.Objects[i].Type(), objectGroups(o), objectGroups(doc.Objects[i]))
		}
	}
	// References 330/340/350/360 of the preserved objects are untouched.
	if raw := got[0].(*RawObject); raw.Owner != 0x72 || raw.Groups[2].Value != "71" {
		t.Errorf("XRECORD = %+v", raw)
	}
	// The unknown objects reappear identically in a document that the writer
	// would build from the prepared objects.
	again := decodeT6(t, t6File(VersionR2013, "OBJECTS", t8Listing(res.Objects[1:1+len(doc.Objects)])))
	for i := range again.Objects {
		if !reflect.DeepEqual(objectGroups(again.Objects[i]), objectGroups(doc.Objects[i])) {
			t.Errorf("object %d does not survive a second read", i)
		}
	}
}

// t8Listing renders objects as a listing for t6File.
func t8Listing(objs []Object) string {
	var b strings.Builder
	for _, o := range objs {
		b.WriteString("0 " + o.Type() + "\n")
		for _, g := range objectGroups(o) {
			b.WriteString(strconv.Itoa(g.Code) + " " + g.Value + "\n")
		}
	}
	return b.String()
}

func TestT8ExistingDictionariesAreReused(t *testing.T) {
	t.Parallel()

	// A file with its own root, ACAD_GROUP with an entry, but no ACAD_LAYOUT
	// and no plot style dictionary.
	doc := decodeT6(t, t6File(VersionR2013, "OBJECTS", `0 DICTIONARY
5 A0
330 0
100 AcDbDictionary
281 1
3 ACAD_GROUP
350 A1
3 ACAD_COLOR
350 A9
0 DICTIONARY
5 A1
330 A0
100 AcDbDictionary
281 1
3 G1
350 A2
0 GROUP
5 A2
330 A1
0 DICTIONARY
5 A9
330 A0
100 AcDbDictionary
281 1`))
	tbl, res := t8Prepare(doc)
	if len(res.Objects) != 9 { // 4 file objects + ACAD_LAYOUT, ACAD_PLOTSTYLENAME, Normal, 2 layouts
		var types []string
		for _, o := range res.Objects {
			types = append(types, o.Type())
		}
		t.Fatalf("objects = %v", types)
	}
	root := res.Objects[0].(*Dictionary)
	if root != res.Objects[0] || root.Handle != "A0" {
		t.Fatalf("root = %+v", root)
	}
	if root.Entries[0].Name != "ACAD_GROUP" || root.Entries[1].Name != "ACAD_COLOR" ||
		root.Entries[2].Name != "ACAD_LAYOUT" || root.Entries[3].Name != "ACAD_PLOTSTYLENAME" || len(root.Entries) != 4 {
		t.Errorf("root entries = %+v", root.Entries)
	}
	// The ACAD_GROUP of the file is untouched and not duplicated.
	groupCount := 0
	for _, o := range res.Objects {
		if d, ok := o.(*Dictionary); ok && d.Owner == 0xA0 && d.ObjectHandle() == 0xA1 {
			groupCount++
			if len(d.Entries) != 1 {
				t.Errorf("ACAD_GROUP entries = %+v", d.Entries)
			}
		}
	}
	if groupCount != 1 {
		t.Errorf("ACAD_GROUP dictionaries = %d", groupCount)
	}
	// The document itself was not changed.
	if r := doc.RootDictionary(); r == root || len(r.Entries) != 2 {
		t.Errorf("document root was modified: %+v", r)
	}
	t8References(t, tbl, res)
	check := &Document{Version: VersionR2013, Tables: *tbl.Tables, Objects: res.Objects}
	if diags := check.ValidateHandles(); len(diags) != 0 {
		t.Errorf("ValidateHandles = %v", diags)
	}
}

func TestT8ClassesOnlyForPresentObjects(t *testing.T) {
	t.Parallel()

	dict := &Dictionary{Handle: "1"}
	if got := r2013Classes(nil, []Object{dict}); len(got) != 0 {
		t.Errorf("DICTIONARY only: classes = %v", got)
	}
	wdf := &Dictionary{Handle: "2", WithDefault: true}
	if got := r2013Classes(nil, []Object{dict, wdf}); len(got) != 1 || got[0].DXFName != "ACDBDICTIONARYWDFLT" || got[0].CppName != "AcDbDictionaryWithDefault" {
		t.Errorf("WDFLT: classes = %v", got)
	}
	objs := []Object{dict, wdf, &Placeholder{Handle: "3"}, &Layout{Handle: "4"}, &RawObject{TypeName: "XRECORD"}}
	got := r2013Classes([]Class{{DXFName: "layout", CppName: "mine"}}, objs)
	var names []string
	for _, c := range got {
		names = append(names, c.DXFName)
	}
	// A class the file declares (any case) is not generated twice.
	if want := []string{"layout", "ACDBDICTIONARYWDFLT", "ACDBPLACEHOLDER"}; !reflect.DeepEqual(names, want) {
		t.Errorf("names = %v, want %v", names, want)
	}
	if got[0].CppName != "mine" {
		t.Errorf("declared class changed: %+v", got[0])
	}
	g := generatedClass("LAYOUT", "AcDbLayout").Groups()
	if want := tagList(t, "1 LAYOUT\n2 AcDbLayout\n3 ObjectDBX Classes\n90 0\n91 0\n280 0\n281 0"); !reflect.DeepEqual(g, want) {
		t.Errorf("generated class groups = %v", g)
	}
}

func TestT8PlainPlotStyleDictionaryIsKept(t *testing.T) {
	t.Parallel()

	// The file's ACAD_PLOTSTYLENAME is a plain DICTIONARY whose Normal is an
	// unknown object: no placeholder, no WDFLT class, no 390 is generated.
	doc := decodeT6(t, t6File(VersionR2013, "OBJECTS", `0 DICTIONARY
5 B0
330 0
100 AcDbDictionary
3 ACAD_PLOTSTYLENAME
350 B1
0 DICTIONARY
5 B1
330 B0
100 AcDbDictionary
3 Normal
350 B2
0 XRECORD
5 B2
330 B1`))
	doc.Tables.EnsureLayer("L1", 1, "Continuous")
	tbl, res := t8Prepare(doc)
	for _, o := range res.Objects {
		if o.Type() == "ACDBPLACEHOLDER" || o.Type() == "ACDBDICTIONARYWDFLT" {
			t.Errorf("generated %s", o.Type())
		}
	}
	var names []string
	for _, c := range res.Classes {
		names = append(names, c.DXFName)
	}
	if !reflect.DeepEqual(names, []string{"LAYOUT"}) {
		t.Errorf("classes = %v", names)
	}
	for _, rec := range tbl.Tables.Find("LAYER").Records {
		if topLevelIndex(rec.Groups, 390) >= 0 {
			t.Errorf("layer %s got a 390", recordText(rec, 2))
		}
	}
}

func TestT8DictionaryRoundTripAndEdit(t *testing.T) {
	t.Parallel()

	listing := `0 DICTIONARY
5 1a
102 {ACAD_REACTORS
330 2B
102 }
102 {ACAD_XDICTIONARY
360 2C
102 }
330 0
100 AcDbDictionary
280 1
281 1
3 First
350 1b
3 Second
360 2d
1001 ACME
1000 data`
	doc := decodeT6(t, t6File(VersionR2013, "OBJECTS", listing))
	d := doc.Objects[0].(*Dictionary)
	if d.Handle != "1a" || d.ObjectHandle() != 0x1A || d.Owner != 0 || d.Cloning != 1 || len(d.Entries) != 2 {
		t.Fatalf("dictionary = %+v", d)
	}
	if d.Entries[1] != (DictionaryEntry{Name: "Second", Value: 0x2D, Hard: true}) {
		t.Errorf("entries = %+v", d.Entries)
	}
	want := tagList(t, listing)[1:]
	if got := d.Groups(); !reflect.DeepEqual(got, want) {
		t.Fatalf("unchanged dictionary:\n got %v\nwant %v", got, want)
	}

	// Edit: change one value, add an entry, change the owner. Reactors, 280,
	// the extended data and the text of the untouched handles stay.
	c := d.clone()
	c.Set("first", 0x99) // case-insensitive replace
	c.Set("Third", 0x3)
	c.Owner = 0x5
	got := c.Groups()
	wantEdit := tagList(t, `5 1a
102 {ACAD_REACTORS
330 2B
102 }
102 {ACAD_XDICTIONARY
360 2C
102 }
330 5
100 AcDbDictionary
280 1
281 1
3 First
350 99
3 Second
360 2d
3 Third
350 3
1001 ACME
1000 data`)
	if !reflect.DeepEqual(got, wantEdit) {
		t.Errorf("edited dictionary:\n got %v\nwant %v", got, wantEdit)
	}
	// The original is untouched.
	if !reflect.DeepEqual(d.Groups(), want) {
		t.Error("clone shares memory with the original")
	}
	// An empty dictionary gets its entries at the end of AcDbDictionary.
	empty := decodeT6(t, t6File(VersionR2013, "OBJECTS", "0 DICTIONARY\n5 1\n330 0\n100 AcDbDictionary\n281 1\n1001 X")).Objects[0].(*Dictionary)
	empty.Set("A", 7)
	if got, want := empty.Groups(), tagList(t, "5 1\n330 0\n100 AcDbDictionary\n281 1\n3 A\n350 7\n1001 X"); !reflect.DeepEqual(got, want) {
		t.Errorf("empty dictionary:\n got %v\nwant %v", got, want)
	}
	// Invalid handle text is reported like for a RawObject.
	bad := t6File(VersionR2013, "OBJECTS", "0 DICTIONARY\n5 ZZ\n330 0")
	if doc := decodeT6(t, bad, WithLenient()); len(doc.Diagnostics) == 0 {
		t.Error("invalid dictionary handle: no diagnostic")
	}
}

func TestT8WithDefaultDictionary(t *testing.T) {
	t.Parallel()

	listing := `0 ACDBDICTIONARYWDFLT
5 4
330 1
100 AcDbDictionary
281 1
3 Normal
350 5
100 AcDbDictionaryWithDefault
340 5`
	d := decodeT6(t, t6File(VersionR2013, "OBJECTS", listing)).Objects[0].(*Dictionary)
	if !d.WithDefault || d.Type() != "ACDBDICTIONARYWDFLT" || d.Default != 5 {
		t.Fatalf("dictionary = %+v", d)
	}
	d.Default = 9
	got := d.Groups()
	if got[len(got)-1] != (Group{Code: 340, Value: "9"}) {
		t.Errorf("groups = %v", got)
	}
	// Built by hand.
	hand := &Dictionary{Handle: "4", Owner: 1, Cloning: 1, WithDefault: true, Default: 5, Entries: []DictionaryEntry{{Name: "Normal", Value: 5}}}
	if got, want := hand.Groups(), tagList(t, listing)[1:]; !reflect.DeepEqual(got, want) {
		t.Errorf("hand-built:\n got %v\nwant %v", got, want)
	}
}

func TestT8LayoutRoundTripAndEdit(t *testing.T) {
	t.Parallel()

	sections := t8Sections(t, t8V00)
	obj := sections["OBJECTS"]
	// Find the LAYOUT "Layout1" record in v00.
	var rec []Group
	for i := 0; i < len(obj); i++ {
		if obj[i].Code == 0 && obj[i].Value == "LAYOUT" {
			j := i + 1
			for j < len(obj) && obj[j].Code != 0 {
				j++
			}
			rec = obj[i+1 : j]
			i = j - 1
		}
	}
	l := newLayout(cloneGroups(rec))
	if l.Name != "Layout1" || l.TabOrder != 1 || l.Handle != "1E" || l.Owner != 3 || l.BlockRecord != 0x18 || len(l.PlotSettings) == 0 {
		t.Fatalf("layout = %+v", l)
	}
	if l.PlotSettings[1] != (Group{Code: 4, Value: "A3"}) {
		t.Errorf("PlotSettings = %v", l.PlotSettings[:3])
	}
	if got := l.Groups(); !reflect.DeepEqual(got, rec) {
		t.Fatalf("unchanged layout differs:\n got %v\nwant %v", got, rec)
	}
	c := l.clone()
	c.Name, c.TabOrder, c.BlockRecord, c.Owner = "Sheet", 4, 0x77, 0x44
	c.PlotSettings[1].Value = "A4"
	got := newLayout(c.Groups())
	if got.Name != "Sheet" || got.TabOrder != 4 || got.BlockRecord != 0x77 || got.Owner != 0x44 || got.PlotSettings[1].Value != "A4" {
		t.Errorf("edited layout = %+v", got)
	}
	if len(got.Raw) != len(rec) {
		t.Errorf("edit changed the number of groups: %d, want %d", len(got.Raw), len(rec))
	}
	if l.Name != "Layout1" || l.PlotSettings[1].Value != "A3" {
		t.Error("clone shares memory with the original")
	}
	// A layout built by hand keeps its fields through Groups.
	hand := &Layout{Handle: "9", Owner: 3, Name: "X", TabOrder: 2, BlockRecord: 0x18, PlotSettings: []Group{{Code: 4, Value: "A0"}}}
	again := newLayout(hand.Groups())
	if again.Name != "X" || again.TabOrder != 2 || again.BlockRecord != 0x18 || again.Owner != 3 || again.PlotSettings[0].Value != "A0" {
		t.Errorf("hand-built layout = %+v", again)
	}
}

func TestT8LayoutsWithBlockRecordsOfAFile(t *testing.T) {
	t.Parallel()

	// A document that has the two layouts but with other handles than v00:
	// prepareR2013Tables keeps the BLOCK_RECORD handles, the 340 of the two
	// records follow the layouts.
	doc := t8EmptyDoc()
	n := Handle(100)
	tb := prepareR2013Tables(doc, func() Handle { n++; return n })
	doc.Tables = *tb.Tables
	doc.Objects = []Object{
		generatedLayout(0x300, 0x301, "Model", 0, true, tb.ModelSpace),
		generatedLayout(0x302, 0x301, "Layout1", 1, false, tb.PaperSpace),
	}
	tbl, res := t8Prepare(doc)
	if res.ModelLayout != 0x300 || res.PaperLayout != 0x302 {
		t.Fatalf("layouts = %s %s", res.ModelLayout, res.PaperLayout)
	}
	nl := 0
	for _, o := range res.Objects {
		if o.Type() == "LAYOUT" {
			nl++
		}
	}
	if nl != 2 {
		t.Errorf("LAYOUT objects = %d, want 2", nl)
	}
	br := tbl.Tables.Find("BLOCK_RECORD")
	i := findRecord(br, "BLOCK_RECORD", "*Paper_Space")
	if got := handleOf(recordText(br.Records[i], 340)); got != 0x302 {
		t.Errorf("*Paper_Space 340 = %s", got)
	}
}

func TestT8Placeholder(t *testing.T) {
	t.Parallel()

	doc := decodeT6(t, t6File(VersionR2013, "OBJECTS", "0 ACDBPLACEHOLDER\n5 5\n102 {ACAD_REACTORS\n330 4\n102 }\n330 4"))
	p, ok := doc.Objects[0].(*Placeholder)
	if !ok || p.Handle != "5" || p.Owner != 4 || p.Type() != "ACDBPLACEHOLDER" || p.ObjectOwner() != 4 {
		t.Fatalf("placeholder = %#v", doc.Objects[0])
	}
	if got, want := p.Groups(), tagList(t, "5 5\n102 {ACAD_REACTORS\n330 4\n102 }\n330 4"); !reflect.DeepEqual(got, want) {
		t.Errorf("groups = %v", got)
	}
	if obj, ok := doc.Lookup(5); !ok || obj != doc.Objects[0] {
		t.Errorf("Lookup(5) = %v %v", obj, ok)
	}
}

func TestT8RootDictionary(t *testing.T) {
	t.Parallel()

	var nilDoc *Document
	if nilDoc.RootDictionary() != nil {
		t.Error("nil document")
	}
	doc := &Document{Objects: []Object{
		&RawObject{TypeName: "XRECORD"},
		&Dictionary{Handle: "2", Owner: 1},
		&Dictionary{Handle: "3", WithDefault: true},
		&Dictionary{Handle: "1"},
	}}
	if r := doc.RootDictionary(); r == nil || r.Handle != "1" {
		t.Errorf("RootDictionary = %+v", r)
	}
	if (&Document{}).RootDictionary() != nil {
		t.Error("document without objects has a root")
	}
}

func TestT8R12IsUnaffected(t *testing.T) {
	t.Parallel()

	// R12 output has no CLASSES and no OBJECTS, even for a document that has
	// them (the conversion rule belongs to T9); the existing writer ignores
	// them and the pre-T8 behaviour of Encode is unchanged.
	doc := NewDocument()
	doc.Classes = []Class{{DXFName: "X", CppName: "Y", AppName: "Z"}}
	doc.Objects = []Object{&Dictionary{Handle: "1"}}
	var b strings.Builder
	if err := Encode(&b, doc); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"CLASSES", "OBJECTS", "AcDbDictionary"} {
		if strings.Contains(b.String(), s) {
			t.Errorf("R12 output contains %s", s)
		}
	}
}
