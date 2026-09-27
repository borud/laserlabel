package fonts

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/borud/laserlabel/pkg/model"
	"golang.org/x/image/font/sfnt"
)

// ErrUnknownFont is returned for font IDs that are not in the catalog.
var ErrUnknownFont = errors.New("unknown font")

// Catalog holds the known fonts and lazily loads their faces. It is safe
// for concurrent use.
type Catalog struct {
	mu     sync.Mutex
	infos  []model.FontInfo
	byID   map[string]model.FontInfo
	loaded map[string]*sfnt.Font
	files  []io.Closer
	buf    sfnt.Buffer
}

// NewCatalog creates a catalog of the fonts found in dirs.
func NewCatalog(dirs []string, logger *slog.Logger) *Catalog {
	c := &Catalog{
		byID:   map[string]model.FontInfo{},
		loaded: map[string]*sfnt.Font{},
	}
	for _, info := range Discover(dirs, logger) {
		c.infos = append(c.infos, info)
		c.byID[info.ID] = info
	}
	return c
}

// AddData adds an in-memory font under the given ID.
func (c *Catalog) AddData(id string, data []byte) (model.FontInfo, error) {
	f, err := sfnt.Parse(data)
	if err != nil {
		return model.FontInfo{}, fmt.Errorf("parse font %q: %w", id, err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	family, style := names(f, &c.buf)
	info := model.FontInfo{ID: id, Family: family, Style: style}
	c.infos = append(c.infos, info)
	c.byID[id] = info
	c.loaded[id] = f
	return info, nil
}

// Fonts returns the fonts in the catalog.
func (c *Catalog) Fonts() []model.FontInfo {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]model.FontInfo, len(c.infos))
	copy(out, c.infos)
	return out
}

// Close releases open font files.
func (c *Catalog) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var errs []error
	for _, f := range c.files {
		errs = append(errs, f.Close())
	}
	c.files = nil
	c.loaded = map[string]*sfnt.Font{}
	return errors.Join(errs...)
}

// face returns the loaded face for id. The caller must hold c.mu.
func (c *Catalog) face(id string) (*sfnt.Font, error) {
	if f, ok := c.loaded[id]; ok {
		return f, nil
	}
	if _, ok := c.byID[id]; !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownFont, id)
	}

	path, index, ok := parseID(id)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownFont, id)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open font: %w", err)
	}
	coll, err := sfnt.ParseCollectionReaderAt(file)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("parse font %q: %w", path, err)
	}
	f, err := coll.Font(index)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("load face %d of %q: %w", index, path, err)
	}

	c.files = append(c.files, file)
	c.loaded[id] = f
	return f, nil
}

// Find looks up a font by ID, or by "Family Style" or "Family"
// (case-insensitive). A bare family prefers the Regular style.
func (c *Catalog) Find(name string) (model.FontInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if info, ok := c.byID[name]; ok {
		return info, true
	}

	var familyMatch *model.FontInfo
	for i, info := range c.infos {
		if strings.EqualFold(info.Family+" "+info.Style, name) {
			return info, true
		}
		if !strings.EqualFold(info.Family, name) {
			continue
		}
		if familyMatch == nil || strings.EqualFold(info.Style, "Regular") {
			familyMatch = &c.infos[i]
		}
	}
	if familyMatch == nil {
		return model.FontInfo{}, false
	}
	return *familyMatch, true
}
