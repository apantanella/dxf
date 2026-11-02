package dxf

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/apantanella/dxf/tags"
)

// This file is the writer of the versions that have handles, owners and
// subclass markers (R2000 and later). The layout of the file is described in
// docs/dxf-r2013.md; the choices that are not obvious are in ADR 0007 and
// ADR 0009. The R12 writer stays in writer.go and is not used here.

// writableVersions lists the $ACADVER values Encode can write, oldest first.
var writableVersions = []string{
	VersionR12, VersionR2000, VersionR2004, VersionR2007, VersionR2010, VersionR2013, VersionR2018,
}

// writableVersion returns the canonical spelling of v when Encode can write it.
func writableVersion(v string) (string, bool) {
	c, ok := canonicalVersion(v)
	if !ok {
		return "", false
	}
	for _, w := range writableVersions {
		if w == c {
			return c, true
		}
	}
	return "", false
}

// writeVersionError reports a version Encode cannot write. It satisfies both
// errors.Is(err, ErrUnsupportedVersion) and errors.Is(err, ErrValidation), and
// errors.As(err, **VersionError).
type writeVersionError struct{ found string }

func (e *writeVersionError) Error() string {
	return fmt.Sprintf("dxf: cannot write version %q (writable: %s)", e.found, strings.Join(writableVersions, ", "))
}

func (e *writeVersionError) Unwrap() []error {
	return []error{&VersionError{Found: e.found}, ErrValidation}
}

// resolveWriteVersion returns the version Encode writes: WithVersion if given,
// else Document.Version, else the $ACADVER of the header, else R12.
func resolveWriteVersion(doc *Document, cfg encodeConfig) (string, error) {
	raw := strings.TrimSpace(cfg.version)
	if raw == "" {
		raw = strings.TrimSpace(doc.Version)
	}
	if raw == "" {
		if v, ok := doc.Header.Get("$ACADVER"); ok {
			raw = v
		}
	}
	if raw == "" {
		return VersionR12, nil
	}
	v, ok := writableVersion(raw)
	if !ok {
		return "", &writeVersionError{found: raw}
	}
	return v, nil
}

// defaultCodePage is the $DWGCODEPAGE written when the document has none: the
// value of the reference files of T1.
const defaultCodePage = "ANSI_1252"

// formatFloatModern formats a real the way the reference files of T1 do (the
// repr of a Python float): the shortest text that reads back to the same value,
// always with a decimal point, and an exponent ("1e+20") only below 1e-4 or from
// 1e16 up. A negative precision selects that form; a precision of zero or more
// writes that many digits after the point. Zero is "0.0".
func formatFloatModern(value float64, precision int) string {
	if value == 0 {
		return "0.0"
	}
	if precision >= 0 {
		return strconv.FormatFloat(value, 'f', precision, 64)
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return strconv.FormatFloat(value, 'g', -1, 64)
	}
	var buf [32]byte
	e := strconv.AppendFloat(buf[:0], value, 'e', -1, 64)
	idx := -1
	for i := len(e) - 1; i >= 0; i-- {
		if e[i] == 'e' {
			idx = i
			break
		}
	}
	exp, _ := strconv.Atoi(string(e[idx+1:]))
	if exp < -4 || exp >= 16 {
		return string(e)
	}
	f := strconv.AppendFloat(buf[:0], value, 'f', -1, 64)
	for _, c := range f {
		if c == '.' {
			return string(f)
		}
	}
	return string(append(f, '.', '0'))
}

// modernWriter writes one document as R2000 or later.
type modernWriter struct {
	tw        *tags.Writer
	cfg       encodeConfig
	err       error
	version   string
	enc       tags.Encoding
	transcode bool
	next      uint64
	tbl       *r2013Tables
	objs      *r2013Objects
	// blockRec holds the handles of the BLOCK_RECORD records, to tell whether
	// the owner an entity carries is a block.
	blockRec map[Handle]bool
}

func (mw *modernWriter) group(code int, value string) {
	if mw.err != nil {
		return
	}
	if mw.transcode && value != "" && !isASCIIString(value) && utf8.ValidString(value) {
		value = mw.enc.Encode(value)
	}
	if err := mw.tw.Write(Group{Code: code, Value: value}); err != nil {
		mw.err = err
	}
}

