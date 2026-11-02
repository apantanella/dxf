package dxf

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/apantanella/dxf/tags"
)

// EncodeOption configures Encode.
type EncodeOption func(*encodeConfig)

type encodeConfig struct {
	precision int
	version   string
}

// WithVersion sets the version Encode writes, overriding Document.Version and
// the $ACADVER of the header. Writable versions are VersionR12, VersionR2000,
// VersionR2004, VersionR2007, VersionR2010, VersionR2013 and VersionR2018 (see
// ADR 0009); any other value makes Encode fail with an error that satisfies
// errors.Is(err, ErrUnsupportedVersion) and errors.Is(err, ErrValidation).
func WithVersion(version string) EncodeOption {
	return func(c *encodeConfig) { c.version = version }
}

// WithPrecision sets the number of digits after the decimal point for real
// values. A negative value writes the shortest representation that round-trips.
func WithPrecision(digits int) EncodeOption {
	return func(c *encodeConfig) { c.precision = digits }
}

// Encode writes doc as a DXF ASCII stream.
//
// The version written is WithVersion if given, else Document.Version, else the
// $ACADVER of the header, else R12. For VersionR12 the writer emits HEADER,
// TABLES, BLOCKS, ENTITIES and EOF with LF line endings. For the other writable
// versions (R2000 to R2018) it emits HEADER, CLASSES, TABLES, BLOCKS, ENTITIES,
// OBJECTS and EOF with handles, owners and subclass markers, CR LF line endings
// and group codes right-aligned on three columns, as in the reference files
// tested in ZWCAD (docs/dxf-r2013.md). Missing layers, tables, dictionaries and
// layouts are added automatically; the document itself is never modified.
//
// A document that cannot be represented in the requested version is rejected
// with an error that wraps ErrValidation: for example a document that holds
// R13+ content (subclass markers, verbatim records, objects not interpreted)
// written as R12, or an entity without subclass markers that this package does
// not interpret written as R2000 or later (ADR 0007).
func Encode(w io.Writer, doc *Document, opts ...EncodeOption) error {
	if w == nil {
		return &ValidationError{Reason: "nil writer"}
	}
	if doc == nil {
		return &ValidationError{Reason: "nil document"}
	}
	cfg := encodeConfig{precision: -1}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	version, err := resolveWriteVersion(doc, cfg)
	if err != nil {
		return err
	}
	if version != VersionR12 {
		return encodeModern(w, doc, cfg, version)
	}
	if err := validateDocument(doc); err != nil {
		return err
	}
	if reason := r12Incompatible(doc); reason != "" {
		return &ValidationError{Reason: reason + ": convert the document to R2000 or later or rebuild it without that content"}
	}
	ew := &entityWriter{tw: tags.NewWriter(w), cfg: cfg}
	ew.writeDocument(doc)
	if ew.err != nil {
		return ew.err
	}
	return ew.tw.Flush()
}

type entityWriter struct {
	tw             *tags.Writer
	cfg            encodeConfig
	err            error
	handle         uint64
	handlesEnabled bool
}

func (ew *entityWriter) group(code int, value string) {
	if ew.err != nil {
		return
	}
	if err := ew.tw.Write(Group{Code: code, Value: value}); err != nil {
		ew.err = err
	}
}

func (ew *entityWriter) groupInt(code, value int) {
	ew.group(code, strconv.Itoa(value))
}

func (ew *entityWriter) groupFloat(code int, value float64) {
	ew.group(code, formatFloat(value, ew.cfg.precision))
}

func (ew *entityWriter) nextHandle() string {
	handle := ew.handle
	ew.handle++
	return strings.ToUpper(strconv.FormatUint(handle, 16))
}

func (ew *entityWriter) writeDocument(doc *Document) {
	ew.handlesEnabled = documentUsesHandles(doc)
	ew.handle = maxHandle(doc) + 1
	ew.writeHeader(doc)
	ew.writeTables(doc)
	ew.writeBlocks(doc)
	ew.group(groupType, "SECTION")
	ew.group(groupName, "ENTITIES")
	for _, entity := range doc.Entities {
		ew.writeEntity(entity)
	}
	ew.group(groupType, "ENDSEC")
	ew.group(groupType, "EOF")
}

