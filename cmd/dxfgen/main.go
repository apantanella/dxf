// Command dxfgen writes DXF R12 verification files.
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
	kindEntities = "entities"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("dxfgen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	out := flags.String("out", "dxfgen.dxf", "output file path")
	kind := flags.String("kind", kindFrame, "document kind: frame or entities")
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
	case kindEntities:
		doc, err = buildEntities()
	default:
		fmt.Fprintf(stderr, "dxfgen: unknown kind %q, want %q or %q\n", *kind, kindFrame, kindEntities)
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

	if _, err := model.AddPanel([]dxf.Point3D{
		{X: 0, Y: 0, Z: 3},
		{X: 5, Y: 0, Z: 3},
		{X: 5, Y: 4, Z: 3},
		{X: 0, Y: 4, Z: 3},
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
