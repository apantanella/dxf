package structural

import (
	"fmt"
	"math"

	"github.com/apantanella/dxf"
)

// Import converts a DXF document into a structural model using mapping.
//
// Entities whose layer is not part of the mapping are skipped and recorded in
// the Report. Import fails only when the document is nil or the mapping is
// inconsistent with the data, never for a single unrecognized entity.
func Import(doc *dxf.Document, mapping Mapping) (*Model, Report, error) {
	if doc == nil {
		return nil, Report{}, fmt.Errorf("structural: nil document")
	}
	model := &Model{Unit: mapping.Unit}
	var report Report
	tolerance := mapping.snapTolerance()
	index := newNodeIndex(tolerance)

	addNode := func(position dxf.Point3D, label string) string {
		if id, ok := index.find(position); ok {
			if label != "" {
				setNodeLabel(model, id, label)
			}
			return id
		}
		id := model.AddNode(position)
		index.add(position, id)
		if label != "" {
			setNodeLabel(model, id, label)
		}
		return id
	}

	for _, entity := range doc.Entities {
		if entity == nil {
			continue
		}
		layer := entity.Layer()
		switch typed := entity.(type) {
		case *dxf.Point:
			if !mapping.inLayers(mapping.NodeLayers, layer) {
				report.skip(entity.Type(), layer, "layer not mapped for nodes")
				continue
			}
			addNode(typed.Position, "")
		case *dxf.Line:
			if mapping.inLayers(mapping.GridLayers, layer) {
				model.Grids = append(model.Grids, Grid{
					ID:    fmt.Sprintf("G%d", len(model.Grids)+1),
					Start: typed.Start,
					End:   typed.End,
				})
				continue
			}
			kind, ok := mapping.memberKind(layer)
			if !ok {
				report.skip(entity.Type(), layer, "layer not mapped for linear members")
				continue
			}
			if kind == MemberGeneric && mapping.ClassifyByOrientation {
				kind = classifyOrientation(typed.Start, typed.End)
			}
			startID := addNode(typed.Start, "")
			endID := addNode(typed.End, "")
			if startID == endID {
				report.ambiguous(entity.Type(), layer, "coincident ends within tolerance")
				continue
			}
			model.Members = append(model.Members, Member{
				ID:    fmt.Sprintf("M%d", len(model.Members)+1),
				Start: startID,
				End:   endID,
				Kind:  kind,
			})
		case *dxf.Text:
			if !mapping.inLayers(mapping.LabelLayers, layer) {
				report.skip(entity.Type(), layer, "layer not mapped for labels")
				continue
			}
			if !attachLabel(model, index, dxf.OCS2WCS(typed.Position, typed.Normal()), typed.Value, tolerance) {
				report.Diagnostics = append(report.Diagnostics, Diagnostic{
					Entity:  entity.Type(),
					Layer:   layer,
					Message: "label without a matching node or grid",
				})
			}
		case *dxf.Polyline:
			if !mapping.inLayers(mapping.PanelLayers, layer) {
				report.skip(entity.Type(), layer, "layer not mapped for panels")
				continue
			}
			if !typed.Closed() {
				report.skip(entity.Type(), layer, "open POLYLINE: not a panel boundary")
				continue
			}
			boundary := make([]dxf.Point3D, 0, len(typed.Vertices))
			for _, vertex := range typed.Vertices {
				boundary = append(boundary, polylinePoint(typed, vertex))
			}
			boundary = removeDuplicates(boundary, tolerance)
			if len(boundary) < 3 {
				report.skip(entity.Type(), layer, "boundary with fewer than three distinct points")
				continue
			}
			model.Panels = append(model.Panels, Panel{
				ID:       fmt.Sprintf("P%d", len(model.Panels)+1),
				Boundary: boundary,
			})
		default:
			report.skip(entity.Type(), layer, "entity type not handled by the structural model")
		}
	}

	if mapping.Unit == UnitUnknown {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{
			Message: "unit not declared: coordinates were not converted",
		})
	}
	return model, report, nil
}

func setNodeLabel(model *Model, id, label string) {
	for i := range model.Nodes {
		if model.Nodes[i].ID == id && model.Nodes[i].Label == "" {
			model.Nodes[i].Label = label
		}
	}
}

// attachLabel assigns an annotation to the node or grid at the given position.
func attachLabel(model *Model, index *nodeIndex, position dxf.Point3D, value string, tolerance float64) bool {
	if id, ok := index.find(position); ok {
		setNodeLabel(model, id, value)
		return true
	}
	for i := range model.Grids {
		if model.Grids[i].Start.Distance(position) <= tolerance || model.Grids[i].End.Distance(position) <= tolerance {
			if model.Grids[i].Label == "" {
				model.Grids[i].Label = value
			}
			return true
		}
	}
	return false
}

func classifyOrientation(start, end dxf.Point3D) MemberKind {
	length := start.Distance(end)
	if length == 0 {
		return MemberGeneric
	}
	vertical := math.Abs(end.Z-start.Z) / length
	switch {
	case vertical > 0.9:
		return MemberColumn
	case vertical < 0.2:
		return MemberBeam
	default:
		return MemberBrace
	}
}

func removeDuplicates(points []dxf.Point3D, tolerance float64) []dxf.Point3D {
	out := make([]dxf.Point3D, 0, len(points))
	for _, point := range points {
		if len(out) == 0 || out[len(out)-1].Distance(point) > tolerance {
			out = append(out, point)
		}
	}
	for len(out) > 1 && out[0].Distance(out[len(out)-1]) <= tolerance {
		out = out[:len(out)-1]
	}
	return out
}

// polylinePoint returns the WCS position of a polyline vertex. 2D polylines
// store their vertices in OCS, with the elevation in the polyline record; 3D
// polylines store their vertices in WCS.
func polylinePoint(polyline *dxf.Polyline, vertex dxf.Vertex) dxf.Point3D {
	if polyline.Flags&8 != 0 {
		return vertex.Position
	}
	ocs := dxf.Point3D{X: vertex.Position.X, Y: vertex.Position.Y, Z: polyline.Elevation.Z}
	return dxf.OCS2WCS(ocs, polyline.Normal())
}

// nodeIndex merges nodes whose positions coincide within a tolerance.
type nodeIndex struct {
	tolerance float64
	byKey     map[[3]int64]string
}

func newNodeIndex(tolerance float64) *nodeIndex {
	if tolerance <= 0 {
		tolerance = 1e-6
	}
	return &nodeIndex{tolerance: tolerance, byKey: map[[3]int64]string{}}
}

func (n *nodeIndex) key(position dxf.Point3D) [3]int64 {
	return [3]int64{
		int64(math.Round(position.X / n.tolerance)),
		int64(math.Round(position.Y / n.tolerance)),
		int64(math.Round(position.Z / n.tolerance)),
	}
}

func (n *nodeIndex) find(position dxf.Point3D) (string, bool) {
	id, ok := n.byKey[n.key(position)]
	return id, ok
}

func (n *nodeIndex) add(position dxf.Point3D, id string) {
	n.byKey[n.key(position)] = id
}
