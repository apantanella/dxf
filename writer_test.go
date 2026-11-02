package dxf

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
)

func encodeString(t *testing.T, doc *Document, opts ...EncodeOption) string {
	t.Helper()
	var builder strings.Builder
	if err := Encode(&builder, doc, opts...); err != nil {
		t.Fatalf("Encode() = %v", err)
	}
	return builder.String()
}

func sampleDocument() *Document {
	doc := NewDocument()
	doc.Tables.EnsureLayer("STR-BEAMS", 1, "CONTINUOUS")
	doc.Tables.EnsureLayer("STR-PANELS", 3, "CONTINUOUS")
	doc.Blocks = []*Block{{
		Name:      "COLONNA",
		LayerName: "0",
		Entities: []Entity{
			&Circle{EntityCommon: EntityCommon{LayerName: "0"}, Center: Point3D{X: 1}, Radius: 1},
		},
	}}
	insert := NewInsert("COLONNA", Point3D{X: 1, Y: 2})
	insert.LayerName = "0"
	insert.Attributes = []Attribute{{Tag: "TAG", Value: "C1", Height: 2.5, XScale: 1}}
	doc.Entities = []Entity{
		&Line{EntityCommon: EntityCommon{LayerName: "STR-BEAMS"}, Start: Point3D{}, End: Point3D{X: 5}},
		&Point{EntityCommon: EntityCommon{LayerName: "0"}, Position: Point3D{X: 1, Y: 1}},
		&Circle{EntityCommon: EntityCommon{LayerName: "0"}, Center: Point3D{X: 2}, Radius: 1.5},
		&Arc{EntityCommon: EntityCommon{LayerName: "0"}, Center: Point3D{}, Radius: 2, StartAngle: 30, EndAngle: 120},
		&Text{EntityCommon: EntityCommon{LayerName: "0"}, Value: "BEAM 1", Position: Point3D{X: 1}, Height: 2.5, XScale: 1},
		&Solid{EntityCommon: EntityCommon{LayerName: "0"}, Corners: [4]Point3D{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}},
		&Trace{EntityCommon: EntityCommon{LayerName: "0"}, Corners: [4]Point3D{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}},
		&Face3D{EntityCommon: EntityCommon{LayerName: "0"}, Corners: [4]Point3D{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}, InvisibleEdges: 1},
		&Polyline{
			EntityCommon: EntityCommon{LayerName: "STR-PANELS"},
			Flags:        1,
			Vertices: []Vertex{
				{Position: Point3D{}},
				{Position: Point3D{X: 1}, Bulge: 0.5},
				{Position: Point3D{X: 1, Y: 1}},
			},
		},
		insert,
	}
	return doc
}

func TestEncodeMinimal(t *testing.T) {
	t.Parallel()

	doc := NewDocument()
	doc.Entities = []Entity{&Line{Start: Point3D{}, End: Point3D{X: 5}}}

	out := encodeString(t, doc)
	for _, want := range []string{
		"0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1009\n9\n$EXTMAX\n10\n5\n20\n0.0\n30\n0.0\n9\n$EXTMIN\n10\n0.0\n20\n0.0\n30\n0.0\n0\nENDSEC\n",
		"0\nSECTION\n2\nTABLES\n",
		"0\nLAYER\n2\n0\n",
		"0\nLTYPE\n2\nCONTINUOUS\n",
		"0\nSTYLE\n2\nSTANDARD\n",
		"0\nSECTION\n2\nBLOCKS\n0\nENDSEC\n",
		"0\nSECTION\n2\nENTITIES\n",
		"0\nLINE\n8\n0\n10\n0.0\n20\n0.0\n30\n0.0\n11\n5\n21\n0.0\n31\n0.0\n",
		"0\nEOF\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\noutput:\n%s", want, out)
		}
	}
	if !strings.HasSuffix(out, "0\nEOF\n") {
		t.Errorf("output non termina con EOF:\n%s", out)
	}
}

func TestEncodeRoundTripStable(t *testing.T) {
	t.Parallel()

	first := encodeString(t, sampleDocument())
	doc, err := Decode(strings.NewReader(first))
	if err != nil {
		t.Fatalf("Decode() = %v\noutput:\n%s", err, first)
	}
	if len(doc.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %v", doc.Diagnostics)
	}
	second := encodeString(t, doc)
	if first != second {
		t.Errorf("encoding is not stable after the round-trip\ndiff:\n%s", diffStrings(first, second))
	}
}

