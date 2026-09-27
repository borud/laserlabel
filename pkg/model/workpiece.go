package model

// Workpiece is the object being labelled. It is always modelled as a
// rectangle; rounded corners and edges are kept clear of the text by
// choosing a large enough margin.
type Workpiece struct {
	ID       int64   `db:"id" json:"id"`
	Name     string  `db:"name" json:"name"`
	WidthMM  float64 `db:"width_mm" json:"widthMM"`
	HeightMM float64 `db:"height_mm" json:"heightMM"`
	MarginMM float64 `db:"margin_mm" json:"marginMM"`
}
