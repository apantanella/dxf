package dxf

import (
	"strings"
)

// Group codes used by the decoder and the writer.
const (
	groupType           = 0
	groupText           = 1
	groupName           = 2
	groupName2          = 3
	groupHandle         = 5
	groupLinetype       = 6
	groupStyle          = 7
	groupLayer          = 8
	groupVariable       = 9
	groupX              = 10
	groupX2             = 11
	groupX3             = 12
	groupX4             = 13
	groupY              = 20
	groupY2             = 21
	groupY3             = 22
	groupY4             = 23
	groupZ              = 30
	groupZ2             = 31
	groupZ3             = 32
	groupZ4             = 33
	groupThickness      = 39
	groupRadius         = 40
	groupWidthStart     = 40
	groupXScale         = 41
	groupWidthEnd       = 41
	groupBulge          = 42
	groupZScale         = 43
	groupColumnSpacing  = 44
	groupRowSpacing     = 45
	groupRotation       = 50
	groupAngle2         = 51
	groupInvisible      = 60
	groupColor          = 62
	groupVerticesFollow = 66
	groupFlags          = 70
	groupFlags2         = 71
	groupHAlign         = 72
	groupVAlign         = 73
	groupFieldLength    = 73
	groupAttribVAlign   = 74
	groupSubclassMarker = 100
	groupControlBrace   = 102
	groupExtrusionX     = 210
	groupExtrusionY     = 220
	groupExtrusionZ     = 230
	groupOwnerHandle    = 330
)

// Entity is a graphical DXF entity.
type Entity interface {
	Type() string
	Layer() string
	Common() *EntityCommon
}

// EntityCommon holds the properties shared by almost every entity.
type EntityCommon struct {
	LayerName string
	Linetype  string
	Color     int
	HasColor  bool
	// Handle is the text of group code 5, exactly as read (case and leading
	// zeros included). It is empty when the entity has no handle. Use
	// HandleValue and SetHandle to work with it as a Handle.
	Handle string
	// Owner is the handle of the owning object (group code 330); the zero value
	// means no owner. DXF R12 has no owner pointers: the R12 reader does not
	// fill Owner (a 330 group stays in Raw and is written back unchanged) and
	// the R12 writer never writes it. From R13 on the reader takes the 330 that
	// sits before the first subclass marker and outside any 102 { ... } group.
	Owner        Handle
	Invisible    bool
	Thickness    float64
	Extrusion    Point3D
	ExtrusionSet bool
	// Subclasses lists the subclass markers (group code 100) of the record in
	// file order, for example AcDbEntity, AcDbLine. The markers are not data:
	// they are not repeated in Raw. It is empty for R12.
	Subclasses []string
	// Raw holds the groups the decoder did not consume, in file order. From R13
	// on this includes the 102 { ... } groups (reactors, extension
	// dictionary, complete from "102 {NAME" to "102 }"), the 360 and the
	// extended data (1001 and following).
	Raw []Group
	// RawSubclass is parallel to Raw and tells where each group sat: the
	// number of subclass markers that precede it. 0 means before the first
	// marker (where 5, 102 and 330 live), k means after the k-th marker, that
	// is in the subclass Subclasses[k-1]. It is nil when the record has no
	// markers (R12) or when Raw has been built by hand; a writer then puts Raw
	// at the end of the record, as the R12 writer does.
	RawSubclass []int
}

// Normal returns the extrusion direction, defaulting to (0, 0, 1).
func (c *EntityCommon) Normal() Point3D {
	if !c.ExtrusionSet {
		return Point3D{Z: 1}
	}
	return c.Extrusion
}

// Line is a LINE entity.
type Line struct {
	EntityCommon
	Start Point3D
	End   Point3D
}

// Type returns the DXF type name.
func (e *Line) Type() string { return "LINE" }

// Layer returns the entity layer.
func (e *Line) Layer() string { return e.LayerName }

// Common returns the shared entity properties.
func (e *Line) Common() *EntityCommon { return &e.EntityCommon }

// Length returns the distance between the two endpoints.
func (e *Line) Length() float64 { return e.Start.Distance(e.End) }

// Point is a POINT entity.
type Point struct {
	EntityCommon
	Position Point3D
	Angle    float64
}

// Type returns the DXF type name.
func (e *Point) Type() string { return "POINT" }

// Layer returns the entity layer.
func (e *Point) Layer() string { return e.LayerName }

// Common returns the shared entity properties.
func (e *Point) Common() *EntityCommon { return &e.EntityCommon }

