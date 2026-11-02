package dxf

import (
	"fmt"
	"strings"
)

func ExampleDecode() {
	const input = "0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1009\n0\nENDSEC\n" +
		"0\nSECTION\n2\nENTITIES\n" +
		"0\nLINE\n8\n0\n10\n0.0\n20\n0.0\n11\n3.0\n21\n4.0\n" +
		"0\nENDSEC\n0\nEOF\n"

	doc, err := Decode(strings.NewReader(input))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	line, ok := doc.Entities[0].(*Line)
	if !ok {
		fmt.Println("unexpected entity")
		return
	}
	fmt.Printf("%s from (%.0f,%.0f) to (%.0f,%.0f), length %.0f\n",
		line.Type(), line.Start.X, line.Start.Y, line.End.X, line.End.Y, line.Length())
	// Output: LINE from (0,0) to (3,4), length 5
}

func ExampleEncode() {
	doc := NewDocument()
	doc.Entities = []Entity{
		&Line{EntityCommon: EntityCommon{LayerName: "STR-BEAMS"}, End: Point3D{X: 3, Y: 4}},
	}

	var builder strings.Builder
	if err := Encode(&builder, doc); err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(strings.Contains(builder.String(), "0\nLINE\n8\nSTR-BEAMS\n"))
	// Output: true
}

func ExampleDocument_Header() {
	doc := NewDocument()
	doc.Header.Set("$MEASUREMENT", 70, "1")
	version, _ := doc.Header.Get("$ACADVER")
	measurement, _ := doc.Header.Get("$MEASUREMENT")
	fmt.Println(version, measurement)
	// Output: AC1009 1
}
