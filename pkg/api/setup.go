package api

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/borud/laserlabel/pkg/geom"
	"github.com/borud/laserlabel/pkg/machine"
)

// Tools that can be selected from the UI.
const (
	ToolProbe = "probe"
	ToolLaser = "laser"
)

// laserExitTimeout bounds the wait for laser mode to switch off.
const laserExitTimeout = 5 * time.Second

// Home homes all axes.
func (m *machineManager) Home() error {
	return m.send("$H")
}

// Pointer switches the probe's laser pointer on or off. The firmware only
// powers it while the probe is loaded, and turns it off after a timeout.
func (m *machineManager) Pointer(on bool) error {
	if on {
		return m.send("M831")
	}
	return m.send("M832")
}

// Unlock clears an alarm that doesn't need a reset.
func (m *machineManager) Unlock() error {
	return m.send("$X")
}

// Reset soft-resets the machine, which is needed for hard faults.
func (m *machineManager) Reset() error {
	c, err := m.connection()
	if err != nil {
		return err
	}
	m.addLog("> reset (Ctrl-X)")
	return c.Realtime(0x18)
}

// Tool starts a guided tool change to the probe or the laser module.
func (m *machineManager) Tool(ctx context.Context, tool string) error {
	if !m.idle() {
		return ErrBusy
	}

	switch tool {
	case ToolProbe:
		if err := m.leaveLaserMode(ctx); err != nil {
			return err
		}
		return m.send("M6 T0")
	case ToolLaser:
		return m.send("M321")
	}
	return fmt.Errorf("%w: unknown tool %q", errBadRequest, tool)
}

// ZeroAxes makes the current position the G54 origin on the given axes
// ("X", "Y" or "XY").
func (m *machineManager) ZeroAxes(ctx context.Context, axes string) error {
	var words string
	switch axes {
	case "X":
		words = "X0"
	case "Y":
		words = "Y0"
	case "XY", "":
		words = "X0 Y0"
	default:
		return fmt.Errorf("%w: cannot zero %q", errBadRequest, axes)
	}
	if !m.idle() {
		return ErrBusy
	}

	if _, err := m.Command(ctx, "G10 L20 P1 "+words); err != nil {
		return err
	}
	m.originMoved()
	m.refreshOffsets(ctx)
	return nil
}

// AcceptOrigin accepts the origin already stored on the machine.
func (m *machineManager) AcceptOrigin() error {
	if _, err := m.connection(); err != nil {
		return err
	}
	m.markOriginSet()
	return nil
}

func (m *machineManager) markOriginSet() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.originSet = true
}

// originMoved records an XY origin change: the old trace no longer applies.
func (m *machineManager) originMoved() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.originSet, m.traced = true, false
}

// NudgeOrigin moves the G54 XY origin by (dx, dy) mm.
func (m *machineManager) NudgeOrigin(ctx context.Context, dx, dy float64) error {
	if !m.idle() {
		return ErrBusy
	}

	o, err := m.Offsets(ctx)
	if err != nil {
		return err
	}
	if _, err := m.Command(ctx, fmt.Sprintf("G10 L2 P1 X%.3f Y%.3f", o.X+dx, o.Y+dy)); err != nil {
		return err
	}
	m.originMoved()
	m.refreshOffsets(ctx)
	return nil
}

// Offsets returns the G54 offsets.
func (m *machineManager) Offsets(ctx context.Context) (machine.Offsets, error) {
	c, err := m.connection()
	if err != nil {
		return machine.Offsets{}, err
	}
	return c.WorkOffsets(ctx)
}

// Probe traces the workpiece outline with the probe's pointer laser
// and/or probes Z at the workpiece centre, using the firmware's M495. The
// work origin is the workpiece centre, and the probe point must lie inside
// the printable area (the workpiece inset by margin) so the probe never
// touches down near an edge. The firmware changes to the probe first if
// needed; it requires the machine to be homed and not in laser mode.
func (m *machineManager) Probe(ctx context.Context, width, height, margin float64, outline, z bool) error {
	if !outline && !z {
		return fmt.Errorf("%w: nothing to do", errBadRequest)
	}
	if width <= 0 || height <= 0 {
		return fmt.Errorf("%w: workpiece size must be positive", errBadRequest)
	}

	// M495 probes at (X+O, Y+F); with X,Y at the lower left corner and
	// O,F at half the size, that is the workpiece centre.
	probeX, probeY := width/2, height/2
	printable := geom.CenteredRect(width, height).Inset(margin)
	probePoint := geom.Point{X: -width/2 + probeX, Y: -height/2 + probeY}
	noArea := printable.Width() <= 0 || printable.Height() <= 0
	if z && (noArea || !printable.Contains(geom.Rect{Min: probePoint, Max: probePoint})) {
		return fmt.Errorf("%w: the probe point must lie inside the margin; the margin (%.1f mm) leaves no printable area", errBadRequest, margin)
	}
	if !m.idle() {
		return ErrBusy
	}

	m.mu.Lock()
	originSet := m.originSet
	m.mu.Unlock()
	if !originSet {
		return fmt.Errorf("%w: set the work origin at the centre of the workpiece first (or accept the saved origin)", errBadRequest)
	}
	if err := m.checkLimits(ctx, width, height); err != nil {
		return err
	}
	if err := m.leaveLaserMode(ctx); err != nil {
		return err
	}

	x, y := -width/2, -height/2
	cmd := fmt.Sprintf("M495 X%.3f Y%.3f", x, y)
	if outline {
		cmd += fmt.Sprintf(" C%.3f D%.3f", width/2, height/2)
	}
	if z {
		cmd += fmt.Sprintf(" O%.3f F%.3f", probeX, probeY)
	}
	if _, err := m.Command(ctx, cmd); err != nil {
		return err
	}

	m.mu.Lock()
	m.pendingTrace = m.pendingTrace || outline
	m.pendingZProbe = m.pendingZProbe || z
	m.mu.Unlock()
	return nil
}

