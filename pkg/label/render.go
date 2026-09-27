// Package label lays out label text on a workpiece.
package label

import (
	"fmt"
	"strings"

	"github.com/borud/laserlabel/pkg/fonts"
	"github.com/borud/laserlabel/pkg/geom"
	"github.com/borud/laserlabel/pkg/model"
)

// defaultLineSpacing is used when a label doesn't specify line spacing.
const defaultLineSpacing = 1.3

// Warning codes.
const (
	WarnOutsideMargin = "outside-margin"
	WarnEmpty         = "empty"
)

// Warning is a non-fatal problem with a rendered label.
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Result is a rendered label.
type Result struct {
	// Paths are closed glyph contours in mm, relative to the workpiece centre.
	Paths geom.Paths

	// Printable is the workpiece rectangle inset by its margin.
	Printable geom.Rect

	Warnings []Warning

	// Scale is the factor applied to the line sizes, 1 unless Fit is set.
	Scale float64
}

// Render lays out the label lines, centres the ink of the text block on
// the workpiece, then applies the label offset.
func Render(c *fonts.Catalog, l model.Label, w model.Workpiece) (Result, error) {
	res := Result{
		Paths:     geom.Paths{},
		Printable: geom.CenteredRect(w.WidthMM, w.HeightMM).Inset(w.MarginMM),
		Warnings:  []Warning{},
		Scale:     1,
	}

	spacing := l.LineSpacing
	if spacing <= 0 {
		spacing = defaultLineSpacing
	}

	lines, err := outlineLines(c, l.Lines)
	if err != nil {
		return Result{}, err
	}

	blockWidth := 0.0
	for _, ps := range lines {
		if b := ps.Bounds(); !b.Empty() {
			blockWidth = max(blockWidth, b.Width())
		}
	}

	var baseline float64
	for i, ps := range lines {
		line := l.Lines[i]
		if i > 0 {
			baseline -= line.SizeMM * spacing
		}

		b := ps.Bounds()
		if b.Empty() {
			continue
		}
		dx := alignOffset(line.Align, b, blockWidth)
		res.Paths = append(res.Paths, ps.Transform(geom.Translate(dx, baseline))...)
	}

	ink := res.Paths.Bounds()
	if ink.Empty() {
		res.Warnings = append(res.Warnings, Warning{WarnEmpty, "label has no visible text"})
		return res, nil
	}

	centre := ink.Center()
	res.Paths = res.Paths.Transform(geom.Translate(-centre.X, -centre.Y))

	if l.Fit {
		scale, ok := fitScale(ink, res.Printable)
		if !ok {
			res.Warnings = append(res.Warnings, Warning{WarnOutsideMargin, "the margin leaves no room for text"})
			return res, nil
		}
		res.Paths = res.Paths.Transform(geom.Scale(scale, scale))
		res.Scale = scale
	}
	res.Paths = res.Paths.Transform(geom.Translate(l.OffsetX, l.OffsetY))

	ink = res.Paths.Bounds()
	if !res.Printable.Contains(ink) {
		res.Warnings = append(res.Warnings, Warning{
			Code: WarnOutsideMargin,
			Message: fmt.Sprintf("text (%.1f x %.1f mm) extends outside the printable area (%.1f x %.1f mm)",
				ink.Width(), ink.Height(), res.Printable.Width(), res.Printable.Height()),
		})
	}
	return res, nil
}

// fitScale returns the largest uniform scale at which ink fits inside area.
func fitScale(ink, area geom.Rect) (float64, bool) {
	if area.Width() <= 0 || area.Height() <= 0 || ink.Width() <= 0 || ink.Height() <= 0 {
		return 0, false
	}
	return min(area.Width()/ink.Width(), area.Height()/ink.Height()), true
}

// outlineLines converts each text line to outlines at baseline zero.
func outlineLines(c *fonts.Catalog, lines []model.TextLine) ([]geom.Paths, error) {
	out := make([]geom.Paths, 0, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line.Text) == "" || line.SizeMM <= 0 {
			out = append(out, geom.Paths{})
			continue
		}

		txt, err := c.Outline(line.FontID, line.Text, line.SizeMM)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		out = append(out, txt.Paths)
	}
	return out, nil
}

// alignOffset returns the horizontal shift that aligns a line's ink
// bounds within a block of the given width centred on x=0.
func alignOffset(a model.Align, b geom.Rect, blockWidth float64) float64 {
	switch a {
	case model.AlignLeft:
		return -blockWidth/2 - b.Min.X
	case model.AlignRight:
		return blockWidth/2 - b.Max.X
	case model.AlignCenter, "":
		return -b.Center().X
	default:
		panic(fmt.Sprintf("unexpected alignment: %q", a))
	}
}
