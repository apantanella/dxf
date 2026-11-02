package dxf

import (
	"strconv"
	"strings"
)

// This file adds typed views over the records of the symbol tables and the
// preparation of the tables for an R2013 (AC1027) file.
//
// Storage does not change: every record stays a Record with its []Group, so the
// tags this package does not interpret (reactors, extended data, DIMSTYLE
// variables, ...) are never lost (ADR 0007). The typed values are read from the
// groups on demand and written back into them in place.

// Group codes used by the table records.
const (
	groupLineWeight  = 370
	groupPlot        = 290
	groupOwner       = 330
	groupSubclass    = 100
	groupDimHandle   = 105
	groupLayoutRef   = 340
	groupReactor     = 102
	groupXDataStart  = 1000
	markerSymbolRec  = "AcDbSymbolTableRecord"
	markerSymbolTbl  = "AcDbSymbolTable"
	markerDimStylTbl = "AcDbDimStyleTable"
)

// Layer is the typed view of a LAYER record.
type Layer struct {
	Name  string // 2
	Flags int    // 70
	// Color is the color index (62). A negative value means the layer is off.
	Color    int
	Linetype string // 6
	// LineWeight is the line weight in hundredths of a millimetre (370). Layer
	// reports -3 (the standard weight) when the record has no 370. SetLayer
	// writes 370 only when LineWeight is not zero or the record already has it.
	LineWeight int
	// Plot is the plot flag (290). Layer reports true when the record has no
	// 290. SetLayer writes 290 only when Plot is false or the record already
	// has it; note that the zero value of Layer therefore describes a layer
	// that does not plot.
	Plot   bool
	Handle string // 5, as text
}

// Linetype is the typed view of an LTYPE record. Pattern lists the elements
// (group 49): positive is a dash, negative a gap, zero a dot.
type Linetype struct {
	Name        string
	Description string
	Flags       int
	Pattern     []float64
	Handle      string
}

// TextStyle is the typed view of a STYLE record. WidthFactor is 1 when the
// record has no 41.
type TextStyle struct {
	Name        string  // 2
	Font        string  // 3
	BigFont     string  // 4
	Height      float64 // 40
	WidthFactor float64 // 41
	Oblique     float64 // 50
	Flags       int     // 70
	GenFlags    int     // 71
	Handle      string  // 5
}

// BlockRecord is the typed view of a BLOCK_RECORD record. Raw keeps the groups
// the view does not interpret (reactors, extended data, unknown codes) in their
// original order; the record header (5, 330 and the 100 subclass markers) is
// regenerated and not kept.
type BlockRecord struct {
	Name   string
	Handle string // 5, as text
	// Layout is the handle of the associated LAYOUT object (340); zero when
	// the record has none.
	Layout Handle
	Units  int // 70
	// Explodable is group 280; true when the record has no 280.
	Explodable bool
	// Scalable is group 281 read as a boolean (non-zero); false when absent.
	Scalable bool
	Raw      []Group
}

// ---------------------------------------------------------------------------
// Group helpers

// topLevelIndex returns the index of the first group with the given code that
// is neither inside a 102 { ... } block nor in extended data (codes >= 1000),
// or -1.
func topLevelIndex(groups []Group, code int) int {
	depth := 0
	for i, g := range groups {
		if g.Code >= groupXDataStart {
			return -1
		}
		if g.Code == groupReactor {
			v := strings.TrimSpace(g.Value)
			switch {
			case strings.HasPrefix(v, "{"):
				depth++
			case v == "}" && depth > 0:
				depth--
			}
			continue
		}
		if depth == 0 && g.Code == code {
			return i
		}
	}
	return -1
}

// topLevelText returns the value of the first top-level group with the code.
func topLevelText(groups []Group, code int) (string, bool) {
	if i := topLevelIndex(groups, code); i >= 0 {
		return groups[i].Value, true
	}
	return "", false
}

func topLevelInt(groups []Group, code, def int) int {
	if s, ok := topLevelText(groups, code); ok {
		if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			return v
		}
	}
	return def
}

func topLevelFloat(groups []Group, code int, def float64) float64 {
	if s, ok := topLevelText(groups, code); ok {
		if v, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
			return v
		}
	}
	return def
}