// Circle is a CIRCLE entity.
type Circle struct {
	EntityCommon
	Center Point3D
	Radius float64
}

// Type returns the DXF type name.
func (e *Circle) Type() string { return "CIRCLE" }

// Layer returns the entity layer.
func (e *Circle) Layer() string { return e.LayerName }

// Common returns the shared entity properties.
func (e *Circle) Common() *EntityCommon { return &e.EntityCommon }

// Arc is an ARC entity.
type Arc struct {
	EntityCommon
	Center     Point3D
	Radius     float64
	StartAngle float64
	EndAngle   float64
}

// Type returns the DXF type name.
func (e *Arc) Type() string { return "ARC" }

// Layer returns the entity layer.
func (e *Arc) Layer() string { return e.LayerName }

// Common returns the shared entity properties.
func (e *Arc) Common() *EntityCommon { return &e.EntityCommon }

// Vertex is a VERTEX record inside a POLYLINE.
type Vertex struct {
	Position Point3D
	Bulge    float64
	Flags    int
	Tangent  float64
	Handle   string
	// Owner is the group code 330 (R13 and later). Encoders recompute it from
	// the polyline.
	Owner Handle
	// Subclasses lists the subclass markers of the record (see
	// EntityCommon.Subclasses).
	Subclasses []string
	Raw        []Group
	// RawSubclass is parallel to Raw (see EntityCommon.RawSubclass).
	RawSubclass []int
}

// Polyline is a POLYLINE entity with its VERTEX records.
type Polyline struct {
	EntityCommon
	// SeqendHandle is the handle of the SEQEND record that closes the
	// polyline, or empty when it has none.
	SeqendHandle string
	// SeqendGroups holds the groups of the SEQEND record other than its handle
	// (330, 100 markers, layer) when the file has subclass markers. R12 SEQEND
	// groups other than the handle are not kept.
	SeqendGroups []Group
	Flags        int
	Elevation    Point3D
	StartWidth   float64
	EndWidth     float64
	Vertices     []Vertex
}

// Type returns the DXF type name.
func (e *Polyline) Type() string { return "POLYLINE" }

// Layer returns the entity layer.
func (e *Polyline) Layer() string { return e.LayerName }

// Common returns the shared entity properties.
func (e *Polyline) Common() *EntityCommon { return &e.EntityCommon }

// Closed reports whether the polyline is closed (flag bit 1).
func (e *Polyline) Closed() bool { return e.Flags&1 != 0 }

func layerOrZero(name string) string {
	if strings.TrimSpace(name) == "" {
		return "0"
	}
	return name
}

// Text is a TEXT entity.
type Text struct {
	EntityCommon
	Value           string
	Position        Point3D
	Alignment       Point3D
	Height          float64
	XScale          float64
	Rotation        float64
	Oblique         float64
	Style           string
	GenerationFlags int
	HAlign          int
	VAlign          int
}

// Type returns the DXF type name.
func (e *Text) Type() string { return "TEXT" }

// Layer returns the entity layer.
func (e *Text) Layer() string { return e.LayerName }

// Common returns the shared entity properties.
func (e *Text) Common() *EntityCommon { return &e.EntityCommon }

// Solid is a SOLID entity.
type Solid struct {
	EntityCommon
	Corners [4]Point3D
}

// Type returns the DXF type name.
func (e *Solid) Type() string { return "SOLID" }

// Layer returns the entity layer.
func (e *Solid) Layer() string { return e.LayerName }

// Common returns the shared entity properties.
func (e *Solid) Common() *EntityCommon { return &e.EntityCommon }

// Trace is a TRACE entity.
type Trace struct {
	EntityCommon
	Corners [4]Point3D
}

// Type returns the DXF type name.
func (e *Trace) Type() string { return "TRACE" }

// Layer returns the entity layer.
func (e *Trace) Layer() string { return e.LayerName }

// Common returns the shared entity properties.
func (e *Trace) Common() *EntityCommon { return &e.EntityCommon }

// Face3D is a 3DFACE entity. Its corners are always in WCS.
type Face3D struct {
	EntityCommon
	Corners        [4]Point3D
	InvisibleEdges int
}

// Type returns the DXF type name.
func (e *Face3D) Type() string { return "3DFACE" }

// Layer returns the entity layer.
func (e *Face3D) Layer() string { return e.LayerName }

// Common returns the shared entity properties.
func (e *Face3D) Common() *EntityCommon { return &e.EntityCommon }

