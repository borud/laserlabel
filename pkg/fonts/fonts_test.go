package fonts

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/borud/laserlabel/pkg/geom"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

func testCatalog(t *testing.T) *Catalog {
	t.Helper()
	c := NewCatalog(nil, nil)
	_, err := c.AddData("goregular", goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestOutlineCapHeight(t *testing.T) {
	c := testCatalog(t)

	txt, err := c.Outline("goregular", "H", 10)
	if err != nil {
		t.Fatal(err)
	}
	b := txt.Paths.Bounds()
	if math.Abs(b.Min.Y) > 0.01 || math.Abs(b.Max.Y-10) > 0.05 {
		t.Fatalf("H bounds %v, want y 0..10", b)
	}
	if len(txt.Paths) != 1 {
		t.Fatalf("H has %d contours, want 1", len(txt.Paths))
	}
}

func TestOutlineHolesAndAdvance(t *testing.T) {
	c := testCatalog(t)

	o, err := c.Outline("goregular", "O", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(o.Paths) != 2 {
		t.Fatalf("O has %d contours, want 2", len(o.Paths))
	}

	// The hole must survive nonzero hatching: no hatch midpoint at centre.
	b := o.Paths.Bounds()
	centre := b.Center()
	for _, p := range geom.Hatch(o.Paths, 0.1, 0, false) {
		mid := p.Points[0].Add(p.Points[1]).Scale(0.5)
		if mid.Dist(centre) < 1 {
			t.Fatalf("hatch inside the hole of O at %v", mid)
		}
	}

	one, err := c.Outline("goregular", "O", 10)
	if err != nil {
		t.Fatal(err)
	}
	two, err := c.Outline("goregular", "OO", 10)
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(two.Advance-2*one.Advance) > 1e-6 || one.Advance <= 0 {
		t.Fatalf("advance: one %v, two %v", one.Advance, two.Advance)
	}
	if two.Paths.Bounds().Max.X <= one.Paths.Bounds().Max.X {
		t.Fatal("second glyph not placed after the first")
	}
}

func TestOutlineUnknownFont(t *testing.T) {
	c := testCatalog(t)
	_, err := c.Outline("nope", "A", 5)
	if !errors.Is(err, ErrUnknownFont) {
		t.Fatalf("got %v, want ErrUnknownFont", err)
	}
}

func TestDiscover(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name string, data []byte) {
		if err := os.WriteFile(name, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "GoRegular.ttf"), goregular.TTF)
	write(filepath.Join(sub, "GoBold.TTF"), gobold.TTF)
	write(filepath.Join(dir, "broken.ttf"), []byte("not a font"))
	write(filepath.Join(dir, "readme.txt"), []byte("hi"))

	infos := Discover([]string{dir, filepath.Join(dir, "missing")}, nil)
	if len(infos) != 2 {
		t.Fatalf("got %d fonts, want 2: %+v", len(infos), infos)
	}
	if infos[0].Family != "Go" || infos[0].Style != "Bold" || infos[1].Style != "Regular" {
		t.Fatalf("unexpected fonts: %+v", infos)
	}

	c := NewCatalog([]string{dir}, nil)
	defer c.Close()
	txt, err := c.Outline(infos[1].ID, "Hi", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(txt.Paths) == 0 {
		t.Fatal("no paths from discovered font")
	}
}

func TestFind(t *testing.T) {
	c := NewCatalog(nil, nil)
	if _, err := c.AddData("bold", gobold.TTF); err != nil {
		t.Fatal(err)
	}
	if _, err := c.AddData("regular", goregular.TTF); err != nil {
		t.Fatal(err)
	}

	for name, want := range map[string]string{"go": "regular", "Go Bold": "bold", "bold": "bold"} {
		info, ok := c.Find(name)
		if !ok || info.ID != want {
			t.Errorf("Find(%q) = %q %v, want %q", name, info.ID, ok, want)
		}
	}
	if _, ok := c.Find("Comic Sans"); ok {
		t.Error("found a font that isn't there")
	}
}

func TestHidden(t *testing.T) {
	if !hidden("") || !hidden(".SF NS") || hidden("Helvetica") {
		t.Fatal("hidden")
	}
}

func TestParseID(t *testing.T) {
	path, idx, ok := parseID("/a/b#c.ttc#3")
	if !ok || path != "/a/b#c.ttc" || idx != 3 {
		t.Fatalf("got %q %d %v", path, idx, ok)
	}
	if _, _, ok := parseID("/a.ttf#x"); ok {
		t.Fatal("expected failure")
	}
}
