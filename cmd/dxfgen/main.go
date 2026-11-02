// Command dxfgen writes DXF verification files. The files are DXF R2013 ASCII,
// the version of dxf.NewDocument since F2; other versions are written by
// programs that call dxf.Encode with dxf.WithVersion.
//
// It generates two kinds of document:
//
//   - frame: a structural frame exported through the structural package, with
//     nodes, columns, beams, a brace, a slab, grids and labels on the STR-* layers.
//   - entities: every entity supported by the writer, each on its own layer, so
//     that a CAD application can be checked entity by entity.
//
// Usage:
//
//	dxfgen [-out file.dxf] [-kind frame|entities] [-unit mm|cm|m|in|ft] [-precision n]
//
// The exit status is 0 on success, 1 on a write error and 2 on invalid arguments.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/apantanella/dxf"
	"github.com/apantanella/dxf/structural"
)

const (
	kindFrame    = "frame"
	kindPlan     = "plan"
	kindEntities = "entities"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("dxfgen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "dxfgen.dxf", "output file path")
	kind := flags.String("kind", kindFrame, "document kind: frame, plan or entities")
	unitName := flags.String("unit", "m", "unit for the frame kind: mm, cm, m, in or ft")
	precision := flags.Int("precision", -1, "digits after the decimal point; negative writes the shortest form")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() > 0 {
		fmt.Fprintf(stderr, "dxfgen: unexpected argument %q\n", flags.Arg(0))
		return 2
	}

	unit, err := parseUnit(*unitName)
	if err != nil {
		fmt.Fprintf(stderr, "dxfgen: %v\n", err)
		return 2
	}

	var doc *dxf.Document
	switch strings.ToLower(*kind) {
	case kindFrame:
		doc, err = buildFrame(unit)
	case kindPlan:
		doc, err = buildPlan(unit)
	case kindEntities:
		doc, err = buildEntities()
	default:
		fmt.Fprintf(stderr, "dxfgen: unknown kind %q, want %q, %q or %q\n", *kind, kindFrame, kindPlan, kindEntities)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "dxfgen: %v\n", err)
		return 1
	}

	file, err := os.Create(*out)
	if err != nil {
		fmt.Fprintf(stderr, "dxfgen: %v\n", err)
		return 1
	}
	options := []dxf.EncodeOption{}
	if *precision >= 0 {
		options = append(options, dxf.WithPrecision(*precision))
	}
	encodeErr := dxf.Encode(file, doc, options...)
	closeErr := file.Close()
	if encodeErr != nil {
		fmt.Fprintf(stderr, "dxfgen: %v\n", encodeErr)
		return 1
	}
	if closeErr != nil {
		fmt.Fprintf(stderr, "dxfgen: %v\n", closeErr)
		return 1
	}

	fmt.Fprintf(stdout, "wrote %s (kind=%s, unit=%s, entities=%d)\n", *out, *kind, unit, len(doc.Entities))
	return 0
}

func parseUnit(name string) (structural.Unit, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "mm", "millimeter", "millimeters":
		return structural.UnitMillimeter, nil
	case "cm", "centimeter", "centimeters":
		return structural.UnitCentimeter, nil
	case "m", "meter", "meters":
		return structural.UnitMeter, nil
	case "in", "inch", "inches":
		return structural.UnitInch, nil
	case "ft", "foot", "feet":
		return structural.UnitFoot, nil
	default:
		return structural.UnitUnknown, fmt.Errorf("unknown unit %q, want mm, cm, m, in or ft", name)
	}
}

