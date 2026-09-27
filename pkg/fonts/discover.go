// Package fonts discovers system fonts and converts text to glyph outlines.
package fonts

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/borud/laserlabel/pkg/model"
	"golang.org/x/image/font/sfnt"
)

// DefaultDirs returns the platform's usual font directories.
func DefaultDirs() []string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		return []string{
			"/System/Library/Fonts",
			"/Library/Fonts",
			filepath.Join(home, "Library/Fonts"),
		}
	case "windows":
		return []string{filepath.Join(os.Getenv("WINDIR"), "Fonts")}
	}
	return []string{
		"/usr/share/fonts",
		"/usr/local/share/fonts",
		filepath.Join(home, ".local/share/fonts"),
		filepath.Join(home, ".fonts"),
	}
}

// Discover walks dirs recursively and returns every font face it can
// parse, sorted by family and style. Unreadable files are skipped.
func Discover(dirs []string, logger *slog.Logger) []model.FontInfo {
	if logger == nil {
		logger = slog.Default()
	}

	infos := []model.FontInfo{}
	for _, dir := range dirs {
		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !isFontFile(path) {
				return nil
			}

			faces, err := describe(path)
			if err != nil {
				logger.Debug("skipping font", "path", path, "err", err)
				return nil
			}
			infos = append(infos, faces...)
			return nil
		})
	}

	slices.SortFunc(infos, func(a, b model.FontInfo) int {
		if c := strings.Compare(a.Family, b.Family); c != 0 {
			return c
		}
		return strings.Compare(a.Style, b.Style)
	})
	return infos
}

func isFontFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttf", ".otf", ".ttc", ".otc":
		return true
	}
	return false
}

// describe returns the faces in the font file at path.
func describe(path string) ([]model.FontInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	coll, err := sfnt.ParseCollectionReaderAt(f)
	if err != nil {
		return nil, err
	}

	var buf sfnt.Buffer
	faces := make([]model.FontInfo, 0, coll.NumFonts())
	for i := range coll.NumFonts() {
		font, err := coll.Font(i)
		if err != nil {
			return nil, err
		}
		family, style := names(font, &buf)
		if hidden(family) {
			continue
		}
		faces = append(faces, model.FontInfo{
			ID:     fontID(path, i),
			Family: family,
			Style:  style,
			Path:   path,
			Index:  i,
		})
	}
	return faces, nil
}

// names returns the family and style names, preferring the typographic
// names where present.
func names(f *sfnt.Font, buf *sfnt.Buffer) (string, string) {
	family, err := f.Name(buf, sfnt.NameIDTypographicFamily)
	if err != nil || family == "" {
		family, _ = f.Name(buf, sfnt.NameIDFamily)
	}
	style, err := f.Name(buf, sfnt.NameIDTypographicSubfamily)
	if err != nil || style == "" {
		style, _ = f.Name(buf, sfnt.NameIDSubfamily)
	}
	return family, style
}

// hidden reports whether a family should be left out of the catalog:
// unnamed faces and macOS system-internal fonts (names starting with ".").
func hidden(family string) bool {
	return family == "" || strings.HasPrefix(family, ".")
}

func fontID(path string, index int) string {
	return path + "#" + strconv.Itoa(index)
}

// parseID splits a font ID into path and face index.
func parseID(id string) (string, int, bool) {
	i := strings.LastIndexByte(id, '#')
	if i < 0 {
		return id, 0, true
	}
	n, err := strconv.Atoi(id[i+1:])
	if err != nil {
		return "", 0, false
	}
	return id[:i], n, true
}