func isASCIIString(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func (mw *modernWriter) groupInt(code, value int) { mw.group(code, strconv.Itoa(value)) }

func (mw *modernWriter) groupFloat(code int, value float64) {
	mw.group(code, formatFloatModern(value, mw.cfg.precision))
}

func (mw *modernWriter) point(code int, p Point3D) {
	mw.groupFloat(code, p.X)
	mw.groupFloat(code+10, p.Y)
	mw.groupFloat(code+20, p.Z)
}

func (mw *modernWriter) alloc() Handle {
	h := mw.next
	mw.next++
	return Handle(h)
}

func (mw *modernWriter) extrusion(c *EntityCommon) {
	if c != nil && c.ExtrusionSet {
		mw.point(groupExtrusionX, c.Extrusion)
	}
}

func (mw *modernWriter) thickness(c *EntityCommon) {
	if c != nil && c.Thickness != 0 {
		mw.groupFloat(groupThickness, c.Thickness)
	}
}

// encodeModern writes doc as the (already resolved) version, R2000 or later.
func encodeModern(w io.Writer, doc *Document, cfg encodeConfig, version string) error {
	if err := validateDocument(doc); err != nil {
		return err
	}
	if err := validateModernContent(doc, version); err != nil {
		return err
	}
	mw := &modernWriter{
		tw:      tags.NewWriter(w, tags.WithCRLF(), tags.WithPaddedCodes()),
		cfg:     cfg,
		version: version,
		enc:     tags.EncodingUTF8,
	}
	cp := defaultCodePage
	if v, ok := doc.Header.Get("$DWGCODEPAGE"); ok && v != "" {
		cp = v
	}
	if version < VersionR2007 {
		if enc, ok := tags.EncodingFromCodePage(cp); ok && enc != tags.EncodingUTF8 {
			mw.enc, mw.transcode = enc, true
		}
	}

	// Handles. New ones start above every handle in use and above the seed the
	// document already declares (ADR 0006).
	start := modernMaxHandle(doc) + 1
	if v, ok := doc.Header.Get("$HANDSEED"); ok {
		if seed, err := parseHandle(v); err == nil && seed > start {
			start = seed
		}
	}
	mw.next = start
	nextHandle := func() Handle { return mw.alloc() }
	mw.tbl = prepareR2013Tables(doc, nextHandle)
	mw.objs = prepareR2013Objects(doc, mw.tbl, nextHandle)
	mw.blockRec = map[Handle]bool{}
	for _, h := range mw.tbl.BlockRecord {
		mw.blockRec[h] = true
	}
	need := modernHandlesNeeded(doc)
	if mw.next > math.MaxUint64-need-(1<<20) {
		return &ValidationError{Reason: "handle space exhausted: cannot assign handles above the highest one in use"}
	}
	seed := mw.next + need

	mw.writeHeader(doc, seed, cp)
	mw.writeClasses(doc)
	mw.writeTables(doc)
	mw.writeBlocks(doc)
	mw.writeEntitiesSection(doc)
	mw.writeObjects()
	mw.writeUnknownSections(doc)
	mw.group(groupType, "EOF")
	if mw.err != nil {
		return mw.err
	}
	if mw.next != seed {
		// The count in modernHandlesNeeded and the writer disagree: the
		// HANDSEED already written would be wrong.
		return &ValidationError{Reason: fmt.Sprintf("internal error: handle count mismatch (expected %d, used %d)", seed, mw.next)}
	}
	return mw.tw.Flush()
}

// modernMaxHandle returns the highest handle in use in doc: maxHandle plus the
// 105 handles of the DIMSTYLE records, which the handle walk does not see.
func modernMaxHandle(doc *Document) uint64 {
	max := maxHandle(doc)
	for _, tb := range doc.Tables.Items {
		if tb == nil {
			continue
		}
		for _, rec := range tb.Records {
			for _, g := range rec.Groups {
				if g.Code == groupDimHandle {
					if v, err := parseHandle(g.Value); err == nil && v > max {
						max = v
					}
				}
			}
		}
	}
	return max
}

// modernHandlesNeeded counts the handles the writer assigns to blocks and
// entities that have none: BLOCK, ENDBLK, entities, VERTEX, ATTRIB and SEQEND.
func modernHandlesNeeded(doc *Document) uint64 {
	var n uint64
	hasModel, hasPaper := false, false
	for _, b := range doc.Blocks {
		if b == nil {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(b.Name)) {
		case "*MODEL_SPACE":
			hasModel = true
		case "*PAPER_SPACE":
			hasPaper = true
		}
		if h, _, _ := splitBlockRaw(b); handleMissing(h) {
			n++
		}
		if handleMissing(b.EndHandle) {
			n++
		}
		n += entitiesHandlesNeeded(b.Entities)
	}
	if !hasModel {
		n += 2
	}
	if !hasPaper {
		n += 2
	}
	return n + entitiesHandlesNeeded(doc.Entities)
}

// handleMissing reports whether the writer must assign a handle to a record
// whose handle text is text: empty, not hexadecimal or zero.
func handleMissing(text string) bool {
	if strings.TrimSpace(text) == "" {
		return true
	}
	h, err := parseHandle(text)
	return err != nil || h == 0
}

func entitiesHandlesNeeded(entities []Entity) uint64 {
	var n uint64
	for _, e := range entities {
		switch t := e.(type) {
		case nil:
		case *RawEntity:
			// Written verbatim: it keeps whatever handle it has.
		case *Polyline:
			if handleMissing(t.Handle) {
				n++
			}
			for i := range t.Vertices {
				if handleMissing(t.Vertices[i].Handle) {
					n++
				}
			}
			if handleMissing(t.SeqendHandle) {
				n++
			}
		case *Insert:
			if handleMissing(t.Handle) {
				n++
			}
			for i := range t.Attributes {
				if handleMissing(t.Attributes[i].Handle) {
					n++
				}
			}
			if len(t.Attributes) > 0 && handleMissing(t.SeqendHandle) {
				n++
			}
		default:
			if c := e.Common(); c == nil || handleMissing(c.Handle) {
				n++
			}
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// HEADER

// modernHeaderOrder is the order of the first variables of the file: the one of
// the reference file v00. The other variables follow in alphabetical order.
var modernHeaderOrder = []string{
	"$ACADVER", "$HANDSEED", "$DWGCODEPAGE", "$INSBASE", "$EXTMIN", "$EXTMAX",
	"$LIMMIN", "$LIMMAX", "$LTSCALE", "$TEXTSTYLE", "$CLAYER", "$INSUNITS", "$MEASUREMENT",
}

func (mw *modernWriter) writeHeader(doc *Document, seed uint64, codePage string) {
	mw.group(groupType, "SECTION")
	mw.group(groupName, "HEADER")
	vars := doc.Header.Variables
	fixed := map[string]bool{}
	for _, n := range modernHeaderOrder {
		fixed[n] = true
	}
	for _, name := range modernHeaderOrder {
		switch name {
		case "$ACADVER":
			mw.variable(name, Group{Code: groupText, Value: mw.version})
		case "$HANDSEED":
			mw.variable(name, Group{Code: groupHandle, Value: strings.ToUpper(strconv.FormatUint(seed, 16))})
		case "$DWGCODEPAGE":
			mw.variable(name, Group{Code: groupName2, Value: codePage})
		case "$EXTMIN", "$EXTMAX":
			if v, ok := vars[name]; ok {
				mw.variableGroups(name, v.Groups)
				continue
			}
			min, max := mw.extents(doc)
			p := min
			if name == "$EXTMAX" {
				p = max
			}
			mw.variable(name, Group{Code: groupX, Value: formatFloatModern(p.X, mw.cfg.precision)},
				Group{Code: groupY, Value: formatFloatModern(p.Y, mw.cfg.precision)},
				Group{Code: groupZ, Value: formatFloatModern(p.Z, mw.cfg.precision)})
		default:
			if v, ok := vars[name]; ok {
				mw.variableGroups(name, v.Groups)
			}
		}
	}
	var rest []string
	for name := range vars {
		// $HANDLING exists only in R12 files: handles are always on from R13.
		if !fixed[name] && name != "$HANDLING" {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	for _, name := range rest {
		mw.variableGroups(name, vars[name].Groups)
	}
	mw.group(groupType, "ENDSEC")
}

func (mw *modernWriter) variable(name string, groups ...Group) { mw.variableGroups(name, groups) }

func (mw *modernWriter) variableGroups(name string, groups []Group) {
	mw.group(groupVariable, name)
	for _, g := range groups {
		mw.group(g.Code, g.Value)
	}
}

// extents returns $EXTMIN and $EXTMAX for a document that does not define
// them: the extents of the entities, or the box of the reference file v00
// ((0,0,0) to (100,100,0)) when there is nothing to measure.
func (mw *modernWriter) extents(doc *Document) (min, max Point3D) {
	if lo, hi, ok := doc.Extents(); ok {
		return lo, hi
	}
	return Point3D{}, Point3D{X: 100, Y: 100}
}

// ---------------------------------------------------------------------------
// CLASSES, TABLES, OBJECTS, unknown sections

func (mw *modernWriter) writeClasses(doc *Document) {
	// A CLASSES section the reader could not interpret is kept as it is.
	for _, sec := range doc.UnknownSections {
		if strings.EqualFold(sec.Name, "CLASSES") {
			mw.writeRawSection(sec)
			return
		}
	}
	mw.group(groupType, "SECTION")
	mw.group(groupName, "CLASSES")
	for _, c := range mw.objs.Classes {
		mw.group(groupType, "CLASS")
		for _, g := range c.Groups() {
			mw.group(g.Code, g.Value)
		}
	}
	mw.group(groupType, "ENDSEC")
}

func (mw *modernWriter) writeTables(doc *Document) {
	mw.group(groupType, "SECTION")
	mw.group(groupName, "TABLES")
	// prepareR2013Tables always gives the DIMSTYLE table header a 71 (the
	// record count, as in v00). A table read from a file that had none (ezdxf)
	// is written without it, so that the tag sequence of the file is kept
	// (ADR 0005).
	drop71 := false
	if orig := doc.Tables.Find("DIMSTYLE"); orig != nil && topLevelIndex(orig.Raw, 71) < 0 {
		drop71 = true
	}
	for _, tb := range mw.tbl.Tables.Items {
		mw.group(groupType, "TABLE")
		mw.group(groupName, tb.Name)
		for _, g := range tb.Raw {
			if drop71 && g.Code == 71 && strings.EqualFold(tb.Name, "DIMSTYLE") {
				continue
			}
			mw.group(g.Code, g.Value)
		}
		for _, rec := range tb.Records {
			mw.group(groupType, rec.TypeName)
			for _, g := range rec.Groups {
				mw.group(g.Code, g.Value)
			}
		}
		mw.group(groupType, "ENDTAB")
	}
	mw.group(groupType, "ENDSEC")
}

func (mw *modernWriter) writeObjects() {
	mw.group(groupType, "SECTION")
	mw.group(groupName, "OBJECTS")
	for _, o := range mw.objs.Objects {
		mw.group(groupType, o.Type())
		for _, g := range objectGroups(o) {
			mw.group(g.Code, g.Value)
		}
	}
	mw.group(groupType, "ENDSEC")
}

func (mw *modernWriter) writeRawSection(sec RawSection) {
	mw.group(groupType, "SECTION")
	mw.group(groupName, sec.Name)
	for _, g := range sec.Groups {
		mw.group(g.Code, g.Value)
	}
	mw.group(groupType, "ENDSEC")
}

// writeUnknownSections writes the sections the reader keeps verbatim
// (THUMBNAILIMAGE, ACDSDATA, ...) after OBJECTS, in document order.
func (mw *modernWriter) writeUnknownSections(doc *Document) {
	for _, sec := range doc.UnknownSections {
		switch strings.ToUpper(sec.Name) {
		case "HEADER", "CLASSES", "TABLES", "BLOCKS", "ENTITIES", "OBJECTS":
			continue
		}
		mw.writeRawSection(sec)
	}
}

// ---------------------------------------------------------------------------
// Records

// rec describes one record with subclass markers: the groups before the first
// marker, the markers, and the groups of each subclass.
type rec struct {
	typ    string
	handle string // text of group 5; empty writes none
	owner  Handle // group 330, written when not zero
	subs   []string
	// common is the EntityCommon of the record. When set (or when layer is
	// not empty) the AcDbEntity subclass is written from it.
	common *EntityCommon
	layer  string // layer of records that have no EntityCommon
	paper  bool   // write 67 1 when raw has none
	raw    []Group
	rawSub []int
	// after, when set, is called after the raw groups of each subclass.
	after func(sub string, occ int)
}

// entityLevelCodes are the codes that belong to AcDbEntity whatever the
// record says: used to place the raw groups of a record whose subclass
// positions are unknown (read from R12, or built by hand).
var entityLevelCodes = map[int]bool{67: true, 410: true}

// writeRecord writes r. body is called after each subclass marker with the
// marker and the number of times the same marker was already written.
func (mw *modernWriter) writeRecord(r rec, body func(sub string, occ int)) {
	mw.group(groupType, r.typ)
	if r.handle != "" {
		mw.group(groupHandle, r.handle)
	}
	n := len(r.subs)
	var slots []int
	if len(r.raw) > 0 {
		slots = make([]int, len(r.raw))
		known := len(r.rawSub) == len(r.raw)
		entityIdx := 0
		for i, s := range r.subs {
			if s == "AcDbEntity" {
				entityIdx = i + 1
				break
			}
		}
		for i, g := range r.raw {
			switch {
			case known:
				slots[i] = r.rawSub[i]
			case entityLevelCodes[g.Code] && entityIdx > 0:
				slots[i] = entityIdx
			default:
				slots[i] = n
			}
			if slots[i] > n {
				slots[i] = n
			}
		}
	}
	emit := func(slot int, only func(Group) bool) {
		for i, g := range r.raw {
			if slots[i] == slot && (only == nil || only(g)) {
				mw.group(g.Code, g.Value)
			}
		}
	}
	emit(0, nil)
	if !r.owner.IsZero() {
		mw.group(groupOwnerHandle, r.owner.String())
	}
	for k, s := range r.subs {
		mw.group(groupSubclassMarker, s)
		occ := 0 // how many times the same marker was written before
		for _, earlier := range r.subs[:k] {
			if earlier == s {
				occ++
			}
		}
		hasLayerRaw := false
		if s == "AcDbEntity" {
			if len(r.raw) > 0 {
				emit(k+1, func(g Group) bool { return entityLevelCodes[g.Code] })
				for i, g := range r.raw {
					if g.Code == groupLayer && slots[i] == k+1 {
						hasLayerRaw = true
					}
				}
			}
			if r.paper && !rawHas(r.raw, 67) {
				mw.groupInt(67, 1)
			}
			if !hasLayerRaw {
				layer := r.layer
				if r.common != nil {
					layer = r.common.LayerName
				}
				mw.group(groupLayer, layerOrZero(layer))
			}
			if c := r.common; c != nil {
				if c.Linetype != "" {
					mw.group(groupLinetype, c.Linetype)
				}
				if c.HasColor {
					mw.groupInt(groupColor, c.Color)
				}
				if c.Invisible {
					mw.groupInt(groupInvisible, 1)
				}
			}
		} else if body != nil {
			body(s, occ)
		}
		if len(r.raw) > 0 {
			if s == "AcDbEntity" {
				emit(k+1, func(g Group) bool { return !entityLevelCodes[g.Code] })
			} else {
				emit(k+1, nil)
			}
		}
		if r.after != nil {
			r.after(s, occ)
		}
	}
}

func rawHas(groups []Group, code int) bool {
	for _, g := range groups {
		if g.Code == code {
			return true
		}
	}
	return false
}

// pickSubs returns the subclass markers to write: the ones read from the file
// when they have at least the expected shape, otherwise the defaults.
func pickSubs(read, def []string) []string {
	if len(read) >= len(def) && len(read) > 0 && len(def) > 0 && read[0] == def[0] {
		return read
	}
	return def
}

func hasPaperFlag(raw []Group) bool {
	for _, g := range raw {
		if g.Code == 67 && strings.TrimSpace(g.Value) == "1" {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// BLOCKS

// splitBlockRaw returns the handle of a BLOCK record (the first top-level
// group 5 of Raw) and the remaining groups with their subclass positions.
func splitBlockRaw(b *Block) (handle string, raw []Group, rawSub []int) {
	idx := -1
	depth := 0
	for i, g := range b.Raw {
		if g.Code == groupControlBrace {
			v := strings.TrimSpace(g.Value)
			if strings.HasPrefix(v, "{") {
				depth++
			} else if v == "}" && depth > 0 {
				depth--
			}
			continue
		}
		if depth == 0 && g.Code == groupHandle {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", b.Raw, b.RawSubclass
	}
	handle = strings.TrimSpace(b.Raw[idx].Value)
	raw = make([]Group, 0, len(b.Raw)-1)
	raw = append(raw, b.Raw[:idx]...)
	raw = append(raw, b.Raw[idx+1:]...)
	if len(b.RawSubclass) == len(b.Raw) {
		rawSub = make([]int, 0, len(raw))
		rawSub = append(rawSub, b.RawSubclass[:idx]...)
		rawSub = append(rawSub, b.RawSubclass[idx+1:]...)
	}
	return handle, raw, rawSub
}

func isPaperBlockName(name string) bool {
	u := strings.ToUpper(strings.TrimSpace(name))
	if !strings.HasPrefix(u, "*PAPER_SPACE") {
		return false
	}
	for _, c := range u[len("*PAPER_SPACE"):] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (mw *modernWriter) writeBlocks(doc *Document) {
	mw.group(groupType, "SECTION")
	mw.group(groupName, "BLOCKS")
	blocks := doc.Blocks
	hasModel, hasPaper := false, false
	for _, b := range blocks {
		if b == nil {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(b.Name)) {
		case "*MODEL_SPACE":
			hasModel = true
		case "*PAPER_SPACE":
			hasPaper = true
		}
	}
	var lead []*Block
	if !hasModel {
		lead = append(lead, &Block{Name: nameModelSpace})
	}
	if !hasPaper {
		lead = append(lead, &Block{Name: namePaperSpace})
	}
	for _, b := range append(lead, blocks...) {
		if b != nil {
			mw.writeBlock(b)
		}
	}
	mw.group(groupType, "ENDSEC")
}

func (mw *modernWriter) writeBlock(b *Block) {
	recH, _ := mw.tbl.BlockRecordHandle(b.Name)
	paper := isPaperBlockName(b.Name)
	handleText, raw, rawSub := splitBlockRaw(b)
	if handleMissing(handleText) {
		handleText = mw.alloc().String()
	}
	subs := pickSubs(b.Subclasses, []string{"AcDbEntity", "AcDbBlockBegin"})
	generated := len(b.Subclasses) == 0
	// The layer of the BLOCK record is the Block layer; the 8 of a record
	// read from a file sits in LayerName, not in Raw.
	mw.writeRecord(rec{
		typ: "BLOCK", handle: handleText, owner: recH, subs: subs,
		layer: b.LayerName, paper: paper && generated, raw: raw, rawSub: rawSub,
	}, func(sub string, occ int) {
		if sub != "AcDbBlockBegin" {
			return
		}
		mw.group(groupName, b.Name)
		mw.groupInt(groupFlags, b.Flags)
		mw.point(groupX, b.BasePoint)
		mw.group(groupName2, b.Name)
		if generated && !rawHas(raw, 1) {
			mw.group(groupText, "")
		}
	})
	for _, e := range b.Entities {
		mw.writeEntity(e, recH, true)
	}
	// ENDBLK
	endH := strings.TrimSpace(b.EndHandle)
	if handleMissing(endH) {
		endH = mw.alloc().String()
	}
	mw.group(groupType, "ENDBLK")
	mw.group(groupHandle, endH)
	if len(b.EndGroups) > 0 {
		mw.writeOwned(b.EndGroups, recH, nil)
		return
	}
	mw.group(groupOwnerHandle, recH.String())
	mw.group(groupSubclassMarker, "AcDbEntity")
	if paper {
		mw.groupInt(67, 1)
	}
	mw.group(groupLayer, layerOrZero(b.LayerName))
	mw.group(groupSubclassMarker, "AcDbBlockEnd")
}

// writeOwned writes groups read from the file with the first top-level 330
// set to owner (added before the first marker when there is none). When accept
// is not nil and the 330 already there satisfies it, it is kept.
func (mw *modernWriter) writeOwned(groups []Group, owner Handle, accept func(Handle) bool) {
	depth := 0
	done := false
	for _, g := range groups {
		if g.Code == groupControlBrace {
			v := strings.TrimSpace(g.Value)
			if strings.HasPrefix(v, "{") {
				depth++
			} else if v == "}" && depth > 0 {
				depth--
			}
		}
		if !done && depth == 0 && g.Code == groupSubclassMarker {
			mw.group(groupOwnerHandle, owner.String())
			done = true
		}
		if !done && depth == 0 && g.Code == groupOwnerHandle {
			done = true
			if accept != nil {
				if h, err := parseHandle(g.Value); err == nil && accept(Handle(h)) {
					mw.group(g.Code, g.Value)
					continue
				}
			}
			mw.group(groupOwnerHandle, owner.String())
			continue
		}
		mw.group(g.Code, g.Value)
	}
	if !done {
		mw.group(groupOwnerHandle, owner.String())
	}
}

// childOwner returns the owner to write for a VERTEX, ATTRIB or SEQEND of the
// record parent. DXF writers disagree on it: this package and ezdxf's own
// documentation make the parent the owner, but the files ezdxf writes give
// the VERTEX records the BLOCK_RECORD of the parent. An owner read from the
// file is therefore kept when it is the parent or a BLOCK_RECORD, so that the
// tag sequence of the file survives (ADR 0005); anything else, and a missing
// owner, becomes the parent.
func (mw *modernWriter) childOwner(read, parent Handle) Handle {
	if mw.acceptChildOwner(parent)(read) {
		return read
	}
	return parent
}

func (mw *modernWriter) acceptChildOwner(parent Handle) func(Handle) bool {
	return func(h Handle) bool { return !h.IsZero() && (h == parent || mw.blockRec[h]) }
}

// ---------------------------------------------------------------------------
// ENTITIES

func (mw *modernWriter) writeEntitiesSection(doc *Document) {
	mw.group(groupType, "SECTION")
	mw.group(groupName, "ENTITIES")
	for _, e := range doc.Entities {
		mw.writeEntity(e, mw.tbl.ModelSpace, false)
	}
	mw.group(groupType, "ENDSEC")
}

// entityHandle returns the handle text to write for a record and its value.
func (mw *modernWriter) entityHandle(text string) (string, Handle) {
	if !handleMissing(text) {
		h, _ := parseHandle(text)
		return strings.TrimSpace(text), Handle(h)
	}
	h := mw.alloc()
	return h.String(), h
}

// ownerOf returns the owner of an entity of the container block record. In
// ENTITIES an owner already set is kept when it is a BLOCK_RECORD (the paper
// space layouts); otherwise the entity belongs to the container.
func (mw *modernWriter) ownerOf(c *EntityCommon, container Handle, inBlock bool) Handle {
	if inBlock {
		return container
	}
	if c != nil {
		if !c.Owner.IsZero() && mw.blockRec[c.Owner] {
			return c.Owner
		}
		if c.Owner.IsZero() && hasPaperFlag(c.Raw) && !mw.tbl.PaperSpace.IsZero() {
			return mw.tbl.PaperSpace
		}
	}
	return container
}

func (mw *modernWriter) writeEntity(e Entity, container Handle, inBlock bool) {
	if mw.err != nil {
		return
	}
	switch t := e.(type) {
	case *RawEntity:
		mw.group(groupType, t.TypeName)
		for _, g := range t.Groups {
			mw.group(g.Code, g.Value)
		}
	case *Line:
		mw.simple("LINE", &t.EntityCommon, container, inBlock, []string{"AcDbEntity", "AcDbLine"}, func(sub string, _ int) {
			if sub == "AcDbLine" {
				mw.thickness(&t.EntityCommon)
				mw.point(groupX, t.Start)
				mw.point(groupX2, t.End)
				mw.extrusion(&t.EntityCommon)
			}
		})
	case *Point:
		mw.simple("POINT", &t.EntityCommon, container, inBlock, []string{"AcDbEntity", "AcDbPoint"}, func(sub string, _ int) {
			if sub == "AcDbPoint" {
				mw.thickness(&t.EntityCommon)
				mw.point(groupX, t.Position)
				mw.extrusion(&t.EntityCommon)
				if t.Angle != 0 {
					mw.groupFloat(groupRotation, t.Angle)
				}
			}
		})
	case *Circle:
		mw.simple("CIRCLE", &t.EntityCommon, container, inBlock, []string{"AcDbEntity", "AcDbCircle"}, func(sub string, _ int) {
			if sub == "AcDbCircle" {
				mw.thickness(&t.EntityCommon)
				mw.point(groupX, t.Center)
				mw.groupFloat(groupRadius, t.Radius)
				mw.extrusion(&t.EntityCommon)
			}
		})
	case *Arc:
		mw.simple("ARC", &t.EntityCommon, container, inBlock, []string{"AcDbEntity", "AcDbCircle", "AcDbArc"}, func(sub string, _ int) {
			switch sub {
			case "AcDbCircle":
				mw.thickness(&t.EntityCommon)
				mw.point(groupX, t.Center)
				mw.groupFloat(groupRadius, t.Radius)
				mw.extrusion(&t.EntityCommon)
			case "AcDbArc":
				mw.groupFloat(groupRotation, t.StartAngle)
				mw.groupFloat(groupAngle2, t.EndAngle)
			}
		})
	case *Text:
		mw.simple("TEXT", &t.EntityCommon, container, inBlock, []string{"AcDbEntity", "AcDbText", "AcDbText"}, func(sub string, occ int) {
			if sub != "AcDbText" {
				return
			}
			if occ == 0 {
				mw.thickness(&t.EntityCommon)
				mw.point(groupX, t.Position)
				mw.groupFloat(groupRadius, t.Height)
				mw.group(groupText, t.Value)
				if t.Rotation != 0 {
					mw.groupFloat(groupRotation, t.Rotation)
				}
				if t.XScale != 0 && t.XScale != 1 {
					mw.groupFloat(groupXScale, t.XScale)
				}
				if t.Oblique != 0 {
					mw.groupFloat(groupAngle2, t.Oblique)
				}
				if t.Style != "" {
					mw.group(groupStyle, t.Style)
				}
				if t.GenerationFlags != 0 {
					mw.groupInt(groupFlags2, t.GenerationFlags)
				}
				if t.HAlign != 0 {
					mw.groupInt(groupHAlign, t.HAlign)
				}
				if t.HAlign != 0 || t.VAlign != 0 {
					mw.point(groupX2, t.Alignment)
				}
				mw.extrusion(&t.EntityCommon)
			} else if t.VAlign != 0 {
				mw.groupInt(groupVAlign, t.VAlign)
			}
		})
	case *Solid:
		mw.corners("SOLID", &t.EntityCommon, container, inBlock, "AcDbTrace", t.Corners, 0, true)
	case *Trace:
		mw.corners("TRACE", &t.EntityCommon, container, inBlock, "AcDbTrace", t.Corners, 0, true)
	case *Face3D:
		mw.corners("3DFACE", &t.EntityCommon, container, inBlock, "AcDbFace", t.Corners, t.InvisibleEdges, false)
	case *Polyline:
		mw.writePolyline(t, container, inBlock)
	case *Insert:
		mw.writeInsert(t, container, inBlock)
	default:
		mw.err = &ValidationError{Entity: e.Type(), Reason: "entity type not supported for writing"}
	}
}

// simple writes an entity that is a single record.
func (mw *modernWriter) simple(typ string, c *EntityCommon, container Handle, inBlock bool, def []string, body func(string, int)) {
	text, _ := mw.entityHandle(c.Handle)
	mw.writeRecord(rec{
		typ: typ, handle: text, owner: mw.ownerOf(c, container, inBlock),
		subs: pickSubs(c.Subclasses, def), common: c, raw: c.Raw, rawSub: c.RawSubclass,
	}, body)
}

func (mw *modernWriter) corners(typ string, c *EntityCommon, container Handle, inBlock bool, marker string, p [4]Point3D, flags int, thickness bool) {
	mw.simple(typ, c, container, inBlock, []string{"AcDbEntity", marker}, func(sub string, _ int) {
		if sub != marker {
			return
		}
		if thickness {
			mw.thickness(c)
		}
		mw.point(groupX, p[0])
		mw.point(groupX2, p[1])
		mw.point(groupX3, p[2])
		mw.point(groupX4, p[3])
		if flags != 0 {
			mw.groupInt(groupFlags, flags)
		}
		if thickness {
			mw.extrusion(c)
		}
	})
}

// polylineMarkers returns the markers of a POLYLINE and of its VERTEX records
// for the flags 70, and false for a polygon or polyface mesh.
func polylineMarkers(flags int) (poly, vertex string, ok bool) {
	switch {
	case flags&(16|64) != 0:
		return "", "", false
	case flags&8 != 0:
		return "AcDb3dPolyline", "AcDb3dPolylineVertex", true
	}
	return "AcDb2dPolyline", "AcDb2dVertex", true
}

func (mw *modernWriter) writePolyline(p *Polyline, container Handle, inBlock bool) {
	c := &p.EntityCommon
	text, handle := mw.entityHandle(c.Handle)
	polyMarker, vertexMarker, plain := polylineMarkers(p.Flags)
	subs := c.Subclasses
	if plain {
		if len(subs) < 2 || subs[0] != "AcDbEntity" || subs[1] != polyMarker {
			subs = []string{"AcDbEntity", polyMarker}
		}
	} else if len(subs) < 2 {
		mw.err = &ValidationError{Entity: "POLYLINE", Reason: "polygon and polyface meshes can only be written when they were read from an R13 or later file"}
		return
	}
	owner := mw.ownerOf(c, container, inBlock)
	paper := hasPaperFlag(c.Raw)
	mw.writeRecord(rec{
		typ: "POLYLINE", handle: text, owner: owner, subs: subs, common: c, raw: c.Raw, rawSub: c.RawSubclass,
	}, func(sub string, _ int) {
		if sub == "AcDbEntity" || sub == "AcDbVertex" {
			return
		}
		mw.groupInt(groupVerticesFollow, 1)
		mw.point(groupX, p.Elevation)
		mw.thickness(c)
		mw.groupInt(groupFlags, p.Flags)
		if p.StartWidth != 0 {
			mw.groupFloat(groupWidthStart, p.StartWidth)
		}
		if p.EndWidth != 0 {
			mw.groupFloat(groupWidthEnd, p.EndWidth)
		}
		mw.extrusion(c)
	})
	for i := range p.Vertices {
		v := &p.Vertices[i]
		vtext, _ := mw.entityHandle(v.Handle)
		vsubs := v.Subclasses
		if plain {
			if len(vsubs) < 3 || vsubs[0] != "AcDbEntity" || vsubs[2] != vertexMarker {
				vsubs = []string{"AcDbEntity", "AcDbVertex", vertexMarker}
			}
		} else if len(vsubs) < 3 {
			vsubs = []string{"AcDbEntity", "AcDbVertex", "AcDbPolygonMeshVertex"}
		}
		mw.writeRecord(rec{
			typ: "VERTEX", handle: vtext, owner: mw.childOwner(v.Owner, handle), subs: vsubs, layer: c.LayerName,
			paper: paper, raw: v.Raw, rawSub: v.RawSubclass,
		}, func(sub string, _ int) {
			if sub == "AcDbEntity" || sub == "AcDbVertex" {
				return
			}
			mw.point(groupX, v.Position)
			if v.Bulge != 0 {
				mw.groupFloat(groupBulge, v.Bulge)
			}
			flags := v.Flags
			if plain && vertexMarker == "AcDb3dPolylineVertex" {
				flags |= 32
			}
			mw.groupInt(groupFlags, flags)
			if v.Tangent != 0 {
				mw.groupFloat(groupRotation, v.Tangent)
			}
		})
	}
	mw.writeSeqend(p.SeqendHandle, p.SeqendGroups, handle, c.LayerName, paper)
}

func (mw *modernWriter) writeSeqend(handle string, groups []Group, owner Handle, layer string, paper bool) {
	text, _ := mw.entityHandle(handle)
	mw.group(groupType, "SEQEND")
	mw.group(groupHandle, text)
	if len(groups) > 0 {
		mw.writeOwned(groups, owner, mw.acceptChildOwner(owner))
		return
	}
	mw.group(groupOwnerHandle, owner.String())
	mw.group(groupSubclassMarker, "AcDbEntity")
	if paper {
		mw.groupInt(67, 1)
	}
	mw.group(groupLayer, layerOrZero(layer))
}

func (mw *modernWriter) writeInsert(in *Insert, container Handle, inBlock bool) {
	c := &in.EntityCommon
	text, handle := mw.entityHandle(c.Handle)
	marker := "AcDbBlockReference"
	if in.ColumnCount > 1 || in.RowCount > 1 {
		marker = "AcDbMInsertBlock"
	}
	subs := c.Subclasses
	if len(subs) < 2 || subs[0] != "AcDbEntity" {
		subs = []string{"AcDbEntity", marker}
	}
	paper := hasPaperFlag(c.Raw)
	mw.writeRecord(rec{
		typ: "INSERT", handle: text, owner: mw.ownerOf(c, container, inBlock),
		subs: subs, common: c, raw: c.Raw, rawSub: c.RawSubclass,
	}, func(sub string, _ int) {
		if sub == "AcDbEntity" {
			return
		}
		if len(in.Attributes) > 0 {
			mw.groupInt(groupVerticesFollow, 1)
		}
		mw.group(groupName, in.BlockName)
		mw.point(groupX, in.Position)
		if in.XScale != 1 {
			mw.groupFloat(groupXScale, in.XScale)
		}
		if in.YScale != 1 {
			mw.groupFloat(groupBulge, in.YScale)
		}
		if in.ZScale != 1 {
			mw.groupFloat(groupZScale, in.ZScale)
		}
		if in.Rotation != 0 {
			mw.groupFloat(groupRotation, in.Rotation)
		}
		if in.ColumnCount > 1 {
			mw.groupInt(groupFlags, in.ColumnCount)
		}
		if in.RowCount > 1 {
			mw.groupInt(groupFlags2, in.RowCount)
		}
		if in.ColumnSpacing != 0 {
			mw.groupFloat(groupColumnSpacing, in.ColumnSpacing)
		}
		if in.RowSpacing != 0 {
			mw.groupFloat(groupRowSpacing, in.RowSpacing)
		}
		mw.extrusion(c)
	})
	for i := range in.Attributes {
		a := &in.Attributes[i]
		atext, _ := mw.entityHandle(a.Handle)
		asubs := pickSubs(a.Subclasses, []string{"AcDbEntity", "AcDbText", "AcDbAttribute"})
		mw.writeRecord(rec{
			typ: "ATTRIB", handle: atext, owner: mw.childOwner(a.Owner, handle), subs: asubs, layer: c.LayerName,
			paper: paper, raw: a.Raw, rawSub: a.RawSubclass,
			// The reader (decodeAttribute) takes the 72 of AcDbAttribute, which
			// R2018 files write after the 71 (attribute type), for the
			// horizontal alignment. A zero would vanish and the tag sequence
			// of the file with it: write it back after the raw groups.
			after: func(sub string, _ int) {
				if sub == "AcDbAttribute" && a.HAlign == 0 && rawHas(a.Raw, 71) && !rawHas(a.Raw, 72) {
					mw.groupInt(groupHAlign, 0)
				}
			},
		}, func(sub string, occ int) {
			switch {
			case sub == "AcDbText" && occ == 0:
				mw.point(groupX, a.Position)
				mw.groupFloat(groupRadius, a.Height)
				mw.group(groupText, a.Value)
				if a.Rotation != 0 {
					mw.groupFloat(groupRotation, a.Rotation)
				}
				if a.XScale != 0 && a.XScale != 1 {
					mw.groupFloat(groupXScale, a.XScale)
				}
				if a.Oblique != 0 {
					mw.groupFloat(groupAngle2, a.Oblique)
				}
				if a.Style != "" {
					mw.group(groupStyle, a.Style)
				}
				if a.HAlign != 0 {
					mw.groupInt(groupHAlign, a.HAlign)
				}
			case sub == "AcDbAttribute":
				if a.Tag != "" {
					mw.group(groupName, a.Tag)
				}
				mw.groupInt(groupFlags, a.Flags)
				if a.FieldLength != 0 {
					mw.groupInt(groupFieldLength, a.FieldLength)
				}
				if a.VAlign != 0 {
					mw.groupInt(groupAttribVAlign, a.VAlign)
				}
			}
		})
	}
	if len(in.Attributes) > 0 {
		mw.writeSeqend(in.SeqendHandle, in.SeqendGroups, handle, c.LayerName, paper)
	}
}

// ---------------------------------------------------------------------------
// Validation

// validateModernContent rejects what cannot be written as the given version.
func validateModernContent(doc *Document, version string) error {
	check := func(entities []Entity) error {
		for _, e := range entities {
			switch t := e.(type) {
			case *RawEntity:
				if !t.Verbatim {
					return &ValidationError{Entity: t.TypeName, Reason: fmt.Sprintf(
						"entity read from a file without subclass markers (R12) and not interpreted by this package: it cannot be written as %s", version)}
				}
			case *Polyline:
				if _, _, plain := polylineMarkers(t.Flags); !plain && len(t.Subclasses) < 2 {
					return &ValidationError{Entity: "POLYLINE", Reason: fmt.Sprintf(
						"polygon and polyface meshes (flags 16 and 64) cannot be written as %s unless they were read from an R13 or later file", version)}
				}
			}
		}
		return nil
	}
	if err := check(doc.Entities); err != nil {
		return err
	}
	for _, b := range doc.Blocks {
		if b != nil {
			if err := check(b.Entities); err != nil {
				return err
			}
		}
	}
	return nil
}

// r12Incompatible returns a reason when doc holds content that exists only
// from R13 on (subclass markers, owners, verbatim records), which the R12
// writer would copy into an invalid file. ADR 0007: Encode rejects such a
// document instead of dropping the content.
func r12Incompatible(doc *Document) string {
	for _, o := range doc.Objects {
		if raw, ok := o.(*RawObject); ok && raw != nil {
			return "object " + raw.TypeName + " (OBJECTS section) cannot be written as R12"
		}
	}
	for _, tb := range doc.Tables.Items {
		if tb == nil {
			continue
		}
		for _, rec := range tb.Records {
			for _, g := range rec.Groups {
				if g.Code == groupSubclassMarker {
					return "table record " + rec.TypeName + " has R13 subclass markers"
				}
			}
		}
	}
	entity := func(e Entity) string {
		if e == nil {
			return ""
		}
		if raw, ok := e.(*RawEntity); ok && raw.Verbatim {
			return raw.TypeName + " was read from an R13 or later file and is kept verbatim"
		}
		if c := e.Common(); c != nil && len(c.Subclasses) > 0 {
			return e.Type() + " carries R13 subclass markers"
		}
		return ""
	}
	for _, e := range doc.Entities {
		if r := entity(e); r != "" {
			return r
		}
	}
	for _, b := range doc.Blocks {
		if b == nil {
			continue
		}
		if len(b.Subclasses) > 0 {
			return "block " + b.Name + " carries R13 subclass markers"
		}
		for _, e := range b.Entities {
			if r := entity(e); r != "" {
				return r
			}
		}
	}
	return ""
}