// ResetCounter resets the completed label counter.
func (m *machineManager) ResetCounter() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.labelsDone = 0
	return nil
}

// checkLimits verifies that the workpiece rectangle around the work origin
// lies within the machine's soft limits.
func (m *machineManager) checkLimits(ctx context.Context, width, height float64) error {
	o, err := m.Offsets(ctx)
	if err != nil {
		return err
	}
	lim, err := m.softLimits(ctx)
	if err != nil {
		return err
	}

	x0, y0 := o.X-width/2, o.Y-height/2
	x1, y1 := o.X+width/2, o.Y+height/2
	if lim.Contains(x0, y0, x1, y1) {
		return nil
	}
	return fmt.Errorf("%w: the workpiece around the work origin spans machine X %.1f..%.1f, Y %.1f..%.1f, outside the travel limits X %.1f..%.1f, Y %.1f..%.1f; move the origin to the centre of the workpiece",
		errBadRequest, x0, x1, y0, y1, lim.XMin, lim.XMax, lim.YMin, lim.YMax)
}

// softLimits returns the machine's soft limits, reading them once per
// connection. When they can't be read, only the home side is checked.
func (m *machineManager) softLimits(ctx context.Context) (machine.Limits, error) {
	m.mu.Lock()
	cached, st := m.limits, m.status
	m.mu.Unlock()
	if cached != nil {
		return *cached, nil
	}

	c, err := m.connection()
	if err != nil {
		return machine.Limits{}, err
	}
	model := 0
	if st != nil {
		model = st.Model
	}

	lim, err := c.SoftLimits(ctx, model)
	if err != nil {
		m.logger.Warn("soft limits unknown, only checking the home side", "err", err)
		return machine.Limits{XMin: math.Inf(-1), YMin: math.Inf(-1), Source: "unknown"}, nil
	}

	m.mu.Lock()
	m.limits = &lim
	m.mu.Unlock()
	return lim, nil
}

// leaveLaserMode sends M322 if the machine is in laser mode and waits for
// it to take effect.
func (m *machineManager) leaveLaserMode(ctx context.Context) error {
	m.mu.Lock()
	inLaser := m.status != nil && m.status.LaserMode
	m.mu.Unlock()
	if !inLaser {
		return nil
	}

	c, err := m.connection()
	if err != nil {
		return err
	}
	if err := m.send("M322"); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, laserExitTimeout)
	defer cancel()
	for {
		st, err := c.Status(ctx)
		if err != nil {
			return fmt.Errorf("leave laser mode: %w", err)
		}
		if !st.LaserMode {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("leave laser mode: %w", ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
}

// send logs and sends a line without waiting for a reply.
func (m *machineManager) send(line string) error {
	c, err := m.connection()
	if err != nil {
		return err
	}
	m.addLog("> " + line)
	return c.Send(line)
}

// Go-to targets.
const (
	TargetOrigin      = "origin"
	TargetCenter      = "center"
	TargetFrontLeft   = "front-left"
	TargetFrontRight  = "front-right"
	TargetBackLeft    = "back-left"
	TargetBackRight   = "back-right"
	gotoCornerInsetMM = 10

	// safeZ is the machine Z to raise to before moving XY; the machine is
	// homed at the top, so this is close to maximum clearance.
	safeZ = -2
)

// GoTo raises Z to a safe height and moves to a named position: a corner
// or the centre of the travel area, or the work origin. X increases to the
// right and Y to the back; homing puts the machine at the back right.
func (m *machineManager) GoTo(ctx context.Context, target string) error {
	if !m.idle() {
		return ErrBusy
	}

	m.mu.Lock()
	homed := m.homed
	m.mu.Unlock()
	if !homed {
		return fmt.Errorf("%w: home the machine first", errBadRequest)
	}

	move, err := m.gotoMove(ctx, target)
	if err != nil {
		return err
	}
	if err := m.send(fmt.Sprintf("G53 G0 Z%d", safeZ)); err != nil {
		return err
	}
	return m.send(move)
}

// gotoMove returns the XY move for a target.
func (m *machineManager) gotoMove(ctx context.Context, target string) (string, error) {
	if target == TargetOrigin {
		return "G90 G0 X0 Y0", nil
	}

	lim, err := m.softLimits(ctx)
	if err != nil {
		return "", err
	}
	if math.IsInf(lim.XMin, 0) || math.IsInf(lim.YMin, 0) {
		return "", fmt.Errorf("travel limits of this machine are unknown")
	}

	left, right := lim.XMin+gotoCornerInsetMM, lim.XMax-gotoCornerInsetMM
	front, back := lim.YMin+gotoCornerInsetMM, lim.YMax-gotoCornerInsetMM

	var x, y float64
	switch target {
	case TargetCenter:
		x, y = (lim.XMin+lim.XMax)/2, (lim.YMin+lim.YMax)/2
	case TargetFrontLeft:
		x, y = left, front
	case TargetFrontRight:
		x, y = right, front
	case TargetBackLeft:
		x, y = left, back
	case TargetBackRight:
		x, y = right, back
	default:
		return "", fmt.Errorf("%w: unknown target %q", errBadRequest, target)
	}
	return fmt.Sprintf("G53 G0 X%.3f Y%.3f", x, y), nil
}
