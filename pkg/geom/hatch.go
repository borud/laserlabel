package geom

import (
	"math"
	"slices"
)

// crossing is where a scanline crosses an outline edge.
type crossing struct {
	x       float64
	winding int
}

// Hatch fills the closed outlines with parallel lines spaced interval
// apart at angleDeg degrees, using the nonzero winding rule. The result is
// a set of two-point open paths ordered scanline by scanline. With bidir
// set, every other scanline runs in the opposite direction.
func Hatch(outlines Paths, interval, angleDeg float64, bidir bool) Paths {
	out := Paths{}
	if interval <= 0 || len(outlines) == 0 {
		return out
	}

	// Work in a rotated frame where the hatch lines are horizontal.
	rotated := outlines.Transform(Rotate(-angleDeg))
	back := Rotate(angleDeg)

	b := rotated.Bounds()
	if b.Empty() {
		return out
	}

	reverse := false
	for y := b.Min.Y + interval/2; y < b.Max.Y; y += interval {
		spans := scanline(rotated, y)
		if reverse {
			slices.Reverse(spans)
			for i := range spans {
				spans[i][0], spans[i][1] = spans[i][1], spans[i][0]
			}
		}
		for _, s := range spans {
			out = append(out, Path{Points: []Point{
				back.Apply(Point{s[0], y}),
				back.Apply(Point{s[1], y}),
			}})
		}
		if bidir {
			reverse = !reverse
		}
	}
	return out
}

// scanline returns the filled x-intervals of the outlines along y, sorted
// left to right.
func scanline(outlines Paths, y float64) [][2]float64 {
	var xs []crossing
	for _, p := range outlines {
		n := len(p.Points)
		for i := range n {
			a, b := p.Points[i], p.Points[(i+1)%n]
			// Half-open rule so that vertices on the scanline count once.
			switch {
			case a.Y <= y && b.Y > y:
				xs = append(xs, crossing{a.X + (y-a.Y)*(b.X-a.X)/(b.Y-a.Y), 1})
			case b.Y <= y && a.Y > y:
				xs = append(xs, crossing{a.X + (y-a.Y)*(b.X-a.X)/(b.Y-a.Y), -1})
			}
		}
	}
	slices.SortFunc(xs, func(a, b crossing) int {
		switch {
		case a.x < b.x:
			return -1
		case a.x > b.x:
			return 1
		}
		return 0
	})

	spans := [][2]float64{}
	winding := 0
	start := 0.0
	for _, c := range xs {
		was := winding
		winding += c.winding
		switch {
		case was == 0 && winding != 0:
			start = c.x
		case was != 0 && winding == 0:
			if c.x-start > 1e-9 {
				spans = append(spans, [2]float64{start, c.x})
			}
		}
	}
	return spans
}

// Reverse returns a copy of p with its points in reverse order.
func (p Path) Reverse() Path {
	pts := slices.Clone(p.Points)
	slices.Reverse(pts)
	return Path{Points: pts, Closed: p.Closed}
}

// Order sorts paths with a greedy nearest-neighbour heuristic, starting
// at the origin, to reduce travel between them. Open paths may be
// reversed and closed paths may be rotated to start at their nearest
// vertex.
func Order(ps Paths) Paths {
	out := make(Paths, 0, len(ps))
	used := make([]bool, len(ps))
	pos := Point{}

	for range ps {
		best, bestIdx, bestDist := -1, 0, math.Inf(1)
		for i, p := range ps {
			if used[i] || len(p.Points) == 0 {
				continue
			}
			idx, d := nearestStart(p, pos)
			if d < bestDist {
				best, bestIdx, bestDist = i, idx, d
			}
		}
		if best < 0 {
			break
		}

		used[best] = true
		p := startAt(ps[best], bestIdx)
		out = append(out, p)
		pos = p.Points[len(p.Points)-1]
		if p.Closed {
			pos = p.Points[0]
		}
	}
	return out
}

// nearestStart returns the index of the best starting point of p relative
// to pos, and its distance. For open paths that is either end; -1 means
// the path should be reversed.
func nearestStart(p Path, pos Point) (int, float64) {
	if !p.Closed {
		first := pos.Dist(p.Points[0])
		last := pos.Dist(p.Points[len(p.Points)-1])
		if last < first {
			return -1, last
		}
		return 0, first
	}

	idx, best := 0, math.Inf(1)
	for i, pt := range p.Points {
		if d := pos.Dist(pt); d < best {
			idx, best = i, d
		}
	}
	return idx, best
}

// startAt returns p arranged to start at idx as chosen by nearestStart.
func startAt(p Path, idx int) Path {
	if idx < 0 {
		return p.Reverse()
	}
	if idx == 0 {
		return p
	}
	pts := append(slices.Clone(p.Points[idx:]), p.Points[:idx]...)
	return Path{Points: pts, Closed: p.Closed}
}
