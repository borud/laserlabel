package geom

import "math"

// Rect is an axis-aligned rectangle.
type Rect struct {
	Min Point `json:"min"`
	Max Point `json:"max"`
}

// EmptyRect returns a rectangle that contains nothing; extending it with a
// point yields a zero-size rectangle at that point.
func EmptyRect() Rect {
	return Rect{
		Min: Point{math.Inf(1), math.Inf(1)},
		Max: Point{math.Inf(-1), math.Inf(-1)},
	}
}

// CenteredRect returns a w x h rectangle centred on the origin.
func CenteredRect(w, h float64) Rect {
	return Rect{Min: Point{-w / 2, -h / 2}, Max: Point{w / 2, h / 2}}
}

// Empty reports whether r contains no points.
func (r Rect) Empty() bool { return r.Min.X > r.Max.X || r.Min.Y > r.Max.Y }

// Width returns the width of r.
func (r Rect) Width() float64 { return r.Max.X - r.Min.X }

// Height returns the height of r.
func (r Rect) Height() float64 { return r.Max.Y - r.Min.Y }

// Center returns the centre of r.
func (r Rect) Center() Point {
	return Point{(r.Min.X + r.Max.X) / 2, (r.Min.Y + r.Max.Y) / 2}
}

// Extend returns the smallest rectangle containing r and p.
func (r Rect) Extend(p Point) Rect {
	return Rect{
		Min: Point{math.Min(r.Min.X, p.X), math.Min(r.Min.Y, p.Y)},
		Max: Point{math.Max(r.Max.X, p.X), math.Max(r.Max.Y, p.Y)},
	}
}

// Union returns the smallest rectangle containing r and o.
func (r Rect) Union(o Rect) Rect {
	if o.Empty() {
		return r
	}
	return r.Extend(o.Min).Extend(o.Max)
}

// Inset returns r shrunk by d on every side.
func (r Rect) Inset(d float64) Rect {
	return Rect{
		Min: Point{r.Min.X + d, r.Min.Y + d},
		Max: Point{r.Max.X - d, r.Max.Y - d},
	}
}

// Contains reports whether o lies entirely within r.
func (r Rect) Contains(o Rect) bool {
	if o.Empty() {
		return true
	}
	return o.Min.X >= r.Min.X && o.Min.Y >= r.Min.Y && o.Max.X <= r.Max.X && o.Max.Y <= r.Max.Y
}

// Path returns r as a closed counter-clockwise path.
func (r Rect) Path() Path {
	return Path{
		Points: []Point{
			r.Min,
			{r.Max.X, r.Min.Y},
			r.Max,
			{r.Min.X, r.Max.Y},
		},
		Closed: true,
	}
}
