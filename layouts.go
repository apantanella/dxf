package dxf

import (
	"strconv"
	"strings"
)

// Markers and group codes of a LAYOUT object.
const (
	markerPlotSettings = "AcDbPlotSettings"
	markerLayout       = "AcDbLayout"
	groupLayoutName    = 1
	groupLayoutTab     = 71
	nameModelLayout    = "Model"
	nameFirstLayout    = "Layout1"
)

// Layout is a LAYOUT object: the plot settings (AcDbPlotSettings) and the
// layout proper (AcDbLayout) of model space or of a paper space.
//
// The typed fields are a view of Raw, with the rules of Class and Dictionary:
// Groups starts from Raw and writes back the fields that no longer agree with
// it, so every group the view does not know (viewports, UCS, extents,
// reactors, extended data) is kept.
type Layout struct {
	Handle string // 5, as text
	Owner  Handle // 330 before the first subclass marker: the ACAD_LAYOUT dictionary
	Name   string // 1 of AcDbLayout
	// TabOrder is group 71 of AcDbLayout; model space is 0.
	TabOrder int
	// BlockRecord is the 330 of AcDbLayout: the BLOCK_RECORD of the space the
	// layout describes.
	BlockRecord Handle
	// PlotSettings holds the groups of the AcDbPlotSettings subclass, without
	// the 100 marker, as read.
	PlotSettings []Group
	// Raw holds the groups that follow the (0, LAYOUT) pair as they were read.
	Raw []Group
}

// Type returns "LAYOUT".
func (l *Layout) Type() string { return typeLayout }

// ObjectHandle returns the numeric value of Handle.
func (l *Layout) ObjectHandle() Handle { return handleOf(l.Handle) }

// ObjectOwner returns Owner.
func (l *Layout) ObjectOwner() Handle { return l.Owner }

func (l *Layout) handleText() string { return l.Handle }

func (l *Layout) clone() *Layout {
	c := *l
	c.PlotSettings = cloneGroups(l.PlotSettings)
	c.Raw = cloneGroups(l.Raw)
	return &c
}

func newLayout(groups []Group) *Layout {
	l := &Layout{Raw: groups}
	if v, ok := topLevelText(groups, groupHandle); ok {
		l.Handle = strings.TrimSpace(v)
	}
	if i := sectionIndex(groups, 0, groupOwner); i >= 0 {
		l.Owner = handleOf(groups[i].Value)
	}
	if ps := subclassIndex(groups, markerPlotSettings); ps >= 0 {
		l.PlotSettings = cloneGroups(groups[ps+1 : sectionEnd(groups, ps+1)])
	}
	if ls := subclassIndex(groups, markerLayout); ls >= 0 {
		if i := sectionIndex(groups, ls+1, groupLayoutName); i >= 0 {
			l.Name = groups[i].Value
		}
		if i := sectionIndex(groups, ls+1, groupLayoutTab); i >= 0 {
			if n, err := strconv.Atoi(strings.TrimSpace(groups[i].Value)); err == nil {
				l.TabOrder = n
			}
		}
		if i := sectionIndex(groups, ls+1, groupOwner); i >= 0 {
			l.BlockRecord = handleOf(groups[i].Value)
		}
	}
	return l
}

func groupsEqual(a, b []Group) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Groups returns the groups that follow the (0, LAYOUT) pair, in file order.
// The result does not share memory with l.
func (l *Layout) Groups() []Group {
	if len(l.Raw) == 0 {
		return l.canonicalGroups()
	}
	g := cloneGroups(l.Raw)
	g = patchHandleText(g, groupHandle, l.Handle)
	if i := sectionIndex(g, 0, groupOwner); i >= 0 {
		if handleOf(g[i].Value) != l.Owner {
			g[i].Value = l.Owner.String()
		}
	} else if !l.Owner.IsZero() {
		g = setGroup(g, groupOwner, l.Owner.String(), groupSubclass)
	}
	if ps := subclassIndex(g, markerPlotSettings); ps >= 0 {
		end := sectionEnd(g, ps+1)
		if !groupsEqual(g[ps+1:end], l.PlotSettings) {
			tail := append(cloneGroups(l.PlotSettings), g[end:]...)
			g = append(g[:ps+1], tail...)
		}
	}
	ls := subclassIndex(g, markerLayout)
	if ls < 0 {
		return g
	}
	if i := sectionIndex(g, ls+1, groupLayoutName); i >= 0 {
		if g[i].Value != l.Name {
			g[i].Value = l.Name
		}
	} else if l.Name != "" {
		g = insertGroup(g, ls+1, Group{Code: groupLayoutName, Value: l.Name})
	}
	if i := sectionIndex(g, ls+1, groupLayoutTab); i >= 0 {
		if n, err := strconv.Atoi(strings.TrimSpace(g[i].Value)); err != nil || n != l.TabOrder {
			g[i].Value = strconv.Itoa(l.TabOrder)
		}
	} else if l.TabOrder != 0 {
		g = insertGroup(g, sectionEnd(g, ls+1), Group{Code: groupLayoutTab, Value: strconv.Itoa(l.TabOrder)})
	}
	if i := sectionIndex(g, ls+1, groupOwner); i >= 0 {
		if handleOf(g[i].Value) != l.BlockRecord {
			g[i].Value = l.BlockRecord.String()
		}
	} else if !l.BlockRecord.IsZero() {
		g = insertGroup(g, sectionEnd(g, ls+1), Group{Code: groupOwner, Value: l.BlockRecord.String()})
	}
	return g
}

