package dxf

import (
	"strings"
	"testing"
)

// benchmarkEntities is the number of entities used by the benchmarks. The
// roadmap requires a benchmark with at least 100,000 entities.
const benchmarkEntities = 100_000

func buildBenchmarkInput(tb testing.TB, entities int) string {
	tb.Helper()
	var builder strings.Builder
	builder.WriteString("0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1009\n0\nENDSEC\n")
	builder.WriteString("0\nSECTION\n2\nENTITIES\n")
	for i := 0; i < entities; i++ {
		builder.WriteString("0\nLINE\n8\n0\n10\n0.0\n20\n0.0\n30\n0.0\n11\n1.0\n21\n0.0\n31\n0.0\n")
	}
	builder.WriteString("0\nENDSEC\n0\nEOF\n")
	return builder.String()
}

func BenchmarkDecodeLines(b *testing.B) {
	input := buildBenchmarkInput(b, benchmarkEntities)
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		doc, err := Decode(strings.NewReader(input))
		if err != nil {
			b.Fatalf("Decode() = %v", err)
		}
		if len(doc.Entities) != benchmarkEntities {
			b.Fatalf("entities = %d, want %d", len(doc.Entities), benchmarkEntities)
		}
	}
}

func BenchmarkEncodeLines(b *testing.B) {
	doc, err := Decode(strings.NewReader(buildBenchmarkInput(b, benchmarkEntities)))
	if err != nil {
		b.Fatalf("Decode() = %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var builder strings.Builder
		if err := Encode(&builder, doc); err != nil {
			b.Fatalf("Encode() = %v", err)
		}
	}
}

func BenchmarkRoundTripLines(b *testing.B) {
	input := buildBenchmarkInput(b, benchmarkEntities)
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		doc, err := Decode(strings.NewReader(input))
		if err != nil {
			b.Fatalf("Decode() = %v", err)
		}
		var builder strings.Builder
		if err := Encode(&builder, doc); err != nil {
			b.Fatalf("Encode() = %v", err)
		}
	}
}