// buildFrame returns a single bay frame with a brace and a slab. The model is
// defined in meters and exported in the requested unit, so the generated file
// exercises the unit conversion of the export path.
func buildFrame(unit structural.Unit) (*dxf.Document, error) {
	model := &structural.Model{Unit: structural.UnitMeter}

	baseA := model.AddNode(dxf.Point3D{})
	topA := model.AddNode(dxf.Point3D{Z: 3})
	topB := model.AddNode(dxf.Point3D{X: 5, Z: 3})
	baseB := model.AddNode(dxf.Point3D{X: 5})
	topC := model.AddNode(dxf.Point3D{X: 5, Y: 4, Z: 3})
	topD := model.AddNode(dxf.Point3D{Y: 4, Z: 3})

	setLabel(model, baseA, "N1")
	setLabel(model, topB, "N3")

	if _, err := model.AddMember(baseA, topA, structural.MemberColumn); err != nil {
		return nil, err
	}
	if _, err := model.AddMember(baseB, topB, structural.MemberColumn); err != nil {
		return nil, err
	}
	if _, err := model.AddMember(topA, topB, structural.MemberBeam); err != nil {
		return nil, err
	}
	if _, err := model.AddMember(topB, topC, structural.MemberBeam); err != nil {
		return nil, err
	}
	if _, err := model.AddMember(topC, topD, structural.MemberBeam); err != nil {
		return nil, err
	}
	if _, err := model.AddMember(topD, topA, structural.MemberBeam); err != nil {
		return nil, err
	}
	if _, err := model.AddMember(baseA, topB, structural.MemberBrace); err != nil {
		return nil, err
	}

	// The slab is inset so that the beams stay visible in a plan view.
	if _, err := model.AddPanel([]dxf.Point3D{
		{X: 0.25, Y: 0.25, Z: 3},
		{X: 4.75, Y: 0.25, Z: 3},
		{X: 4.75, Y: 3.75, Z: 3},
		{X: 0.25, Y: 3.75, Z: 3},
	}, structural.PanelSlab); err != nil {
		return nil, err
	}

	model.Grids = append(model.Grids,
		structural.Grid{ID: "G1", Start: dxf.Point3D{X: -1}, End: dxf.Point3D{X: 6}, Label: "A"},
		structural.Grid{ID: "G2", Start: dxf.Point3D{Y: -1}, End: dxf.Point3D{Y: 5}, Label: "1"},
	)

	mapping := structural.DefaultMapping()
	mapping.Unit = unit
	return structural.Export(model, mapping)
}

func setLabel(model *structural.Model, id, label string) {
	for i := range model.Nodes {
		if model.Nodes[i].ID == id {
			model.Nodes[i].Label = label
		}
	}
}

// unitFactor returns how many drawing units one meter corresponds to.
func unitFactor(unit structural.Unit) (float64, error) {
	switch unit {
	case structural.UnitMillimeter:
		return 1000, nil
	case structural.UnitCentimeter:
		return 100, nil
	case structural.UnitMeter:
		return 1, nil
	case structural.UnitInch:
		return 1 / 0.0254, nil
	case structural.UnitFoot:
		return 1 / 0.3048, nil
	default:
		return 0, fmt.Errorf("unknown unit")
	}
}

// buildPlan returns a 2D plan of the same frame, so that every element is
// visible in the default view of a 2D CAD application: columns are square
// footprints, beams are lines and the slab is an inset closed polyline.
func buildPlan(unit structural.Unit) (*dxf.Document, error) {
	factor, err := unitFactor(unit)
	if err != nil {
		return nil, err
	}
	point := func(x, y float64) dxf.Point3D {
		return dxf.Point3D{X: x * factor, Y: y * factor}
	}

	doc := dxf.NewDocument()
	for _, layer := range structural.LayerStyles() {
		doc.Tables.EnsureLayer(layer.Name, layer.Color, layer.Linetype)
	}

	column := func(x, y float64) dxf.Entity {
		const half = 0.2
		return &dxf.Polyline{
			EntityCommon: dxf.EntityCommon{LayerName: structural.LayerColumns},
			Flags:        1,
			Vertices: []dxf.Vertex{
				{Position: point(x-half, y-half)},
				{Position: point(x+half, y-half)},
				{Position: point(x+half, y+half)},
				{Position: point(x-half, y+half)},
			},
		}
	}
	beam := func(x1, y1, x2, y2 float64) dxf.Entity {
		return &dxf.Line{
			EntityCommon: dxf.EntityCommon{LayerName: structural.LayerBeams},
			Start:        point(x1, y1),
			End:          point(x2, y2),
		}
	}

	doc.Entities = []dxf.Entity{
		column(0, 0),
		column(5, 0),
		column(5, 4),
		column(0, 4),
		beam(0, 0, 5, 0),
		beam(5, 0, 5, 4),
		beam(5, 4, 0, 4),
		beam(0, 4, 0, 0),
		&dxf.Polyline{
			EntityCommon: dxf.EntityCommon{LayerName: structural.LayerPanels},
			Flags:        1,
			Vertices: []dxf.Vertex{
				{Position: point(0.25, 0.25)},
				{Position: point(4.75, 0.25)},
				{Position: point(4.75, 3.75)},
				{Position: point(0.25, 3.75)},
			},
		},
		&dxf.Line{EntityCommon: dxf.EntityCommon{LayerName: structural.LayerGrids}, Start: point(-1, 0), End: point(6, 0)},
		&dxf.Line{EntityCommon: dxf.EntityCommon{LayerName: structural.LayerGrids}, Start: point(0, -1), End: point(0, 5)},
		planLabel("A", point(-1, 0.2), factor),
		planLabel("1", point(0.2, -1), factor),
		&dxf.Point{EntityCommon: dxf.EntityCommon{LayerName: structural.LayerNodes}, Position: point(0, 0)},
		&dxf.Point{EntityCommon: dxf.EntityCommon{LayerName: structural.LayerNodes}, Position: point(5, 0)},
		planLabel("N1", point(0, 0), factor),
		planLabel("N3", point(5, 0), factor),
	}
	return doc, nil
}

