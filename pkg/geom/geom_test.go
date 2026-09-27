package geom

import (
	"math"
	"testing"
)

const eps = 1e-9

func near(a, b float64) bool { return math.Abs(a-b) < eps }

func TestRect(t *testing.T) {
	r := CenteredRect(62, 62)
	if !near(r.Width(), 62) || !near(r.Height(), 62) {
		t.Fatalf("size: %v", r)
	}

	in := r.Inset(4)
	if !near(in.Width(), 54) || !near(in.Min.X, -27) {
		t.Fatalf("inset: %v", in)
	}
	if !r.Contains(in) || in.Contains(r) {
		t.Fatal("contains")
	}
	if !r.Contains(EmptyRect()) {
		t.Fatal("empty rect should be contained")
	}

	p := r.Path()
	if !near(p.Length(), 4*62) {
		t.Fatalf("perimeter: %v", p.Length())
	}
	if p.Bounds() != r {
		t.Fatalf("bounds: %v", p.Bounds())
	}
}

func TestPathsBoundsAndTransform(t *testing.T) {
	ps := Paths{
		{Points: []Point{{0, 0}, {1, 2}}},
		{Points: []Point{{-3, 1}, {2, -1}}},
	}
	b := ps.Bounds()
	if b.Min != (Point{-3, -1}) || b.Max != (Point{2, 2}) {
		t.Fatalf("bounds: %v", b)
	}

	moved := ps.Transform(Translate(10, 20))
	if moved[0].Points[1] != (Point{11, 22}) {
		t.Fatalf("translate: %v", moved[0].Points[1])
	}
	if ps[0].Points[1] != (Point{1, 2}) {
		t.Fatal("transform modified input")
	}

	if !EmptyRect().Empty() || !(Paths{}).Bounds().Empty() {
		t.Fatal("empty bounds")
	}
}

func TestAffine(t *testing.T) {
	m := Scale(2, 2).Then(Rotate(90)).Then(Translate(1, 0))
	got := m.Apply(Point{1, 0})
	if !near(got.X, 1) || !near(got.Y, 2) {
		t.Fatalf("got %v", got)
	}
	if Identity.Apply(Point{3, 4}) != (Point{3, 4}) {
		t.Fatal("identity")
	}
}

func TestFlattenCubicTolerance(t *testing.T) {
	// Quarter circle approximation with radius 10.
	k := 0.5522847498 * 10
	p0, p1, p2, p3 := Point{10, 0}, Point{10, k}, Point{k, 10}, Point{0, 10}

	for _, tol := range []float64{0.5, 0.05, 0.005} {
		pts := FlattenCubic(p0, p1, p2, p3, tol)
		if pts[len(pts)-1] != p3 {
			t.Fatalf("tol %v: does not end at p3", tol)
		}

		// Midpoints of segments should lie close to the circle.
		prev := p0
		for _, pt := range pts {
			mid := prev.Add(pt).Scale(0.5)
			if d := 10 - math.Hypot(mid.X, mid.Y); d > tol+0.003 {
				t.Fatalf("tol %v: chord deviation %v", tol, d)
			}
			prev = pt
		}
	}

	coarse := len(FlattenCubic(p0, p1, p2, p3, 0.5))
	fine := len(FlattenCubic(p0, p1, p2, p3, 0.005))
	if fine <= coarse {
		t.Fatalf("finer tolerance should give more segments: %d vs %d", fine, coarse)
	}
}

func TestFlattenQuad(t *testing.T) {
	line := FlattenQuad(Point{0, 0}, Point{1, 0}, Point{2, 0}, 0.01)
	if len(line) != 1 || line[0] != (Point{2, 0}) {
		t.Fatalf("straight quad: %v", line)
	}

	pts := FlattenQuad(Point{0, 0}, Point{5, 10}, Point{10, 0}, 0.01)
	if len(pts) < 10 || pts[len(pts)-1] != (Point{10, 0}) {
		t.Fatalf("curved quad: %d points", len(pts))
	}
}
