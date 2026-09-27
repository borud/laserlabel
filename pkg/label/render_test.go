package label

import (
	"math"
	"testing"

	"github.com/borud/laserlabel/pkg/fonts"
	"github.com/borud/laserlabel/pkg/model"
	"golang.org/x/image/font/gofont/goregular"
)

const tol = 0.05

var lid = model.Workpiece{Name: "lid", WidthMM: 62, HeightMM: 62, MarginMM: 4}

func testCatalog(t *testing.T) *fonts.Catalog {
	t.Helper()
	c := fonts.NewCatalog(nil, nil)
	if _, err := c.AddData("go", goregular.TTF); err != nil {
		t.Fatal(err)
	}
	return c
}

func line(text string, size float64, a model.Align) model.TextLine {
	return model.TextLine{Text: text, FontID: "go", SizeMM: size, Align: a}
}

func TestRenderCentred(t *testing.T) {
	res, err := Render(testCatalog(t), model.Label{
		Lines: []model.TextLine{line("Jam", 8, model.AlignCenter), line("2026", 5, model.AlignCenter)},
	}, lid)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", res.Warnings)
	}

	c := res.Paths.Bounds().Center()
	if math.Abs(c.X) > tol || math.Abs(c.Y) > tol {
		t.Fatalf("ink centre %v, want origin", c)
	}
	if res.Printable.Width() != 54 {
		t.Fatalf("printable %v", res.Printable)
	}
}

func TestRenderOffset(t *testing.T) {
	res, err := Render(testCatalog(t), model.Label{
		Lines:   []model.TextLine{line("X", 5, model.AlignCenter)},
		OffsetX: 3,
		OffsetY: -2,
	}, lid)
	if err != nil {
		t.Fatal(err)
	}
	c := res.Paths.Bounds().Center()
	if math.Abs(c.X-3) > tol || math.Abs(c.Y+2) > tol {
		t.Fatalf("ink centre %v, want (3,-2)", c)
	}
}

func TestRenderAlignment(t *testing.T) {
	cat := testCatalog(t)
	render := func(a model.Align) (float64, float64, float64, float64) {
		res, err := Render(cat, model.Label{
			Lines: []model.TextLine{line("WWWWW", 5, model.AlignCenter), line("i", 5, a)},
		}, lid)
		if err != nil {
			t.Fatal(err)
		}
		// The short line is the last contours; compare its bounds with the block.
		block := res.Paths.Bounds()
		short := res.Paths[len(res.Paths)-2:].Bounds()
		return block.Min.X, block.Max.X, short.Min.X, short.Max.X
	}

	bmin, _, smin, _ := render(model.AlignLeft)
	if math.Abs(bmin-smin) > tol {
		t.Fatalf("left: block %v, line %v", bmin, smin)
	}
	_, bmax, _, smax := render(model.AlignRight)
	if math.Abs(bmax-smax) > tol {
		t.Fatalf("right: block %v, line %v", bmax, smax)
	}
	bmin, bmax, smin, smax = render(model.AlignCenter)
	if math.Abs((bmin+bmax)/2-(smin+smax)/2) > tol {
		t.Fatal("center: line not centred on block")
	}
}

func TestRenderLineOrder(t *testing.T) {
	res, err := Render(testCatalog(t), model.Label{
		Lines:       []model.TextLine{line("A", 5, model.AlignCenter), line("", 5, model.AlignCenter), line("B", 5, model.AlignCenter)},
		LineSpacing: 1.5,
	}, lid)
	if err != nil {
		t.Fatal(err)
	}
	// The blank line keeps its space: A's cap top to B's baseline spans two line steps.
	height := res.Paths.Bounds().Height()
	if math.Abs(height-(5+2*5*1.5)) > tol {
		t.Fatalf("block height %v, want %v", height, 5+2*5*1.5)
	}
}

func TestRenderWarnings(t *testing.T) {
	cat := testCatalog(t)

	res, err := Render(cat, model.Label{Lines: []model.TextLine{line("TOO WIDE FOR LID", 10, model.AlignCenter)}}, lid)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 1 || res.Warnings[0].Code != WarnOutsideMargin {
		t.Fatalf("warnings %v", res.Warnings)
	}

	res, err = Render(cat, model.Label{Lines: []model.TextLine{line("  ", 10, model.AlignCenter)}}, lid)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 1 || res.Warnings[0].Code != WarnEmpty {
		t.Fatalf("warnings %v", res.Warnings)
	}

	_, err = Render(cat, model.Label{Lines: []model.TextLine{{Text: "x", FontID: "nope", SizeMM: 5}}}, lid)
	if err == nil {
		t.Fatal("expected error for unknown font")
	}
}

func TestRenderFit(t *testing.T) {
	cat := testCatalog(t)

	// Wide text: width is the limiting dimension.
	res, err := Render(cat, model.Label{Lines: []model.TextLine{line("WIDE WORD", 5, model.AlignCenter)}, Fit: true}, lid)
	if err != nil {
		t.Fatal(err)
	}
	ink := res.Paths.Bounds()
	if math.Abs(ink.Width()-res.Printable.Width()) > tol || ink.Height() > res.Printable.Height()+tol {
		t.Fatalf("fitted ink %v in %v", ink, res.Printable)
	}
	if res.Scale <= 1 || len(res.Warnings) != 0 {
		t.Fatalf("scale %v warnings %v", res.Scale, res.Warnings)
	}

	// Tall text: height limits, and text that is too big shrinks.
	res, err = Render(cat, model.Label{Lines: []model.TextLine{
		line("A", 30, model.AlignCenter), line("B", 30, model.AlignCenter), line("C", 30, model.AlignCenter),
	}, Fit: true}, lid)
	if err != nil {
		t.Fatal(err)
	}
	ink = res.Paths.Bounds()
	if math.Abs(ink.Height()-res.Printable.Height()) > tol || res.Scale >= 1 {
		t.Fatalf("tall: ink %v scale %v", ink, res.Scale)
	}
	c := ink.Center()
	if math.Abs(c.X) > tol || math.Abs(c.Y) > tol {
		t.Fatalf("fitted text not centred: %v", c)
	}

	// No printable area.
	res, err = Render(cat, model.Label{Lines: []model.TextLine{line("X", 5, model.AlignCenter)}, Fit: true},
		model.Workpiece{WidthMM: 10, HeightMM: 10, MarginMM: 6})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("warnings %v", res.Warnings)
	}
}
