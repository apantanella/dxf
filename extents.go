package dxf

import (
	"math"
	"strings"
)

// maxExtentsDepth bounds the recursion through nested INSERT entities, so that a
// block that (directly or indirectly) inserts itself cannot loop forever.
const maxExtentsDepth = 8

// Extents returns the bounding box of the model-space entities in WCS. ok is
// false when the document contains nothing that has a position.
//
// The box is conservative rather than exact: arcs and circles contribute their
// full circle, text contributes an estimate of its width, and arrays of inserts
// contribute only their first element. That is enough for the "zoom to extents"
// of a CAD application, which is what $EXTMIN and $EXTMAX are used for.
func (d *Document) Extents() (min, max Point3D, ok bool) {
	if d == nil {
		return Point3D{}, Point3D{}, false
	}
	blocks := make(map[string]*Block, len(d.Blocks))
	for _, block := range d.Blocks {
		if block != nil {
			blocks[normalizeBlockName(block.Name)] = block
		}
	}
	box := extentsBox{}
	for _, entity := range d.Entities {
		box.addEntity(entity, identityTransform, blocks, 0)
	}
	return box.min, box.max, box.ok
}

// UpdateExtents recomputes $EXTMIN and $EXTMAX from the current entities. It
// removes both variables when the document has no extents. Call it after
// editing a document that was read from a file: Encode only fills the two
// variables when they are missing.
func (d *Document) UpdateExtents() {
	if d == nil {
		return
	}
	min, max, ok := d.Extents()
	if !ok {
		d.Header.Delete("$EXTMIN")
		d.Header.Delete("$EXTMAX")
		return
	}
	d.Header.setPoint("$EXTMIN", min, -1)
	d.Header.setPoint("$EXTMAX", max, -1)
}

func (h *Header) setPoint(name string, p Point3D, precision int) {
	if h.Variables == nil {
		h.Variables = map[string]Variable{}
	}
	h.Variables[name] = Variable{Name: name, Groups: []Group{
		{Code: groupX, Value: formatFloat(p.X, precision)},
		{Code: groupY, Value: formatFloat(p.Y, precision)},
		{Code: groupZ, Value: formatFloat(p.Z, precision)},
	}}
}

type transform func(Point3D) Point3D

func identityTransform(p Point3D) Point3D { return p }

func normalizeBlockName(name string) string {
	return strings.ToUpper(strings.TrimSpace(name))
}

type extentsBox struct {
	min, max Point3D
	ok       bool
}

func (b *extentsBox) add(p Point3D) {
	if !finite(p.X) || !finite(p.Y) || !finite(p.Z) {
		return
	}
	if !b.ok {
		b.min, b.max, b.ok = p, p, true
		return
	}
	b.min = Point3D{X: math.Min(b.min.X, p.X), Y: math.Min(b.min.Y, p.Y), Z: math.Min(b.min.Z, p.Z)}
	b.max = Point3D{X: math.Max(b.max.X, p.X), Y: math.Max(b.max.Y, p.Y), Z: math.Max(b.max.Z, p.Z)}
}

func (b *extentsBox) addEntity(entity Entity, xf transform, blocks map[string]*Block, depth int) {
	if entity == nil {
		return
	}
	add := func(p Point3D) { b.add(xf(p)) }
	ocs := func(p Point3D, c *EntityCommon) Point3D { return OCS2WCS(p, c.Normal()) }
	switch typed := entity.(type) {
	case *Line:
		add(typed.Start)
		add(typed.End)
	case *Point:
		add(typed.Position)
	case *Circle:
		b.addRound(typed.Center, typed.Radius, &typed.EntityCommon, xf)
	case *Arc:
		b.addRound(typed.Center, typed.Radius, &typed.EntityCommon, xf)
	case *Text:
		position := ocs(typed.Position, &typed.EntityCommon)
		add(position)
		// Estimated end of the string: 0.6 of the height per character.
		width := float64(len(typed.Value)) * typed.Height * 0.6 * nonZero(typed.XScale)
		angle := typed.Rotation * math.Pi / 180
		add(Point3D{
			X: position.X + width*math.Cos(angle) - typed.Height*math.Sin(angle),
			Y: position.Y + width*math.Sin(angle) + typed.Height*math.Cos(angle),
			Z: position.Z,
		})
	case *Solid:
		for _, corner := range typed.Corners {
			add(ocs(corner, &typed.EntityCommon))
		}
	case *Trace:
		for _, corner := range typed.Corners {
			add(ocs(corner, &typed.EntityCommon))
		}
	case *Face3D:
		for _, corner := range typed.Corners {
			add(corner)
		}
	case *Polyline:
		for _, vertex := range typed.Vertices {
			// Polyface face records (flag 128 without 64) carry indices, not points.
			if vertex.Flags&128 != 0 && vertex.Flags&64 == 0 {
				continue
			}
			add(polylineVertexWCS(typed, vertex))
		}
	case *Insert:
		b.addInsert(typed, xf, blocks, depth)
	}
}

// addRound adds the bounding box of a circle or arc, taken as a full circle.
func (b *extentsBox) addRound(center Point3D, radius float64, common *EntityCommon, xf transform) {
	c := OCS2WCS(center, common.Normal())
	if isDefaultNormal(common.Normal()) {
		b.add(xf(Point3D{X: c.X - radius, Y: c.Y - radius, Z: c.Z}))
		b.add(xf(Point3D{X: c.X + radius, Y: c.Y + radius, Z: c.Z}))
		return
	}
	b.add(xf(Point3D{X: c.X - radius, Y: c.Y - radius, Z: c.Z - radius}))
	b.add(xf(Point3D{X: c.X + radius, Y: c.Y + radius, Z: c.Z + radius}))
}

func (b *extentsBox) addInsert(insert *Insert, xf transform, blocks map[string]*Block, depth int) {
	block := blocks[normalizeBlockName(insert.BlockName)]
	if block == nil || depth >= maxExtentsDepth {
		b.add(xf(insert.Position))
		return
	}
	angle := insert.Rotation * math.Pi / 180
	cos, sin := math.Cos(angle), math.Sin(angle)
	local := func(p Point3D) Point3D {
		x := (p.X - block.BasePoint.X) * insert.XScale
		y := (p.Y - block.BasePoint.Y) * insert.YScale
		z := (p.Z - block.BasePoint.Z) * insert.ZScale
		return xf(Point3D{
			X: insert.Position.X + x*cos - y*sin,
			Y: insert.Position.Y + x*sin + y*cos,
			Z: insert.Position.Z + z,
		})
	}
	var inner extentsBox
	for _, entity := range block.Entities {
		inner.addEntity(entity, local, blocks, depth+1)
	}
	if !inner.ok {
		b.add(xf(insert.Position))
		return
	}
	b.add(inner.min)
	b.add(inner.max)
}

// polylineVertexWCS returns the WCS position of a polyline vertex. 2D polylines
// store their vertices in OCS with the elevation in the polyline record; 3D
// polylines, polygon meshes and polyface meshes store them in WCS.
func polylineVertexWCS(polyline *Polyline, vertex Vertex) Point3D {
	if polyline.Flags&(8|16|64) != 0 {
		return vertex.Position
	}
	ocs := Point3D{X: vertex.Position.X, Y: vertex.Position.Y, Z: polyline.Elevation.Z}
	return OCS2WCS(ocs, polyline.Normal())
}

func nonZero(value float64) float64 {
	if value == 0 {
		return 1
	}
	return value
}
