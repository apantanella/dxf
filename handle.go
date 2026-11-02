package dxf

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/apantanella/dxf/tags"
)

// Handle is the unique identifier of a DXF object (group code 5). Handles are
// written in hexadecimal. The zero Handle means "no handle": it is also what
// group code 330 uses to say "no owner".
type Handle uint64

// String returns the handle as upper-case hexadecimal digits without leading
// zeros. Handle(0) is "0".
func (h Handle) String() string {
	return strings.ToUpper(strconv.FormatUint(uint64(h), 16))
}

// IsZero reports whether h is the zero handle, which is not a valid object
// handle in a file.
func (h Handle) IsZero() bool { return h == 0 }

// ParseHandle parses a hexadecimal handle. Surrounding spaces are ignored and
// both upper and lower case digits are accepted. The value "0" is accepted and
// returns the zero Handle, because group code 330 uses it for "no owner"; an
// empty string, a prefix such as "0x", a sign or a value above 64 bits is a
// *SyntaxError (see ErrSyntax).
func ParseHandle(s string) (Handle, error) {
	trimmed := strings.TrimSpace(s)
	v, err := strconv.ParseUint(trimmed, 16, 64)
	if err != nil {
		return 0, &tags.SyntaxError{Code: groupHandle, Value: s, Reason: "invalid handle: not a 64-bit hexadecimal number"}
	}
	return Handle(v), nil
}

// HandleValue returns the numeric value of the Handle string. It returns the
// zero Handle when the handle is missing or is not valid hexadecimal. The
// Handle field is kept as a string so that the exact text read from a file
// survives a round trip (case and leading zeros included).
func (c *EntityCommon) HandleValue() Handle { return handleOf(c.Handle) }

// SetHandle stores h in the Handle field using its canonical text form. The
// zero Handle clears the field.
func (c *EntityCommon) SetHandle(h Handle) {
	if h == 0 {
		c.Handle = ""
		return
	}
	c.Handle = h.String()
}

// HandleValue returns the numeric value of the vertex Handle string, or the
// zero Handle when it is missing or invalid.
func (v Vertex) HandleValue() Handle { return handleOf(v.Handle) }

// HandleValue returns the numeric value of the attribute Handle string, or the
// zero Handle when it is missing or invalid.
func (a Attribute) HandleValue() Handle { return handleOf(a.Handle) }

func handleOf(s string) Handle {
	v, err := parseHandle(s)
	if err != nil {
		return 0
	}
	return Handle(v)
}

// handleRef is one handle found while walking a document.
type handleRef struct {
	value   string // handle text as stored
	obj     any    // object identified by the handle; nil for SEQEND/ENDBLK
	section string
	entity  string // type name used in diagnostics
	primary bool   // the handle of the object itself (not a stray group in Raw)
}

// walkHandles visits every handle stored in the document in file order: tables,
// blocks, entities, and finally the groups of sections kept raw. Handles that
// sit in preserved Raw groups are reported with primary=false: they count
// towards the highest handle in use but are not indexed or checked for
// duplicates. The walk stops as soon as fn returns true, and walkHandles then
// returns true.
func (d *Document) walkHandles(fn func(handleRef) bool) bool {
	for _, table := range d.Tables.Items {
		if table == nil {
			continue
		}
		if walkGroupHandles(table.Raw, "TABLES", "TABLE "+table.Name, table, fn) {
			return true
		}
		for i := range table.Records {
			rec := &table.Records[i]
			if walkGroupHandles(rec.Groups, "TABLES", rec.TypeName, rec, fn) {
				return true
			}
		}
	}
	for _, block := range d.Blocks {
		if block == nil {
			continue
		}
		if walkGroupHandles(block.Raw, "BLOCKS", "BLOCK", block, fn) {
			return true
		}
		if walkEntityHandles("BLOCKS", block.Entities, fn) {
			return true
		}
		if block.EndHandle != "" && fn(handleRef{value: block.EndHandle, section: "BLOCKS", entity: "ENDBLK", primary: true}) {
			return true
		}
	}
	if walkEntityHandles("ENTITIES", d.Entities, fn) {
		return true
	}
	if walkObjectHandles(d.Objects, fn) {
		return true
	}
	for _, sec := range d.UnknownSections {
		for _, g := range sec.Groups {
			if g.Code == groupHandle && fn(handleRef{value: g.Value, section: sec.Name, entity: sec.Name}) {
				return true
			}
		}
	}
	return false
}

// walkGroupHandles reports the code 5 groups of a raw group list. The first one
// belongs to obj; any further one is a stray group.
func walkGroupHandles(groups []Group, section, entity string, obj any, fn func(handleRef) bool) bool {
	first := true
	for _, g := range groups {
		if g.Code != groupHandle {
			continue
		}
		if fn(handleRef{value: g.Value, obj: obj, section: section, entity: entity, primary: first}) {
			return true
		}
		first = false
	}
	return false
}

// walkObjectHandles reports the handles of the OBJECTS section. A RawObject
// keeps its handle in Groups, so the first code 5 group is the primary handle
// and the text is reported as stored.
func walkObjectHandles(objects []Object, fn func(handleRef) bool) bool {
	for _, obj := range objects {
		if obj == nil {
			continue
		}
		if raw, ok := obj.(*RawObject); ok {
			if raw == nil {
				continue
			}
			if walkGroupHandles(raw.Groups, "OBJECTS", raw.TypeName, raw, fn) {
				return true
			}
			continue
		}
		if t, ok := obj.(interface{ handleText() string }); ok {
			// Typed objects report their handle text as stored, like a
			// RawObject, so an invalid handle is diagnosed the same way.
			if text := t.handleText(); text != "" && fn(handleRef{value: text, obj: obj, section: "OBJECTS", entity: obj.Type(), primary: true}) {
				return true
			}
			continue
		}
		if h := obj.ObjectHandle(); !h.IsZero() &&
			fn(handleRef{value: h.String(), obj: obj, section: "OBJECTS", entity: obj.Type(), primary: true}) {
			return true
		}
	}
	return false
}

