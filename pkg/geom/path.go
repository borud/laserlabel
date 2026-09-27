// Package geom provides the 2D geometry used to build laser toolpaths.
// Coordinates are in millimetres with Y pointing up.
package geom

import "math"

// Point is a 2D point or vector.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Path is a polyline. A closed path implicitly connects its last point
// back to its first.
type Path struct {
	Points []Point `json:"points"`
	Closed bool    `json:"closed"`
}

// Paths is a collection of paths.
type Paths []Path

// Add returns p+q.
func (p Point) Add(q Point) Point { return Point{p.X + q.X, p.Y + q.Y} }

// Sub returns p-q.
func (p Point) Sub(q Point) Point { return Point{p.X - q.X, p.Y - q.Y} }

// Scale returns p scaled by s.
func (p Point) Scale(s float64) Point { return Point{p.X * s, p.Y * s} }

// Dist returns the distance between p and q.
func (p Point) Dist(q Point) float64 { return math.Hypot(p.X-q.X, p.Y-q.Y) }

// Length returns the length of the path, including the closing segment.
func (p Path) Length() float64 {
	if len(p.Points) < 2 {
		return 0
	}

	var l float64
	for i := 1; i < len(p.Points); i++ {
		l += p.Points[i-1].Dist(p.Points[i])
	}
	if p.Closed {
		l += p.Points[len(p.Points)-1].Dist(p.Points[0])
	}
	return l
}

// Bounds returns the bounding rectangle of the path.
func (p Path) Bounds() Rect {
	r := EmptyRect()
	for _, pt := range p.Points {
		r = r.Extend(pt)
	}
	return r
}

// Bounds returns the bounding rectangle of all paths.
func (ps Paths) Bounds() Rect {
	r := EmptyRect()
	for _, p := range ps {
		r = r.Union(p.Bounds())
	}
	return r
}

// Transform returns a copy of the paths with m applied to every point.
func (ps Paths) Transform(m Affine) Paths {
	out := make(Paths, 0, len(ps))
	for _, p := range ps {
		pts := make([]Point, len(p.Points))
		for i, pt := range p.Points {
			pts[i] = m.Apply(pt)
		}
		out = append(out, Path{Points: pts, Closed: p.Closed})
	}
	return out
}
