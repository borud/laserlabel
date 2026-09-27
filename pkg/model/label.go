package model

// Align is the horizontal alignment of a text line.
type Align string

// Horizontal alignments.
const (
	AlignLeft   Align = "left"
	AlignCenter Align = "center"
	AlignRight  Align = "right"
)

// TextLine is a single line of label text.
type TextLine struct {
	Text   string  `json:"text"`
	FontID string  `json:"fontID"`
	SizeMM float64 `json:"sizeMM"`
	Align  Align   `json:"align"`
}

// Label is the content to be burned onto a workpiece. The text block is
// centred on the workpiece and then shifted by OffsetX and OffsetY.
type Label struct {
	Lines []TextLine `json:"lines"`

	// LineSpacing is the baseline distance as a multiple of line size.
	LineSpacing float64 `json:"lineSpacing"`
	OffsetX     float64 `json:"offsetX"`
	OffsetY     float64 `json:"offsetY"`

	// Fit scales the whole text block to the largest size that fits inside
	// the workpiece margin. Line sizes then only set the lines' relative
	// sizes.
	Fit bool `json:"fit"`
}
