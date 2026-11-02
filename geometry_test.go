package dxf

import (
	"math"
	"testing"
)

func TestArbitraryAxis(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		normal Point3D
		wantX  Point3D
		wantY  Point3D
	}{
		{
			name:   "default normal",
			normal: Point3D{Z: 1},
			wantX:  Point3D{X: 1},
			wantY:  Point3D{Y: 1},
		},
		{
			name:   "normal along Y",
			normal: Point3D{Y: 1},
			wantX:  Point3D{X: -1},
			wantY:  Point3D{Z: 1},
		},
		{
			name:   "normal opposite to Z",
			normal: Point3D{Z: -1},
			wantX:  Point3D{X: -1},
			wantY:  Point3D{Y: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotX, gotY := ArbitraryAxis(tt.normal)
			if !almostEqual(gotX, tt.wantX) {
				t.Errorf("ArbitraryAxis() X = %+v, want %+v", gotX, tt.wantX)
			}
			if !almostEqual(gotY, tt.wantY) {
				t.Errorf("ArbitraryAxis() Y = %+v, want %+v", gotY, tt.wantY)
			}
		})
	}
}

func TestOCS2WCS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		point  Point3D
		normal Point3D
		want   Point3D
	}{
		{
			name:   "default normal is identity",
			point:  Point3D{X: 1, Y: 2, Z: 3},
			normal: Point3D{Z: 1},
			want:   Point3D{X: 1, Y: 2, Z: 3},
		},
		{
			name:   "zero normal is identity",
			point:  Point3D{X: 1, Y: 2, Z: 3},
			normal: Point3D{},
			want:   Point3D{X: 1, Y: 2, Z: 3},
		},
		{
			name:   "elevation with normal opposite to Z",
			point:  Point3D{Z: 5},
			normal: Point3D{Z: -1},
			want:   Point3D{Z: -5},
		},
		{
			name:   "point rotated to the XZ plane",
			point:  Point3D{X: 1},
			normal: Point3D{Y: 1},
			want:   Point3D{X: -1},
		},
		{
			name:   "point rotated to Z with normal along Y",
			point:  Point3D{Y: 1},
			normal: Point3D{Y: 1},
			want:   Point3D{Z: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := OCS2WCS(tt.point, tt.normal)
			if !almostEqual(got, tt.want) {
				t.Errorf("OCS2WCS(%+v, %+v) = %+v, want %+v", tt.point, tt.normal, got, tt.want)
			}
		})
	}
}

func TestWCS2OCSRoundTrip(t *testing.T) {
	t.Parallel()

	normal := Point3D{X: 0.3, Y: -0.4, Z: 0.5}
	point := Point3D{X: 1.5, Y: -2.5, Z: 3.5}
	got := WCS2OCS(OCS2WCS(point, normal), normal)
	if !almostEqual(got, point) {
		t.Errorf("round-trip = %+v, want %+v", got, point)
	}
}

func almostEqual(a, b Point3D) bool {
	const epsilon = 1e-9
	return math.Abs(a.X-b.X) < epsilon &&
		math.Abs(a.Y-b.Y) < epsilon &&
		math.Abs(a.Z-b.Z) < epsilon
}
