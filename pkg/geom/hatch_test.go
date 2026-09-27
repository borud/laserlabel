package geom

import (
	"math"
	"testing"
)

func square(x0, y0, x1, y1 float64, ccw bool) Path {
	p := Rect{Min: Point{x0, y0}, Max: Point{x1, y1}}.Path()
	if !ccw {
		return p.Reverse()
	}
	return p
}

func TestHatchSquare(t *testing.T) {
	h := Hatch(Paths{square(0, 0, 10, 10, true)}, 1, 0, false)
	if len(h) != 10 {
		t.Fatalf("got %d lines, want 10", len(h))
	}
	for _, p := range h {
		if !near(p.Length(), 10) {
			t.Fatalf("line length %v", p.Length())
		}
		if p.Points[0].X > p.Points[1].X {
			t.Fatal("unidirectional hatch should run left to right")
		}
	}
}

func TestHatchBidirectional(t *testing.T) {
	h := Hatch(Paths{square(0, 0, 10, 10, true)}, 1, 0, true)
	for i, p := range h {
		leftToRight := p.Points[0].X < p.Points[1].X
		if leftToRight != (i%2 == 0) {
			t.Fatalf("line %d has wrong direction", i)
		}
	}
}

func TestHatchHole(t *testing.T) {
	outlines := Paths{
		square(0, 0, 10, 10, true),
		square(3, 3, 7, 7, false),
	}
	h := Hatch(outlines, 1, 0, false)

	// Scanlines at y=0.5..9.5; the four crossing the hole split in two.
	if len(h) != 14 {
		t.Fatalf("got %d segments, want 14", len(h))
	}
	for _, p := range h {
		mid := p.Points[0].Add(p.Points[1]).Scale(0.5)
		if mid.X > 3 && mid.X < 7 && mid.Y > 3 && mid.Y < 7 {
			t.Fatalf("segment %v lies in the hole", p.Points)
		}
	}
}

func TestHatchSameDirectionOverlapIsFilled(t *testing.T) {
	// Two overlapping squares wound the same way: nonzero fills the union.
	outlines := Paths{
		square(0, 0, 6, 6, true),
		square(4, 0, 10, 6, true),
	}
	h := Hatch(outlines, 1, 0, false)
	if len(h) != 6 {
		t.Fatalf("got %d segments, want 6", len(h))
	}
	for _, p := range h {
		if !near(p.Length(), 10) {
			t.Fatalf("segment length %v, want 10", p.Length())
		}
	}
}

func TestHatchAngle(t *testing.T) {
	h := Hatch(Paths{square(0, 0, 10, 10, true)}, 0.5, 45, false)
	if len(h) == 0 {
		t.Fatal("no lines")
	}

	var area float64
	for _, p := range h {
		d := p.Points[1].Sub(p.Points[0])
		if !near(math.Abs(d.X), math.Abs(d.Y)) {
			t.Fatalf("line not at 45°: %v", d)
		}
		for _, pt := range p.Points {
			if pt.X < -eps || pt.X > 10+eps || pt.Y < -eps || pt.Y > 10+eps {
				t.Fatalf("point %v outside square", pt)
			}
		}
		area += p.Length() * 0.5
	}
	if math.Abs(area-100) > 2 {
		t.Fatalf("covered area %v, want ~100", area)
	}
}

func TestHatchDegenerate(t *testing.T) {
	if len(Hatch(nil, 1, 0, false)) != 0 {
		t.Fatal("nil outlines")
	}
	if len(Hatch(Paths{square(0, 0, 1, 1, true)}, 0, 0, false)) != 0 {
		t.Fatal("zero interval")
	}
}

func TestOrder(t *testing.T) {
	ps := Paths{
		{Points: []Point{{20, 0}, {30, 0}}},
		{Points: []Point{{10, 0}, {1, 0}}},
		square(40, 0, 42, 2, true),
	}
	got := Order(ps)
	if len(got) != 3 {
		t.Fatalf("got %d paths", len(got))
	}

	// Nearest to origin is the second path's end, so it is reversed.
	if got[0].Points[0] != (Point{1, 0}) {
		t.Fatalf("first path starts at %v", got[0].Points[0])
	}
	if got[1].Points[0] != (Point{20, 0}) {
		t.Fatalf("second path starts at %v", got[1].Points[0])
	}
	if got[2].Points[0] != (Point{40, 0}) || !got[2].Closed {
		t.Fatalf("third path %v", got[2])
	}

	closed := Order(Paths{square(5, 5, 6, 6, true)})
	if closed[0].Points[0] != (Point{5, 5}) || len(closed[0].Points) != 4 {
		t.Fatalf("closed path %v", closed[0])
	}
}
