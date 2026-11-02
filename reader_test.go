package dxf

import (
	"errors"
	"strings"
	"testing"
)

const minimalDXF = "0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1009\n0\nENDSEC\n" +
	"0\nSECTION\n2\nTABLES\n0\nTABLE\n2\nLAYER\n0\nLAYER\n2\n0\n70\n0\n62\n7\n6\nCONTINUOUS\n0\nENDTAB\n0\nENDSEC\n" +
	"0\nSECTION\n2\nBLOCKS\n0\nENDSEC\n" +
	"0\nSECTION\n2\nENTITIES\n" +
	"0\nLINE\n8\nSTR-BEAMS\n10\n0.0\n20\n0.0\n30\n0.0\n11\n5.0\n21\n0.0\n31\n0.0\n" +
	"0\nENDSEC\n0\nEOF\n"

const minimalLine = "0\nLINE\n8\nSTR-BEAMS\n10\n0.0\n20\n0.0\n30\n0.0\n11\n5.0\n21\n0.0\n31\n0.0\n"

const polylineDXF = "0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1009\n0\nENDSEC\n" +
	"0\nSECTION\n2\nENTITIES\n" +
	"0\nPOLYLINE\n8\nSTR-PANELS\n66\n1\n70\n1\n" +
	"0\nVERTEX\n10\n0.0\n20\n0.0\n" +
	"0\nVERTEX\n10\n1.0\n20\n0.0\n42\n0.5\n" +
	"0\nVERTEX\n10\n1.0\n20\n1.0\n" +
	"0\nSEQEND\n" +
	"0\nENDSEC\n0\nEOF\n"

const insertDXF = "0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1009\n0\nENDSEC\n" +
	"0\nSECTION\n2\nENTITIES\n" +
	"0\nINSERT\n8\n0\n2\nCOLONNA\n10\n1.0\n20\n2.0\n66\n1\n" +
	"0\nATTRIB\n1\nC1\n2\nTAG\n10\n1.0\n20\n2.0\n40\n2.5\n" +
	"0\nSEQEND\n" +
	"0\nENDSEC\n0\nEOF\n"

func decodeString(t *testing.T, input string, opts ...DecodeOption) *Document {
	t.Helper()
	doc, err := Decode(strings.NewReader(input), opts...)
	if err != nil {
		t.Fatalf("Decode() = %v", err)
	}
	return doc
}

func TestDecodeMinimal(t *testing.T) {
	t.Parallel()

	doc := decodeString(t, minimalDXF)
	if doc.Version != VersionR12 {
		t.Errorf("Version = %q, want %q", doc.Version, VersionR12)
	}
	if len(doc.Entities) != 1 {
		t.Fatalf("entities = %d, want 1", len(doc.Entities))
	}
	line, ok := doc.Entities[0].(*Line)
	if !ok {
		t.Fatalf("type = %T, want *Line", doc.Entities[0])
	}
	if line.Layer() != "STR-BEAMS" || line.End != (Point3D{X: 5}) {
		t.Errorf("line = %+v", line)
	}
	layers := doc.Tables.LayerNames()
	if len(layers) != 1 || layers[0] != "0" {
		t.Errorf("layers = %v", layers)
	}
	if len(doc.Diagnostics) != 0 {
		t.Errorf("unexpected diagnostics: %v", doc.Diagnostics)
	}
}

func TestDecodePolyline(t *testing.T) {
	t.Parallel()

	doc := decodeString(t, polylineDXF)
	if len(doc.Entities) != 1 {
		t.Fatalf("entities = %d, want 1", len(doc.Entities))
	}
	polyline, ok := doc.Entities[0].(*Polyline)
	if !ok {
		t.Fatalf("type = %T, want *Polyline", doc.Entities[0])
	}
	if len(polyline.Vertices) != 3 {
		t.Fatalf("vertices = %d, want 3", len(polyline.Vertices))
	}
	if !polyline.Closed() {
		t.Error("polyline should be closed")
	}
	if polyline.Vertices[1].Bulge != 0.5 {
		t.Errorf("bulge = %v", polyline.Vertices[1].Bulge)
	}
}

