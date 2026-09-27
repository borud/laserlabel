package model

// RenderMode determines how glyph outlines are turned into toolpaths.
type RenderMode string

// Render modes.
const (
	RenderFill    RenderMode = "fill"
	RenderOutline RenderMode = "outline"
	RenderBoth    RenderMode = "both"
)

// LaserSettings are the burn parameters, typically saved as a material preset.
type LaserSettings struct {
	ID   int64  `db:"id" json:"id"`
	Name string `db:"name" json:"name"`

	// Power is the laser power in the range 0 to 1.
	Power        float64    `db:"power" json:"power"`
	FeedMMPerMin float64    `db:"feed" json:"feed"`
	Passes       int        `db:"passes" json:"passes"`
	Mode         RenderMode `db:"mode" json:"mode"`

	HatchInterval float64 `db:"hatch_interval_mm" json:"hatchIntervalMM"`
	HatchAngle    float64 `db:"hatch_angle_deg" json:"hatchAngleDeg"`
	Bidirectional bool    `db:"bidirectional" json:"bidirectional"`
}
