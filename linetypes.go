package dxf

import (
	"sort"
	"strconv"
	"strings"
)

// standardLinetype is a linetype of the AutoCAD acad.lin library. The pattern
// lists the dash lengths: positive is a dash, negative a gap and zero a dot.
type standardLinetype struct {
	description string
	pattern     []float64
}

var standardLinetypes = map[string]standardLinetype{
	"DASHED":   {"Dashed __ __ __ __ __ __ __ __ __ __ __ __ __", []float64{0.5, -0.25}},
	"HIDDEN":   {"Hidden __ __ __ __ __ __ __ __ __ __ __ __ __", []float64{0.25, -0.125}},
	"CENTER":   {"Center ____ _ ____ _ ____ _ ____ _ ____ _ ____", []float64{1.25, -0.25, 0.25, -0.25}},
	"PHANTOM":  {"Phantom ______ __ __ ______ __ __ ______", []float64{1.25, -0.25, 0.25, -0.25, 0.25, -0.25}},
	"DASHDOT":  {"Dash dot __ . __ . __ . __ . __ . __ . __ .", []float64{0.5, -0.25, 0, -0.25}},
	"DOT":      {"Dot . . . . . . . . . . . . . . . . . . . . . .", []float64{0, -0.25}},
	"DIVIDE":   {"Divide ____ . . ____ . . ____ . . ____ . .", []float64{0.5, -0.25, 0, -0.25, 0, -0.25}},
	"BORDER":   {"Border __ __ . __ __ . __ __ . __ __ . __ __ .", []float64{0.5, -0.25, 0.5, -0.25, 0, -0.25}},
	"DASHDOT2": {"Dash dot (.5x) __.__.__.__.__.__.__.__.__.__.__.", []float64{0.25, -0.125, 0, -0.125}},
}

// isBuiltinLinetypeName reports the names that never need an LTYPE record
// lookup: CONTINUOUS is always defined and BYLAYER/BYBLOCK are not linetypes.
func isBuiltinLinetypeName(upper string) bool {
	switch upper {
	case "", "CONTINUOUS", "BYLAYER", "BYBLOCK":
		return true
	}
	return false
}

// linetypeRecord builds the R12 LTYPE record of a standard linetype.
func linetypeRecord(name string, def standardLinetype) Record {
	total := 0.0
	for _, element := range def.pattern {
		if element < 0 {
			total -= element
		} else {
			total += element
		}
	}
	groups := []Group{
		{Code: 2, Value: name},
		{Code: 70, Value: "0"},
		{Code: 3, Value: def.description},
		{Code: 72, Value: "65"},
		{Code: 73, Value: strconv.Itoa(len(def.pattern))},
		{Code: 40, Value: formatFloat(total, -1)},
	}
	for _, element := range def.pattern {
		groups = append(groups, Group{Code: 49, Value: formatFloat(element, -1)})
	}
	return Record{TypeName: "LTYPE", Groups: groups}
}

// referencedLinetypes returns the linetype names used by the layers, entities
// and blocks of doc, keyed by upper-case name. The value is the name as first
// written.
func referencedLinetypes(doc *Document) map[string]string {
	refs := map[string]string{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		upper := strings.ToUpper(name)
		if isBuiltinLinetypeName(upper) {
			return
		}
		if _, ok := refs[upper]; !ok {
			refs[upper] = name
		}
	}
	if layers := doc.Tables.Find("LAYER"); layers != nil {
		for _, record := range layers.Records {
			if !strings.EqualFold(record.TypeName, "LAYER") {
				continue
			}
			if name, ok := recordValue(record, groupLinetype); ok {
				add(name)
			}
		}
	}
	visit := func(entities []Entity) {
		for _, entity := range entities {
			if entity == nil {
				continue
			}
			if common := entity.Common(); common != nil {
				add(common.Linetype)
			}
		}
	}
	visit(doc.Entities)
	for _, block := range doc.Blocks {
		if block != nil {
			visit(block.Entities)
		}
	}
	return refs
}

// sortedKeys returns the keys of m in alphabetical order.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// definedLinetype reports whether doc already has an LTYPE record named name.
func definedLinetype(doc *Document, name string) bool {
	table := doc.Tables.Find("LTYPE")
	return table != nil && hasRecordName(table, "LTYPE", name)
}

// checkLinetypes verifies that every referenced linetype is either defined in
// the LTYPE table or is a standard one the writer can define.
func checkLinetypes(doc *Document) error {
	refs := referencedLinetypes(doc)
	for _, upper := range sortedKeys(refs) {
		if definedLinetype(doc, refs[upper]) {
			continue
		}
		if _, ok := standardLinetypes[upper]; ok {
			continue
		}
		return &ValidationError{Reason: "linetype " + strconv.Quote(refs[upper]) + " is not defined in the LTYPE table"}
	}
	return nil
}