func (ew *entityWriter) writeHeader(doc *Document) {
	ew.group(groupType, "SECTION")
	ew.group(groupName, "HEADER")
	ew.group(groupVariable, "$ACADVER")
	ew.group(groupText, VersionR12)
	if ew.handlesEnabled {
		seed := ew.handle + uint64(countMissingHandles(doc))
		ew.group(groupVariable, "$HANDLING")
		ew.group(groupFlags, "1")
		ew.group(groupVariable, "$HANDSEED")
		ew.group(groupHandle, strings.ToUpper(strconv.FormatUint(seed, 16)))
	}
	header := ew.headerWithExtents(doc)
	for _, name := range header.Names() {
		switch name {
		case "$ACADVER", "$HANDLING", "$HANDSEED":
			continue
		}
		variable := header.Variables[name]
		ew.group(groupVariable, name)
		for _, group := range variable.Groups {
			ew.group(group.Code, group.Value)
		}
	}
	ew.group(groupType, "ENDSEC")
}

// headerWithExtents returns the header to write: the document variables plus
// $EXTMIN and $EXTMAX computed from the entities when the document does not
// define them. The document itself is not modified.
func (ew *entityWriter) headerWithExtents(doc *Document) Header {
	header := Header{Variables: make(map[string]Variable, len(doc.Header.Variables)+2)}
	for name, variable := range doc.Header.Variables {
		header.Variables[name] = variable
	}
	_, hasMin := header.Variables["$EXTMIN"]
	_, hasMax := header.Variables["$EXTMAX"]
	if hasMin && hasMax {
		return header
	}
	if min, max, ok := doc.Extents(); ok {
		if !hasMin {
			header.setPoint("$EXTMIN", min, ew.cfg.precision)
		}
		if !hasMax {
			header.setPoint("$EXTMAX", max, ew.cfg.precision)
		}
	}
	return header
}

func (ew *entityWriter) writeTables(doc *Document) {
	ew.group(groupType, "SECTION")
	ew.group(groupName, "TABLES")
	for _, table := range orderedTables(preparedTables(doc)) {
		ew.group(groupType, "TABLE")
		ew.group(groupName, table.Name)
		for _, group := range table.Raw {
			ew.group(group.Code, group.Value)
		}
		for _, record := range table.Records {
			ew.group(groupType, record.TypeName)
			for _, group := range record.Groups {
				ew.group(group.Code, group.Value)
			}
		}
		ew.group(groupType, "ENDTAB")
	}
	ew.group(groupType, "ENDSEC")
}

// preparedTables returns a copy of the document tables with the records that a
// valid R12 file needs: layer 0, every referenced layer, CONTINUOUS and STANDARD.
func preparedTables(doc *Document) *Tables {
	tables := &Tables{}
	for _, table := range doc.Tables.Items {
		if table == nil {
			continue
		}
		// The writer adds records to the tables it emits: work on a copy so that
		// Encode never modifies the caller's document.
		clone := *table
		clone.Records = append([]Record(nil), table.Records...)
		tables.Items = append(tables.Items, &clone)
	}
	tables.EnsureLinetype("CONTINUOUS")
	refs := referencedLinetypes(doc)
	for _, upper := range sortedKeys(refs) {
		if _, ok := standardLinetypes[upper]; ok {
			tables.EnsureLinetype(refs[upper])
		}
	}
	tables.EnsureStyle("STANDARD")
	tables.EnsureLayer("0", 7, "CONTINUOUS")
	for _, layer := range referencedLayers(doc) {
		tables.EnsureLayer(layer, 7, "CONTINUOUS")
	}
	return tables
}

// orderedTables puts the tables that CAD applications expect first.
func orderedTables(tables *Tables) []*Table {
	preferred := []string{"VPORT", "LTYPE", "LAYER", "STYLE", "VIEW", "UCS", "APPID", "DIMSTYLE"}
	var out []*Table
	seen := map[string]bool{}
	for _, name := range preferred {
		if table := tables.Find(name); table != nil {
			out = append(out, table)
			seen[strings.ToUpper(table.Name)] = true
		}
	}
	for _, table := range tables.Items {
		if !seen[strings.ToUpper(table.Name)] {
			out = append(out, table)
		}
	}
	return out
}

