package api

import (
	"fmt"
	"net/http"
)

// maxNudgeMM bounds a single origin nudge.
const maxNudgeMM = 10

func (s *Server) handleSimple(f func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if err := f(); err != nil {
			s.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, s.machine.State())
	}
}

func (s *Server) handleTool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Tool string `json:"tool"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	if err := s.machine.Tool(r.Context(), req.Tool); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.machine.State())
}

func (s *Server) handleOffsets(w http.ResponseWriter, r *http.Request) {
	o, err := s.machine.Offsets(r.Context())
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// handleOrigin sets the work origin: zero axes at the current position,
// accept the saved origin, or nudge it (optionally re-tracing the
// workpiece afterwards).
func (s *Server) handleOrigin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Here    bool    `json:"here"`
		Axes    string  `json:"axes"`
		Accept  bool    `json:"accept"`
		DX      float64 `json:"dx"`
		DY      float64 `json:"dy"`
		Retrace *struct {
			WidthMM  float64 `json:"widthMM"`
			HeightMM float64 `json:"heightMM"`
			MarginMM float64 `json:"marginMM"`
		} `json:"retrace"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	var err error
	switch {
	case req.Here || req.Axes != "":
		err = s.machine.ZeroAxes(r.Context(), req.Axes)
	case req.Accept:
		err = s.machine.AcceptOrigin()
	case abs(req.DX) > maxNudgeMM || abs(req.DY) > maxNudgeMM:
		err = fmt.Errorf("%w: nudge at most %d mm", errBadRequest, maxNudgeMM)
	case req.DX == 0 && req.DY == 0:
		err = fmt.Errorf("%w: nothing to do", errBadRequest)
	default:
		err = s.machine.NudgeOrigin(r.Context(), req.DX, req.DY)
		if err == nil && req.Retrace != nil {
			err = s.machine.Probe(r.Context(), req.Retrace.WidthMM, req.Retrace.HeightMM, req.Retrace.MarginMM, true, false)
		}
	}
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.machine.State())
}

func (s *Server) handleProbe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WidthMM  float64 `json:"widthMM"`
		HeightMM float64 `json:"heightMM"`
		MarginMM float64 `json:"marginMM"`
		Outline  bool    `json:"outline"`
		Z        bool    `json:"z"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	if err := s.machine.Probe(r.Context(), req.WidthMM, req.HeightMM, req.MarginMM, req.Outline, req.Z); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.machine.State())
}

func (s *Server) handlePointer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		On bool `json:"on"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	if err := s.machine.Pointer(req.On); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.machine.State())
}

func (s *Server) handleGoTo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Target string `json:"target"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	if err := s.machine.GoTo(r.Context(), req.Target); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.machine.State())
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