func diffStrings(first, second string) string {
	firstLines := strings.Split(first, "\n")
	secondLines := strings.Split(second, "\n")
	var builder strings.Builder
	for i := 0; i < len(firstLines) || i < len(secondLines); i++ {
		var a, b string
		if i < len(firstLines) {
			a = firstLines[i]
		}
		if i < len(secondLines) {
			b = secondLines[i]
		}
		if a != b {
			builder.WriteString("line ")
			builder.WriteString(strconv.Itoa(i + 1))
			builder.WriteString(": ")
			builder.WriteString(a)
			builder.WriteString(" != ")
			builder.WriteString(b)
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func TestEncodeValidation(t *testing.T) {
	t.Parallel()

	docWithInsert := func(blockName string) *Document {
		doc := NewDocument()
		doc.Entities = []Entity{NewInsert(blockName, Point3D{})}
		return doc
	}

	tests := []struct {
		name    string
		doc     func() *Document
		wantErr error
	}{
		{
			name: "unsupported version",
			doc: func() *Document {
				doc := NewDocument()
				doc.Version = VersionR14 // R2000 and later are writable since T9
				return doc
			},
			wantErr: ErrUnsupportedVersion,
		},
		{
			name: "inconsistent header version",
			doc: func() *Document {
				doc := NewDocument()
				doc.Version = ""
				doc.Header.Set("$ACADVER", 1, VersionR13)
				return doc
			},
			wantErr: ErrUnsupportedVersion,
		},
		{
			name:    "INSERT without block",
			doc:     func() *Document { return docWithInsert("INESISTENTE") },
			wantErr: ErrValidation,
		},
		{
			name: "INSERT with zero scale",
			doc: func() *Document {
				doc := NewDocument()
				doc.Blocks = []*Block{{Name: "B"}}
				doc.Entities = []Entity{&Insert{BlockName: "B"}}
				return doc
			},
			wantErr: ErrValidation,
		},
		{
			name: "CIRCLE with zero radius",
			doc: func() *Document {
				doc := NewDocument()
				doc.Entities = []Entity{&Circle{}}
				return doc
			},
			wantErr: ErrValidation,
		},
		{
			name: "TEXT without height",
			doc: func() *Document {
				doc := NewDocument()
				doc.Entities = []Entity{&Text{Value: "x"}}
				return doc
			},
			wantErr: ErrValidation,
		},
		{
			name: "POLYLINE without vertices",
			doc: func() *Document {
				doc := NewDocument()
				doc.Entities = []Entity{&Polyline{}}
				return doc
			},
			wantErr: ErrValidation,
		},
		{
			name: "closed POLYLINE with two vertices",
			doc: func() *Document {
				doc := NewDocument()
				doc.Entities = []Entity{&Polyline{Flags: 1, Vertices: []Vertex{{}, {Position: Point3D{X: 1}}}}}
				return doc
			},
			wantErr: ErrValidation,
		},
		{
			name: "non-finite coordinates",
			doc: func() *Document {
				doc := NewDocument()
				doc.Entities = []Entity{&Line{End: Point3D{X: math.NaN()}}}
				return doc
			},
			wantErr: ErrValidation,
		},
		{
			name: "invalid handle",
			doc: func() *Document {
				doc := NewDocument()
				doc.Entities = []Entity{&Line{EntityCommon: EntityCommon{Handle: "ZZ"}}}
				return doc
			},
			wantErr: ErrValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var builder strings.Builder
			err := Encode(&builder, tt.doc())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Encode() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestEncodeNilArguments(t *testing.T) {
	t.Parallel()

	if err := Encode(nil, NewDocument()); !errors.Is(err, ErrValidation) {
		t.Fatalf("Encode(nil, doc) = %v, want ErrValidation", err)
	}
	var builder strings.Builder
	if err := Encode(&builder, nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("Encode(w, nil) = %v, want ErrValidation", err)
	}
}

func TestEncodeAddsMissingLayer(t *testing.T) {
	t.Parallel()

	doc := NewDocument()
	doc.Entities = []Entity{&Line{EntityCommon: EntityCommon{LayerName: "STR-COLUMNS"}, End: Point3D{X: 3}}}

	out := encodeString(t, doc)
	if !strings.Contains(out, "0\nLAYER\n2\nSTR-COLUMNS\n") {
		t.Errorf("layer missing from output:\n%s", out)
	}
	decoded, err := Decode(strings.NewReader(out))
	if err != nil {
		t.Fatalf("Decode() = %v", err)
	}
	layers := decoded.Tables.LayerNames()
	found := false
	for _, layer := range layers {
		if layer == "STR-COLUMNS" {
			found = true
		}
	}
	if !found {
		t.Errorf("layer decodificati = %v", layers)
	}
}

func TestEncodeWithPrecision(t *testing.T) {
	t.Parallel()

	doc := NewDocument()
	doc.Entities = []Entity{&Point{Position: Point3D{X: 1.23456, Y: 0.0001}}}

	out := encodeString(t, doc, WithPrecision(2))
	if !strings.Contains(out, "10\n1.23\n") {
		t.Errorf("precision not applied:\n%s", out)
	}
	if !strings.Contains(out, "20\n0.00\n") {
		t.Errorf("precision not applied to Y:\n%s", out)
	}
}

func TestEncodeHandles(t *testing.T) {
	t.Parallel()

	doc := NewDocument()
	doc.Entities = []Entity{
		&Line{EntityCommon: EntityCommon{Handle: "10"}, End: Point3D{X: 1}},
		&Line{End: Point3D{X: 2}},
	}

	first := encodeString(t, doc)
	if !strings.Contains(first, "9\n$HANDLING\n70\n1\n") {
		t.Fatalf("$HANDLING missing:\n%s", first)
	}
	if !strings.Contains(first, "9\n$HANDSEED\n5\n12\n") {
		t.Fatalf("wrong $HANDSEED:\n%s", first)
	}

	decoded, err := Decode(strings.NewReader(first))
	if err != nil {
		t.Fatalf("Decode() = %v", err)
	}
	second := encodeString(t, decoded)
	if first != second {
		t.Errorf("handle round-trip is not stable\ndiff:\n%s", diffStrings(first, second))
	}
}

func TestFormatFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value     float64
		precision int
		want      string
	}{
		{value: 0, precision: -1, want: "0.0"},
		{value: 5, precision: -1, want: "5"},
		{value: -0.5, precision: -1, want: "-0.5"},
		{value: 1.239, precision: 2, want: "1.24"},
		{value: 1000000, precision: -1, want: "1000000"},
	}

	for _, tt := range tests {
		if got := formatFloat(tt.value, tt.precision); got != tt.want {
			t.Errorf("formatFloat(%v, %d) = %q, want %q", tt.value, tt.precision, got, tt.want)
		}
	}
}
