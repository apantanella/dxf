package dxf

import (
	"os"
	"strings"
	"testing"
)

// r2018Seed is a small R2018 file with reactors, an extension dictionary,
// extended data, an unknown entity and unknown objects.
var r2018Seed = t6File(VersionR2018,
	"CLASSES", "0 CLASS\n1 XRECORD\n2 AcDbXrecord\n3 ObjectDBX Classes\n90 0\n91 0\n280 0\n281 0",
	"ENTITIES", `0 LINE
5 1F
102 {ACAD_XDICTIONARY
360 4A
102 }
102 {ACAD_REACTORS
330 4B
102 }
330 17
100 AcDbEntity
8 0
100 AcDbLine
10 0.0
20 0.0
30 0.0
11 100.0
21 50.0
31 0.0
1001 ACAD
1000 note
0 LWPOLYLINE
5 22
330 17
100 AcDbEntity
8 0
100 AcDbPolyline
90 2
10 0.0
20 0.0
10 5.0
20 0.0
0 POLYLINE
5 30
330 17
100 AcDbEntity
100 AcDb2dPolyline
66 1
0 VERTEX
5 31
330 30
100 AcDbEntity
100 AcDbVertex
100 AcDb2dVertex
10 1.0
20 2.0
0 SEQEND
5 32
330 30
100 AcDbEntity`,
	"OBJECTS", "0 XRECORD\n5 70\n102 {ACAD_REACTORS\n330 71\n102 }\n330 72\n100 AcDbXrecord\n280 1\n1 data")

func FuzzDecode(f *testing.F) {
	f.Add(minimalDXF)
	f.Add(polylineDXF)
	f.Add(insertDXF)
	f.Add("0\nSECTION\n2\nENTITIES\n0\nENDSEC\n0\nEOF\n")
	f.Add("0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1015\n0\nENDSEC\n0\nEOF\n")
	f.Add(r2018Seed)
	if data, err := os.ReadFile("testdata/t1/v00-baseline.dxf"); err == nil {
		f.Add(string(data))
	}
	f.Add("AutoCAD Binary DXF\r\n\x1a\x00")
	f.Add("")
	f.Add("\x00\x01\x02")

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 64*1024 {
			t.Skip()
		}
		limits := Limits{
			MaxEntities:     10_000,
			MaxBlocks:       1_000,
			MaxValueBytes:   4096,
			MaxNestingDepth: 8,
		}
		doc, err := Decode(strings.NewReader(input), WithLenient(), WithLimits(limits))
		if err != nil {
			return
		}
		// A successfully decoded document must be encodable without panics.
		var builder strings.Builder
		_ = Encode(&builder, doc)
	})
}