// Attribute is an ATTRIB record inside an INSERT.
type Attribute struct {
	Value       string
	Tag         string
	Position    Point3D
	Height      float64
	XScale      float64
	Rotation    float64
	Oblique     float64
	Style       string
	Flags       int
	FieldLength int
	HAlign      int
	VAlign      int
	Handle      string
	// Owner is the group code 330 (R13 and later). Encoders recompute it from
	// the INSERT.
	Owner Handle
	// Subclasses lists the subclass markers of the record (see
	// EntityCommon.Subclasses).
	Subclasses []string
	Raw        []Group
	// RawSubclass is parallel to Raw (see EntityCommon.RawSubclass).
	RawSubclass []int
}

// Insert is an INSERT entity.
type Insert struct {
	EntityCommon
	BlockName     string
	Position      Point3D
	XScale        float64
	YScale        float64
	ZScale        float64
	Rotation      float64
	ColumnCount   int
	RowCount      int
	ColumnSpacing float64
	RowSpacing    float64
	Attributes    []Attribute
	// SeqendHandle is the handle of the SEQEND record that follows the
	// attributes, or empty when it has none.
	SeqendHandle string
	// SeqendGroups holds the groups of the SEQEND record other than its handle
	// when the file has subclass markers (see Polyline.SeqendGroups).
	SeqendGroups []Group
}

// NewInsert returns an INSERT with unit scale factors.
func NewInsert(blockName string, position Point3D) *Insert {
	return &Insert{BlockName: blockName, Position: position, XScale: 1, YScale: 1, ZScale: 1}
}

// Type returns the DXF type name.
func (e *Insert) Type() string { return "INSERT" }

// Layer returns the entity layer.
func (e *Insert) Layer() string { return e.LayerName }

// Common returns the shared entity properties.
func (e *Insert) Common() *EntityCommon { return &e.EntityCommon }

// RawEntity is an entity the package does not decode.
//
// In an R12 record Groups holds every group the decoder did not consume (the
// common groups such as layer and handle are in the embedded EntityCommon).
//
// In a record with subclass markers (R13 and later) Verbatim is true and Groups
// holds every group that follows the (0, TypeName) pair, in the original order
// and unchanged: 5, 330, the 100 markers, 102 groups, data and extended data
// included (ADR 0007). The embedded EntityCommon is then only a read-only view
// of some of those groups (layer, linetype, color, handle, owner, visibility,
// subclass names) and a writer must write Groups alone.
type RawEntity struct {
	EntityCommon
	TypeName string
	Groups   []Group
	// Verbatim tells that Groups is the complete record (see above).
	Verbatim bool
}

// Type returns the DXF type name.
func (e *RawEntity) Type() string { return e.TypeName }

// Layer returns the entity layer.
func (e *RawEntity) Layer() string { return e.LayerName }

// Common returns the shared entity properties.
func (e *RawEntity) Common() *EntityCommon { return &e.EntityCommon }

// groupSet tracks which group codes of an entity have been consumed, so that
// unhandled codes can be preserved in EntityCommon.Raw.
type groupSet struct {
	groups []Group
	used   []bool

	// The fields below are set only for records with subclass markers (R13 and
	// later); for R12 they stay nil and mark behaves as it always did.
	blocked []bool   // groups inside a 102 { ... } group: never consumed
	sub     []int    // number of 100 markers before each group
	names   []string // subclass names: names[k-1] is the k-th marker
	pref    []string // subclasses mark looks in first, see prefer
}

func newGroupSet(groups []Group) *groupSet {
	return &groupSet{groups: groups, used: make([]bool, len(groups))}
}

// hasSubclassMarkers reports whether the record contains a 100 or 102 group,
// which only R13 and later files have.
func hasSubclassMarkers(groups []Group) bool {
	for _, g := range groups {
		if g.Code == groupSubclassMarker || g.Code == groupControlBrace {
			return true
		}
	}
	return false
}

// newGroupSetMode returns a groupSet that understands subclass markers when
// modern is true or when the record contains markers.
func newGroupSetMode(groups []Group, modern bool) *groupSet {
	s := newGroupSet(groups)
	if modern || hasSubclassMarkers(groups) {
		s.initSubclasses()
	}
	return s
}

