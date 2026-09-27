package api

import (
	"fmt"
	"math"
	"net/http"

	"github.com/borud/laserlabel/pkg/gcode"
	"github.com/borud/laserlabel/pkg/geom"
	"github.com/borud/laserlabel/pkg/label"
	"github.com/borud/laserlabel/pkg/model"
)

// maxGridCells bounds the size of test grids.
const maxGridCells = 100

type labelRequest struct {
	Label     model.Label         `json:"label"`
	Workpiece model.Workpiece     `json:"workpiece"`
	Settings  model.LaserSettings `json:"settings"`
	Framing   string              `json:"framing"`
}

type gridRequest struct {
	Powers        []float64 `json:"powers"`
	Feeds         []float64 `json:"feeds"`
	CellMM        float64   `json:"cellMM"`
	GapMM         float64   `json:"gapMM"`
	HatchInterval float64   `json:"hatchIntervalMM"`
	Framing       string    `json:"framing"`
}

// preview is a compact toolpath for drawing, with segments as
// [x1, y1, x2, y2].
type preview struct {
	Burn        [][4]float64    `json:"burn"`
	Travel      [][4]float64    `json:"travel"`
	Bounds      geom.Rect       `json:"bounds"`
	Workpiece   *geom.Rect      `json:"workpiece"`
	Printable   *geom.Rect      `json:"printable"`
	Warnings    []label.Warning `json:"warnings"`
	BurnMM      float64         `json:"burnMM"`
	TravelMM    float64         `json:"travelMM"`
	EstimateSec float64         `json:"estimateSec"`
	Scale       float64         `json:"scale"`
	Lines       int             `json:"lines"`
	GCode       string          `json:"gcode"`
}

func (s *Server) handlePreviewLabel(w http.ResponseWriter, r *http.Request) {
	var req labelRequest
	if err := decode(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	prog, res, err := s.labelProgram(req)
	if err != nil {
		s.writeError(w, err)
		return
	}

	p := newPreview(prog)
	wp := geom.CenteredRect(req.Workpiece.WidthMM, req.Workpiece.HeightMM)
	p.Workpiece = &wp
	p.Printable = &res.Printable
	p.Warnings = res.Warnings
	p.Scale = res.Scale
	p.Bounds = p.Bounds.Union(wp)
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handlePreviewTestGrid(w http.ResponseWriter, r *http.Request) {
	var req gridRequest
	if err := decode(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	prog, err := gridProgram(req)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newPreview(prog))
}

func (s *Server) handleJobLabel(w http.ResponseWriter, r *http.Request) {
	var req labelRequest
	if err := decode(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	prog, _, err := s.labelProgram(req)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.startJob(w, "label", prog)
}

func (s *Server) handleJobTestGrid(w http.ResponseWriter, r *http.Request) {
	var req gridRequest
	if err := decode(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	prog, err := gridProgram(req)
	if err != nil {
		s.writeError(w, err)
		return
	}
	s.startJob(w, "testgrid", prog)
}

func (s *Server) startJob(w http.ResponseWriter, name string, prog gcode.Program) {
	if err := s.machine.StartJob(name, prog.String()); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, s.machine.State())
}

func (s *Server) labelProgram(req labelRequest) (gcode.Program, label.Result, error) {
	framing, err := parseFraming(req.Framing)
	if err != nil {
		return gcode.Program{}, label.Result{}, err
	}
	if err := validateSettings(req.Settings); err != nil {
		return gcode.Program{}, label.Result{}, err
	}
	if req.Workpiece.WidthMM <= 0 || req.Workpiece.HeightMM <= 0 {
		return gcode.Program{}, label.Result{}, fmt.Errorf("%w: workpiece size must be positive", errBadRequest)
	}

	res, err := label.Render(s.catalog, req.Label, req.Workpiece)
	if err != nil {
		return gcode.Program{}, label.Result{}, err
	}
	return gcode.Burn(res.Paths, req.Settings, gcode.Options{Framing: framing}), res, nil
}

func gridProgram(req gridRequest) (gcode.Program, error) {
	framing, err := parseFraming(req.Framing)
	if err != nil {
		return gcode.Program{}, err
	}

	switch {
	case len(req.Powers) == 0 || len(req.Feeds) == 0:
		return gcode.Program{}, fmt.Errorf("%w: need at least one power and one feed", errBadRequest)
	case len(req.Powers)*len(req.Feeds) > maxGridCells:
		return gcode.Program{}, fmt.Errorf("%w: at most %d cells", errBadRequest, maxGridCells)
	case req.CellMM <= 0 || req.GapMM < 0 || req.HatchInterval <= 0:
		return gcode.Program{}, fmt.Errorf("%w: cell size and interval must be positive", errBadRequest)
	}
	for _, p := range req.Powers {
		if p <= 0 || p > 1 {
			return gcode.Program{}, fmt.Errorf("%w: power %v outside 0..1", errBadRequest, p)
		}
	}
	for _, f := range req.Feeds {
		if f <= 0 {
			return gcode.Program{}, fmt.Errorf("%w: feed %v must be positive", errBadRequest, f)
		}
	}

	return gcode.TestGrid(gcode.GridOptions{
		Powers:        req.Powers,
		Feeds:         req.Feeds,
		CellMM:        req.CellMM,
		GapMM:         req.GapMM,
		HatchInterval: req.HatchInterval,
	}, gcode.Options{Framing: framing}), nil
}

func validateSettings(st model.LaserSettings) error {
	switch {
	case st.Power <= 0 || st.Power > 1:
		return fmt.Errorf("%w: power must be in 0..1", errBadRequest)
	case st.FeedMMPerMin <= 0:
		return fmt.Errorf("%w: feed must be positive", errBadRequest)
	case st.Mode != model.RenderOutline && st.HatchInterval <= 0:
		return fmt.Errorf("%w: hatch interval must be positive", errBadRequest)
	case st.Passes > 10:
		return fmt.Errorf("%w: at most 10 passes", errBadRequest)
	}

	switch st.Mode {
	case model.RenderFill, model.RenderOutline, model.RenderBoth, "":
		return nil
	}
	return fmt.Errorf("%w: unknown mode %q", errBadRequest, st.Mode)
}

func parseFraming(s string) (gcode.Framing, error) {
	switch s {
	case "full", "":
		return gcode.FramingFull, nil
	case "resume":
		return gcode.FramingResume, nil
	}
	return 0, fmt.Errorf("%w: unknown framing %q", errBadRequest, s)
}

func newPreview(p gcode.Program) preview {
	pv := preview{
		Burn:        make([][4]float64, 0, len(p.Moves)),
		Travel:      [][4]float64{},
		Bounds:      geom.EmptyRect(),
		Warnings:    []label.Warning{},
		BurnMM:      p.Stats.BurnMM,
		TravelMM:    p.Stats.TravelMM,
		EstimateSec: p.Stats.Duration.Seconds(),
		Scale:       1,
		Lines:       len(p.Lines),
		GCode:       p.String(),
	}
	for _, m := range p.Moves {
		seg := [4]float64{round(m.From.X), round(m.From.Y), round(m.To.X), round(m.To.Y)}
		if m.Burn {
			pv.Burn = append(pv.Burn, seg)
			pv.Bounds = pv.Bounds.Extend(m.From).Extend(m.To)
			continue
		}
		pv.Travel = append(pv.Travel, seg)
	}
	if pv.Bounds.Empty() {
		pv.Bounds = geom.Rect{}
	}
	return pv
}

func round(v float64) float64 { return math.Round(v*1000) / 1000 }
