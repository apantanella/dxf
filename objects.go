package dxf

import (
	"strconv"
	"strings"
)

// Object is a record of the OBJECTS section (R13 and later): dictionaries,
// layouts, XRECORDs and so on.
type Object interface {
	// Type returns the DXF type name, for example "DICTIONARY".
	Type() string
	// ObjectHandle returns the handle of the object, or the zero Handle when it
	// has none.
	ObjectHandle() Handle
	// ObjectOwner returns the owner (group code 330) of the object, or the zero
	// Handle when it has none.
	ObjectOwner() Handle
}

// RawObject is an object the package does not interpret. Groups holds every
// group that follows the (0, TypeName) pair, in the original order and without
// any change: handle, owner, subclass markers, reactors and data included, so
// a round trip loses nothing (ADR 0007).
//
// Handle and Owner are a read-only view of two groups of Groups. A writer must
// write Groups, not the two fields.
type RawObject struct {
	TypeName string
	// Handle is the text of group code 5, trimmed. It is empty when the object
	// has no handle.
	Handle string
	// Owner is the first group code 330 that is not inside a 102 { ... } group.
	Owner  Handle
	Groups []Group
}

// Type returns the DXF type name.
func (o *RawObject) Type() string { return o.TypeName }

// ObjectHandle returns the numeric value of Handle, or the zero Handle when it
// is missing or not valid hexadecimal.
func (o *RawObject) ObjectHandle() Handle { return handleOf(o.Handle) }

// ObjectOwner returns Owner.
func (o *RawObject) ObjectOwner() Handle { return o.Owner }

// Group codes and markers of the objects interpreted by this package.
const (
	groupDictName    = 3
	groupDictCloning = 281
	groupDictSoft    = 350
	groupDictHard    = 360
	groupDictDefault = 340

	markerDictionary     = "AcDbDictionary"
	markerDictionaryWDF  = "AcDbDictionaryWithDefault"
	typeDictionary       = "DICTIONARY"
	typeDictionaryWDFLT  = "ACDBDICTIONARYWDFLT"
	typePlaceholder      = "ACDBPLACEHOLDER"
	typeLayout           = "LAYOUT"
	nameACADGroup        = "ACAD_GROUP"
	nameACADLayout       = "ACAD_LAYOUT"
	nameACADPlotStyle    = "ACAD_PLOTSTYLENAME"
	namePlotStyleDefault = "Normal"
)

// objectGroups returns the groups that follow the (0, TypeName) pair of an
// object, ready to be written. For the objects the package interprets the
// groups are rebuilt from the typed fields (see Dictionary.Groups); for any
// other object they are the groups as read. The result may share memory with
// the object only for a RawObject.
func objectGroups(o Object) []Group {
	switch v := o.(type) {
	case *RawObject:
		return v.Groups
	case *Dictionary:
		return v.Groups()
	case *Layout:
		return v.Groups()
	case *Placeholder:
		return v.Groups()
	}
	return nil
}

// interpretObject builds the typed object for the records of OBJECTS that the
// package interprets (DICTIONARY, ACDBDICTIONARYWDFLT, ACDBPLACEHOLDER,
// LAYOUT) and a RawObject for all the others.
func interpretObject(typeName string, groups []Group) Object {
	switch strings.ToUpper(strings.TrimSpace(typeName)) {
	case typeDictionary:
		return newDictionary(false, groups)
	case typeDictionaryWDFLT:
		return newDictionary(true, groups)
	case typePlaceholder:
		return newPlaceholder(groups)
	case typeLayout:
		return newLayout(groups)
	}
	return newRawObject(typeName, groups)
}

// DictionaryEntry is one entry of a Dictionary: a name (group 3) and the
// handle of the object it refers to (350, or 360 when Hard).
type DictionaryEntry struct {
	Name  string
	Value Handle
	// Hard is true when the entry is a hard owner (360) and false for a soft
	// owner (350).
	Hard bool
}