// initSubclasses records, for every group, how many 100 markers precede it and
// whether it sits inside a 102 { ... } group. The 100 markers are consumed
// (they go to the names list); the 102 groups stay unconsumed, so they end up
// in the leftovers, but mark never returns them (a 330 inside
// "102 {ACAD_REACTORS" is not the owner).
func (s *groupSet) initSubclasses() {
	n := len(s.groups)
	s.blocked = make([]bool, n)
	s.sub = make([]int, n)
	depth := 0
	cur := 0
	for i, g := range s.groups {
		switch {
		case g.Code == groupControlBrace:
			v := strings.TrimSpace(g.Value)
			switch {
			case strings.HasPrefix(v, "{"):
				depth++
				s.blocked[i] = true
			case v == "}" && depth > 0:
				s.blocked[i] = true
				depth--
			case depth > 0:
				s.blocked[i] = true
			}
		case depth > 0:
			s.blocked[i] = true
		case g.Code == groupSubclassMarker:
			cur++
			s.names = append(s.names, strings.TrimSpace(g.Value))
			s.used[i] = true
		}
		s.sub[i] = cur
	}
}

// prefer makes the following mark calls look first in the named subclasses
// and fall back to the rest of the record when the group is not there. The
// empty name stands for the part before the first marker. Nil clears the
// preference. It has no effect on records without markers. This is how codes
// that repeat in different subclasses are told apart: ATTRIB, for instance,
// has a 71 in AcDbText and another in AcDbAttribute, and the decoder must not
// confuse them.
func (s *groupSet) prefer(names []string) { s.pref = names }

// Subclass preferences for groupSet.prefer. They are package-level slices so
// that selecting one does not allocate when decoding R12 files.
var (
	prefHeader     = []string{""}
	prefEntity     = []string{"AcDbEntity"}
	prefLine       = []string{"AcDbLine"}
	prefPoint      = []string{"AcDbPoint"}
	prefCircle     = []string{"AcDbCircle"}
	prefArc        = []string{"AcDbArc"}
	prefText       = []string{"AcDbText"}
	prefTrace      = []string{"AcDbTrace"}
	prefFace       = []string{"AcDbFace"}
	prefInsert     = []string{"AcDbBlockReference", "AcDbMInsertBlock"}
	prefPolyline   = []string{"AcDb2dPolyline", "AcDb3dPolyline", "AcDbPolyFaceMesh", "AcDbPolygonMesh"}
	prefVertex     = []string{"AcDb2dVertex", "AcDb3dPolylineVertex", "AcDbPolygonMeshVertex", "AcDbPolyFaceMeshVertex"}
	prefAttribute  = []string{"AcDbAttribute"}
	prefBlockBegin = []string{"AcDbBlockBegin"}
)

func (s *groupSet) preferred(i int) bool {
	name := ""
	if ord := s.sub[i]; ord > 0 {
		name = s.names[ord-1]
	}
	for _, p := range s.pref {
		if p == name {
			return true
		}
	}
	return false
}

// owner consumes the first 330 group that sits before the first marker and
// outside any 102 group, and returns it as a Handle. A 330 that is not a valid
// handle is left alone. It does nothing for records without markers, where 330
// has no owner meaning.
func (s *groupSet) owner() (Handle, bool) {
	if s.sub == nil {
		return 0, false
	}
	for i, g := range s.groups {
		if g.Code != groupOwnerHandle || s.blocked[i] || s.sub[i] != 0 {
			continue
		}
		h, err := ParseHandle(g.Value)
		if err != nil {
			return 0, false
		}
		s.used[i] = true
		return h, true
	}
	return 0, false
}

// mark returns the first group with the given code and marks it as consumed.
// Further groups with the same code stay unmarked, so they are preserved in the
// leftovers instead of being dropped.
func (s *groupSet) mark(code int) (Group, bool) {
	if s.sub != nil {
		return s.markSubclass(code)
	}
	for i, g := range s.groups {
		if g.Code == code {
			s.used[i] = true
			return g, true
		}
	}
	return Group{}, false
}

// markSubclass is mark for records with subclass markers: it skips the groups
// inside 102 groups and prefers the subclasses named by prefer.
func (s *groupSet) markSubclass(code int) (Group, bool) {
	fallback := -1
	for i, g := range s.groups {
		if g.Code != code || s.blocked[i] {
			continue
		}
		if len(s.pref) == 0 || s.preferred(i) {
			s.used[i] = true
			return g, true
		}
		if fallback < 0 {
			fallback = i
		}
	}
	if fallback >= 0 {
		s.used[fallback] = true
		return s.groups[fallback], true
	}
	return Group{}, false
}

func (s *groupSet) str(code int) (string, bool) {
	g, ok := s.mark(code)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(g.Value), true
}

// raw returns the value without trimming surrounding spaces. It is used for
// free text, whose content must survive a round-trip unchanged.
func (s *groupSet) raw(code int) (string, bool) {
	g, ok := s.mark(code)
	if !ok {
		return "", false
	}
	return g.Value, true
}

