package structural

import (
	"fmt"

	"github.com/apantanella/dxf"
)

// LayerMembers is used for members whose kind is not classified.
const LayerMembers = "STR-MEMBERS"

// Export converts a structural model into a DXF document using mapping. The
// document has the version of dxf.NewDocument (R2013 since F2): write it with
// dxf.Encode, or pass dxf.WithVersion(dxf.VersionR12) to write it as R12.
//
// Coordinates are converted from the model unit to mapping.Unit. If mapping.Unit
// is UnitUnknown, coordinates are written in the model unit unchanged.
func Export(model *Model, mapping Mapping) (*dxf.Document, error) {
	if err := model.Validate(); err != nil {
		return nil, err
	}
	documentUnit := mapping.Unit
	if documentUnit == UnitUnknown {
		documentUnit = model.Unit
	}

	convert := func(point dxf.Point3D) (dxf.Point3D, error) {
		if mapping.Unit == UnitUnknown || model.Unit == UnitUnknown {
			return point, nil
		}
		x, err := Convert(point.X, model.Unit, mapping.Unit)
		if err != nil {
			return dxf.Point3D{}, err
		}
		y, err := Convert(point.Y, model.Unit, mapping.Unit)
		if err != nil {
			return dxf.Point3D{}, err
		}
		z, err := Convert(point.Z, model.Unit, mapping.Unit)
		if err != nil {
			return dxf.Point3D{}, err
		}
		return dxf.Point3D{X: x, Y: y, Z: z}, nil
	}

	labelHeight, err := Convert(mapping.labelHeight(), model.Unit, documentUnit)
	if err != nil || labelHeight <= 0 {
		labelHeight = mapping.labelHeight()
	}

	doc := dxf.NewDocument()
	for _, layer := range LayerStyles() {
		doc.Tables.EnsureLayer(layer.Name, layer.Color, layer.Linetype)
	}

	label := func(text string, position dxf.Point3D) {
		doc.Entities = append(doc.Entities, &dxf.Text{
			EntityCommon: dxf.EntityCommon{LayerName: LayerLabels},
			Value:        text,
			Position:     position,
			Height:       labelHeight,
			XScale:       1,
		})
	}

	for _, node := range model.Nodes {
		position, err := convert(node.Position)
		if err != nil {
			return nil, err
		}
		doc.Entities = append(doc.Entities, &dxf.Point{
			EntityCommon: dxf.EntityCommon{LayerName: LayerNodes},
			Position:     position,
		})
		if node.Label != "" {
			label(node.Label, position)
		}
	}

	for _, member := range model.Members {
		start, ok := model.Node(member.Start)
		if !ok {
			return nil, fmt.Errorf("structural: member %q references node %q", member.ID, member.Start)
		}
		end, ok := model.Node(member.End)
		if !ok {
			return nil, fmt.Errorf("structural: member %q references node %q", member.ID, member.End)
		}
		startPoint, err := convert(start.Position)
		if err != nil {
			return nil, err
		}
		endPoint, err := convert(end.Position)
		if err != nil {
			return nil, err
		}
		doc.Entities = append(doc.Entities, &dxf.Line{
			EntityCommon: dxf.EntityCommon{LayerName: memberLayer(member.Kind)},
			Start:        startPoint,
			End:          endPoint,
		})
		if member.Label != "" {
			midpoint := dxf.Point3D{
				X: (startPoint.X + endPoint.X) / 2,
				Y: (startPoint.Y + endPoint.Y) / 2,
				Z: (startPoint.Z + endPoint.Z) / 2,
			}
			label(member.Label, midpoint)
		}
	}

	for _, panel := range model.Panels {
		vertices := make([]dxf.Vertex, 0, len(panel.Boundary))
		for _, point := range panel.Boundary {
			converted, err := convert(point)
			if err != nil {
				return nil, err
			}
			vertices = append(vertices, dxf.Vertex{Position: converted})
		}
		doc.Entities = append(doc.Entities, &dxf.Polyline{
			EntityCommon: dxf.EntityCommon{LayerName: LayerPanels},
			Flags:        1,
			Vertices:     vertices,
		})
		if panel.Label != "" {
			label(panel.Label, vertices[0].Position)
		}
	}

	for _, grid := range model.Grids {
		start, err := convert(grid.Start)
		if err != nil {
			return nil, err
		}
		end, err := convert(grid.End)
		if err != nil {
			return nil, err
		}
		doc.Entities = append(doc.Entities, &dxf.Line{
			EntityCommon: dxf.EntityCommon{LayerName: LayerGrids},
			Start:        start,
			End:          end,
		})
		if grid.Label != "" {
			label(grid.Label, start)
		}
	}

	return doc, nil
}

func memberLayer(kind MemberKind) string {
	switch kind {
	case MemberBeam:
		return LayerBeams
	case MemberColumn:
		return LayerColumns
	case MemberBrace:
		return LayerBraces
	default:
		return LayerMembers
	}
}