func (ew *entityWriter) writeBlocks(doc *Document) {
	ew.group(groupType, "SECTION")
	ew.group(groupName, "BLOCKS")
	for _, block := range doc.Blocks {
		if block == nil {
			continue
		}
		ew.group(groupType, "BLOCK")
		ew.group(groupLayer, layerOrZero(block.LayerName))
		ew.group(groupName, block.Name)
		ew.groupInt(groupFlags, block.Flags)
		ew.groupFloat(groupX, block.BasePoint.X)
		ew.groupFloat(groupY, block.BasePoint.Y)
		ew.groupFloat(groupZ, block.BasePoint.Z)
		ew.group(groupName2, block.Name)
		for _, group := range block.Raw {
			ew.group(group.Code, group.Value)
		}
		for _, entity := range block.Entities {
			ew.writeEntity(entity)
		}
		ew.group(groupType, "ENDBLK")
		if block.EndHandle != "" {
			ew.group(groupHandle, block.EndHandle)
		}
	}
	ew.group(groupType, "ENDSEC")
}

func (ew *entityWriter) writeEntity(entity Entity) {
	if entity == nil {
		ew.err = &ValidationError{Reason: "nil entity"}
		return
	}
	ew.group(groupType, entity.Type())
	switch typed := entity.(type) {
	case *Polyline, *Insert:
		// Composite entities write the common groups themselves, after their
		// own header, so that they are not written twice.
	case *RawEntity:
		// A verbatim RawEntity (read from R13 or later) already holds every
		// group, handle and layer included.
		if !typed.Verbatim {
			ew.writeCommon(entity.Common())
		}
	default:
		ew.writeCommon(entity.Common())
	}
	switch typed := entity.(type) {
	case *Line:
		ew.writePoint(groupX, typed.Start)
		ew.writePoint(groupX2, typed.End)
		ew.writeExtrusion(&typed.EntityCommon)
		ew.writeRaw(&typed.EntityCommon)
	case *Point:
		ew.writePoint(groupX, typed.Position)
		if typed.Angle != 0 {
			ew.groupFloat(groupRotation, typed.Angle)
		}
		ew.writeExtrusion(&typed.EntityCommon)
		ew.writeRaw(&typed.EntityCommon)
	case *Circle:
		ew.writePoint(groupX, typed.Center)
		ew.groupFloat(groupRadius, typed.Radius)
		ew.writeExtrusion(&typed.EntityCommon)
		ew.writeRaw(&typed.EntityCommon)
	case *Arc:
		ew.writePoint(groupX, typed.Center)
		ew.groupFloat(groupRadius, typed.Radius)
		ew.groupFloat(groupRotation, typed.StartAngle)
		ew.groupFloat(groupAngle2, typed.EndAngle)
		ew.writeExtrusion(&typed.EntityCommon)
		ew.writeRaw(&typed.EntityCommon)
	case *Text:
		ew.group(groupText, typed.Value)
		ew.writePoint(groupX, typed.Position)
		ew.groupFloat(groupRadius, typed.Height)
		if typed.XScale != 0 && typed.XScale != 1 {
			ew.groupFloat(groupXScale, typed.XScale)
		}
		if typed.Rotation != 0 {
			ew.groupFloat(groupRotation, typed.Rotation)
		}
		if typed.Oblique != 0 {
			ew.groupFloat(groupAngle2, typed.Oblique)
		}
		if typed.Style != "" {
			ew.group(groupStyle, typed.Style)
		}
		if typed.GenerationFlags != 0 {
			ew.groupInt(groupFlags2, typed.GenerationFlags)
		}
		if typed.HAlign != 0 {
			ew.groupInt(groupHAlign, typed.HAlign)
		}
		if typed.VAlign != 0 {
			ew.groupInt(groupVAlign, typed.VAlign)
		}
		if typed.HAlign != 0 || typed.VAlign != 0 {
			ew.writePoint(groupX2, typed.Alignment)
		}
		ew.writeExtrusion(&typed.EntityCommon)
		ew.writeRaw(&typed.EntityCommon)
	case *Solid:
		ew.writeCorners(typed.Corners)
		ew.writeExtrusion(&typed.EntityCommon)
		ew.writeRaw(&typed.EntityCommon)
	case *Trace:
		ew.writeCorners(typed.Corners)
		ew.writeExtrusion(&typed.EntityCommon)
		ew.writeRaw(&typed.EntityCommon)
	case *Face3D:
		ew.writeCorners(typed.Corners)
		if typed.InvisibleEdges != 0 {
			ew.groupInt(groupFlags, typed.InvisibleEdges)
		}
		ew.writeRaw(&typed.EntityCommon)
	case *Polyline:
		ew.writePolyline(typed)
	case *Insert:
		ew.writeInsert(typed)
	case *RawEntity:
		for _, group := range typed.Groups {
			ew.group(group.Code, group.Value)
		}
	default:
		ew.err = &ValidationError{Entity: entity.Type(), Reason: "entity type not supported for writing"}
	}
}

