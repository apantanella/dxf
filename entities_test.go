package dxf

import (
	"errors"
	"testing"
)

func TestGroupFloatAndInt(t *testing.T) {
	t.Parallel()

	if got, err := (Group{Code: 10, Value: " 1.5 "}).Float(); err != nil || got != 1.5 {
		t.Fatalf("Float() = %v, %v", got, err)
	}
	if got, err := (Group{Code: 70, Value: "2.0"}).Int(); err != nil || got != 2 {
		t.Fatalf("Int() = %v, %v", got, err)
	}
	if _, err := (Group{Code: 70, Value: "2.5"}).Int(); !errors.Is(err, ErrSyntax) {
		t.Fatalf("Int() on 2.5 = %v, want ErrSyntax", err)
	}
	if _, err := (Group{Code: 10, Value: "abc"}).Float(); !errors.Is(err, ErrSyntax) {
		t.Fatalf("Float() on abc = %v, want ErrSyntax", err)
	}
}

func TestDecodeEntity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		typeName string
		groups   []Group
		check    func(t *testing.T, entity Entity)
	}{
		{
			name:     "LINE",
			typeName: "LINE",
			groups: []Group{
				{Code: 8, Value: "STR-BEAMS"},
				{Code: 10, Value: "1"},
				{Code: 20, Value: "2"},
				{Code: 30, Value: "3"},
				{Code: 11, Value: "4"},
				{Code: 21, Value: "5"},
				{Code: 31, Value: "6"},
			},
			check: func(t *testing.T, entity Entity) {
				line, ok := entity.(*Line)
				if !ok {
					t.Fatalf("type = %T, want *Line", entity)
				}
				if line.Layer() != "STR-BEAMS" {
					t.Errorf("layer = %q", line.Layer())
				}
				if line.Start != (Point3D{X: 1, Y: 2, Z: 3}) || line.End != (Point3D{X: 4, Y: 5, Z: 6}) {
					t.Errorf("endpoints = %+v, %+v", line.Start, line.End)
				}
				if got := line.Length(); got <= 0 {
					t.Errorf("Length() = %v", got)
				}
			},
		},
		{
			name:     "CIRCLE",
			typeName: "CIRCLE",
			groups: []Group{
				{Code: 10, Value: "1"},
				{Code: 20, Value: "1"},
				{Code: 40, Value: "2.5"},
			},
			check: func(t *testing.T, entity Entity) {
				circle, ok := entity.(*Circle)
				if !ok {
					t.Fatalf("type = %T, want *Circle", entity)
				}
				if circle.Radius != 2.5 || circle.Center != (Point3D{X: 1, Y: 1}) {
					t.Errorf("circle = %+v", circle)
				}
			},
		},
		{
			name:     "ARC",
			typeName: "ARC",
			groups: []Group{
				{Code: 10, Value: "0"},
				{Code: 20, Value: "0"},
				{Code: 40, Value: "1"},
				{Code: 50, Value: "30"},
				{Code: 51, Value: "120"},
			},
			check: func(t *testing.T, entity Entity) {
				arc, ok := entity.(*Arc)
				if !ok {
					t.Fatalf("type = %T, want *Arc", entity)
				}
				if arc.StartAngle != 30 || arc.EndAngle != 120 || arc.Radius != 1 {
					t.Errorf("arc = %+v", arc)
				}
			},
		},
		{
			name:     "POINT",
			typeName: "POINT",
			groups: []Group{
				{Code: 10, Value: "7"},
				{Code: 20, Value: "8"},
				{Code: 50, Value: "45"},
			},
			check: func(t *testing.T, entity Entity) {
				point, ok := entity.(*Point)
				if !ok {
					t.Fatalf("type = %T, want *Point", entity)
				}
				if point.Position != (Point3D{X: 7, Y: 8}) || point.Angle != 45 {
					t.Errorf("point = %+v", point)
				}
			},
		},
		{
			name:     "TEXT",
			typeName: "TEXT",
			groups: []Group{
				{Code: 1, Value: "BEAM 1"},
				{Code: 10, Value: "0"},
				{Code: 20, Value: "0"},
				{Code: 40, Value: "2.5"},
				{Code: 7, Value: "STANDARD"},
				{Code: 72, Value: "1"},
				{Code: 11, Value: "5"},
				{Code: 21, Value: "0"},
			},
			check: func(t *testing.T, entity Entity) {
				text, ok := entity.(*Text)
				if !ok {
					t.Fatalf("type = %T, want *Text", entity)
				}
				if text.Value != "BEAM 1" || text.Height != 2.5 || text.Style != "STANDARD" {
					t.Errorf("text = %+v", text)
				}
				if text.HAlign != 1 || text.Alignment.X != 5 {
					t.Errorf("alignment = %+v", text.Alignment)
				}
				if text.XScale != 1 {
					t.Errorf("default XScale = %v, want 1", text.XScale)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entity, err := decodeEntity(tt.typeName, tt.groups)
			if err != nil {
				t.Fatalf("decodeEntity() = %v", err)
			}
			tt.check(t, entity)
		})
	}
}