func walkRawHandles(groups []Group, section, entity string, fn func(handleRef) bool) bool {
	for _, g := range groups {
		if g.Code == groupHandle && fn(handleRef{value: g.Value, section: section, entity: entity}) {
			return true
		}
	}
	return false
}

func walkEntityHandles(section string, entities []Entity, fn func(handleRef) bool) bool {
	for _, entity := range entities {
		if entity == nil {
			continue
		}
		typ := entity.Type()
		if common := entity.Common(); common != nil {
			if common.Handle != "" && fn(handleRef{value: common.Handle, obj: entity, section: section, entity: typ, primary: true}) {
				return true
			}
			if walkRawHandles(common.Raw, section, typ, fn) {
				return true
			}
		}
		switch typed := entity.(type) {
		case *RawEntity:
			if walkRawHandles(typed.Groups, section, typ, fn) {
				return true
			}
		case *Polyline:
			for i := range typed.Vertices {
				v := &typed.Vertices[i]
				if v.Handle != "" && fn(handleRef{value: v.Handle, obj: v, section: section, entity: "VERTEX", primary: true}) {
					return true
				}
				if walkRawHandles(v.Raw, section, "VERTEX", fn) {
					return true
				}
			}
			if typed.SeqendHandle != "" && fn(handleRef{value: typed.SeqendHandle, section: section, entity: "SEQEND", primary: true}) {
				return true
			}
		case *Insert:
			for i := range typed.Attributes {
				a := &typed.Attributes[i]
				if a.Handle != "" && fn(handleRef{value: a.Handle, obj: a, section: section, entity: "ATTRIB", primary: true}) {
					return true
				}
				if walkRawHandles(a.Raw, section, "ATTRIB", fn) {
					return true
				}
			}
			if typed.SeqendHandle != "" && fn(handleRef{value: typed.SeqendHandle, section: section, entity: "SEQEND", primary: true}) {
				return true
			}
		}
	}
	return false
}

// Lookup returns the object whose handle is h. The result is one of:
//
//   - an Entity (the pointer stored in Entities or in Block.Entities),
//   - *Vertex or *Attribute (pointing into the Vertices/Attributes slice of
//     the owning entity),
//   - *Block, *Table or *Record (a record of a symbol table),
//   - an Object of the OBJECTS section (*RawObject for the ones the package
//     does not interpret).
//
// The handles of SEQEND and ENDBLK records are not returned: they carry no
// data of their own. When a (malformed) document uses the same handle twice,
// the first object in file order (tables, blocks, entities) is returned.
//
// The fields of Document are public and can be changed at any time, so there is
// no cache that could become stale: every call scans the document in O(n) and
// does not allocate. To resolve many handles use HandleIndex, which builds a
// map once.
func (d *Document) Lookup(h Handle) (any, bool) {
	if d == nil || h.IsZero() {
		return nil, false
	}
	var found any
	d.walkHandles(func(r handleRef) bool {
		if !r.primary || r.obj == nil {
			return false
		}
		if v, err := parseHandle(r.value); err == nil && Handle(v) == h {
			found = r.obj
			return true
		}
		return false
	})
	return found, found != nil
}

// HandleIndex returns a snapshot that maps every handle of the document to the
// object Lookup would return. The map is not updated when the document is
// modified: call HandleIndex again after any change.
func (d *Document) HandleIndex() map[Handle]any {
	index := map[Handle]any{}
	if d == nil {
		return index
	}
	d.walkHandles(func(r handleRef) bool {
		if !r.primary || r.obj == nil {
			return false
		}
		if v, err := parseHandle(r.value); err == nil && v != 0 {
			if _, dup := index[Handle(v)]; !dup {
				index[Handle(v)] = r.obj
			}
		}
		return false
	})
	return index
}

// ValidateHandles checks the handles of the document and returns one Diagnostic
// per problem, in file order:
//
//   - SeverityError for a handle used by more than one object (the value is
//     compared numerically, so "1a" and "1A" are the same handle);
//   - SeverityWarning for a handle that is not valid hexadecimal or is zero.
//
// A missing handle is not a problem. The document is not modified.
func (d *Document) ValidateHandles() []Diagnostic {
	if d == nil {
		return nil
	}
	var diags []Diagnostic
	var seen map[Handle]handleRef
	d.walkHandles(func(r handleRef) bool {
		if !r.primary {
			return false
		}
		text := strings.TrimSpace(r.value)
		if text == "" {
			return false
		}
		v, err := parseHandle(text)
		switch {
		case err != nil:
			diags = append(diags, Diagnostic{Severity: SeverityWarning, Section: r.section, Entity: r.entity,
				Message: fmt.Sprintf("invalid handle %q: not a hexadecimal number", text)})
		case v == 0:
			diags = append(diags, Diagnostic{Severity: SeverityWarning, Section: r.section, Entity: r.entity,
				Message: "invalid handle 0: zero is not a valid object handle"})
		default:
			h := Handle(v)
			if first, dup := seen[h]; dup {
				diags = append(diags, Diagnostic{Severity: SeverityError, Section: r.section, Entity: r.entity,
					Message: fmt.Sprintf("duplicate handle %s: already used by %s in %s", h, first.entity, first.section)})
			} else {
				if seen == nil {
					seen = map[Handle]handleRef{}
				}
				seen[h] = r
			}
		}
		return false
	})
	return diags
}