func (s *groupSet) float(code int) (float64, bool, error) {
	g, ok := s.mark(code)
	if !ok {
		return 0, false, nil
	}
	v, err := g.Float()
	if err != nil {
		return 0, true, err
	}
	return v, true, nil
}

func (s *groupSet) int(code int) (int, bool, error) {
	g, ok := s.mark(code)
	if !ok {
		return 0, false, nil
	}
	v, err := g.Int()
	if err != nil {
		return 0, true, err
	}
	return v, true, nil
}

func (s *groupSet) point(xCode, yCode, zCode int) (Point3D, bool, error) {
	var p Point3D
	ok := false
	if v, has, err := s.float(xCode); err != nil {
		return p, true, err
	} else if has {
		p.X = v
		ok = true
	}
	if v, has, err := s.float(yCode); err != nil {
		return p, true, err
	} else if has {
		p.Y = v
		ok = true
	}
	if v, has, err := s.float(zCode); err != nil {
		return p, true, err
	} else if has {
		p.Z = v
		ok = true
	}
	return p, ok, nil
}

func (s *groupSet) leftovers() []Group {
	var out []Group
	for i, g := range s.groups {
		if !s.used[i] {
			out = append(out, g)
		}
	}
	return out
}

// leftoverSubclasses returns, for each group leftovers returns, the number of
// markers before it. It returns nil for records without markers or when there
// are no leftovers.
func (s *groupSet) leftoverSubclasses() []int {
	if s.sub == nil {
		return nil
	}
	var out []int
	for i := range s.groups {
		if !s.used[i] {
			out = append(out, s.sub[i])
		}
	}
	return out
}

// fill stores the unconsumed groups, with their subclass positions, in c.
func (s *groupSet) fill(c *EntityCommon) {
	c.Raw = s.leftovers()
	c.RawSubclass = s.leftoverSubclasses()
}

func (s *groupSet) common() (EntityCommon, error) {
	var c EntityCommon
	c.Subclasses = s.names
	s.prefer(prefEntity)
	if v, ok := s.str(groupLayer); ok {
		c.LayerName = v
	}
	if v, ok := s.str(groupLinetype); ok {
		c.Linetype = v
	}
	if v, ok, err := s.int(groupColor); err != nil {
		return c, err
	} else if ok {
		c.Color = v
		c.HasColor = true
	}
	s.prefer(prefHeader)
	if v, ok := s.str(groupHandle); ok {
		c.Handle = v
	}
	if h, ok := s.owner(); ok {
		c.Owner = h
	}
	s.prefer(prefEntity)
	if v, ok, err := s.int(groupInvisible); err != nil {
		return c, err
	} else if ok {
		c.Invisible = v != 0
	}
	s.prefer(nil)
	if v, ok, err := s.float(groupThickness); err != nil {
		return c, err
	} else if ok {
		c.Thickness = v
	}
	if p, ok, err := s.point(groupExtrusionX, groupExtrusionY, groupExtrusionZ); err != nil {
		return c, err
	} else if ok {
		c.Extrusion = p
		c.ExtrusionSet = true
	}
	return c, nil
}

// decodeEntity decodes one entity from the groups that follow its type record.
// Records with subclass markers are recognised by the markers themselves.
func decodeEntity(typeName string, groups []Group) (Entity, error) {
	return decodeEntityMode(typeName, groups, false)
}

// decodeEntityMode is decodeEntity with modern set when the file version is
// R13 or later, so that a record without markers is still read as such.
func decodeEntityMode(typeName string, groups []Group, modern bool) (Entity, error) {
	gs := newGroupSetMode(groups, modern)
	upper := strings.ToUpper(strings.TrimSpace(typeName))
	switch upper {
	case "LINE", "POINT", "CIRCLE", "ARC", "TEXT", "SOLID", "TRACE", "3DFACE", "INSERT":
	default:
		if gs.sub != nil {
			return decodeVerbatim(typeName, groups, gs), nil
		}
	}
	common, err := gs.common()
	if err != nil {
		return nil, err
	}
	switch upper {
	case "LINE":
		return decodeLine(gs, common)
	case "POINT":
		return decodePoint(gs, common)
	case "CIRCLE":
		return decodeCircle(gs, common)
	case "ARC":
		return decodeArc(gs, common)
	case "TEXT":
		return decodeText(gs, common)
	case "SOLID":
		return decodeSolid(gs, common)
	case "TRACE":
		return decodeTrace(gs, common)
	case "3DFACE":
		return decodeFace3D(gs, common)
	case "INSERT":
		return decodeInsert(gs, common, nil)
	default:
		common.Raw = gs.leftovers()
		return &RawEntity{EntityCommon: common, TypeName: strings.TrimSpace(typeName), Groups: common.Raw}, nil
	}
}