func TestDecodeEntityMore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		typeName string
		groups   []Group
		check    func(t *testing.T, entity Entity)
	}{
		{
			name:     "SOLID",
			typeName: "SOLID",
			groups: []Group{
				{Code: 10, Value: "0"},
				{Code: 20, Value: "0"},
				{Code: 11, Value: "1"},
				{Code: 21, Value: "0"},
				{Code: 12, Value: "1"},
				{Code: 22, Value: "1"},
				{Code: 13, Value: "0"},
				{Code: 23, Value: "1"},
			},
			check: func(t *testing.T, entity Entity) {
				solid, ok := entity.(*Solid)
				if !ok {
					t.Fatalf("type = %T, want *Solid", entity)
				}
				if solid.Corners[2] != (Point3D{X: 1, Y: 1}) {
					t.Errorf("corners = %+v", solid.Corners)
				}
			},
		},
		{
			name:     "3DFACE",
			typeName: "3DFACE",
			groups: []Group{
				{Code: 10, Value: "0"},
				{Code: 20, Value: "0"},
				{Code: 30, Value: "0"},
				{Code: 11, Value: "1"},
				{Code: 21, Value: "0"},
				{Code: 31, Value: "0"},
				{Code: 12, Value: "1"},
				{Code: 22, Value: "1"},
				{Code: 32, Value: "0"},
				{Code: 13, Value: "0"},
				{Code: 23, Value: "1"},
				{Code: 33, Value: "0"},
				{Code: 70, Value: "3"},
			},
			check: func(t *testing.T, entity Entity) {
				face, ok := entity.(*Face3D)
				if !ok {
					t.Fatalf("type = %T, want *Face3D", entity)
				}
				if face.InvisibleEdges != 3 {
					t.Errorf("InvisibleEdges = %d", face.InvisibleEdges)
				}
			},
		},
		{
			name:     "INSERT",
			typeName: "INSERT",
			groups: []Group{
				{Code: 2, Value: "COLONNA"},
				{Code: 10, Value: "1"},
				{Code: 20, Value: "2"},
				{Code: 41, Value: "2"},
				{Code: 42, Value: "2"},
				{Code: 50, Value: "90"},
			},
			check: func(t *testing.T, entity Entity) {
				insert, ok := entity.(*Insert)
				if !ok {
					t.Fatalf("type = %T, want *Insert", entity)
				}
				if insert.BlockName != "COLONNA" || insert.XScale != 2 || insert.Rotation != 90 {
					t.Errorf("insert = %+v", insert)
				}
				if insert.ZScale != 1 || insert.ColumnCount != 1 {
					t.Errorf("valori predefiniti = %+v", insert)
				}
			},
		},
		{
			name:     "unknown entity",
			typeName: "SPLINE",
			groups: []Group{
				{Code: 8, Value: "0"},
				{Code: 70, Value: "8"},
				{Code: 1001, Value: "ACAD"},
			},
			check: func(t *testing.T, entity Entity) {
				raw, ok := entity.(*RawEntity)
				if !ok {
					t.Fatalf("type = %T, want *RawEntity", entity)
				}
				if raw.Type() != "SPLINE" {
					t.Errorf("Type() = %q", raw.Type())
				}
				if len(raw.Groups) != 2 {
					t.Fatalf("preserved groups = %+v", raw.Groups)
				}
				if raw.Groups[0].Code != 70 || raw.Groups[1].Code != 1001 {
					t.Errorf("groups = %+v", raw.Groups)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entity, err := decodeEntity(tt.typeName, tt.groups)
			if err != nil {
				t.Fatalf("decodeEntity() = %v", err)
			}
			tt.check(t, entity)
		})
	}
}

func TestDecodeEntityInvalidValue(t *testing.T) {
	t.Parallel()

	_, err := decodeEntity("LINE", []Group{{Code: 10, Value: "abc"}})
	if !errors.Is(err, ErrSyntax) {
		t.Fatalf("decodeEntity() = %v, want ErrSyntax", err)
	}
}

func TestEntityCommonNormal(t *testing.T) {
	t.Parallel()

	common := &EntityCommon{}
	if got := common.Normal(); got != (Point3D{Z: 1}) {
		t.Errorf("Normal() = %+v, want (0,0,1)", got)
	}
	common.Extrusion = Point3D{X: 1}
	common.ExtrusionSet = true
	if got := common.Normal(); got != (Point3D{X: 1}) {
		t.Errorf("Normal() = %+v", got)
	}
}

func TestDecodeVertex(t *testing.T) {
	t.Parallel()

	vertex, err := decodeVertex([]Group{
		{Code: 10, Value: "1"},
		{Code: 20, Value: "2"},
		{Code: 42, Value: "0.5"},
		{Code: 70, Value: "32"},
	})
	if err != nil {
		t.Fatalf("decodeVertex() = %v", err)
	}
	if vertex.Position != (Point3D{X: 1, Y: 2}) || vertex.Bulge != 0.5 || vertex.Flags != 32 {
		t.Errorf("vertex = %+v", vertex)
	}
}
