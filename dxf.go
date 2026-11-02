package dxf

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

// $ACADVER values of the DXF versions the reader accepts. Encode writes
// VersionR12 and VersionR2000 or later (R2000, R2004, R2007, R2010, R2013,
// R2018); it does not write VersionR13 and VersionR14 (ADR 0009).
const (
	// VersionR12 is the $ACADVER value shared by DXF R11 and R12.
	VersionR12 = "AC1009"
	// VersionR13 is DXF R13. It is read on a best-effort basis.
	VersionR13 = "AC1012"
	// VersionR14 is DXF R14. It is read on a best-effort basis.
	VersionR14 = "AC1014"
	// VersionR2000 is DXF R2000.
	VersionR2000 = "AC1015"
	// VersionR2004 is DXF R2004.
	VersionR2004 = "AC1018"
	// VersionR2007 is DXF R2007, the first version whose text is UTF-8.
	VersionR2007 = "AC1021"
	// VersionR2010 is DXF R2010.
	VersionR2010 = "AC1024"
	// VersionR2013 is DXF R2013.
	VersionR2013 = "AC1027"
	// VersionR2018 is DXF R2018.
	VersionR2018 = "AC1032"
)

// supportedVersions lists the versions Decode accepts, oldest first.
var supportedVersions = []string{
	VersionR12, VersionR13, VersionR14, VersionR2000, VersionR2004,
	VersionR2007, VersionR2010, VersionR2013, VersionR2018,
}

// canonicalVersion returns the canonical spelling of v when it is one of
// supportedVersions (compared case-insensitively).
func canonicalVersion(v string) (string, bool) {
	v = strings.TrimSpace(v)
	for _, s := range supportedVersions {
		if strings.EqualFold(v, s) {
			return s, true
		}
	}
	return "", false
}

// isModernVersion reports whether records of version v carry subclass markers
// (R13 and later). Unknown AC10xx versions newer than R12 count as modern.
func isModernVersion(v string) bool {
	v = strings.ToUpper(strings.TrimSpace(v))
	return strings.HasPrefix(v, "AC10") && v > VersionR12
}

// Color index values for EntityCommon.
const (
	ColorByBlock = 0
	ColorByLayer = 256
)

// Point3D is a point in DXF coordinates.
type Point3D struct {
	X float64
	Y float64
	Z float64
}

// IsZero reports whether all coordinates are zero.
func (p Point3D) IsZero() bool { return p.X == 0 && p.Y == 0 && p.Z == 0 }

// Distance returns the Euclidean distance between p and q.
func (p Point3D) Distance(q Point3D) float64 {
	dx := p.X - q.X
	dy := p.Y - q.Y
	dz := p.Z - q.Z
	return math.Sqrt(dx*dx + dy*dy + dz*dz)
}

// RawSection stores a section the package does not interpret.
type RawSection struct {
	Name   string
	Groups []Group
}

// Document is an in-memory DXF document. One type serves every version; the
// Version field tells which one it was read from.
type Document struct {
	Version  string
	Header   Header
	Tables   Tables
	Blocks   []*Block
	Entities []Entity
	// Classes holds the records of the CLASSES section (R13 and later) in file
	// order.
	Classes []Class
	// Objects holds the records of the OBJECTS section (R13 and later) in file
	// order. Objects the package does not interpret are *RawObject; DICTIONARY,
	// ACDBDICTIONARYWDFLT, ACDBPLACEHOLDER and LAYOUT are *Dictionary,
	// *Placeholder and *Layout.
	Objects         []Object
	UnknownSections []RawSection
	Diagnostics     []Diagnostic
}

// NewDocument returns an empty R12 document with the default header and tables.
func NewDocument() *Document {
	doc := &Document{Version: VersionR12, Header: Header{Variables: map[string]Variable{}}}
	doc.Header.Set("$ACADVER", 1, VersionR12)
	doc.Tables.EnsureLayer("0", 7, "CONTINUOUS")
	doc.Tables.EnsureLinetype("CONTINUOUS")
	doc.Tables.EnsureStyle("STANDARD")
	return doc
}

