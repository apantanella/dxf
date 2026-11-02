package dxf

import (
	"strconv"
	"strings"
)

// Group codes of a CLASS record.
const (
	groupClassDXFName   = 1
	groupClassCppName   = 2
	groupClassAppName   = 3
	groupClassFlags     = 90
	groupClassInstances = 91
	groupClassWasProxy  = 280
	groupClassIsEntity  = 281
)

// defaultClassApp is the application name (group 3) of the classes the
// library generates.
const defaultClassApp = "ObjectDBX Classes"

// Class is a record of the CLASSES section (R13 and later). The classes of a
// file read by Decode are kept in Document.Classes and written back unchanged
// (R9).
//
// The typed fields are a view of Raw. When Raw is empty (a class built by
// hand) Groups writes every field; otherwise it starts from Raw, so the
// groups the view does not know, their order and the text of the numbers are
// kept, and it rewrites only the values whose typed field no longer agrees
// with Raw. A field changed by the caller therefore wins over Raw.
type Class struct {
	DXFName   string // 1
	CppName   string // 2
	AppName   string // 3
	Flags     int    // 90, proxy capabilities flags
	Instances int    // 91
	WasProxy  bool   // 280
	IsEntity  bool   // 281
	// Raw holds the groups that follow (0, CLASS) as they were read.
	Raw []Group
}

// newClass builds a Class from the groups that follow its (0, CLASS) pair.
func newClass(groups []Group) Class {
	c := Class{Raw: groups}
	if v, ok := topLevelText(groups, groupClassDXFName); ok {
		c.DXFName = v
	}
	if v, ok := topLevelText(groups, groupClassCppName); ok {
		c.CppName = v
	}
	if v, ok := topLevelText(groups, groupClassAppName); ok {
		c.AppName = v
	}
	c.Flags = topLevelInt(groups, groupClassFlags, 0)
	c.Instances = topLevelInt(groups, groupClassInstances, 0)
	c.WasProxy = topLevelInt(groups, groupClassWasProxy, 0) != 0
	c.IsEntity = topLevelInt(groups, groupClassIsEntity, 0) != 0
	return c
}

// generatedClass returns the class the library writes for objects of the given
// DXF type, with the values of testdata/t1 v00 (docs/dxf-r2013.md).
func generatedClass(dxfName, cppName string) Class {
	c := Class{DXFName: dxfName, CppName: cppName, AppName: defaultClassApp}
	c.Raw = c.Groups()
	return c
}

// Groups returns the groups of the record that follow (0, CLASS), in file
// order. The result does not share memory with c.
func (c Class) Groups() []Group {
	if len(c.Raw) == 0 {
		return []Group{
			{Code: groupClassDXFName, Value: c.DXFName},
			{Code: groupClassCppName, Value: c.CppName},
			{Code: groupClassAppName, Value: c.AppName},
			{Code: groupClassFlags, Value: strconv.Itoa(c.Flags)},
			{Code: groupClassInstances, Value: strconv.Itoa(c.Instances)},
			{Code: groupClassWasProxy, Value: boolDigit(c.WasProxy)},
			{Code: groupClassIsEntity, Value: boolDigit(c.IsEntity)},
		}
	}
	g := cloneGroups(c.Raw)
	g = patchText(g, groupClassDXFName, c.DXFName)
	g = patchText(g, groupClassCppName, c.CppName)
	g = patchText(g, groupClassAppName, c.AppName)
	g = patchInt(g, groupClassFlags, c.Flags)
	g = patchInt(g, groupClassInstances, c.Instances)
	g = patchInt(g, groupClassWasProxy, boolInt(c.WasProxy))
	g = patchInt(g, groupClassIsEntity, boolInt(c.IsEntity))
	return g
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// patchText makes the top-level group with the code carry value. A group that
// is absent is added (before the extended data) only when value is not empty.
func patchText(groups []Group, code int, value string) []Group {
	if i := topLevelIndex(groups, code); i >= 0 {
		if groups[i].Value != value {
			groups[i].Value = value
		}
		return groups
	}
	if value == "" {
		return groups
	}
	return setGroup(groups, code, value)
}

// patchInt is patchText for integers: a group whose text already parses to v
// is left alone (so "007" survives), and an absent group is added only when v
// is not zero.
func patchInt(groups []Group, code, v int) []Group {
	if i := topLevelIndex(groups, code); i >= 0 {
		if n, err := strconv.Atoi(strings.TrimSpace(groups[i].Value)); err != nil || n != v {
			groups[i].Value = strconv.Itoa(v)
		}
		return groups
	}
	if v == 0 {
		return groups
	}
	return setGroup(groups, code, strconv.Itoa(v))
}

// clone returns a deep copy of c.
func (c Class) clone() Class {
	c.Raw = cloneGroups(c.Raw)
	return c
}

// classesFromGroups converts the groups of a CLASSES section into classes. It
// reports false when the section is not a plain list of (0, CLASS) records, in
// which case the caller keeps the section as raw data.
func classesFromGroups(groups []Group) ([]Class, bool) {
	var out []Class
	for i := 0; i < len(groups); {
		if groups[i].Code != groupType || !strings.EqualFold(strings.TrimSpace(groups[i].Value), "CLASS") {
			return nil, false
		}
		j := i + 1
		for j < len(groups) && groups[j].Code != groupType {
			j++
		}
		out = append(out, newClass(groups[i+1:j:j]))
		i = j
	}
	return out, true
}

// The classes the library generates for the objects it writes, in the order of
// testdata/t1 v00.
var generatedClassTable = []struct{ dxf, cpp string }{
	{"ACDBDICTIONARYWDFLT", "AcDbDictionaryWithDefault"},
	{"ACDBPLACEHOLDER", "AcDbPlaceHolder"},
	{"LAYOUT", "AcDbLayout"},
}

// r2013Classes returns the CLASSES records for a document being written as
// R2013: the classes of the document first, unchanged and in their order (R9),
// then the classes of generatedClassTable whose object type is present in
// objects and which the document does not already declare.
func r2013Classes(existing []Class, objects []Object) []Class {
	out := make([]Class, 0, len(existing)+len(generatedClassTable))
	have := map[string]bool{}
	for _, c := range existing {
		out = append(out, c.clone())
		have[strings.ToUpper(strings.TrimSpace(c.DXFName))] = true
	}
	present := map[string]bool{}
	for _, o := range objects {
		if o != nil {
			present[strings.ToUpper(o.Type())] = true
		}
	}
	for _, g := range generatedClassTable {
		if present[g.dxf] && !have[g.dxf] {
			out = append(out, generatedClass(g.dxf, g.cpp))
		}
	}
	return out
}