func (ew *entityWriter) writeCommon(common *EntityCommon) {
	if common == nil {
		return
	}
	if common.Handle != "" {
		ew.group(groupHandle, common.Handle)
	} else if ew.handlesEnabled {
		ew.group(groupHandle, ew.nextHandle())
	}
	ew.group(groupLayer, layerOrZero(common.LayerName))
	if common.Linetype != "" && !strings.EqualFold(common.Linetype, "BYLAYER") {
		ew.group(groupLinetype, common.Linetype)
	}
	if common.HasColor {
		ew.groupInt(groupColor, common.Color)
	}
	if common.Invisible {
		ew.groupInt(groupInvisible, 1)
	}
	if common.Thickness != 0 {
		ew.groupFloat(groupThickness, common.Thickness)
	}
}

func (ew *entityWriter) writePoint(code int, point Point3D) {
	ew.groupFloat(code, point.X)
	ew.groupFloat(code+10, point.Y)
	ew.groupFloat(code+20, point.Z)
}

func (ew *entityWriter) writeCorners(corners [4]Point3D) {
	ew.writePoint(groupX, corners[0])
	ew.writePoint(groupX2, corners[1])
	ew.writePoint(groupX3, corners[2])
	ew.writePoint(groupX4, corners[3])
}

func (ew *entityWriter) writeExtrusion(common *EntityCommon) {
	if common == nil || !common.ExtrusionSet {
		return
	}
	ew.groupFloat(groupExtrusionX, common.Extrusion.X)
	ew.groupFloat(groupExtrusionY, common.Extrusion.Y)
	ew.groupFloat(groupExtrusionZ, common.Extrusion.Z)
}

func (ew *entityWriter) writeRaw(common *EntityCommon) {
	if common == nil {
		return
	}
	for _, group := range common.Raw {
		ew.group(group.Code, group.Value)
	}
}

func (ew *entityWriter) writePolyline(entity *Polyline) {
	ew.writeCommon(&entity.EntityCommon)
	ew.groupInt(groupVerticesFollow, 1)
	ew.writePoint(groupX, entity.Elevation)
	ew.groupInt(groupFlags, entity.Flags)
	if entity.StartWidth != 0 {
		ew.groupFloat(groupWidthStart, entity.StartWidth)
	}
	if entity.EndWidth != 0 {
		ew.groupFloat(groupWidthEnd, entity.EndWidth)
	}
	ew.writeExtrusion(&entity.EntityCommon)
	ew.writeRaw(&entity.EntityCommon)
	for _, vertex := range entity.Vertices {
		ew.group(groupType, "VERTEX")
		if vertex.Handle != "" {
			ew.group(groupHandle, vertex.Handle)
		} else if ew.handlesEnabled {
			ew.group(groupHandle, ew.nextHandle())
		}
		ew.writePoint(groupX, vertex.Position)
		if vertex.Bulge != 0 {
			ew.groupFloat(groupBulge, vertex.Bulge)
		}
		if vertex.Flags != 0 {
			ew.groupInt(groupFlags, vertex.Flags)
		}
		for _, group := range vertex.Raw {
			ew.group(group.Code, group.Value)
		}
	}
	ew.writeSeqend(entity.SeqendHandle)
}