// NewDocumentVersion returns an empty document of the given version with the
// default header of the reference file v00 (docs/dxf-r2013.md): $INSBASE,
// $LIMMIN, $LIMMAX, $LTSCALE, $TEXTSTYLE, $CLAYER, $INSUNITS (4, millimetres),
// $MEASUREMENT (1, metric) and $DWGCODEPAGE. VersionR12 gives the same document
// as NewDocument.
//
// For the versions from R2000 on the tables, dictionaries, layouts and classes
// are not stored in the document: Encode adds the ones that are missing, so the
// empty document written as R2013 equals v00 without its three entities, apart
// from the handles. $EXTMIN and $EXTMAX are not stored either: Encode computes
// them from the entities, or writes (0,0,0) and (100,100,0) when there are none.
// A version Encode cannot write (for example VersionR14) yields a document whose
// Version is v as given, and Encode then returns an error.
func NewDocumentVersion(v string) *Document {
	if c, ok := canonicalVersion(v); ok && c == VersionR12 {
		return NewDocument()
	}
	version := v
	if c, ok := canonicalVersion(v); ok {
		version = c
	}
	doc := &Document{Version: version, Header: Header{Variables: map[string]Variable{}}}
	h := &doc.Header
	h.Set("$ACADVER", 1, version)
	h.Set("$DWGCODEPAGE", 3, defaultCodePage)
	h.Variables["$INSBASE"] = Variable{Name: "$INSBASE", Groups: []Group{{Code: 10, Value: "0.0"}, {Code: 20, Value: "0.0"}, {Code: 30, Value: "0.0"}}}
	h.Variables["$LIMMIN"] = Variable{Name: "$LIMMIN", Groups: []Group{{Code: 10, Value: "0.0"}, {Code: 20, Value: "0.0"}}}
	h.Variables["$LIMMAX"] = Variable{Name: "$LIMMAX", Groups: []Group{{Code: 10, Value: "420.0"}, {Code: 20, Value: "297.0"}}}
	h.Set("$LTSCALE", 40, "1.0")
	h.Set("$TEXTSTYLE", 7, "Standard")
	h.Set("$CLAYER", 8, "0")
	h.Set("$INSUNITS", 70, "4")
	h.Set("$MEASUREMENT", 70, "1")
	return doc
}

// Header holds the variables of the HEADER section.
type Header struct {
	Variables map[string]Variable
}

// Variable is a header variable with its group code/value pairs.
type Variable struct {
	Name   string
	Groups []Group
}

// Get returns the first value of the named variable.
func (h *Header) Get(name string) (string, bool) {
	if h == nil || h.Variables == nil {
		return "", false
	}
	v, ok := h.Variables[strings.ToUpper(name)]
	if !ok || len(v.Groups) == 0 {
		return "", false
	}
	return strings.TrimSpace(v.Groups[0].Value), true
}

// Set replaces the named variable with a single value.
func (h *Header) Set(name string, code int, value string) {
	if h.Variables == nil {
		h.Variables = map[string]Variable{}
	}
	name = strings.ToUpper(name)
	h.Variables[name] = Variable{Name: name, Groups: []Group{{Code: code, Value: value}}}
}

// Delete removes the named variable.
func (h *Header) Delete(name string) {
	if h == nil || h.Variables == nil {
		return
	}
	delete(h.Variables, strings.ToUpper(name))
}