// Dictionary is a DICTIONARY object, or an ACDBDICTIONARYWDFLT one when
// WithDefault is true. The root dictionary (the named object dictionary) is the
// one with a zero Owner: see Document.RootDictionary.
//
// The typed fields are a view of Raw, with the rules of Class: Groups starts
// from Raw (reactors, extended dictionary, unknown groups and the text of the
// handles are kept) and writes back the fields that no longer agree with it.
// A Dictionary built by hand has an empty Raw and Groups writes the canonical
// layout of testdata/t1 v00.
type Dictionary struct {
	Handle  string // 5, as text
	Owner   Handle // 330
	Cloning int    // 281
	// Entries are the 3/350 (or 3/360) pairs, in file order.
	Entries []DictionaryEntry
	// WithDefault marks an ACDBDICTIONARYWDFLT; Default is its 340, the handle
	// of the default entry (the placeholder "Normal" of ACAD_PLOTSTYLENAME).
	WithDefault bool
	Default     Handle
	// Raw holds the groups that follow the (0, type) pair as they were read.
	Raw []Group
}

// Type returns "DICTIONARY" or "ACDBDICTIONARYWDFLT".
func (d *Dictionary) Type() string {
	if d.WithDefault {
		return typeDictionaryWDFLT
	}
	return typeDictionary
}

// ObjectHandle returns the numeric value of Handle.
func (d *Dictionary) ObjectHandle() Handle { return handleOf(d.Handle) }

// ObjectOwner returns Owner.
func (d *Dictionary) ObjectOwner() Handle { return d.Owner }

func (d *Dictionary) handleText() string { return d.Handle }

// Lookup returns the handle of the entry with the given name (compared
// case-insensitively).
func (d *Dictionary) Lookup(name string) (Handle, bool) {
	if i := d.entryIndex(name); i >= 0 {
		return d.Entries[i].Value, true
	}
	return 0, false
}

// Set gives the entry with the given name the value h, adding it at the end
// when the dictionary has no such entry. The kind (350 or 360) of an existing
// entry is kept.
func (d *Dictionary) Set(name string, h Handle) {
	if i := d.entryIndex(name); i >= 0 {
		d.Entries[i].Value = h
		return
	}
	d.Entries = append(d.Entries, DictionaryEntry{Name: name, Value: h})
}

func (d *Dictionary) entryIndex(name string) int {
	for i, e := range d.Entries {
		if strings.EqualFold(e.Name, name) {
			return i
		}
	}
	return -1
}

func (d *Dictionary) clone() *Dictionary {
	c := *d
	c.Entries = append([]DictionaryEntry(nil), d.Entries...)
	c.Raw = cloneGroups(d.Raw)
	return &c
}

func newDictionary(withDefault bool, groups []Group) *Dictionary {
	d := &Dictionary{WithDefault: withDefault, Raw: groups}
	if v, ok := topLevelText(groups, groupHandle); ok {
		d.Handle = strings.TrimSpace(v)
	}
	if v, ok := topLevelText(groups, groupOwner); ok {
		d.Owner = handleOf(v)
	}
	d.Cloning = topLevelInt(groups, groupDictCloning, 0)
	d.Entries, _ = dictionaryEntries(groups)
	if mi := subclassIndex(groups, markerDictionaryWDF); mi >= 0 {
		if j := sectionIndex(groups, mi+1, groupDictDefault); j >= 0 {
			d.Default = handleOf(groups[j].Value)
		}
	}
	return d
}

// dictionaryEntries returns the entries of a dictionary record and the index of
// the name group of each (the value group follows it).
func dictionaryEntries(groups []Group) ([]DictionaryEntry, []int) {
	var entries []DictionaryEntry
	var at []int
	depth := 0
	for i := 0; i < len(groups); i++ {
		g := groups[i]
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
		if depth != 0 || g.Code != groupDictName || i+1 >= len(groups) {
			continue
		}
		next := groups[i+1]
		if next.Code != groupDictSoft && next.Code != groupDictHard {
			continue
		}
		entries = append(entries, DictionaryEntry{Name: g.Value, Value: handleOf(next.Value), Hard: next.Code == groupDictHard})
		at = append(at, i)
		i++
	}
	return entries, at
}

