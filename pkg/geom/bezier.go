package geom

import "math"

// maxSegments bounds the subdivision of a single curve.
const maxSegments = 256

// FlattenQuad approximates the quadratic Bézier p0-p1-p2 with line
// segments no further than tol from the curve. The returned points exclude
// p0 and end with p2.
func FlattenQuad(p0, p1, p2 Point, tol float64) []Point {
	// The deviation of a quadratic from its chord is bounded by
	// |p0 - 2p1 + p2| / 4; the error of n segments falls with 1/n².
	dd := p0.Sub(p1.Scale(2)).Add(p2)
	n := segments(math.Hypot(dd.X, dd.Y)/4, tol)

	pts := make([]Point, 0, n)
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		u := 1 - t
		pts = append(pts, Point{
			X: u*u*p0.X + 2*u*t*p1.X + t*t*p2.X,
			Y: u*u*p0.Y + 2*u*t*p1.Y + t*t*p2.Y,
		})
	}
	return pts
}

// FlattenCubic approximates the cubic Bézier p0-p1-p2-p3 with line
// segments no further than tol from the curve. The returned points exclude
// p0 and end with p3.
func FlattenCubic(p0, p1, p2, p3 Point, tol float64) []Point {
	// Bound on the second derivative: 6 * max(|p0-2p1+p2|, |p1-2p2+p3|);
	// the error of n uniform segments is at most that / (8n²).
	d1 := p0.Sub(p1.Scale(2)).Add(p2)
	d2 := p1.Sub(p2.Scale(2)).Add(p3)
	dev := 6 * math.Max(math.Hypot(d1.X, d1.Y), math.Hypot(d2.X, d2.Y)) / 8
	n := segments(dev, tol)

	pts := make([]Point, 0, n)
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		u := 1 - t
		pts = append(pts, Point{
			X: u*u*u*p0.X + 3*u*u*t*p1.X + 3*u*t*t*p2.X + t*t*t*p3.X,
			Y: u*u*u*p0.Y + 3*u*u*t*p1.Y + 3*u*t*t*p2.Y + t*t*t*p3.Y,
		})
	}
	return pts
}

// segments returns the number of segments needed to bring a deviation of
// dev (for a single segment) down to tol.
func segments(dev, tol float64) int {
	if tol <= 0 || dev <= tol {
		return 1
	}
	n := int(math.Ceil(math.Sqrt(dev / tol)))
	return min(n, maxSegments)
}