func TestDecodeInsertWithAttributes(t *testing.T) {
	t.Parallel()

	doc := decodeString(t, insertDXF)
	insert, ok := doc.Entities[0].(*Insert)
	if !ok {
		t.Fatalf("type = %T, want *Insert", doc.Entities[0])
	}
	if insert.BlockName != "COLONNA" || len(insert.Attributes) != 1 {
		t.Fatalf("insert = %+v", insert)
	}
	if insert.Attributes[0].Tag != "TAG" || insert.Attributes[0].Value != "C1" {
		t.Errorf("attribute = %+v", insert.Attributes[0])
	}
}

func TestDecodeUnknownEntityAndSection(t *testing.T) {
	t.Parallel()

	input := "0\nSECTION\n2\nHEADER\n9\n$ACADVER\n1\nAC1009\n0\nENDSEC\n" +
		"0\nSECTION\n2\nCLASSES\n0\nCLASS\n1\nSPLINE\n0\nENDSEC\n" +
		"0\nSECTION\n2\nENTITIES\n0\nSPLINE\n8\n0\n70\n8\n0\nENDSEC\n0\nEOF\n"
	doc := decodeString(t, input)
	// CLASSES is interpreted since T8.
	if len(doc.UnknownSections) != 0 || len(doc.Classes) != 1 || doc.Classes[0].DXFName != "SPLINE" {
		t.Fatalf("unknown sections = %+v, classes = %+v", doc.UnknownSections, doc.Classes)
	}
	raw, ok := doc.Entities[0].(*RawEntity)
	if !ok {
		t.Fatalf("type = %T, want *RawEntity", doc.Entities[0])
	}
	if raw.Type() != "SPLINE" || len(raw.Groups) != 1 || raw.Groups[0].Code != 70 {
		t.Errorf("raw entity = %+v", raw)
	}
}

func TestDecodeErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr error
	}{
		{
			name:    "not DXF",
			input:   "this is not a DXF file\n",
			wantErr: ErrNotDXF,
		},
		{
			name:    "unsupported version",
			input:   strings.Replace(minimalDXF, "AC1009", "AC1099", 1),
			wantErr: ErrUnsupportedVersion,
		},
		{
			name:    "missing EOF",
			input:   strings.Replace(minimalDXF, "0\nEOF\n", "", 1),
			wantErr: ErrSyntax,
		},
		{
			name:    "missing ACADVER",
			input:   strings.Replace(minimalDXF, "9\n$ACADVER\n1\nAC1009\n", "", 1),
			wantErr: ErrSyntax,
		},
		{
			name:    "VERTEX outside POLYLINE",
			input:   strings.Replace(minimalDXF, minimalLine, "0\nVERTEX\n10\n0.0\n20\n0.0\n", 1),
			wantErr: ErrSyntax,
		},
		{
			name:    "POLYLINE without SEQEND",
			input:   strings.Replace(minimalDXF, minimalLine, "0\nPOLYLINE\n66\n1\n0\nVERTEX\n10\n0.0\n20\n0.0\n", 1),
			wantErr: ErrSyntax,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Decode(strings.NewReader(tt.input))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Decode() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeLenient(t *testing.T) {
	t.Parallel()

	input := strings.Replace(minimalDXF, "AC1009", "AC1099", 1)
	input = strings.Replace(input, "0\nEOF\n", "", 1)
	doc := decodeString(t, input, WithLenient())
	if doc.Version != "AC1099" {
		t.Errorf("Version = %q", doc.Version)
	}
	if len(doc.Diagnostics) < 2 {
		t.Fatalf("diagnostics = %v", doc.Diagnostics)
	}
	for _, diagnostic := range doc.Diagnostics {
		if diagnostic.Severity != SeverityWarning {
			t.Errorf("severity = %v, want warning", diagnostic.Severity)
		}
	}
}

func TestDecodeLimits(t *testing.T) {
	t.Parallel()

	input := strings.Replace(minimalDXF, minimalLine,
		"0\nLINE\n10\n0.0\n20\n0.0\n11\n1.0\n21\n0.0\n0\nLINE\n10\n0.0\n20\n0.0\n11\n1.0\n21\n0.0\n", 1)
	limits := Limits{MaxEntities: 1, MaxBlocks: 1, MaxValueBytes: 1024, MaxNestingDepth: 1}
	_, err := Decode(strings.NewReader(input), WithLimits(limits))
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("Decode() = %v, want ErrLimit", err)
	}
}

func TestDecodeNilReader(t *testing.T) {
	t.Parallel()

	if _, err := Decode(nil); !errors.Is(err, ErrValidation) {
		t.Fatalf("Decode(nil) = %v, want ErrValidation", err)
	}
}