// topLevelFloats returns every top-level value of a repeated float code.
func topLevelFloats(groups []Group, code int) []float64 {
	var out []float64
	depth := 0
	for _, g := range groups {
		if g.Code >= groupXDataStart {
			break
		}
		if g.Code == groupReactor {
			v := strings.TrimSpace(g.Value)
			if strings.HasPrefix(v, "{") {
				depth++
			} else if v == "}" && depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 && g.Code == code {
			if v, err := strconv.ParseFloat(strings.TrimSpace(g.Value), 64); err == nil {
				out = append(out, v)
			}
		}
	}
	return out
}

// xdataStart returns the index of the first extended-data group, or len.
func xdataStart(groups []Group) int {
	for i, g := range groups {
		if g.Code >= groupXDataStart {
			return i
		}
	}
	return len(groups)
}

// insertGroup inserts g at index i.
func insertGroup(groups []Group, i int, g Group) []Group {
	groups = append(groups, Group{})
	copy(groups[i+1:], groups[i:])
	groups[i] = g
	return groups
}

// setGroup sets the first top-level group with the code. When there is none the
// group is inserted before the first top-level group whose code is in before,
// or else before the extended data (at the end when there is none).
func setGroup(groups []Group, code int, value string, before ...int) []Group {
	if i := topLevelIndex(groups, code); i >= 0 {
		groups[i].Value = value
		return groups
	}
	pos := xdataStart(groups)
	for _, b := range before {
		if i := topLevelIndex(groups, b); i >= 0 && i < pos {
			pos = i
		}
	}
	return insertGroup(groups, pos, Group{Code: code, Value: value})
}

func cloneGroups(groups []Group) []Group {
	if groups == nil {
		return nil
	}
	return append([]Group(nil), groups...)
}

// findRecord returns the index of the record of the given type and name in the
// table (name compared case-insensitively), or -1.
func findRecord(table *Table, typeName, name string) int {
	if table == nil {
		return -1
	}
	for i, record := range table.Records {
		if !strings.EqualFold(record.TypeName, typeName) {
			continue
		}
		if v, ok := recordValue(record, groupName); ok && strings.EqualFold(v, strings.TrimSpace(name)) {
			return i
		}
	}
	return -1
}

func recordText(record Record, code int) string {
	s, _ := topLevelText(record.Groups, code)
	return s
}

// ---------------------------------------------------------------------------
// Typed views

// Layer returns the layer with the given name (compared case-insensitively).
// Records and tags the view does not cover are untouched.
func (t *Tables) Layer(name string) (Layer, bool) {
	table := t.Find("LAYER")
	i := findRecord(table, "LAYER", name)
	if i < 0 {
		return Layer{}, false
	}
	g := table.Records[i].Groups
	n, _ := topLevelText(g, groupName)
	l := Layer{
		Name:       strings.TrimSpace(n),
		Flags:      topLevelInt(g, groupFlags, 0),
		Color:      topLevelInt(g, groupColor, 0),
		LineWeight: topLevelInt(g, groupLineWeight, -3),
		Plot:       topLevelInt(g, groupPlot, 1) != 0,
	}
	l.Linetype = strings.TrimSpace(recordText(table.Records[i], groupLinetype))
	l.Handle = strings.TrimSpace(recordText(table.Records[i], groupHandle))
	return l, true
}

// SetLayer writes l into the LAYER table. An existing record with the same name
// (case-insensitive) is updated in place and keeps every group the view does not
// cover; otherwise a record is appended. An empty Name means layer "0". A
// non-empty Handle replaces the record handle. The name of an existing record
// is never changed.
func (t *Tables) SetLayer(l Layer) {
	if l.Name == "" {
		l.Name = "0"
	}
	table := t.Ensure("LAYER")
	i := findRecord(table, "LAYER", l.Name)
	if i < 0 {
		table.Records = append(table.Records, Record{TypeName: "LAYER", Groups: []Group{{Code: groupName, Value: l.Name}}})
		i = len(table.Records) - 1
	}
	g := table.Records[i].Groups
	g = setGroup(g, groupFlags, strconv.Itoa(l.Flags))
	g = setGroup(g, groupColor, strconv.Itoa(l.Color))
	g = setGroup(g, groupLinetype, l.Linetype)
	if l.LineWeight != 0 || topLevelIndex(g, groupLineWeight) >= 0 {
		g = setGroup(g, groupLineWeight, strconv.Itoa(l.LineWeight), 390)
	}
	if !l.Plot || topLevelIndex(g, groupPlot) >= 0 {
		v := "1"
		if !l.Plot {
			v = "0"
		}
		g = setGroup(g, groupPlot, v, groupLineWeight, 390)
	}
	if h := strings.TrimSpace(l.Handle); h != "" {
		if j := topLevelIndex(g, groupHandle); j >= 0 {
			g[j].Value = h
		} else {
			g = insertGroup(g, 0, Group{Code: groupHandle, Value: h})
		}
	}
	table.Records[i].Groups = g
}

// Linetype returns the linetype with the given name (case-insensitive).
func (t *Tables) Linetype(name string) (Linetype, bool) {
	table := t.Find("LTYPE")
	i := findRecord(table, "LTYPE", name)
	if i < 0 {
		return Linetype{}, false
	}
	rec := table.Records[i]
	g := rec.Groups
	n, _ := topLevelText(g, groupName)
	desc, _ := topLevelText(g, groupName2)
	return Linetype{
		Name:        strings.TrimSpace(n),
		Description: desc,
		Flags:       topLevelInt(g, groupFlags, 0),
		Pattern:     topLevelFloats(g, 49),
		Handle:      strings.TrimSpace(recordText(rec, groupHandle)),
	}, true
}

// TextStyle returns the text style with the given name (case-insensitive).
func (t *Tables) TextStyle(name string) (TextStyle, bool) {
	table := t.Find("STYLE")
	i := findRecord(table, "STYLE", name)
	if i < 0 {
		return TextStyle{}, false
	}
	rec := table.Records[i]
	g := rec.Groups
	n, _ := topLevelText(g, groupName)
	font, _ := topLevelText(g, groupName2)
	big, _ := topLevelText(g, 4)
	return TextStyle{
		Name:        strings.TrimSpace(n),
		Font:        font,
		BigFont:     big,
		Height:      topLevelFloat(g, 40, 0),
		WidthFactor: topLevelFloat(g, 41, 1),
		Oblique:     topLevelFloat(g, 50, 0),
		Flags:       topLevelInt(g, groupFlags, 0),
		GenFlags:    topLevelInt(g, 71, 0),
		Handle:      strings.TrimSpace(recordText(rec, groupHandle)),
	}, true
}

// ---------------------------------------------------------------------------
// BLOCK_RECORD

// blockRecordFromRecord builds the typed view of one BLOCK_RECORD record.
func blockRecordFromRecord(rec Record) BlockRecord {
	g := rec.Groups
	name, _ := topLevelText(g, groupName)
	b := BlockRecord{
		Name:       strings.TrimSpace(name),
		Handle:     strings.TrimSpace(recordText(rec, groupHandle)),
		Units:      topLevelInt(g, groupFlags, 0),
		Explodable: topLevelInt(g, 280, 1) != 0,
		Scalable:   topLevelInt(g, 281, 0) != 0,
	}
	if s, ok := topLevelText(g, groupLayoutRef); ok {
		if h, err := ParseHandle(s); err == nil {
			b.Layout = h
		}
	}
	// Collect the groups the view does not cover. Only the first top-level
	// occurrence of an interpreted code is consumed; groups inside 102 blocks
	// and extended data always go to Raw.
	consumed := map[int]bool{}
	for _, code := range []int{groupName, groupHandle, groupOwner, groupFlags, 280, 281, groupLayoutRef} {
		if i := topLevelIndex(g, code); i >= 0 {
			consumed[i] = true
		}
	}
	for i, grp := range g {
		if consumed[i] {
			continue
		}
		if grp.Code == groupSubclass && i < xdataStart(g) {
			if v := strings.TrimSpace(grp.Value); v == markerSymbolRec || v == "AcDbBlockTableRecord" {
				continue
			}
		}
		b.Raw = append(b.Raw, grp)
	}
	return b
}

// record converts the view back to a Record. The record header is added later
// by prepareR2013Tables. 340 is written only when Layout is set.
func (b BlockRecord) record() Record {
	var g []Group
	if b.Handle != "" {
		g = append(g, Group{Code: groupHandle, Value: b.Handle})
	}
	g = append(g, Group{Code: groupName, Value: b.Name})
	if !b.Layout.IsZero() {
		g = append(g, Group{Code: groupLayoutRef, Value: b.Layout.String()})
	}
	g = append(g,
		Group{Code: groupFlags, Value: strconv.Itoa(b.Units)},
		Group{Code: 280, Value: boolDigit(b.Explodable)},
		Group{Code: 281, Value: boolDigit(b.Scalable)},
	)
	g = append(g, b.Raw...)
	return Record{TypeName: "BLOCK_RECORD", Groups: g}
}

func boolDigit(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// LoadBlockRecords fills BlockRecords from the BLOCK_RECORD table, keeping the
// groups it does not interpret in BlockRecord.Raw. It is idempotent: entries
// already in BlockRecords whose name is not in the table (records added by the
// caller and not yet written) are kept after the ones read from the table.
//
// Where it is wired: the T6 reader calls it once after the TABLES section has
// been read (R12 files have no BLOCK_RECORD table, and the call is then a
// no-op). The table stays the source of truth: prepareR2013Tables converts to
// records only the entries the table does not have.
func (t *Tables) LoadBlockRecords() {
	if t == nil {
		return
	}
	var loaded []BlockRecord
	seen := map[string]bool{}
	if table := t.Find("BLOCK_RECORD"); table != nil {
		for _, rec := range table.Records {
			if !strings.EqualFold(rec.TypeName, "BLOCK_RECORD") {
				continue
			}
			b := blockRecordFromRecord(rec)
			loaded = append(loaded, b)
			seen[strings.ToUpper(b.Name)] = true
		}
	}
	for _, b := range t.BlockRecords {
		if !seen[strings.ToUpper(b.Name)] {
			loaded = append(loaded, b)
			seen[strings.ToUpper(b.Name)] = true
		}
	}
	t.BlockRecords = loaded
}

// ---------------------------------------------------------------------------
// R2013 preparation

const (
	nameModelSpace = "*Model_Space"
	namePaperSpace = "*Paper_Space"
)

// r2013TableOrder is the order of the tables in an R2013 file.
var r2013TableOrder = []string{"VPORT", "LTYPE", "LAYER", "STYLE", "VIEW", "UCS", "APPID", "DIMSTYLE", "BLOCK_RECORD"}

// r2013RecordMarker is the 100 subclass marker of the records of each table.
var r2013RecordMarker = map[string]string{
	"VPORT":        "AcDbViewportTableRecord",
	"LTYPE":        "AcDbLinetypeTableRecord",
	"LAYER":        "AcDbLayerTableRecord",
	"STYLE":        "AcDbTextStyleTableRecord",
	"VIEW":         "AcDbViewTableRecord",
	"UCS":          "AcDbUCSTableRecord",
	"APPID":        "AcDbRegAppTableRecord",
	"DIMSTYLE":     "AcDbDimStyleTableRecord",
	"BLOCK_RECORD": "AcDbBlockTableRecord",
}

func gp(code int, value string) Group { return Group{Code: code, Value: value} }

// r2013RequiredRecords lists, per table, the records an R2013 file always has
// (R8), in file order, with the values of testdata/t1 v00. Handles, owners and
// markers are added later. The DIMSTYLE 340 (handle of STYLE Standard) is added
// when handles are known.
func r2013RequiredRecords() map[string][]Record {
	ltype := func(name, desc string) Record {
		return Record{TypeName: "LTYPE", Groups: []Group{gp(2, name), gp(70, "0"), gp(3, desc), gp(72, "65"), gp(73, "0"), gp(40, "0.0")}}
	}
	return map[string][]Record{
		"VPORT": {{TypeName: "VPORT", Groups: []Group{
			gp(2, "*Active"), gp(70, "0"),
			gp(10, "0.0"), gp(20, "0.0"), gp(11, "1.0"), gp(21, "1.0"),
			gp(12, "0.0"), gp(22, "0.0"), gp(13, "0.0"), gp(23, "0.0"),
			gp(14, "0.5"), gp(24, "0.5"), gp(15, "0.5"), gp(25, "0.5"),
			gp(16, "0.0"), gp(26, "0.0"), gp(36, "1.0"),
			gp(17, "0.0"), gp(27, "0.0"), gp(37, "0.0"),
			gp(40, "300.0"), gp(41, "1.34"), gp(42, "50.0"), gp(43, "0.0"), gp(44, "0.0"),
			gp(50, "0.0"), gp(51, "0.0"),
			gp(71, "0"), gp(72, "1000"), gp(73, "1"), gp(74, "3"),
			gp(75, "0"), gp(76, "0"), gp(77, "0"), gp(78, "0"),
			gp(281, "0"), gp(65, "0"), gp(146, "0.0"),
		}}},
		"LTYPE": {ltype("ByBlock", ""), ltype("ByLayer", ""), ltype("Continuous", "Solid line")},
		"LAYER": {{TypeName: "LAYER", Groups: []Group{gp(2, "0"), gp(70, "0"), gp(62, "7"), gp(6, "Continuous"), gp(370, "-3")}}},
		"STYLE": {{TypeName: "STYLE", Groups: []Group{
			gp(2, "Standard"), gp(70, "0"), gp(40, "0.0"), gp(41, "1.0"), gp(50, "0.0"), gp(71, "0"), gp(42, "2.5"), gp(3, "txt"), gp(4, ""),
		}}},
		"APPID":    {{TypeName: "APPID", Groups: []Group{gp(2, "ACAD"), gp(70, "0")}}},
		"DIMSTYLE": {{TypeName: "DIMSTYLE", Groups: []Group{gp(2, "Standard"), gp(70, "0")}}},
	}
}

func defaultBlockRecord(name string) Record {
	return Record{TypeName: "BLOCK_RECORD", Groups: []Group{gp(2, name), gp(70, "0"), gp(280, "1"), gp(281, "0")}}
}

// cloneTables returns a copy of src that shares no group slice with it.
func cloneTables(src *Tables) *Tables {
	out := &Tables{BlockRecords: append([]BlockRecord(nil), src.BlockRecords...)}
	for _, table := range src.Items {
		if table == nil {
			continue
		}
		c := &Table{Name: table.Name, Raw: cloneGroups(table.Raw)}
		for _, rec := range table.Records {
			c.Records = append(c.Records, Record{TypeName: rec.TypeName, Groups: cloneGroups(rec.Groups)})
		}
		out.Items = append(out.Items, c)
	}
	return out
}

// r2013Tables is the result of prepareR2013Tables.
type r2013Tables struct {
	// Tables is the prepared copy: the document's tables are never modified.
	Tables *Tables
	// TableHandle maps the upper-case table name to its handle.
	TableHandle map[string]Handle
	// BlockRecord maps the upper-case block name (including *MODEL_SPACE and
	// *PAPER_SPACE) to the handle of its BLOCK_RECORD. It is the owner (330) of
	// the BLOCK/ENDBLK records and of the entities of that block.
	BlockRecord map[string]Handle
	// ModelSpace and PaperSpace are the handles of the two fixed BLOCK_RECORDs.
	ModelSpace, PaperSpace Handle
}

// BlockRecordHandle returns the handle of the BLOCK_RECORD of the named block,
// compared case-insensitively. Use ModelSpace for the ENTITIES section.
func (r *r2013Tables) BlockRecordHandle(name string) (Handle, bool) {
	h, ok := r.BlockRecord[strings.ToUpper(strings.TrimSpace(name))]
	return h, ok
}

// SetLayoutHandles writes the 340 of the *Model_Space and *Paper_Space
// BLOCK_RECORDs (the LAYOUT objects "Model" and "Layout1"). A zero handle is
// skipped.
func (r *r2013Tables) SetLayoutHandles(model, paper Handle) {
	table := r.Tables.Find("BLOCK_RECORD")
	if table == nil {
		return
	}
	set := func(name string, h Handle) {
		if h.IsZero() {
			return
		}
		if i := findRecord(table, "BLOCK_RECORD", name); i >= 0 {
			table.Records[i].Groups = setGroup(table.Records[i].Groups, groupLayoutRef, h.String(), groupFlags, 280, 281)
		}
	}
	set(nameModelSpace, model)
	set(namePaperSpace, paper)
}

// xdataAppIDs returns the application names used by the extended data (group
// code 1001) of the document, in the order they first appear. Every name must
// have an APPID record. The scan covers entities (and the vertices and
// attributes of the composite ones), block entities, objects, table records and
// the sections kept verbatim.
func xdataAppIDs(doc *Document) []string {
	seen := map[string]bool{}
	var out []string
	add := func(groups []Group) {
		for _, g := range groups {
			if g.Code != 1001 {
				continue
			}
			name := strings.TrimSpace(g.Value)
			if name == "" || seen[strings.ToUpper(name)] {
				continue
			}
			seen[strings.ToUpper(name)] = true
			out = append(out, name)
		}
	}
	addEntities := func(entities []Entity) {
		for _, e := range entities {
			if e == nil {
				continue
			}
			if raw, ok := e.(*RawEntity); ok {
				add(raw.Groups)
			} else if c := e.Common(); c != nil {
				add(c.Raw)
			}
			switch t := e.(type) {
			case *Polyline:
				for _, v := range t.Vertices {
					add(v.Raw)
				}
			case *Insert:
				for _, a := range t.Attributes {
					add(a.Raw)
				}
			}
		}
	}
	addEntities(doc.Entities)
	for _, b := range doc.Blocks {
		if b == nil {
			continue
		}
		add(b.Raw)
		addEntities(b.Entities)
	}
	for _, o := range doc.Objects {
		add(objectGroups(o))
	}
	for _, tb := range doc.Tables.Items {
		if tb == nil {
			continue
		}
		add(tb.Raw)
		for _, rec := range tb.Records {
			add(rec.Groups)
		}
	}
	for _, sec := range doc.UnknownSections {
		add(sec.Groups)
	}
	return out
}

// prepareR2013Tables returns a copy of doc.Tables ready to be written as R2013:
//
//   - the tables VPORT, LTYPE, LAYER, STYLE, VIEW, UCS, APPID, DIMSTYLE and
//     BLOCK_RECORD exist, in this order (other tables follow);
//   - the required records are present (LTYPE ByBlock/ByLayer/Continuous, APPID
//     ACAD, LAYER 0, STYLE Standard, VPORT *Active, DIMSTYLE Standard), matched
//     by name without regard to case, so a document that has CONTINUOUS gets no
//     second Continuous. Missing records are put first in their table;
//   - the layers used by entities and blocks and the standard linetypes they
//     use are added, as in the R12 writer;
//   - an APPID record is added for every application named by extended data
//     (group code 1001) that the document does not define (see xdataAppIDs);
//   - there is a BLOCK_RECORD for *Model_Space, *Paper_Space and every Block
//     of doc.Blocks that has none (records in doc.Tables.BlockRecords that the
//     table lacks are used instead of the defaults);
//   - every table and every record has a handle, an owner (330) and the 100
//     subclass markers; DIMSTYLE records use 105 for the handle. Existing
//     handles are kept (ADR 0006), new ones come from next, skipping the
//     handles already used in the tables (105 included);
//   - the count in each table header (70, and 71 for DIMSTYLE) is correct.
//
// Records that already have the R2000+ header (a 100 AcDbSymbolTableRecord) are
// kept as they are except for a missing handle. Unknown tables are copied
// untouched. Layout handles (340 of the two fixed BLOCK_RECORDs) are not known
// yet: call SetLayoutHandles once the LAYOUT objects have handles (T8).
//
// Where it is wired: the T9 writer calls it instead of preparedTables when
// writing AC1027, passing the writer's handle counter (maxHandle(doc)+1
// onwards), writes the tables in Tables.Items order and uses BlockRecord and
// ModelSpace/PaperSpace as the 330 of BLOCK, ENDBLK and entities. It does not
// touch the document, so Encode keeps not modifying its argument.
func prepareR2013Tables(doc *Document, next func() Handle) *r2013Tables {
	out := cloneTables(&doc.Tables)

	// 1. Required records, first in their table.
	required := r2013RequiredRecords()
	dimStyleGenerated := false
	for _, name := range r2013TableOrder {
		table := out.Ensure(name)
		var missing []Record
		for _, rec := range required[name] {
			n, _ := recordValue(rec, groupName)
			if findRecord(table, rec.TypeName, n) < 0 {
				missing = append(missing, rec)
			}
		}
		table.Records = append(missing, table.Records...)
		if name == "DIMSTYLE" && len(missing) > 0 {
			dimStyleGenerated = true
		}
	}

	// 2. Layers and standard linetypes referenced by the document.
	continuous := "Continuous"
	if tb := out.Find("LTYPE"); tb != nil {
		if i := findRecord(tb, "LTYPE", "Continuous"); i >= 0 {
			continuous = strings.TrimSpace(recordText(tb.Records[i], groupName))
		}
	}
	refs := referencedLinetypes(doc)
	for _, upper := range sortedKeys(refs) {
		if _, ok := standardLinetypes[upper]; ok {
			out.EnsureLinetype(refs[upper])
		}
	}
	for _, layer := range referencedLayers(doc) {
		out.EnsureLayer(layer, 7, continuous)
	}

	// 2b. APPID records for the applications used by extended data (1001).
	// Dimensions and MLEADERs refer to ACAD and ACAD_DSTYLE_* there, and a file
	// that uses an application it does not define is rejected by some CAD
	// applications. The R12 path is left unchanged: there the caller keeps
	// calling EnsureAppID.
	for _, app := range xdataAppIDs(doc) {
		out.EnsureAppID(app)
	}

	// 3. BLOCK_RECORD records.
	ensureBlockRecordRecords(out, doc.Blocks)

	// 4. Order the tables.
	ordered := make([]*Table, 0, len(out.Items))
	placed := map[*Table]bool{}
	for _, name := range r2013TableOrder {
		if tb := out.Find(name); tb != nil && !placed[tb] {
			ordered = append(ordered, tb)
			placed[tb] = true
		}
	}
	for _, tb := range out.Items {
		if !placed[tb] {
			ordered = append(ordered, tb)
		}
	}
	out.Items = ordered

	// 5. Handles, owners, markers, counts.
	used := map[Handle]bool{}
	for _, tb := range out.Items {
		collectUsedHandles(tb.Raw, used)
		for _, rec := range tb.Records {
			collectUsedHandles(rec.Groups, used)
		}
	}
	alloc := func() Handle {
		for {
			h := next()
			if h.IsZero() {
				continue // zero is never a valid object handle
			}
			if !used[h] {
				used[h] = true
				return h
			}
		}
	}
	res := &r2013Tables{Tables: out, TableHandle: map[string]Handle{}, BlockRecord: map[string]Handle{}}
	known := func(tb *Table) (string, bool) {
		upper := strings.ToUpper(tb.Name)
		_, ok := r2013RecordMarker[upper]
		return upper, ok
	}
	for _, tb := range out.Items {
		if upper, ok := known(tb); ok {
			res.TableHandle[upper] = finalizeTableHeader(tb, upper, alloc)
		}
	}
	for _, tb := range out.Items {
		upper, ok := known(tb)
		if !ok {
			continue
		}
		handleCode := groupHandle
		if upper == "DIMSTYLE" {
			handleCode = groupDimHandle
		}
		for i := range tb.Records {
			h := finalizeRecordHeader(&tb.Records[i], handleCode, res.TableHandle[upper], r2013RecordMarker[upper], alloc)
			if upper == "BLOCK_RECORD" {
				res.BlockRecord[strings.ToUpper(strings.TrimSpace(recordText(tb.Records[i], groupName)))] = h
			}
		}
	}
	res.ModelSpace = res.BlockRecord[strings.ToUpper(nameModelSpace)]
	res.PaperSpace = res.BlockRecord[strings.ToUpper(namePaperSpace)]

	// 6. DIMSTYLE Standard points to STYLE Standard.
	if dimStyleGenerated {
		style := out.Find("STYLE")
		if si := findRecord(style, "STYLE", "Standard"); si >= 0 {
			if h := strings.TrimSpace(recordText(style.Records[si], groupHandle)); h != "" {
				dim := out.Find("DIMSTYLE")
				dim.Records[0].Groups = setGroup(dim.Records[0].Groups, groupLayoutRef, h)
			}
		}
	}

	// 7. Counts depend on the final record lists.
	for _, tb := range out.Items {
		if upper, ok := known(tb); ok {
			setTableCount(tb, upper)
		}
	}
	return res
}

// ensureBlockRecordRecords adds the missing BLOCK_RECORD records: *Model_Space
// and *Paper_Space first, then the typed entries of t.BlockRecords the table
// lacks, then one default record per block.
func ensureBlockRecordRecords(t *Tables, blocks []*Block) {
	table := t.Ensure("BLOCK_RECORD")
	have := map[string]bool{}
	for _, rec := range table.Records {
		if strings.EqualFold(rec.TypeName, "BLOCK_RECORD") {
			have[strings.ToUpper(strings.TrimSpace(recordText(rec, groupName)))] = true
		}
	}
	view := map[string]BlockRecord{}
	for _, b := range t.BlockRecords {
		view[strings.ToUpper(strings.TrimSpace(b.Name))] = b
	}
	var front, back []Record
	for _, name := range []string{nameModelSpace, namePaperSpace} {
		key := strings.ToUpper(name)
		if have[key] {
			continue
		}
		have[key] = true
		if b, ok := view[key]; ok {
			front = append(front, b.record())
		} else {
			front = append(front, defaultBlockRecord(name))
		}
	}
	for _, b := range t.BlockRecords {
		key := strings.ToUpper(strings.TrimSpace(b.Name))
		if key == "" || have[key] {
			continue
		}
		have[key] = true
		back = append(back, b.record())
	}
	for _, block := range blocks {
		if block == nil {
			continue
		}
		key := strings.ToUpper(strings.TrimSpace(block.Name))
		if key == "" || have[key] {
			continue
		}
		have[key] = true
		back = append(back, defaultBlockRecord(strings.TrimSpace(block.Name)))
	}
	table.Records = append(append(front, table.Records...), back...)
}

func collectUsedHandles(groups []Group, used map[Handle]bool) {
	for _, grp := range groups {
		if grp.Code == groupHandle || grp.Code == groupDimHandle {
			if h, err := ParseHandle(grp.Value); err == nil && !h.IsZero() {
				used[h] = true
			}
		}
	}
}

// existingHandle returns the handle stored in the first top-level group with
// code, and whether it is valid and non-zero.
func existingHandle(groups []Group, code int) (Handle, bool) {
	if s, ok := topLevelText(groups, code); ok {
		if h, err := ParseHandle(s); err == nil && !h.IsZero() {
			return h, true
		}
	}
	return 0, false
}

// finalizeTableHeader gives the table its handle, owner 0 and subclass marker
// (plus AcDbDimStyleTable for DIMSTYLE) and returns the table handle. A header
// that already has the 100 AcDbSymbolTable marker is only completed.
func finalizeTableHeader(tb *Table, upper string, alloc func() Handle) Handle {
	h, ok := existingHandle(tb.Raw, groupHandle)
	if !ok {
		h = alloc()
	}
	if hasSubclass(tb.Raw, markerSymbolTbl) {
		if i := topLevelIndex(tb.Raw, groupHandle); i >= 0 {
			tb.Raw[i].Value = h.String()
		} else {
			tb.Raw = insertGroup(tb.Raw, 0, gp(groupHandle, h.String()))
		}
		if topLevelIndex(tb.Raw, groupOwner) < 0 {
			tb.Raw = insertGroup(tb.Raw, subclassIndex(tb.Raw, markerSymbolTbl), gp(groupOwner, "0"))
		}
		if upper == "DIMSTYLE" && !hasSubclass(tb.Raw, markerDimStylTbl) {
			pos := len(tb.Raw)
			if i := topLevelIndex(tb.Raw, groupFlags); i >= 0 {
				pos = i + 1
			}
			tb.Raw = insertGroup(tb.Raw, pos, gp(groupSubclass, markerDimStylTbl))
		}
		return h
	}
	header := []Group{gp(groupHandle, h.String()), gp(groupOwner, "0"), gp(groupSubclass, markerSymbolTbl), gp(groupFlags, "0")}
	if upper == "DIMSTYLE" {
		header = append(header, gp(groupSubclass, markerDimStylTbl), gp(71, "0"))
	}
	for _, grp := range tb.Raw {
		switch grp.Code {
		case groupHandle, groupOwner, groupSubclass, groupFlags, 71:
		default:
			header = append(header, grp)
		}
	}
	tb.Raw = header
	return h
}

func hasSubclass(groups []Group, marker string) bool { return subclassIndex(groups, marker) >= 0 }

func subclassIndex(groups []Group, marker string) int {
	for i, grp := range groups {
		if grp.Code >= groupXDataStart {
			return -1
		}
		if grp.Code == groupSubclass && strings.TrimSpace(grp.Value) == marker {
			return i
		}
	}
	return -1
}

// setTableCount writes the number of records into 70 (and 71 for DIMSTYLE).
func setTableCount(tb *Table, upper string) {
	n := strconv.Itoa(len(tb.Records))
	tb.Raw = setGroup(tb.Raw, groupFlags, n)
	if upper == "DIMSTYLE" {
		tb.Raw = setGroup(tb.Raw, 71, n)
	}
}

// finalizeRecordHeader makes sure the record starts with the R2013 header
// (handle, 330 owner = table handle, 100 AcDbSymbolTableRecord, 100 marker) and
// returns its handle. A record that already has the 100 AcDbSymbolTableRecord
// marker keeps its layout and gets only a missing handle (and owner).
func finalizeRecordHeader(rec *Record, handleCode int, owner Handle, marker string, alloc func() Handle) Handle {
	groups := rec.Groups
	h, ok := existingHandle(groups, handleCode)
	if !ok {
		h, ok = existingHandle(groups, groupHandle+groupDimHandle-handleCode) // the other handle code
	}
	if hasSubclass(groups, markerSymbolRec) {
		if !ok {
			h = alloc()
		}
		if i := topLevelIndex(groups, handleCode); i >= 0 {
			groups[i].Value = h.String()
		} else {
			groups = insertGroup(groups, 0, gp(handleCode, h.String()))
		}
		if topLevelIndex(groups, groupOwner) < 0 {
			groups = insertGroup(groups, subclassIndex(groups, markerSymbolRec), gp(groupOwner, owner.String()))
		}
		if !hasSubclass(groups, marker) {
			groups = insertGroup(groups, subclassIndex(groups, markerSymbolRec)+1, gp(groupSubclass, marker))
		}
		rec.Groups = groups
		return h
	}
	if !ok {
		h = alloc()
	}
	// R12-style record: take the leading header groups out and rebuild them.
	i := 0
	var markers []string
	ownerText := owner.String()
	for i < len(groups) {
		c := groups[i].Code
		if c != groupHandle && c != groupDimHandle && c != groupOwner && c != groupSubclass {
			break
		}
		switch c {
		case groupOwner:
			ownerText = strings.TrimSpace(groups[i].Value)
		case groupSubclass:
			if v := strings.TrimSpace(groups[i].Value); v != marker {
				markers = append(markers, groups[i].Value)
			}
		}
		i++
	}
	header := []Group{gp(handleCode, h.String()), gp(groupOwner, ownerText), gp(groupSubclass, markerSymbolRec), gp(groupSubclass, marker)}
	for _, m := range markers {
		header = append(header, gp(groupSubclass, m))
	}
	rec.Groups = append(header, groups[i:]...)
	return h
}