func (ew *entityWriter) writeInsert(entity *Insert) {
	ew.writeCommon(&entity.EntityCommon)
	ew.group(groupName, entity.BlockName)
	ew.writePoint(groupX, entity.Position)
	if entity.XScale != 1 {
		ew.groupFloat(groupXScale, entity.XScale)
	}
	if entity.YScale != 1 {
		ew.groupFloat(groupBulge, entity.YScale)
	}
	if entity.ZScale != 1 {
		ew.groupFloat(groupZScale, entity.ZScale)
	}
	if entity.Rotation != 0 {
		ew.groupFloat(groupRotation, entity.Rotation)
	}
	if entity.ColumnCount > 1 {
		ew.groupInt(groupFlags, entity.ColumnCount)
	}
	if entity.RowCount > 1 {
		ew.groupInt(groupFlags2, entity.RowCount)
	}
	if entity.ColumnSpacing != 0 {
		ew.groupFloat(groupColumnSpacing, entity.ColumnSpacing)
	}
	if entity.RowSpacing != 0 {
		ew.groupFloat(groupRowSpacing, entity.RowSpacing)
	}
	if len(entity.Attributes) > 0 {
		ew.groupInt(groupVerticesFollow, 1)
	}
	ew.writeExtrusion(&entity.EntityCommon)
	ew.writeRaw(&entity.EntityCommon)
	for i := range entity.Attributes {
		ew.writeAttribute(&entity.Attributes[i])
	}
	if len(entity.Attributes) > 0 {
		ew.writeSeqend(entity.SeqendHandle)
	}
}

// writeSeqend writes a SEQEND record, keeping its handle or assigning a new one.
func (ew *entityWriter) writeSeqend(handle string) {
	ew.group(groupType, "SEQEND")
	if handle != "" {
		ew.group(groupHandle, handle)
	} else if ew.handlesEnabled {
		ew.group(groupHandle, ew.nextHandle())
	}
}

func (ew *entityWriter) writeAttribute(attribute *Attribute) {
	ew.group(groupType, "ATTRIB")
	if attribute.Handle != "" {
		ew.group(groupHandle, attribute.Handle)
	} else if ew.handlesEnabled {
		ew.group(groupHandle, ew.nextHandle())
	}
	ew.group(groupText, attribute.Value)
	ew.writePoint(groupX, attribute.Position)
	ew.groupFloat(groupRadius, attribute.Height)
	if attribute.Tag != "" {
		ew.group(groupName, attribute.Tag)
	}
	if attribute.Flags != 0 {
		ew.groupInt(groupFlags, attribute.Flags)
	}
	if attribute.FieldLength != 0 {
		ew.groupInt(groupFieldLength, attribute.FieldLength)
	}
	if attribute.XScale != 0 && attribute.XScale != 1 {
		ew.groupFloat(groupXScale, attribute.XScale)
	}
	if attribute.Rotation != 0 {
		ew.groupFloat(groupRotation, attribute.Rotation)
	}
	if attribute.Oblique != 0 {
		ew.groupFloat(groupAngle2, attribute.Oblique)
	}
	if attribute.Style != "" {
		ew.group(groupStyle, attribute.Style)
	}
	if attribute.HAlign != 0 {
		ew.groupInt(groupHAlign, attribute.HAlign)
	}
	if attribute.VAlign != 0 {
		ew.groupInt(groupAttribVAlign, attribute.VAlign)
	}
	for _, group := range attribute.Raw {
		ew.group(group.Code, group.Value)
	}
}

// validateDocument checks the content of doc, whatever the version written.
// The version itself is checked by resolveWriteVersion.
func validateDocument(doc *Document) error {
	if err := checkLinetypes(doc); err != nil {
		return err
	}
	blocks := make(map[string]bool, len(doc.Blocks))
	for _, block := range doc.Blocks {
		if block == nil {
			return &ValidationError{Entity: "BLOCK", Reason: "nil definition"}
		}
		if strings.TrimSpace(block.Name) == "" {
			return &ValidationError{Entity: "BLOCK", Reason: "missing name"}
		}
		if err := checkFinite("BLOCK", "base point", block.BasePoint); err != nil {
			return err
		}
		blocks[strings.ToUpper(block.Name)] = true
	}
	for _, entity := range doc.Entities {
		if err := validateEntity(entity, blocks); err != nil {
			return err
		}
	}
	for _, block := range doc.Blocks {
		for _, entity := range block.Entities {
			if err := validateEntity(entity, blocks); err != nil {
				return err
			}
		}
	}
	return checkHandles(doc)
}

