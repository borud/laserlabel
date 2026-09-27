package gcode

import (
	"fmt"

	"github.com/borud/laserlabel/pkg/geom"
	"github.com/borud/laserlabel/pkg/model"
)

// Burn generates a program that burns the closed outlines using the given
// settings.
func Burn(outlines geom.Paths, s model.LaserSettings, o Options) Program {
	w := newWriter(o, "laserlabel")

	paths := toolpaths(outlines, s)
	for range max(s.Passes, 1) {
		for _, p := range paths {
			w.path(p, s.Power, s.FeedMMPerMin)
		}
	}
	return w.finish()
}

// toolpaths converts outlines to the paths the laser follows.
func toolpaths(outlines geom.Paths, s model.LaserSettings) geom.Paths {
	fill := func() geom.Paths {
		return geom.Hatch(outlines, s.HatchInterval, s.HatchAngle, s.Bidirectional)
	}

	switch s.Mode {
	case model.RenderFill, "":
		return fill()
	case model.RenderOutline:
		return geom.Order(outlines)
	case model.RenderBoth:
		return append(fill(), geom.Order(outlines)...)
	default:
		panic(fmt.Sprintf("unexpected render mode: %q", s.Mode))
	}
}