func (l *Layout) canonicalGroups() []Group {
	var g []Group
	if l.Handle != "" {
		g = append(g, Group{Code: groupHandle, Value: l.Handle})
	}
	g = append(g, Group{Code: groupOwner, Value: l.Owner.String()}, Group{Code: groupSubclass, Value: markerPlotSettings})
	g = append(g, l.PlotSettings...)
	g = append(g,
		Group{Code: groupSubclass, Value: markerLayout},
		Group{Code: groupLayoutName, Value: l.Name},
		Group{Code: groupFlags, Value: "1"},
		Group{Code: groupLayoutTab, Value: strconv.Itoa(l.TabOrder)},
	)
	if !l.BlockRecord.IsZero() {
		g = append(g, Group{Code: groupOwner, Value: l.BlockRecord.String()})
	}
	return g
}

// generatedLayout returns a LAYOUT with the values of testdata/t1 v00 (A3 paper,
// 420 x 297, no viewports). model selects the plot flags of the "Model" layout.
func generatedLayout(handle, owner Handle, name string, tab int, model bool, blockRecord Handle) *Layout {
	plotFlags := "0"
	if model {
		plotFlags = "1024"
	}
	ps := []Group{
		{Code: 1, Value: ""}, {Code: 4, Value: "A3"}, {Code: 6, Value: ""},
		{Code: 40, Value: "7.5"}, {Code: 41, Value: "20.0"}, {Code: 42, Value: "7.5"}, {Code: 43, Value: "20.0"},
		{Code: 44, Value: "420.0"}, {Code: 45, Value: "297.0"},
		{Code: 46, Value: "0.0"}, {Code: 47, Value: "0.0"}, {Code: 48, Value: "0.0"}, {Code: 49, Value: "0.0"},
		{Code: 140, Value: "0.0"}, {Code: 141, Value: "0.0"}, {Code: 142, Value: "1.0"}, {Code: 143, Value: "1.0"},
		{Code: 70, Value: plotFlags}, {Code: 72, Value: "1"}, {Code: 73, Value: "0"}, {Code: 74, Value: "5"},
		{Code: 7, Value: ""}, {Code: 75, Value: "16"}, {Code: 76, Value: "0"}, {Code: 77, Value: "2"}, {Code: 78, Value: "300"},
		{Code: 147, Value: "1.0"}, {Code: 148, Value: "0.0"}, {Code: 149, Value: "0.0"},
	}
	raw := []Group{{Code: groupHandle, Value: handle.String()}, {Code: groupOwner, Value: owner.String()}, {Code: groupSubclass, Value: markerPlotSettings}}
	raw = append(raw, ps...)
	raw = append(raw,
		Group{Code: groupSubclass, Value: markerLayout},
		Group{Code: 1, Value: name}, Group{Code: 70, Value: "1"}, Group{Code: 71, Value: strconv.Itoa(tab)},
		Group{Code: 10, Value: "0.0"}, Group{Code: 20, Value: "0.0"}, Group{Code: 11, Value: "420.0"}, Group{Code: 21, Value: "297.0"},
		Group{Code: 12, Value: "0.0"}, Group{Code: 22, Value: "0.0"}, Group{Code: 32, Value: "0.0"},
		Group{Code: 14, Value: "1e+20"}, Group{Code: 24, Value: "1e+20"}, Group{Code: 34, Value: "1e+20"},
		Group{Code: 15, Value: "-1e+20"}, Group{Code: 25, Value: "-1e+20"}, Group{Code: 35, Value: "-1e+20"},
		Group{Code: 146, Value: "0.0"},
		Group{Code: 13, Value: "0.0"}, Group{Code: 23, Value: "0.0"}, Group{Code: 33, Value: "0.0"},
		Group{Code: 16, Value: "1.0"}, Group{Code: 26, Value: "0.0"}, Group{Code: 36, Value: "0.0"},
		Group{Code: 17, Value: "0.0"}, Group{Code: 27, Value: "1.0"}, Group{Code: 37, Value: "0.0"},
		Group{Code: 76, Value: "1"},
		Group{Code: groupOwner, Value: blockRecord.String()},
	)
	return newLayout(raw)
}