// checkHandles rejects a document that cannot be written without corrupting
// it: a handle used twice, or no room left above the highest handle for the
// handles the writer must assign.
func checkHandles(doc *Document) error {
	for _, d := range doc.ValidateHandles() {
		if d.Severity == SeverityError {
			return &ValidationError{Entity: d.Entity, Reason: d.Message}
		}
	}
	if documentUsesHandles(doc) {
		if missing := uint64(countMissingHandles(doc)); maxHandle(doc) > math.MaxUint64-missing-1 {
			return &ValidationError{Reason: "handle space exhausted: cannot assign handles above the highest one in use"}
		}
	}
	return nil
}

func validateEntity(entity Entity, blocks map[string]bool) error {
	if entity == nil {
		return &ValidationError{Reason: "nil entity"}
	}
	if common := entity.Common(); common != nil {
		if common.Handle != "" {
			if _, err := parseHandle(common.Handle); err != nil {
				return &ValidationError{Entity: entity.Type(), Reason: "invalid handle: " + common.Handle}
			}
		}
		if err := checkFinite(entity.Type(), "extrusion", common.Extrusion); err != nil {
			return err
		}
	}
	switch typed := entity.(type) {
	case *Line:
		if err := checkFinite("LINE", "start point", typed.Start); err != nil {
			return err
		}
		return checkFinite("LINE", "end point", typed.End)
	case *Point:
		return checkFinite("POINT", "position", typed.Position)
	case *Circle:
		if err := checkFinite("CIRCLE", "center", typed.Center); err != nil {
			return err
		}
		if !(typed.Radius > 0) {
			return &ValidationError{Entity: "CIRCLE", Reason: "radius must be greater than zero"}
		}
	case *Arc:
		if err := checkFinite("ARC", "center", typed.Center); err != nil {
			return err
		}
		if !(typed.Radius > 0) {
			return &ValidationError{Entity: "ARC", Reason: "radius must be greater than zero"}
		}
		if !finite(typed.StartAngle) || !finite(typed.EndAngle) {
			return &ValidationError{Entity: "ARC", Reason: "non-finite angles"}
		}
	case *Text:
		if err := checkFinite("TEXT", "position", typed.Position); err != nil {
			return err
		}
		if !(typed.Height > 0) {
			return &ValidationError{Entity: "TEXT", Reason: "height must be greater than zero"}
		}
		if !finite(typed.Rotation) || !finite(typed.Oblique) || !finite(typed.XScale) {
			return &ValidationError{Entity: "TEXT", Reason: "non-finite text parameters"}
		}
	case *Solid:
		return checkCorners("SOLID", typed.Corners)
	case *Trace:
		return checkCorners("TRACE", typed.Corners)
	case *Face3D:
		return checkCorners("3DFACE", typed.Corners)
	case *Polyline:
		if len(typed.Vertices) == 0 {
			return &ValidationError{Entity: "POLYLINE", Reason: "no vertices"}
		}
		if typed.Closed() && len(typed.Vertices) < 3 {
			return &ValidationError{Entity: "POLYLINE", Reason: "a closed POLYLINE requires at least three vertices"}
		}
		if !typed.Closed() && len(typed.Vertices) < 2 {
			return &ValidationError{Entity: "POLYLINE", Reason: "an open POLYLINE requires at least two vertices"}
		}
		for _, vertex := range typed.Vertices {
			if err := checkFinite("POLYLINE", "vertex", vertex.Position); err != nil {
				return err
			}
		}
	case *Insert:
		if strings.TrimSpace(typed.BlockName) == "" {
			return &ValidationError{Entity: "INSERT", Reason: "missing block name"}
		}
		if !blocks[strings.ToUpper(typed.BlockName)] {
			return &ValidationError{Entity: "INSERT", Reason: fmt.Sprintf("block %q is not defined", typed.BlockName)}
		}
		if typed.XScale == 0 || typed.YScale == 0 || typed.ZScale == 0 {
			return &ValidationError{Entity: "INSERT", Reason: "scale factors must not be zero"}
		}
		if err := checkFinite("INSERT", "position", typed.Position); err != nil {
			return err
		}
		if !finite(typed.Rotation) {
			return &ValidationError{Entity: "INSERT", Reason: "non-finite rotation"}
		}
		for _, attribute := range typed.Attributes {
			if err := checkFinite("ATTRIB", "position", attribute.Position); err != nil {
				return err
			}
		}
	case *RawEntity:
		return nil
	default:
		return &ValidationError{Entity: entity.Type(), Reason: "entity type not supported for writing"}
	}
	return nil
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func checkFinite(entity, field string, point Point3D) error {
	if !finite(point.X) || !finite(point.Y) || !finite(point.Z) {
		return &ValidationError{Entity: entity, Reason: field + ": non-finite coordinates"}
	}
	return nil
}

func checkCorners(entity string, corners [4]Point3D) error {
	for i, corner := range corners {
		if err := checkFinite(entity, fmt.Sprintf("corner %d", i+1), corner); err != nil {
			return err
		}
	}
	return nil
}

// referencedLayers returns every layer used by entities and blocks, plus layer 0.
func referencedLayers(doc *Document) []string {
	set := map[string]bool{"0": true}
	add := func(entity Entity) {
		if entity == nil {
			return
		}
		set[layerOrZero(entity.Layer())] = true
	}
	for _, entity := range doc.Entities {
		add(entity)
	}
	for _, block := range doc.Blocks {
		if block == nil {
			continue
		}
		set[layerOrZero(block.LayerName)] = true
		for _, entity := range block.Entities {
			add(entity)
		}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func documentUsesHandles(doc *Document) bool {
	if value, ok := doc.Header.Get("$HANDLING"); ok && strings.TrimSpace(value) == "1" {
		return true
	}
	if handlesIn(doc.Entities) {
		return true
	}
	for _, block := range doc.Blocks {
		if block != nil && handlesIn(block.Entities) {
			return true
		}
	}
	return false
}

func handlesIn(entities []Entity) bool {
	for _, entity := range entities {
		if entity == nil {
			continue
		}
		if common := entity.Common(); common != nil && common.Handle != "" {
			return true
		}
		switch typed := entity.(type) {
		case *Polyline:
			if typed.SeqendHandle != "" {
				return true
			}
			for _, vertex := range typed.Vertices {
				if vertex.Handle != "" {
					return true
				}
			}
		case *Insert:
			if typed.SeqendHandle != "" {
				return true
			}
			for _, attribute := range typed.Attributes {
				if attribute.Handle != "" {
					return true
				}
			}
		}
	}
	return false
}

// maxHandle returns the highest handle in use anywhere in the document: object
// handles and handles inside preserved Raw groups and unknown sections.
func maxHandle(doc *Document) uint64 {
	var max uint64
	doc.walkHandles(func(r handleRef) bool {
		if value, err := parseHandle(r.value); err == nil && value > max {
			max = value
		}
		return false
	})
	return max
}

func countMissingHandles(doc *Document) int {
	count := 0
	countEntities := func(entities []Entity) {
		for _, entity := range entities {
			if entity == nil {
				continue
			}
			if common := entity.Common(); common == nil || common.Handle == "" {
				count++
			}
			switch typed := entity.(type) {
			case *Polyline:
				for _, vertex := range typed.Vertices {
					if vertex.Handle == "" {
						count++
					}
				}
				if typed.SeqendHandle == "" {
					count++ // SEQEND
				}
			case *Insert:
				for _, attribute := range typed.Attributes {
					if attribute.Handle == "" {
						count++
					}
				}
				if len(typed.Attributes) > 0 && typed.SeqendHandle == "" {
					count++ // SEQEND
				}
			}
		}
	}
	countEntities(doc.Entities)
	for _, block := range doc.Blocks {
		if block != nil {
			countEntities(block.Entities)
		}
	}
	return count
}

func parseHandle(handle string) (uint64, error) {
	handle = strings.TrimSpace(handle)
	if handle == "" {
		return 0, nil
	}
	return strconv.ParseUint(handle, 16, 64)
}

func formatFloat(value float64, precision int) string {
	if value == 0 {
		return "0.0"
	}
	if precision >= 0 {
		return strconv.FormatFloat(value, 'f', precision, 64)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}
