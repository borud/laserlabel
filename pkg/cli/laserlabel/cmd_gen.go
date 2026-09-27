package laserlabel

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/borud/laserlabel/pkg/fonts"
	"github.com/borud/laserlabel/pkg/gcode"
	"github.com/borud/laserlabel/pkg/label"
	"github.com/borud/laserlabel/pkg/model"
)

type genCmd struct {
	Label    genLabelCmd    `kong:"cmd,help='generate G-code for a text label'"`
	Testgrid genTestgridCmd `kong:"cmd,help='generate a power/speed test grid'"`
}

// laserFlags are the burn settings shared by gen subcommands.
type laserFlags struct {
	Power    float64 `kong:"default='0.7',help='laser power 0..1'"`
	Feed     float64 `kong:"default='2800',help='feed rate in mm/min'"`
	Passes   int     `kong:"default='1',help='number of passes'"`
	Mode     string  `kong:"default='fill',enum='fill,outline,both',help='render mode (${enum})'"`
	Interval float64 `kong:"default='0.1',help='hatch line interval in mm'"`
	Angle    float64 `kong:"default='0',help='hatch angle in degrees'"`
	Unidir   bool    `kong:"help='hatch in one direction only'"`
	Framing  string  `kong:"default='full',enum='full,resume,none',help='program framing: full (M321..M322), resume (M321.2), none (${enum})'"`
	Out      string  `kong:"short='o',help='output file (default stdout)'"`
}

type genLabelCmd struct {
	laserFlags `kong:"embed"`

	Text    []string `kong:"short='t',required,help='text line (repeat for more lines)'"`
	Font    string   `kong:"short='f',required,help='font ID, \"Family Style\" or \"Family\"'"`
	Size    float64  `kong:"default='6',help='cap height in mm'"`
	Align   string   `kong:"default='center',enum='left,center,right',help='line alignment (${enum})'"`
	Spacing float64  `kong:"default='1.3',help='line spacing as a multiple of size'"`
	Fit     bool     `kong:"help='scale the text to the largest size that fits inside the margin'"`
	Width   float64  `kong:"default='50',help='workpiece width in mm'"`
	Height  float64  `kong:"default='50',help='workpiece height in mm'"`
	Margin  float64  `kong:"default='4',help='workpiece margin in mm'"`
	FontDir []string `kong:"help='font directories to scan (default: system font directories)'"`
}

type genTestgridCmd struct {
	laserFlags `kong:"embed"`

	Powers []float64 `kong:"default='0.2,0.3,0.4,0.5,0.6',help='row powers 0..1'"`
	Feeds  []float64 `kong:"default='500,1000,1500,2000',help='column feed rates in mm/min'"`
	Cell   float64   `kong:"default='5',help='cell size in mm'"`
	Gap    float64   `kong:"default='2',help='gap between cells in mm'"`
}

func (g *genLabelCmd) Run(_ *Options) error {
	dirs := g.FontDir
	if len(dirs) == 0 {
		dirs = fonts.DefaultDirs()
	}
	cat := fonts.NewCatalog(dirs, slog.Default())
	defer cat.Close()

	font, ok := cat.Find(g.Font)
	if !ok {
		return fmt.Errorf("font %q not found (see 'laserlabel fonts ls')", g.Font)
	}

	lines := make([]model.TextLine, 0, len(g.Text))
	for _, t := range g.Text {
		lines = append(lines, model.TextLine{Text: t, FontID: font.ID, SizeMM: g.Size, Align: model.Align(g.Align)})
	}
	wp := model.Workpiece{WidthMM: g.Width, HeightMM: g.Height, MarginMM: g.Margin}

	res, err := label.Render(cat, model.Label{Lines: lines, LineSpacing: g.Spacing, Fit: g.Fit}, wp)
	if err != nil {
		return err
	}
	for _, w := range res.Warnings {
		slog.Warn(w.Message, "code", w.Code)
	}

	prog := gcode.Burn(res.Paths, g.settings(), gcode.Options{Framing: g.framing()})
	return g.write(prog)
}

func (g *genTestgridCmd) Run(_ *Options) error {
	prog := gcode.TestGrid(gcode.GridOptions{
		Powers:        g.Powers,
		Feeds:         g.Feeds,
		CellMM:        g.Cell,
		GapMM:         g.Gap,
		HatchInterval: g.Interval,
	}, gcode.Options{Framing: g.framing()})
	return g.write(prog)
}

func (l laserFlags) settings() model.LaserSettings {
	return model.LaserSettings{
		Power:         l.Power,
		FeedMMPerMin:  l.Feed,
		Passes:        l.Passes,
		Mode:          model.RenderMode(l.Mode),
		HatchInterval: l.Interval,
		HatchAngle:    l.Angle,
		Bidirectional: !l.Unidir,
	}
}

func (l laserFlags) framing() gcode.Framing {
	switch l.Framing {
	case "full":
		return gcode.FramingFull
	case "resume":
		return gcode.FramingResume
	case "none":
		return gcode.FramingNone
	default:
		panic(fmt.Sprintf("unexpected framing: %q", l.Framing))
	}
}

func (l laserFlags) write(p gcode.Program) error {
	slog.Info("generated program",
		"lines", len(p.Lines),
		"burnMM", fmt.Sprintf("%.0f", p.Stats.BurnMM),
		"travelMM", fmt.Sprintf("%.0f", p.Stats.TravelMM),
		"estimate", p.Stats.Duration.Round(1e9))

	var w io.Writer = os.Stdout
	if l.Out != "" {
		f, err := os.Create(l.Out)
		if err != nil {
			return fmt.Errorf("create output: %w", err)
		}
		defer f.Close()
		w = f
	}

	_, err := io.WriteString(w, p.String())
	return err
}
