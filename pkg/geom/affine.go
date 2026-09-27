package geom

import "math"

// Affine is a 2D affine transform [a b c d e f] mapping
// (x, y) to (a*x + b*y + c, d*x + e*y + f).
type Affine [6]float64

// Identity is the identity transform.
var Identity = Affine{1, 0, 0, 0, 1, 0}

// Translate returns a translation by (dx, dy).
func Translate(dx, dy float64) Affine { return Affine{1, 0, dx, 0, 1, dy} }

// Scale returns a scaling by (sx, sy).
func Scale(sx, sy float64) Affine { return Affine{sx, 0, 0, 0, sy, 0} }

// Rotate returns a counter-clockwise rotation by deg degrees.
func Rotate(deg float64) Affine {
	s, c := math.Sincos(deg * math.Pi / 180)
	return Affine{c, -s, 0, s, c, 0}
}

// Then returns the transform that applies m first and then n.
func (m Affine) Then(n Affine) Affine {
	return Affine{
		n[0]*m[0] + n[1]*m[3],
		n[0]*m[1] + n[1]*m[4],
		n[0]*m[2] + n[1]*m[5] + n[2],
		n[3]*m[0] + n[4]*m[3],
		n[3]*m[1] + n[4]*m[4],
		n[3]*m[2] + n[4]*m[5] + n[5],
	}
}

// Apply transforms p.
func (m Affine) Apply(p Point) Point {
	return Point{
		X: m[0]*p.X + m[1]*p.Y + m[2],
		Y: m[3]*p.X + m[4]*p.Y + m[5],
	}
}
