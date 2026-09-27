package gcode

import (
	"flag"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/borud/laserlabel/pkg/geom"
	"github.com/borud/laserlabel/pkg/model"
)

var update = flag.Bool("update", false, "update golden files")

var square = geom.Paths{geom.Rect{Min: geom.Point{X: 0, Y: 0}, Max: geom.Point{X: 2, Y: 2}}.Path()}

func settings(mode model.RenderMode) model.LaserSettings {
	return model.LaserSettings{
		Power:         0.4,
		FeedMMPerMin:  1000,
		Passes:        1,
		Mode:          mode,
		HatchInterval: 0.5,
		Bidirectional: true,
	}
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("%s mismatch; got:\n%s", name, got)
	}
}

// checkSafe verifies that the laser is never left on across a rapid move
// and that burn moves only happen with laser mode armed.
func checkSafe(t *testing.T, p Program, framed bool) {
	t.Helper()
	power := 0.0
	armed := !framed
	for i, line := range p.Lines {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(line, ";") {
			continue
		}
		for _, f := range fields[1:] {
			if f[0] == 'S' {
				v, err := strconv.ParseFloat(f[1:], 64)
				if err != nil {
					t.Fatalf("line %d: bad S word %q", i+1, f)
				}
				if v < 0 || v > 1 {
					t.Fatalf("line %d: S out of range: %q", i+1, line)
				}
				power = v
			}
		}
		switch fields[0] {
		case "M3":
			armed = true
		case "M5":
			armed = false
		case "G0":
			if power > 0 {
				t.Fatalf("line %d: rapid move with S=%v: %q", i+1, power, line)
			}
		case "G1":
			if power > 0 && !armed {
				t.Fatalf("line %d: burn move before M3: %q", i+1, line)
			}
		}
	}
}

func TestBurnOutlineGolden(t *testing.T) {
	p := Burn(square, settings(model.RenderOutline), Options{Framing: FramingFull})
	checkSafe(t, p, true)
	golden(t, "outline_full.nc", p.String())
}

func TestBurnFillGolden(t *testing.T) {
	p := Burn(square, settings(model.RenderFill), Options{Framing: FramingResume, Origin: geom.Point{X: 10, Y: -5}})
	checkSafe(t, p, true)
	golden(t, "fill_resume.nc", p.String())
}

func TestBurnFraming(t *testing.T) {
	full := Burn(square, settings(model.RenderOutline), Options{Framing: FramingFull})
	if full.Lines[2] != "M321" || full.Lines[len(full.Lines)-1] != "M322" {
		t.Fatalf("full framing: %v", full.Lines)
	}

	resume := Burn(square, settings(model.RenderOutline), Options{Framing: FramingResume})
	text := resume.String()
	if resume.Lines[2] != "M321.2" || strings.Contains(text, "M322") {
		t.Fatalf("resume framing: %v", resume.Lines)
	}

	none := Burn(square, settings(model.RenderOutline), Options{Framing: FramingNone})
	text = none.String()
	if strings.Contains(text, "M3") || strings.Contains(text, "M5") {
		t.Fatalf("no framing: %v", none.Lines)
	}
}

func TestBurnStatsAndPasses(t *testing.T) {
	one := Burn(square, settings(model.RenderOutline), Options{})
	if math.Abs(one.Stats.BurnMM-8) > 1e-9 {
		t.Fatalf("burn length %v, want 8", one.Stats.BurnMM)
	}
	if one.Stats.TravelMM != 0 {
		t.Fatalf("travel %v, want 0 (starts at origin)", one.Stats.TravelMM)
	}
	if want := time.Duration(8.0 / 1000 * float64(time.Minute)); one.Stats.Duration != want {
		t.Fatalf("duration %v, want %v", one.Stats.Duration, want)
	}

	s := settings(model.RenderOutline)
	s.Passes = 3
	three := Burn(square, s, Options{})
	if math.Abs(three.Stats.BurnMM-24) > 1e-9 {
		t.Fatalf("3 passes: burn length %v, want 24", three.Stats.BurnMM)
	}

	both := Burn(square, settings(model.RenderBoth), Options{})
	fill := Burn(square, settings(model.RenderFill), Options{})
	if math.Abs(both.Stats.BurnMM-fill.Stats.BurnMM-8) > 1e-9 {
		t.Fatal("both mode should add the outline to the fill")
	}
}

func TestTestGrid(t *testing.T) {
	p := TestGrid(GridOptions{
		Powers:        []float64{0.2, 0.4, 0.6},
		Feeds:         []float64{500, 1000},
		CellMM:        5,
		GapMM:         2,
		HatchInterval: 0.5,
	}, Options{Framing: FramingFull})
	checkSafe(t, p, true)

	cells := 0
	for _, l := range p.Lines {
		if strings.HasPrefix(l, "; power") {
			cells++
		}
	}
	if cells != 6 {
		t.Fatalf("got %d cells, want 6", cells)
	}

	b := geom.EmptyRect()
	for _, m := range p.Moves {
		if m.Burn {
			b = b.Extend(m.From).Extend(m.To)
		}
	}
	if math.Abs(b.Width()-12) > 1e-9 || math.Abs(b.Center().X) > 1e-9 || math.Abs(b.Center().Y) > 1e-9 {
		t.Fatalf("grid bounds %v", b)
	}
	if math.Abs(b.Height()-(19-0.5)) > 1e-9 {
		t.Fatalf("grid height %v", b.Height())
	}
}

func TestNum(t *testing.T) {
	for v, want := range map[float64]string{1: "1", 0.5: "0.5", 1.23456: "1.235", -0.0001: "0", -2.5: "-2.5", 1000: "1000"} {
		if got := num(v); got != want {
			t.Errorf("num(%v) = %q, want %q", v, got, want)
		}
	}
}
