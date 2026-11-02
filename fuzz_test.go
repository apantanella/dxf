package dxf

import (
	"strings"
	"testing"
)

func FuzzDecode(f *testing.F) {
	f.Add(minimalDXF)
	f.Add(polylineDXF)
	f.Add(insertDXF)
	f.Add("0\nSECTION\n2\nENTITIES\n0\nENDSEC\n0\nEOF\n")
	f.Add("0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1015\n0\nENDSEC\n0\nEOF\n")
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
