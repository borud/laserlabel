package gcode

import (
	"fmt"

	"github.com/borud/laserlabel/pkg/geom"
)

// GridOptions describe a power/speed test grid.
type GridOptions struct {
	// Powers are the row powers (0..1), bottom row first.
	Powers []float64

	// Feeds are the column feed rates in mm/min, left column first.
	Feeds []float64

	CellMM        float64
	GapMM         float64
	HatchInterval float64
}

// TestGrid generates a grid of filled squares, one per power/feed
// combination, centred on the origin.
func TestGrid(g GridOptions, o Options) Program {
	w := newWriter(o, fmt.Sprintf("laserlabel test grid: powers %v, feeds %v", g.Powers, g.Feeds))

	pitch := g.CellMM + g.GapMM
	width := float64(len(g.Feeds))*pitch - g.GapMM
	height := float64(len(g.Powers))*pitch - g.GapMM

	for row, power := range g.Powers {
		for col, feed := range g.Feeds {
			x := -width/2 + float64(col)*pitch
			y := -height/2 + float64(row)*pitch
			cell := geom.Rect{Min: geom.Point{X: x, Y: y}, Max: geom.Point{X: x + g.CellMM, Y: y + g.CellMM}}

			w.emit(fmt.Sprintf("; power %s feed %s", num(power), num(feed)))
			for _, p := range geom.Hatch(geom.Paths{cell.Path()}, g.HatchInterval, 0, true) {
				w.path(p, power, feed)
			}
		}
	}
	return w.finish()
}