// decodeVerbatim builds the RawEntity of a record with subclass markers: all
// groups are kept in Groups, and the common fields are only a view. Malformed
// numbers in the view are ignored, because the record is not interpreted.
func decodeVerbatim(typeName string, groups []Group, gs *groupSet) Entity {
	var c EntityCommon
	c.Subclasses = gs.names
	gs.prefer(prefEntity)
	if v, ok := gs.str(groupLayer); ok {
		c.LayerName = v
	}
	if v, ok := gs.str(groupLinetype); ok {
		c.Linetype = v
	}
	if v, ok, err := gs.int(groupColor); ok && err == nil {
		c.Color = v
		c.HasColor = true
	}
	if v, ok, err := gs.int(groupInvisible); ok && err == nil {
		c.Invisible = v != 0
	}
	gs.prefer(prefHeader)
	if v, ok := gs.str(groupHandle); ok {
		c.Handle = v
	}
	if h, ok := gs.owner(); ok {
		c.Owner = h
	}
	return &RawEntity{EntityCommon: c, TypeName: strings.TrimSpace(typeName), Groups: groups, Verbatim: true}
}

func decodeLine(gs *groupSet, common EntityCommon) (Entity, error) {
	gs.prefer(prefLine)
	start, _, err := gs.point(groupX, groupY, groupZ)
	if err != nil {
		return nil, err
	}
	end, _, err := gs.point(groupX2, groupY2, groupZ2)
	if err != nil {
		return nil, err
	}
	ent := &Line{EntityCommon: common, Start: start, End: end}
	gs.fill(&ent.EntityCommon)
	return ent, nil
}

func decodePoint(gs *groupSet, common EntityCommon) (Entity, error) {
	gs.prefer(prefPoint)
	position, _, err := gs.point(groupX, groupY, groupZ)
	if err != nil {
		return nil, err
	}
	ent := &Point{EntityCommon: common, Position: position}
	if v, ok, err := gs.float(groupRotation); err != nil {
		return nil, err
	} else if ok {
		ent.Angle = v
	}
	gs.fill(&ent.EntityCommon)
	return ent, nil
}

func decodeCircle(gs *groupSet, common EntityCommon) (Entity, error) {
	gs.prefer(prefCircle)
	center, _, err := gs.point(groupX, groupY, groupZ)
	if err != nil {
		return nil, err
	}
	ent := &Circle{EntityCommon: common, Center: center}
	if v, ok, err := gs.float(groupRadius); err != nil {
		return nil, err
	} else if ok {
		ent.Radius = v
	}
	gs.fill(&ent.EntityCommon)
	return ent, nil
}

func decodeArc(gs *groupSet, common EntityCommon) (Entity, error) {
	gs.prefer(prefCircle)
	center, _, err := gs.point(groupX, groupY, groupZ)
	if err != nil {
		return nil, err
	}
	ent := &Arc{EntityCommon: common, Center: center}
	if v, ok, err := gs.float(groupRadius); err != nil {
		return nil, err
	} else if ok {
		ent.Radius = v
	}
	gs.prefer(prefArc)
	if v, ok, err := gs.float(groupRotation); err != nil {
		return nil, err
	} else if ok {
		ent.StartAngle = v
	}
	if v, ok, err := gs.float(groupAngle2); err != nil {
		return nil, err
	} else if ok {
		ent.EndAngle = v
	}
	gs.fill(&ent.EntityCommon)
	return ent, nil
}

