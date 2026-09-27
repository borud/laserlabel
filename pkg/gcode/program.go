// Package gcode generates laser G-code for the Makera Carvera family.
//
// The dialect is Smoothieware with Makera extensions: M321 enters laser
// mode (on the Air this triggers a tool change to the laser module),
// M321.2 enters it without a tool change, M322 leaves it. Power is S in
// the range 0 to 1 and only fires on G1/G2/G3 moves.
package gcode

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/borud/laserlabel/pkg/geom"
)

// travelFeed is the assumed rapid feed rate in mm/min, used for estimates.
const travelFeed = 3000

// Framing selects the preamble and postamble of a program.
type Framing int

// Framing modes.
const (
	// FramingFull enters laser mode with M321 (including the tool change)
	// and leaves it with M322 at the end.
	FramingFull Framing = iota

	// FramingResume enters laser mode with M321.2 and stays in laser mode
	// afterwards, for burning many workpieces in a row.
	FramingResume

	// FramingNone emits only the motion, for embedding in other programs.
	FramingNone
)

// Options control program generation.
type Options struct {
	Framing Framing

	// Origin is added to every coordinate.
	Origin geom.Point
}

// Move is a straight move of the tool, for previews.
type Move struct {
	From geom.Point `json:"from"`
	To   geom.Point `json:"to"`
	Burn bool       `json:"burn"`
}

// Stats summarise a program.
type Stats struct {
	BurnMM   float64       `json:"burnMM"`
	TravelMM float64       `json:"travelMM"`
	Duration time.Duration `json:"duration"`
}

// Program is generated G-code together with its toolpath.
type Program struct {
	Lines []string `json:"lines"`
	Moves []Move   `json:"moves"`
	Stats Stats    `json:"stats"`
}

// String returns the program text.
func (p Program) String() string {
	return strings.Join(p.Lines, "\n") + "\n"
}

// writer accumulates a program while tracking the tool position.
type writer struct {
	opts Options
	prog Program
	pos  geom.Point
}

func newWriter(o Options, comment string) *writer {
	w := &writer{opts: o, prog: Program{Lines: []string{}, Moves: []Move{}}}
	w.emit("; " + comment)
	w.emit("G90 G21")

	switch o.Framing {
	case FramingFull:
		w.emit("M321")
	case FramingResume:
		w.emit("M321.2")
	case FramingNone:
		return w
	default:
		panic(fmt.Sprintf("unexpected framing: %d", o.Framing))
	}
	w.emit("G0 Z0")
	w.emit("M3")
	return w
}

func (w *writer) emit(line string) { w.prog.Lines = append(w.prog.Lines, line) }

// path burns p at the given power (0..1) and feed (mm/min).
func (w *writer) path(p geom.Path, power, feed float64) {
	if len(p.Points) < 2 {
		return
	}

	start := p.Points[0].Add(w.opts.Origin)
	w.emit("G0 " + xy(start))
	w.move(start, false, travelFeed)

	pts := p.Points[1:]
	if p.Closed {
		pts = append(pts[:len(pts):len(pts)], p.Points[0])
	}
	for i, pt := range pts {
		to := pt.Add(w.opts.Origin)
		line := "G1 " + xy(to)
		if i == 0 {
			line += " S" + num(power) + " F" + num(feed)
		}
		w.emit(line)
		w.move(to, true, feed)
	}
	w.emit("G1 S0")
}

func (w *writer) move(to geom.Point, burn bool, feed float64) {
	d := w.pos.Dist(to)
	if d == 0 {
		return
	}

	w.prog.Moves = append(w.prog.Moves, Move{From: w.pos, To: to, Burn: burn})
	if burn {
		w.prog.Stats.BurnMM += d
	} else {
		w.prog.Stats.TravelMM += d
	}
	w.prog.Stats.Duration += time.Duration(d / feed * float64(time.Minute))
	w.pos = to
}

func (w *writer) finish() Program {
	switch w.opts.Framing {
	case FramingFull:
		w.emit("M5")
		w.emit("M322")
	case FramingResume:
		w.emit("M5")
	case FramingNone:
	default:
		panic(fmt.Sprintf("unexpected framing: %d", w.opts.Framing))
	}
	return w.prog
}

func xy(p geom.Point) string { return "X" + num(p.X) + " Y" + num(p.Y) }

// num formats v with at most three decimals and no trailing zeros.
func num(v float64) string {
	s := strconv.FormatFloat(v, 'f', 3, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimSuffix(s, ".")
	if s == "-0" {
		return "0"
	}
	return s
}
