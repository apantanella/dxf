package dxf

import "math"

// arbitraryAxisThreshold is the 1/64 boundary of the DXF arbitrary axis
// algorithm. It is chosen so that the comparison is portable across machines.
const arbitraryAxisThreshold = 1.0 / 64.0

// ArbitraryAxis returns the unit X and Y axes of the object coordinate system
// defined by normal, following the DXF arbitrary axis algorithm. The Z axis of
// the OCS is the normalized normal itself.
//
// A zero normal is treated as (0, 0, 1).
func ArbitraryAxis(normal Point3D) (Point3D, Point3D) {
	z := unitOrZ(normal)
	var ax Point3D
	if math.Abs(z.X) < arbitraryAxisThreshold && math.Abs(z.Y) < arbitraryAxisThreshold {
		ax = cross(Point3D{Y: 1}, z)
	} else {
		ax = cross(Point3D{Z: 1}, z)
	}
	ax = unitOrZ(ax)
	ay := unitOrZ(cross(z, ax))
	return ax, ay
}

// OCS2WCS converts a point expressed in the object coordinate system to the
// world coordinate system. The OCS origin coincides with the WCS origin, so the
// conversion is a pure rotation.
//
// A zero or default normal returns the point unchanged.
func OCS2WCS(point Point3D, normal Point3D) Point3D {
	if isDefaultNormal(normal) {
		return point
	}
	ax, ay := ArbitraryAxis(normal)
	z := unitOrZ(normal)
	return Point3D{
		X: point.X*ax.X + point.Y*ay.X + point.Z*z.X,
		Y: point.X*ax.Y + point.Y*ay.Y + point.Z*z.Y,
		Z: point.X*ax.Z + point.Y*ay.Z + point.Z*z.Z,
	}
}

// WCS2OCS converts a world coordinate point to the object coordinate system
// defined by normal. It is the inverse of OCS2WCS.
func WCS2OCS(point Point3D, normal Point3D) Point3D {
	if isDefaultNormal(normal) {
		return point
	}
	ax, ay := ArbitraryAxis(normal)
	z := unitOrZ(normal)
	return Point3D{
		X: dot(point, ax),
		Y: dot(point, ay),
		Z: dot(point, z),
	}
}

func isDefaultNormal(normal Point3D) bool {
	const epsilon = 1e-12
	return math.Abs(normal.X) < epsilon &&
		math.Abs(normal.Y) < epsilon &&
		math.Abs(normal.Z-1) < epsilon
}

func cross(a, b Point3D) Point3D {
	return Point3D{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}

func dot(a, b Point3D) float64 {
	return a.X*b.X + a.Y*b.Y + a.Z*b.Z
}

// unitOrZ returns the unit vector of v, or (0, 0, 1) when v is the zero vector.
func unitOrZ(v Point3D) Point3D {
	length := math.Sqrt(dot(v, v))
	if length == 0 {
		return Point3D{Z: 1}
	}
	return Point3D{X: v.X / length, Y: v.Y / length, Z: v.Z / length}
}
