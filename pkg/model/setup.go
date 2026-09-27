package model

// ProbeResult records a completed machine setup: the G54 origin (machine
// coordinates of the workpiece centre and surface) for a given workpiece.
type ProbeResult struct {
	ID          int64    `db:"id" json:"id"`
	WorkpieceID int64    `db:"workpiece_id" json:"workpieceID"`
	OriginX     float64  `db:"origin_x" json:"originX"`
	OriginY     float64  `db:"origin_y" json:"originY"`
	Z           float64  `db:"z" json:"z"`
	TakenAt     UnixTime `db:"taken_at" json:"takenAt"`
}