func planLabel(value string, position dxf.Point3D, factor float64) dxf.Entity {
	return &dxf.Text{
		EntityCommon: dxf.EntityCommon{LayerName: structural.LayerLabels},
		Value:        value,
		Position:     position,
		Height:       0.3 * factor,
		XScale:       1,
	}
}

// buildEntities returns one entity for every type supported by the writer.
func buildEntities() (*dxf.Document, error) {
	doc := dxf.NewDocument()
	layers := []string{
		"DXF-LINES", "DXF-POINTS", "DXF-CIRCLES", "DXF-ARCS", "DXF-POLYLINES",
		"DXF-TEXT", "DXF-SOLIDS", "DXF-FACES", "DXF-BLOCKS",
	}
	for _, layer := range layers {
		doc.Tables.EnsureLayer(layer, 3, "CONTINUOUS")
	}

	doc.Blocks = []*dxf.Block{{
		Name:      "SAMPLE",
		LayerName: "DXF-BLOCKS",
		Entities: []dxf.Entity{
			&dxf.Line{
				EntityCommon: dxf.EntityCommon{LayerName: "DXF-BLOCKS"},
				Start:        dxf.Point3D{X: -0.5},
				End:          dxf.Point3D{X: 0.5},
			},
		},
	}}

	insert := dxf.NewInsert("SAMPLE", dxf.Point3D{X: 12, Y: 6})
	insert.LayerName = "DXF-BLOCKS"
	insert.Rotation = 30
	insert.Attributes = []dxf.Attribute{{
		Tag:      "TAG",
		Value:    "C1",
		Position: dxf.Point3D{X: 12, Y: 6},
		Height:   0.5,
		XScale:   1,
	}}

	doc.Entities = []dxf.Entity{
		&dxf.Line{EntityCommon: dxf.EntityCommon{LayerName: "DXF-LINES"}, Start: dxf.Point3D{}, End: dxf.Point3D{X: 5, Z: 1}},
		&dxf.Point{EntityCommon: dxf.EntityCommon{LayerName: "DXF-POINTS"}, Position: dxf.Point3D{X: 1, Y: 1}},
		&dxf.Circle{EntityCommon: dxf.EntityCommon{LayerName: "DXF-CIRCLES"}, Center: dxf.Point3D{X: 3}, Radius: 1},
		&dxf.Arc{EntityCommon: dxf.EntityCommon{LayerName: "DXF-ARCS"}, Center: dxf.Point3D{X: 6}, Radius: 1.5, StartAngle: 30, EndAngle: 120},
		&dxf.Polyline{
			EntityCommon: dxf.EntityCommon{LayerName: "DXF-POLYLINES"},
			Flags:        1,
			Vertices: []dxf.Vertex{
				{Position: dxf.Point3D{X: 8}},
				{Position: dxf.Point3D{X: 10}, Bulge: 0.5},
				{Position: dxf.Point3D{X: 10, Y: 2}},
			},
		},
		&dxf.Text{
			EntityCommon: dxf.EntityCommon{LayerName: "DXF-TEXT"},
			Value:        "SAMPLE TEXT",
			Position:     dxf.Point3D{Y: 4},
			Height:       0.5,
			XScale:       1,
		},
		&dxf.Solid{
			EntityCommon: dxf.EntityCommon{LayerName: "DXF-SOLIDS"},
			Corners:      [4]dxf.Point3D{{Y: 6}, {X: 1, Y: 6}, {X: 1, Y: 7}, {Y: 7}},
		},
		&dxf.Trace{
			EntityCommon: dxf.EntityCommon{LayerName: "DXF-SOLIDS"},
			Corners:      [4]dxf.Point3D{{X: 2, Y: 6}, {X: 3, Y: 6}, {X: 3, Y: 7}, {X: 2, Y: 7}},
		},
		&dxf.Face3D{
			EntityCommon:   dxf.EntityCommon{LayerName: "DXF-FACES"},
			Corners:        [4]dxf.Point3D{{Y: 8, Z: 1}, {X: 1, Y: 8, Z: 1}, {X: 1, Y: 9, Z: 1}, {Y: 9, Z: 1}},
			InvisibleEdges: 1,
		},
		insert,
	}
	return doc, nil
}