func decodeText(gs *groupSet, common EntityCommon) (Entity, error) {
	// Both subclasses of a TEXT are named AcDbText; 73 (vertical alignment)
	// lives in the second one and is found by the same preference.
	gs.prefer(prefText)
	ent := &Text{EntityCommon: common, XScale: 1}
	if v, ok := gs.raw(groupText); ok {
		ent.Value = v
	}
	position, _, err := gs.point(groupX, groupY, groupZ)
	if err != nil {
		return nil, err
	}
	ent.Position = position
	alignment, _, err := gs.point(groupX2, groupY2, groupZ2)
	if err != nil {
		return nil, err
	}
	ent.Alignment = alignment
	if v, ok, err := gs.float(groupRadius); err != nil {
		return nil, err
	} else if ok {
		ent.Height = v
	}
	if v, ok, err := gs.float(groupXScale); err != nil {
		return nil, err
	} else if ok {
		ent.XScale = v
	}
	if v, ok, err := gs.float(groupRotation); err != nil {
		return nil, err
	} else if ok {
		ent.Rotation = v
	}
	if v, ok, err := gs.float(groupAngle2); err != nil {
		return nil, err
	} else if ok {
		ent.Oblique = v
	}
	if v, ok := gs.str(groupStyle); ok {
		ent.Style = v
	}
	if v, ok, err := gs.int(groupFlags2); err != nil {
		return nil, err
	} else if ok {
		ent.GenerationFlags = v
	}
	if v, ok, err := gs.int(groupHAlign); err != nil {
		return nil, err
	} else if ok {
		ent.HAlign = v
	}
	if v, ok, err := gs.int(groupVAlign); err != nil {
		return nil, err
	} else if ok {
		ent.VAlign = v
	}
	gs.fill(&ent.EntityCommon)
	return ent, nil
}

func decodeSolid(gs *groupSet, common EntityCommon) (Entity, error) {
	gs.prefer(prefTrace)
	corners, err := decodeCorners(gs)
	if err != nil {
		return nil, err
	}
	ent := &Solid{EntityCommon: common, Corners: corners}
	gs.fill(&ent.EntityCommon)
	return ent, nil
}

func decodeTrace(gs *groupSet, common EntityCommon) (Entity, error) {
	gs.prefer(prefTrace)
	corners, err := decodeCorners(gs)
	if err != nil {
		return nil, err
	}
	ent := &Trace{EntityCommon: common, Corners: corners}
	gs.fill(&ent.EntityCommon)
	return ent, nil
}

func decodeFace3D(gs *groupSet, common EntityCommon) (Entity, error) {
	gs.prefer(prefFace)
	corners, err := decodeCorners(gs)
	if err != nil {
		return nil, err
	}
	ent := &Face3D{EntityCommon: common, Corners: corners}
	if v, ok, err := gs.int(groupFlags); err != nil {
		return nil, err
	} else if ok {
		ent.InvisibleEdges = v
	}
	gs.fill(&ent.EntityCommon)
	return ent, nil
}

func decodeCorners(gs *groupSet) ([4]Point3D, error) {
	var corners [4]Point3D
	codes := [4][3]int{
		{groupX, groupY, groupZ},
		{groupX2, groupY2, groupZ2},
		{groupX3, groupY3, groupZ3},
		{groupX4, groupY4, groupZ4},
	}
	for i, c := range codes {
		p, _, err := gs.point(c[0], c[1], c[2])
		if err != nil {
			return corners, err
		}
		corners[i] = p
	}
	return corners, nil
}

func decodeInsert(gs *groupSet, common EntityCommon, attributes []Attribute) (Entity, error) {
	ent := &Insert{
		EntityCommon: common,
		XScale:       1,
		YScale:       1,
		ZScale:       1,
		ColumnCount:  1,
		RowCount:     1,
		Attributes:   attributes,
	}
	gs.prefer(prefInsert)
	if v, ok := gs.str(groupName); ok {
		ent.BlockName = v
	}
	if _, _, err := gs.int(groupVerticesFollow); err != nil {
		return nil, err
	}
	position, _, err := gs.point(groupX, groupY, groupZ)
	if err != nil {
		return nil, err
	}
	ent.Position = position
	if v, ok, err := gs.float(groupXScale); err != nil {
		return nil, err
	} else if ok {
		ent.XScale = v
	}
	if v, ok, err := gs.float(groupBulge); err != nil {
		return nil, err
	} else if ok {
		ent.YScale = v
	}
	if v, ok, err := gs.float(groupZScale); err != nil {
		return nil, err
	} else if ok {
		ent.ZScale = v
	}
	if v, ok, err := gs.float(groupRotation); err != nil {
		return nil, err
	} else if ok {
		ent.Rotation = v
	}
	if v, ok, err := gs.int(groupFlags); err != nil {
		return nil, err
	} else if ok {
		ent.ColumnCount = v
	}
	if v, ok, err := gs.int(groupFlags2); err != nil {
		return nil, err
	} else if ok {
		ent.RowCount = v
	}
	if v, ok, err := gs.float(groupColumnSpacing); err != nil {
		return nil, err
	} else if ok {
		ent.ColumnSpacing = v
	}
	if v, ok, err := gs.float(groupRowSpacing); err != nil {
		return nil, err
	} else if ok {
		ent.RowSpacing = v
	}
	gs.fill(&ent.EntityCommon)
	return ent, nil
}

