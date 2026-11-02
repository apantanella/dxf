package dxf

import (
	"math"
	"sort"
	"strconv"
	"strings"
)

// VersionR12 is the $ACADVER value shared by DXF R11 and R12.
const VersionR12 = "AC1009"

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

// Document is an in-memory DXF R12 document.
type Document struct {
	Version         string
	Header          Header
	Tables          Tables
	Blocks          []*Block
	Entities        []Entity
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
