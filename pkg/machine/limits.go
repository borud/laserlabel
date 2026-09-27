package machine

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Machine models as reported in the status C field and by "model".
const (
	ModelCarveraAir = 2
	ModelZ1         = 4
)

// Limits are the soft travel limits in machine coordinates. Homing puts
// the machine at the maximum end of each axis, so positions run from Min
// up to about 0.
type Limits struct {
	XMin float64 `json:"xMin"`
	YMin float64 `json:"yMin"`
	XMax float64 `json:"xMax"`
	YMax float64 `json:"yMax"`

	// Source says where the limits came from: "machine" or "default".
	Source string `json:"source"`
}

// defaultLimits are fallbacks per model, used when the machine doesn't
// answer: the Carvera Air values come from config2.default, the Z1 values
// were read from a Z1 Pro (2026-09-27).
var defaultLimits = map[int]Limits{
	ModelCarveraAir: {XMin: -302, YMin: -212, Source: "default"},
	ModelZ1:         {XMin: -207, YMin: -206, Source: "default"},
}

// ErrNoLimits is returned when the machine doesn't report its limits.
var ErrNoLimits = errors.New("soft limits not available")

// SoftLimits reads the soft limits from the machine configuration, falling
// back to known defaults for the model.
func (c *Conn) SoftLimits(ctx context.Context, model int) (Limits, error) {
	xmin, xerr := c.configFloat(ctx, "soft_endstop.x_min")
	ymin, yerr := c.configFloat(ctx, "soft_endstop.y_min")
	if xerr == nil && yerr == nil {
		return Limits{XMin: xmin, YMin: ymin, Source: "machine"}, nil
	}
	if l, ok := defaultLimits[model]; ok {
		return l, nil
	}
	return Limits{}, errors.Join(ErrNoLimits, xerr, yerr)
}

// configFloat reads a numeric setting. Machines differ in which config
// sources answer (the Z1 only answers "sd", even without an SD card), so
// the sd source is tried first and then the config cache.
func (c *Conn) configFloat(ctx context.Context, key string) (float64, error) {
	var errs []error
	for _, cmd := range []string{"config-get sd " + key, "config-get " + key} {
		lines, err := c.Command(ctx, cmd)
		if err != nil {
			return 0, err
		}
		v, err := parseConfigValue(lines, key)
		if err == nil {
			return v, nil
		}
		errs = append(errs, err)
	}
	return 0, errors.Join(errs...)
}

// parseConfigValue parses "<source>: <key> is set to <value>".
func parseConfigValue(lines []string, key string) (float64, error) {
	for _, l := range lines {
		_, value, ok := strings.Cut(l, key+" is set to ")
		if !ok {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return 0, fmt.Errorf("parse %s: %w", key, err)
		}
		return v, nil
	}
	return 0, fmt.Errorf("%s not reported", key)
}

// Contains reports whether the rectangle (x0,y0)-(x1,y1) in machine
// coordinates lies within the limits.
func (l Limits) Contains(x0, y0, x1, y1 float64) bool {
	return x0 >= l.XMin && y0 >= l.YMin && x1 <= l.XMax && y1 <= l.YMax
}