func decodePolyline(groups []Group, vertices []Vertex, seqendHandle string, seqendGroups []Group, modern bool) (Entity, error) {
	gs := newGroupSetMode(groups, modern)
	common, err := gs.common()
	if err != nil {
		return nil, err
	}
	ent := &Polyline{EntityCommon: common, Vertices: vertices, SeqendHandle: seqendHandle, SeqendGroups: seqendGroups}
	gs.prefer(prefPolyline)
	if _, _, err := gs.int(groupVerticesFollow); err != nil {
		return nil, err
	}
	if v, ok, err := gs.int(groupFlags); err != nil {
		return nil, err
	} else if ok {
		ent.Flags = v
	}
	elevation, _, err := gs.point(groupX, groupY, groupZ)
	if err != nil {
		return nil, err
	}
	ent.Elevation = elevation
	if v, ok, err := gs.float(groupWidthStart); err != nil {
		return nil, err
	} else if ok {
		ent.StartWidth = v
	}
	if v, ok, err := gs.float(groupWidthEnd); err != nil {
		return nil, err
	} else if ok {
		ent.EndWidth = v
	}
	gs.fill(&ent.EntityCommon)
	return ent, nil
}

func decodeVertex(groups []Group) (Vertex, error) { return decodeVertexMode(groups, false) }

func decodeVertexMode(groups []Group, modern bool) (Vertex, error) {
	gs := newGroupSetMode(groups, modern)
	var v Vertex
	v.Subclasses = gs.names
	gs.prefer(prefHeader)
	if s, ok := gs.str(groupHandle); ok {
		v.Handle = s
	}
	if h, ok := gs.owner(); ok {
		v.Owner = h
	}
	gs.prefer(prefVertex)
	position, _, err := gs.point(groupX, groupY, groupZ)
	if err != nil {
		return v, err
	}
	v.Position = position
	if f, ok, err := gs.float(groupBulge); err != nil {
		return v, err
	} else if ok {
		v.Bulge = f
	}
	if i, ok, err := gs.int(groupFlags); err != nil {
		return v, err
	} else if ok {
		v.Flags = i
	}
	if f, ok, err := gs.float(groupRotation); err != nil {
		return v, err
	} else if ok {
		v.Tangent = f
	}
	v.Raw = gs.leftovers()
	v.RawSubclass = gs.leftoverSubclasses()
	return v, nil
}

func decodeAttribute(groups []Group, modern bool) (Attribute, error) {
	gs := newGroupSetMode(groups, modern)
	var a Attribute
	a.XScale = 1
	a.Subclasses = gs.names
	gs.prefer(prefHeader)
	if v, ok := gs.str(groupHandle); ok {
		a.Handle = v
	}
	if h, ok := gs.owner(); ok {
		a.Owner = h
	}
	gs.prefer(prefText)
	if v, ok := gs.raw(groupText); ok {
		a.Value = v
	}
	position, _, err := gs.point(groupX, groupY, groupZ)
	if err != nil {
		return a, err
	}
	a.Position = position
	if v, ok, err := gs.float(groupRadius); err != nil {
		return a, err
	} else if ok {
		a.Height = v
	}
	if v, ok, err := gs.float(groupXScale); err != nil {
		return a, err
	} else if ok {
		a.XScale = v
	}
	if v, ok, err := gs.float(groupRotation); err != nil {
		return a, err
	} else if ok {
		a.Rotation = v
	}
	if v, ok, err := gs.float(groupAngle2); err != nil {
		return a, err
	} else if ok {
		a.Oblique = v
	}
	if v, ok := gs.str(groupStyle); ok {
		a.Style = v
	}
	// Tag, flags, field length and vertical alignment belong to the second
	// subclass, AcDbAttribute.
	gs.prefer(prefAttribute)
	if v, ok := gs.str(groupName); ok {
		a.Tag = v
	}
	if v, ok, err := gs.int(groupFlags); err != nil {
		return a, err
	} else if ok {
		a.Flags = v
	}
	if v, ok, err := gs.int(groupAttribVAlign); err != nil {
		return a, err
	} else if ok {
		a.VAlign = v
	}
	gs.prefer(prefText)
	if v, ok, err := gs.int(groupHAlign); err != nil {
		return a, err
	} else if ok {
		a.HAlign = v
	}
	gs.prefer(prefAttribute)
	if v, ok, err := gs.int(groupFieldLength); err != nil {
		return a, err
	} else if ok {
		a.FieldLength = v
	}
	a.Raw = gs.leftovers()
	a.RawSubclass = gs.leftoverSubclasses()
	return a, nil
}