// r2013Objects is the result of prepareR2013Objects.
type r2013Objects struct {
	// Objects are the records of the OBJECTS section in write order. Write each
	// with its Type() and objectGroups.
	Objects []Object
	// Classes are the records of the CLASSES section in write order; write each
	// with "CLASS" and Class.Groups.
	Classes []Class
	// RootDictionary is the handle of the root dictionary.
	RootDictionary Handle
	// ModelLayout and PaperLayout are the handles of the LAYOUT objects of
	// *Model_Space and *Paper_Space.
	ModelLayout, PaperLayout Handle
	// PlotStyleNormal is the handle of the placeholder "Normal" of
	// ACAD_PLOTSTYLENAME.
	PlotStyleNormal Handle
}

// prepareR2013Objects returns the CLASSES and OBJECTS of doc ready to be
// written as R2013, and completes the prepared tables tbl:
//
//   - the objects of doc.Objects are kept in their order, unchanged (the typed
//     ones are copied, the document is not modified; a typed object without a
//     handle gets one);
//   - if the document has no root dictionary (a DICTIONARY with a zero owner)
//     one is generated and written first;
//   - the root has the entries ACAD_GROUP, ACAD_LAYOUT and ACAD_PLOTSTYLENAME;
//     those that are missing are generated, in this order, after the objects of
//     the document: the dictionary ACAD_GROUP (empty), ACAD_LAYOUT, the
//     ACDBDICTIONARYWDFLT ACAD_PLOTSTYLENAME with the placeholder "Normal", and
//     the layouts. Entries that exist are never changed, so a file that has its
//     own dictionaries gets no second one;
//   - there is a LAYOUT for *Model_Space ("Model") and one for *Paper_Space
//     ("Layout1"), generated with the values of testdata/t1 v00 when missing,
//     owned by ACAD_LAYOUT and listed in it. The 340 of the two BLOCK_RECORDs
//     of tbl is set to the handles of these layouts (SetLayoutHandles);
//   - when the placeholder "Normal" had to be generated, every LAYER record of
//     tbl without a 390 gets it;
//   - the classes of doc.Classes come first, unchanged; then the classes of the
//     generated types that the objects use (ACDBDICTIONARYWDFLT,
//     ACDBPLACEHOLDER, LAYOUT) if the document does not declare them.
//
// tbl must come from prepareR2013Tables for the same document, because the
// layouts refer to its BLOCK_RECORD handles; next must be the same counter
// (the handles in use in doc and in tbl are skipped).
func prepareR2013Objects(doc *Document, tbl *r2013Tables, next func() Handle) *r2013Objects {
	used := map[Handle]bool{}
	doc.walkHandles(func(r handleRef) bool {
		if v, err := parseHandle(r.value); err == nil && v != 0 {
			used[Handle(v)] = true
		}
		return false
	})
	for _, tb := range tbl.Tables.Items {
		collectUsedHandles(tb.Raw, used)
		for _, rec := range tb.Records {
			collectUsedHandles(rec.Groups, used)
		}
	}
	alloc := func() Handle {
		for {
			h := next()
			if h.IsZero() || used[h] {
				continue
			}
			used[h] = true
			return h
		}
	}

	objs := make([]Object, 0, len(doc.Objects))
	byHandle := map[Handle]Object{}
	dicts := map[Handle]*Dictionary{}
	var layouts []*Layout
	var root *Dictionary
	for _, o := range doc.Objects {
		switch v := o.(type) {
		case nil:
			continue
		case *Dictionary:
			o = v.clone()
		case *Layout:
			o = v.clone()
		case *Placeholder:
			o = v.clone()
		}
		switch v := o.(type) {
		case *Dictionary:
			if v.Handle == "" {
				v.Handle = alloc().String()
			}
			dicts[v.ObjectHandle()] = v
			if root == nil && !v.WithDefault && v.Owner.IsZero() {
				root = v
			}
		case *Layout:
			if v.Handle == "" {
				v.Handle = alloc().String()
			}
			layouts = append(layouts, v)
		case *Placeholder:
			if v.Handle == "" {
				v.Handle = alloc().String()
			}
		}
		if h := o.ObjectHandle(); !h.IsZero() {
			if _, dup := byHandle[h]; !dup {
				byHandle[h] = o
			}
		}
		objs = append(objs, o)
	}

	var first, gen []Object
	if root == nil {
		root = &Dictionary{Handle: alloc().String(), Cloning: 1}
		first = append(first, root)
	}
	rootH := root.ObjectHandle()

	// sub returns the dictionary the root lists under name, generating it
	// (and its entry) when the root has none.
	sub := func(name string, withDefault bool) *Dictionary {
		if h, ok := root.Lookup(name); ok {
			if d := dicts[h]; d != nil {
				return d
			}
		}
		d := &Dictionary{Handle: alloc().String(), Owner: rootH, Cloning: 1, WithDefault: withDefault}
		dicts[d.ObjectHandle()] = d
		root.Set(name, d.ObjectHandle())
		gen = append(gen, d)
		return d
	}
	sub(nameACADGroup, false)
	layoutDict := sub(nameACADLayout, false)
	plotDict := sub(nameACADPlotStyle, true)

	res := &r2013Objects{RootDictionary: rootH}

	// Plot style placeholder.
	generatedNormal := false
	if h, ok := plotDict.Lookup(namePlotStyleDefault); ok && byHandle[h] != nil {
		res.PlotStyleNormal = h
	} else {
		ph := &Placeholder{Handle: alloc().String(), Owner: plotDict.ObjectHandle()}
		res.PlotStyleNormal = ph.ObjectHandle()
		plotDict.Set(namePlotStyleDefault, res.PlotStyleNormal)
		if plotDict.WithDefault && plotDict.Default.IsZero() {
			plotDict.Default = res.PlotStyleNormal
		}
		gen = append(gen, ph)
		generatedNormal = true
	}

	// Layouts.
	var model, paper *Layout
	maxTab := 0
	for _, l := range layouts {
		if l.TabOrder > maxTab {
			maxTab = l.TabOrder
		}
		if model == nil && strings.EqualFold(strings.TrimSpace(l.Name), nameModelLayout) {
			model = l
		}
		if paper == nil && !tbl.PaperSpace.IsZero() && l.BlockRecord == tbl.PaperSpace {
			paper = l
		}
	}
	if paper == nil {
		for _, l := range layouts {
			if l != model && strings.EqualFold(strings.TrimSpace(l.Name), nameFirstLayout) {
				paper = l
				break
			}
		}
	}
	layoutDictH := layoutDict.ObjectHandle()
	if model == nil {
		model = generatedLayout(alloc(), layoutDictH, nameModelLayout, 0, true, tbl.ModelSpace)
		layoutDict.Set(nameModelLayout, model.ObjectHandle())
		gen = append(gen, model)
	}
	if paper == nil {
		tab := 1
		if maxTab >= tab {
			tab = maxTab + 1
		}
		paper = generatedLayout(alloc(), layoutDictH, nameFirstLayout, tab, false, tbl.PaperSpace)
		layoutDict.Set(nameFirstLayout, paper.ObjectHandle())
		gen = append(gen, paper)
	}
	res.ModelLayout, res.PaperLayout = model.ObjectHandle(), paper.ObjectHandle()
	tbl.SetLayoutHandles(res.ModelLayout, res.PaperLayout)

	// The layers of a library-made document use the generated "Normal".
	if generatedNormal {
		if table := tbl.Tables.Find("LAYER"); table != nil {
			for i := range table.Records {
				if topLevelIndex(table.Records[i].Groups, 390) < 0 {
					table.Records[i].Groups = setGroup(table.Records[i].Groups, 390, res.PlotStyleNormal.String())
				}
			}
		}
	}

	res.Objects = append(append(append(res.Objects, first...), objs...), gen...)
	res.Classes = r2013Classes(doc.Classes, res.Objects)
	return res
}
