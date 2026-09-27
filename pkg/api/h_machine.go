package api

import (
	"fmt"
	"net/http"
	"strings"

)

// maxJogMM bounds a single jog step.
const maxJogMM = 50

func (s *Server) handleFonts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.catalog.Fonts())
}

func (s *Server) handleState(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.machine.State())
}

func (s *Server) handleMachines(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.discovery.list())
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Host string `json:"host"`
		Name string `json:"name"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, err)
		return
	}

	if err := s.machine.Connect(r.Context(), strings.TrimSpace(req.Host), strings.TrimSpace(req.Name)); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.machine.State())
}

func (s *Server) handleDisconnect(w http.ResponseWriter, _ *http.Request) {
	s.machine.Disconnect()
	writeJSON(w, http.StatusOK, s.machine.State())
}

func (s *Server) handleCommand(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Line string `json:"line"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	line := strings.TrimSpace(req.Line)
	if line == "" {
		s.writeError(w, fmt.Errorf("%w: empty command", errBadRequest))
		return
	}

	lines, err := s.machine.Command(r.Context(), line)
	if err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"lines": lines})
}

func (s *Server) handleJog(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Axis string  `json:"axis"`
		MM   float64 `json:"mm"`
	}
	if err := decode(r, &req); err != nil {
		s.writeError(w, err)
		return
	}
	if req.MM == 0 || req.MM > maxJogMM || req.MM < -maxJogMM {
		s.writeError(w, fmt.Errorf("%w: jog distance must be within ±%d mm", errBadRequest, maxJogMM))
		return
	}

	if err := s.machine.Jog(strings.ToUpper(req.Axis), req.MM); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.machine.State())
}

func (s *Server) handleAbort(w http.ResponseWriter, _ *http.Request) {
	if err := s.machine.Abort(); err != nil {
		s.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.machine.State())
}

func (s *Server) handleRealtime(ch byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if err := s.machine.Realtime(ch); err != nil {
			s.writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, s.machine.State())
	}
}