// sectionIndex returns the index of the first top-level group with the code,
// searching from index from up to the next 100 marker, the extended data or
// the end; -1 when there is none.
func sectionIndex(groups []Group, from, code int) int {
	depth := 0
	for i := from; i < len(groups); i++ {
		g := groups[i]
		if g.Code >= groupXDataStart {
			return -1
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
		if depth > 0 {
			continue
		}
		if g.Code == groupSubclass {
			return -1
		}
		if g.Code == code {
			return i
		}
	}
	return -1
}

// sectionEnd returns the index where the subclass section that starts at from
// ends: the next 100 marker, the extended data, or len(groups).
func sectionEnd(groups []Group, from int) int {
	for i := from; i < len(groups); i++ {
		if groups[i].Code == groupSubclass || groups[i].Code >= groupXDataStart {
			return i
		}
	}
	return len(groups)
}

// patchHandleText makes the top-level group with the code carry the handle
// text. The existing text is kept when it only differs in white space. An empty
// text removes the group.
func patchHandleText(groups []Group, code int, text string, before ...int) []Group {
	text = strings.TrimSpace(text)
	i := topLevelIndex(groups, code)
	switch {
	case i >= 0 && text == "":
		return append(groups[:i], groups[i+1:]...)
	case i >= 0:
		if strings.TrimSpace(groups[i].Value) != text {
			groups[i].Value = text
		}
		return groups
	case text == "":
		return groups
	}
	return setGroup(groups, code, text, before...)
}

// patchHandle makes the top-level group with the code carry h. A group whose
// text already parses to h is left alone. A zero h with no group writes
// nothing, unless force is true.
func patchHandle(groups []Group, code int, h Handle, force bool, before ...int) []Group {
	if i := topLevelIndex(groups, code); i >= 0 {
		if handleOf(groups[i].Value) != h {
			groups[i].Value = h.String()
		}
		return groups
	}
	if h.IsZero() && !force {
		return groups
	}
	return setGroup(groups, code, h.String(), before...)
}

// Groups returns the groups that follow the (0, type) pair, in file order. The
// result does not share memory with d.
func (d *Dictionary) Groups() []Group {
	if len(d.Raw) == 0 {
		return d.canonicalGroups()
	}
	g := cloneGroups(d.Raw)
	g = patchHandleText(g, groupHandle, d.Handle)
	g = patchHandle(g, groupOwner, d.Owner, false, groupSubclass)
	mi := subclassIndex(g, markerDictionary)
	// 281
	if i := topLevelIndex(g, groupDictCloning); i >= 0 {
		g = patchInt(g, groupDictCloning, d.Cloning)
	} else if d.Cloning != 0 {
		pos := 0
		if mi >= 0 {
			pos = mi + 1
		}
		g = insertGroup(g, pos, Group{Code: groupDictCloning, Value: strconv.Itoa(d.Cloning)})
	}
	g = d.patchEntries(g)
	if d.WithDefault {
		g = d.patchDefault(g)
	}
	return g
}

func (d *Dictionary) patchEntries(g []Group) []Group {
	have, at := dictionaryEntries(g)
	if len(have) == len(d.Entries) {
		same := true
		for i := range have {
			if have[i] != d.Entries[i] {
				same = false
				break
			}
		}
		if same {
			return g
		}
	}
	// Rebuild the entry groups, reusing the text of the original value of an
	// entry that has not changed.
	var block []Group
	for _, e := range d.Entries {
		code := groupDictSoft
		if e.Hard {
			code = groupDictHard
		}
		val := Group{Code: code, Value: e.Value.String()}
		for k, h := range have {
			if h == e {
				val = g[at[k]+1]
				break
			}
		}
		block = append(block, Group{Code: groupDictName, Value: e.Name}, val)
	}
	skip := map[int]bool{}
	for _, i := range at {
		skip[i], skip[i+1] = true, true
	}
	out := make([]Group, 0, len(g)+len(block))
	inserted := false
	for i, grp := range g {
		if skip[i] {
			if !inserted {
				out = append(out, block...)
				inserted = true
			}
			continue
		}
		out = append(out, grp)
	}
	if !inserted {
		// No entry in the record: they go at the end of the AcDbDictionary
		// section.
		pos := xdataStart(out)
		if mi := subclassIndex(out, markerDictionary); mi >= 0 {
			pos = sectionEnd(out, mi+1)
		}
		out = append(out[:pos], append(block, out[pos:]...)...)
	}
	return out
}

func (d *Dictionary) patchDefault(g []Group) []Group {
	mi := subclassIndex(g, markerDictionaryWDF)
	if mi < 0 {
		if d.Default.IsZero() {
			return g
		}
		pos := xdataStart(g)
		g = insertGroup(g, pos, Group{Code: groupSubclass, Value: markerDictionaryWDF})
		return insertGroup(g, pos+1, Group{Code: groupDictDefault, Value: d.Default.String()})
	}
	if j := sectionIndex(g, mi+1, groupDictDefault); j >= 0 {
		if handleOf(g[j].Value) != d.Default {
			g[j].Value = d.Default.String()
		}
		return g
	}
	if d.Default.IsZero() {
		return g
	}
	return insertGroup(g, mi+1, Group{Code: groupDictDefault, Value: d.Default.String()})
}

func (d *Dictionary) canonicalGroups() []Group {
	var g []Group
	if d.Handle != "" {
		g = append(g, Group{Code: groupHandle, Value: d.Handle})
	}
	g = append(g,
		Group{Code: groupOwner, Value: d.Owner.String()},
		Group{Code: groupSubclass, Value: markerDictionary},
		Group{Code: groupDictCloning, Value: strconv.Itoa(d.Cloning)},
	)
	for _, e := range d.Entries {
		code := groupDictSoft
		if e.Hard {
			code = groupDictHard
		}
		g = append(g, Group{Code: groupDictName, Value: e.Name}, Group{Code: code, Value: e.Value.String()})
	}
	if d.WithDefault {
		g = append(g, Group{Code: groupSubclass, Value: markerDictionaryWDF}, Group{Code: groupDictDefault, Value: d.Default.String()})
	}
	return g
}

// Placeholder is an ACDBPLACEHOLDER object, such as the plot style "Normal".
// It has no data of its own.
type Placeholder struct {
	Handle string // 5, as text
	Owner  Handle // 330
	// Raw holds the groups that follow the (0, ACDBPLACEHOLDER) pair as read.
	Raw []Group
}

// Type returns "ACDBPLACEHOLDER".
func (p *Placeholder) Type() string { return typePlaceholder }

// ObjectHandle returns the numeric value of Handle.
func (p *Placeholder) ObjectHandle() Handle { return handleOf(p.Handle) }

// ObjectOwner returns Owner.
func (p *Placeholder) ObjectOwner() Handle { return p.Owner }

func (p *Placeholder) handleText() string { return p.Handle }

func (p *Placeholder) clone() *Placeholder {
	c := *p
	c.Raw = cloneGroups(p.Raw)
	return &c
}

func newPlaceholder(groups []Group) *Placeholder {
	p := &Placeholder{Raw: groups}
	if v, ok := topLevelText(groups, groupHandle); ok {
		p.Handle = strings.TrimSpace(v)
	}
	if v, ok := topLevelText(groups, groupOwner); ok {
		p.Owner = handleOf(v)
	}
	return p
}

// Groups returns the groups that follow the (0, ACDBPLACEHOLDER) pair. The
// result does not share memory with p.
func (p *Placeholder) Groups() []Group {
	if len(p.Raw) == 0 {
		var g []Group
		if p.Handle != "" {
			g = append(g, Group{Code: groupHandle, Value: p.Handle})
		}
		return append(g, Group{Code: groupOwner, Value: p.Owner.String()})
	}
	g := cloneGroups(p.Raw)
	g = patchHandleText(g, groupHandle, p.Handle)
	return patchHandle(g, groupOwner, p.Owner, false, groupSubclass)
}

// RootDictionary returns the root dictionary of the OBJECTS section (the named
// object dictionary): the first DICTIONARY with a zero Owner. It returns nil
// when the document has none.
func (d *Document) RootDictionary() *Dictionary {
	if d == nil {
		return nil
	}
	for _, o := range d.Objects {
		if dict, ok := o.(*Dictionary); ok && dict != nil && !dict.WithDefault && dict.Owner.IsZero() {
			return dict
		}
	}
	return nil
}

// newRawObject builds a RawObject from the groups that follow its type record.
func newRawObject(typeName string, groups []Group) *RawObject {
	obj := &RawObject{TypeName: strings.TrimSpace(typeName), Groups: groups}
	gs := newGroupSetMode(groups, true)
	gs.prefer(prefHeader)
	if v, ok := gs.str(groupHandle); ok {
		obj.Handle = v
	}
	if h, ok := gs.owner(); ok {
		obj.Owner = h
	}
	return obj
}
