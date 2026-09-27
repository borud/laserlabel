package fonts

import (
	"errors"
	"fmt"

	"github.com/borud/laserlabel/pkg/geom"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// flattenTolerance is the maximum deviation, in mm, of flattened curves.
const flattenTolerance = 0.01

// Text is a line of text converted to outlines.
type Text struct {
	// Paths are closed glyph contours, with the baseline at y=0 and the
	// text starting at x=0.
	Paths geom.Paths

	// Advance is the total advance width in mm.
	Advance float64
}

// Outline converts text in the given font to closed contours scaled so
// that capital letters are sizeMM tall.
func (c *Catalog) Outline(fontID, text string, sizeMM float64) (Text, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	f, err := c.face(fontID)
	if err != nil {
		return Text{}, err
	}

	// Load glyphs at one pixel per font unit so coordinates are in font units.
	upem := fixed.I(int(f.UnitsPerEm()))
	capHeight, err := c.capHeight(f, upem)
	if err != nil {
		return Text{}, err
	}
	scale := sizeMM / capHeight
	tol := flattenTolerance / scale

	out := Text{Paths: geom.Paths{}}
	var x float64
	prev := sfnt.GlyphIndex(0)
	for i, r := range text {
		gi, err := f.GlyphIndex(&c.buf, r)
		if err != nil {
			return Text{}, fmt.Errorf("glyph for %q: %w", r, err)
		}

		if i > 0 {
			k, err := f.Kern(&c.buf, prev, gi, upem, font.HintingNone)
			if err == nil {
				x += units(k)
			}
		}

		segs, err := f.LoadGlyph(&c.buf, gi, upem, nil)
		if err != nil {
			return Text{}, fmt.Errorf("load glyph %q: %w", r, err)
		}
		out.Paths = append(out.Paths, contours(segs, x, tol)...)

		adv, err := f.GlyphAdvance(&c.buf, gi, upem, font.HintingNone)
		if err != nil {
			return Text{}, fmt.Errorf("advance for %q: %w", r, err)
		}
		x += units(adv)
		prev = gi
	}

	out.Paths = out.Paths.Transform(geom.Scale(scale, scale))
	out.Advance = x * scale
	return out, nil
}

// capHeight returns the cap height in font units, measuring the glyph "H"
// when the font doesn't declare it.
func (c *Catalog) capHeight(f *sfnt.Font, upem fixed.Int26_6) (float64, error) {
	m, err := f.Metrics(&c.buf, upem, font.HintingNone)
	if err == nil && m.CapHeight > 0 {
		return units(m.CapHeight), nil
	}

	gi, err := f.GlyphIndex(&c.buf, 'H')
	if err == nil && gi != 0 {
		b, _, err := f.GlyphBounds(&c.buf, gi, upem, font.HintingNone)
		if err == nil && b.Min.Y < 0 {
			return -units(b.Min.Y), nil
		}
	}

	// Fall back to the conventional 0.7 em.
	if f.UnitsPerEm() == 0 {
		return 0, errors.New("font has no units per em")
	}
	return 0.7 * float64(f.UnitsPerEm()), nil
}

// contours converts glyph segments to closed paths, shifting them right by
// dx and flipping Y so that it points up.
func contours(segs sfnt.Segments, dx, tol float64) geom.Paths {
	pt := func(p fixed.Point26_6) geom.Point {
		return geom.Point{X: units(p.X) + dx, Y: -units(p.Y)}
	}

	paths := geom.Paths{}
	var cur []geom.Point
	flush := func() {
		if len(cur) > 2 {
			if cur[0] == cur[len(cur)-1] {
				cur = cur[:len(cur)-1]
			}
			paths = append(paths, geom.Path{Points: cur, Closed: true})
		}
		cur = nil
	}

	for _, s := range segs {
		switch s.Op {
		case sfnt.SegmentOpMoveTo:
			flush()
			cur = []geom.Point{pt(s.Args[0])}
		case sfnt.SegmentOpLineTo:
			cur = append(cur, pt(s.Args[0]))
		case sfnt.SegmentOpQuadTo:
			last := cur[len(cur)-1]
			cur = append(cur, geom.FlattenQuad(last, pt(s.Args[0]), pt(s.Args[1]), tol)...)
		case sfnt.SegmentOpCubeTo:
			last := cur[len(cur)-1]
			cur = append(cur, geom.FlattenCubic(last, pt(s.Args[0]), pt(s.Args[1]), pt(s.Args[2]), tol)...)
		default:
			panic(fmt.Sprintf("unexpected segment op: %d", s.Op))
		}
	}
	flush()
	return paths
}

func units(v fixed.Int26_6) float64 { return float64(v) / 64 }
