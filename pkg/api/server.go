// Package api implements the laserlabel HTTP API and serves the web UI.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/borud/laserlabel/pkg/fonts"
)

// maxBody bounds request bodies.
const maxBody = 1 << 20

// Config configures the server.
type Config struct {
	Catalog *fonts.Catalog
	Logger  *slog.Logger

	// Static is the web UI to serve at /. It may be nil.
	Static fs.FS
}

// Server is the HTTP API.
type Server struct {
	catalog *fonts.Catalog
	logger  *slog.Logger
	machine   *machineManager
	discovery *discovery
	mux       *http.ServeMux
}

// New creates a server.
func New(cfg Config) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		catalog: cfg.Catalog,
		logger:  logger,
		machine:   newMachineManager(logger),
		discovery: newDiscovery(logger),
		mux:       http.NewServeMux(),
	}

	s.mux.HandleFunc("GET /api/fonts", s.handleFonts)
	s.mux.HandleFunc("POST /api/preview/label", s.handlePreviewLabel)
	s.mux.HandleFunc("POST /api/preview/testgrid", s.handlePreviewTestGrid)
	s.mux.HandleFunc("POST /api/job/label", s.handleJobLabel)
	s.mux.HandleFunc("POST /api/job/testgrid", s.handleJobTestGrid)

	s.mux.HandleFunc("GET /api/machine/state", s.handleState)
	s.mux.HandleFunc("GET /api/machines", s.handleMachines)
	s.mux.HandleFunc("POST /api/machine/connect", s.handleConnect)
	s.mux.HandleFunc("POST /api/machine/disconnect", s.handleDisconnect)
	s.mux.HandleFunc("POST /api/machine/cmd", s.handleCommand)
	s.mux.HandleFunc("POST /api/machine/jog", s.handleJog)
	s.mux.HandleFunc("POST /api/machine/abort", s.handleAbort)
	s.mux.HandleFunc("POST /api/machine/hold", s.handleRealtime('!'))
	s.mux.HandleFunc("POST /api/machine/resume", s.handleRealtime('~'))
	s.mux.HandleFunc("POST /api/machine/home", s.handleSimple(s.machine.Home))
	s.mux.HandleFunc("POST /api/machine/unlock", s.handleSimple(s.machine.Unlock))
	s.mux.HandleFunc("POST /api/machine/reset", s.handleSimple(s.machine.Reset))
	s.mux.HandleFunc("POST /api/machine/tool", s.handleTool)
	s.mux.HandleFunc("GET /api/machine/offsets", s.handleOffsets)
	s.mux.HandleFunc("POST /api/machine/origin", s.handleOrigin)
	s.mux.HandleFunc("POST /api/machine/probe", s.handleProbe)
	s.mux.HandleFunc("POST /api/machine/goto", s.handleGoTo)
	s.mux.HandleFunc("POST /api/machine/pointer", s.handlePointer)
	s.mux.HandleFunc("POST /api/machine/counter/reset", s.handleSimple(s.machine.ResetCounter))

	if cfg.Static != nil {
		s.mux.Handle("GET /", http.FileServerFS(cfg.Static))
	}
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Discover listens for machines on the network until ctx is done.
func (s *Server) Discover(ctx context.Context) {
	s.discovery.run(ctx)
}

// Close disconnects from the machine.
func (s *Server) Close() {
	s.machine.Disconnect()
}

// Connect connects to a machine, discovering it when addr is empty.
func (s *Server) Connect(ctx context.Context, addr, name string) error {
	return s.machine.Connect(ctx, addr, name)
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrNotConnected), errors.Is(err, ErrBusy):
		status = http.StatusConflict
	case errors.Is(err, errBadRequest), errors.Is(err, fonts.ErrUnknownFont):
		status = http.StatusBadRequest
	}
	if status == http.StatusInternalServerError {
		s.logger.Error("request failed", "err", err)
	}
	writeJSON(w, status, errorResponse{Error: err.Error()})
}

var errBadRequest = errors.New("bad request")

func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBody))
	if err := dec.Decode(v); err != nil {
		return errors.Join(errBadRequest, err)
	}
	return nil
}
