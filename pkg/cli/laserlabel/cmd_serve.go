package laserlabel

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/borud/laserlabel/pkg/api"
	"github.com/borud/laserlabel/pkg/fonts"
	"github.com/borud/laserlabel/web"
)

type serveCmd struct {
	Listen   string   `kong:"default='127.0.0.1:8080',env='LASERLABEL_LISTEN',help='listen address; use 0.0.0.0:8080 to allow other devices on the network'"`
	BasePath string   `kong:"env='LASERLABEL_BASE_PATH',help='URL path to serve under, e.g. /laser when behind a reverse proxy'"`
	Host     string   `kong:"short='H',env='LASERLABEL_HOST',help='optional machine address to connect to at startup (machines can also be picked in the UI)'"`
	Name     string   `kong:"env='LASERLABEL_MACHINE',help='optional machine name to discover and connect to at startup'"`
	FontDir  []string `kong:"env='LASERLABEL_FONT_DIRS',help='font directories to scan (default: system font directories)'"`
}

func (s *serveCmd) Run(_ *Options) error {
	dirs := s.FontDir
	if len(dirs) == 0 {
		dirs = fonts.DefaultDirs()
	}
	cat := fonts.NewCatalog(dirs, slog.Default())
	defer cat.Close()
	slog.Info("fonts loaded", "count", len(cat.Fonts()))

	srv := api.New(api.Config{Catalog: cat, Logger: slog.Default(), Static: web.Dist()})
	defer srv.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	go srv.Discover(ctx)

	if s.Host != "" || s.Name != "" {
		if err := srv.Connect(ctx, s.Host, s.Name); err != nil {
			slog.Warn("could not connect to machine", "err", err)
		}
	}

	hs := &http.Server{Addr: s.Listen, Handler: withBasePath(s.BasePath, srv), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = hs.Shutdown(shutdownCtx)
	}()

	slog.Info("serving web UI", "url", "http://"+s.Listen+strings.TrimSuffix(s.BasePath, "/")+"/")
	err := hs.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// withBasePath serves h under base (e.g. "/laser"), redirecting the bare
// base path to base + "/" so that relative URLs in the UI resolve.
func withBasePath(base string, h http.Handler) http.Handler {
	base = "/" + strings.Trim(base, "/")
	if base == "/" {
		return h
	}

	mux := http.NewServeMux()
	mux.Handle(base+"/", http.StripPrefix(base, h))
	mux.Handle(base, http.RedirectHandler(base+"/", http.StatusMovedPermanently))
	return mux
}