// Names returns the variable names in alphabetical order.
func (h *Header) Names() []string {
	names := make([]string, 0, len(h.Variables))
	for name := range h.Variables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Tables holds the symbol tables of the TABLES section.
type Tables struct {
	Items []*Table
	// BlockRecords is the typed view of the BLOCK_RECORD table (see
	// LoadBlockRecords). The BLOCK_RECORD table in Items is authoritative for
	// records that exist there.
	BlockRecords []BlockRecord
}

// Table is a symbol table such as LAYER or STYLE.
type Table struct {
	Name    string
	Records []Record
	Raw     []Group
}

// Record is a single record inside a symbol table. Groups holds the pairs that
// follow the leading (0, TypeName) pair.
type Record struct {
	TypeName string
	Groups   []Group
}

// Find returns the table with the given name, or nil. The name is compared
// case-insensitively.
func (t *Tables) Find(name string) *Table {
	if t == nil {
		return nil
	}
	for _, item := range t.Items {
		if strings.EqualFold(item.Name, name) {
			return item
		}
	}
	return nil
}

// Ensure returns the table with the given name, creating it if missing.
func (t *Tables) Ensure(name string) *Table {
	if table := t.Find(name); table != nil {
		return table
	}
	table := &Table{Name: strings.ToUpper(name)}
	t.Items = append(t.Items, table)
	return table
}

// FindRecord returns the first record with the given type name.
func (t *Table) FindRecord(typeName string) (Record, bool) {
	if t == nil {
		return Record{}, false
	}
	for _, record := range t.Records {
		if strings.EqualFold(record.TypeName, typeName) {
			return record, true
		}
	}
	return Record{}, false
}

// LayerNames returns the layer names defined in the LAYER table, sorted.
func (t *Tables) LayerNames() []string {
	table := t.Find("LAYER")
	if table == nil {
		return nil
	}
	var names []string
	for _, record := range table.Records {
		if !strings.EqualFold(record.TypeName, "LAYER") {
			continue
		}
		if name, ok := recordValue(record, 2); ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// EnsureLayer adds a LAYER record when the layer is not defined yet.
func (t *Tables) EnsureLayer(name string, color int, linetype string) {
	if name == "" {
		name = "0"
	}
	table := t.Ensure("LAYER")
	if hasRecordName(table, "LAYER", name) {
		return
	}
	table.Records = append(table.Records, Record{
		TypeName: "LAYER",
		Groups: []Group{
			{Code: 2, Value: name},
			{Code: 70, Value: "0"},
			{Code: 62, Value: strconv.Itoa(color)},
			{Code: 6, Value: linetype},
		},
	})
}

// EnsureLinetype adds a linetype record when it is not defined yet. The names
// of the standard AutoCAD linetypes (DASHED, CENTER, HIDDEN, ...) get their
// usual pattern; any other name gets a solid line.
func (t *Tables) EnsureLinetype(name string) {
	if name == "" {
		name = "CONTINUOUS"
	}
	table := t.Ensure("LTYPE")
	if hasRecordName(table, "LTYPE", name) {
		return
	}
	if def, ok := standardLinetypes[strings.ToUpper(strings.TrimSpace(name))]; ok {
		table.Records = append(table.Records, linetypeRecord(strings.ToUpper(strings.TrimSpace(name)), def))
		return
	}
	table.Records = append(table.Records, Record{
		TypeName: "LTYPE",
		Groups: []Group{
			{Code: 2, Value: name},
			{Code: 70, Value: "0"},
			{Code: 3, Value: "Solid line"},
			{Code: 72, Value: "65"},
			{Code: 73, Value: "0"},
			{Code: 40, Value: "0.0"},
		},
	})
}

// EnsureStyle adds a text style record when it is not defined yet.
func (t *Tables) EnsureStyle(name string) {
	if name == "" {
		name = "STANDARD"
	}
	table := t.Ensure("STYLE")
	if hasRecordName(table, "STYLE", name) {
		return
	}
	table.Records = append(table.Records, Record{
		TypeName: "STYLE",
		Groups: []Group{
			{Code: 2, Value: name},
			{Code: 70, Value: "0"},
			{Code: 40, Value: "0.0"},
			{Code: 41, Value: "1.0"},
			{Code: 50, Value: "0.0"},
			{Code: 71, Value: "0"},
			{Code: 42, Value: "2.5"},
			{Code: 3, Value: "txt"},
			{Code: 4, Value: ""},
		},
	})
}

// EnsureAppID adds an APPID record when it is not defined yet.
func (t *Tables) EnsureAppID(name string) {
	if name == "" {
		return
	}
	table := t.Ensure("APPID")
	if hasRecordName(table, "APPID", name) {
		return
	}
	table.Records = append(table.Records, Record{
		TypeName: "APPID",
		Groups: []Group{
			{Code: 2, Value: name},
			{Code: 70, Value: "0"},
		},
	})
}

// Block is a block definition from the BLOCKS section.
type Block struct {
	Name      string
	LayerName string
	BasePoint Point3D
	Flags     int
	Entities  []Entity
	Raw       []Group
	// EndHandle is the handle of the ENDBLK record, or empty when it has none.
	EndHandle string
	// Subclasses lists the subclass markers (group code 100) of the BLOCK
	// record, in file order. It is empty for R12.
	Subclasses []string
	// Owner is the group code 330 of the BLOCK record (R13 and later).
	Owner Handle
	// RawSubclass is parallel to Raw: see EntityCommon.RawSubclass.
	RawSubclass []int
	// EndGroups holds the groups of the ENDBLK record other than its handle
	// (330, 100 markers, layer) when the file has subclass markers.
	EndGroups []Group
}

// Layer returns the layer of the block definition.
func (b *Block) Layer() string { return b.LayerName }

func hasRecordName(table *Table, typeName, name string) bool {
	for _, record := range table.Records {
		if !strings.EqualFold(record.TypeName, typeName) {
			continue
		}
		if value, ok := recordValue(record, 2); ok && strings.EqualFold(value, name) {
			return true
		}
	}
	return false
}

func recordValue(record Record, code int) (string, bool) {
	for _, group := range record.Groups {
		if group.Code == code {
			return strings.TrimSpace(group.Value), true
		}
	}
	return "", false
}
