package machine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// Offsets are the G54 work coordinate offsets in machine coordinates.
type Offsets struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// WorkOffsets reads the G54 offsets with "$#".
func (c *Conn) WorkOffsets(ctx context.Context) (Offsets, error) {
	lines, err := c.Command(ctx, "$#")
	if err != nil {
		return Offsets{}, err
	}
	return parseG54(lines)
}

// parseG54 finds "[G54:x,y,z,...]" in $# output.
func parseG54(lines []string) (Offsets, error) {
	for _, l := range lines {
		rest, ok := strings.CutPrefix(strings.TrimSpace(l), "[G54:")
		if !ok {
			continue
		}
		f := strings.Split(strings.TrimSuffix(rest, "]"), ",")
		if len(f) < 3 {
			break
		}

		var v [3]float64
		for i := range 3 {
			n, err := strconv.ParseFloat(strings.TrimSpace(f[i]), 64)
			if err != nil {
				return Offsets{}, fmt.Errorf("parse G54 offsets %q: %w", l, err)
			}
			v[i] = n
		}
		return Offsets{X: v[0], Y: v[1], Z: v[2]}, nil
	}
	return Offsets{}, fmt.Errorf("no G54 offsets in reply %q", lines)
}
