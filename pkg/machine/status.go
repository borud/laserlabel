package machine

import (
	"fmt"
	"strconv"
	"strings"
)

// State is the machine state reported in status reports.
type State string

// Known machine states.
const (
	StateIdle  State = "Idle"
	StateRun   State = "Run"
	StateHold  State = "Hold"
	StateAlarm State = "Alarm"
	StateHome  State = "Home"
	StateTool  State = "Tool"
	StateWait  State = "Wait"
	StatePause State = "Pause"
)

// Status is a parsed status report, e.g.
//
//	<Idle|MPos:-1,-1,-1,0,0|WPos:147,111,99,-40,0|F:0,3000,100|T:1,-15.491,-1|L:0,0,0,0,100|C:2,1,0,1>
type Status struct {
	State State `json:"state"`

	// MPos and WPos are X, Y, Z positions in machine and work coordinates.
	MPos [3]float64 `json:"mpos"`
	WPos [3]float64 `json:"wpos"`

	Feed float64 `json:"feed"`

	// Tool is the active tool number: 0 is the probe, 8888 the Air
	// laser module, -1 none.
	Tool       int     `json:"tool"`
	ToolOffset float64 `json:"toolOffset"`
	TargetTool int     `json:"targetTool"`

	LaserMode    bool    `json:"laserMode"`
	LaserOn      bool    `json:"laserOn"`
	LaserTesting bool    `json:"laserTesting"`
	LaserPower   float64 `json:"laserPower"`
	LaserScale   float64 `json:"laserScale"`

	// Playing is set while a file is being played; Progress holds the
	// played lines, percentage and elapsed seconds.
	Playing  bool   `json:"playing"`
	Progress [3]int `json:"progress"`
	Model    int    `json:"model"`

	// Halt is the halt reason while the machine is halted (see HaltReason),
	// 0 otherwise.
	Halt int    `json:"halt"`
	Raw  string `json:"raw"`

	// Fields holds every field of the report by name.
	Fields map[string][]string `json:"-"`
}

// ParseStatus parses a status report.
func ParseStatus(s string) (Status, error) {
	raw := strings.TrimSpace(s)
	start := strings.IndexByte(raw, '<')
	end := strings.LastIndexByte(raw, '>')
	if start < 0 || end < start {
		return Status{}, fmt.Errorf("malformed status report %q", s)
	}

	parts := strings.Split(raw[start+1:end], "|")
	st := Status{State: State(parts[0]), Raw: raw, Fields: map[string][]string{}, Tool: -1, TargetTool: -1}
	for _, p := range parts[1:] {
		name, value, ok := strings.Cut(p, ":")
		if !ok {
			continue
		}
		vals := strings.Split(value, ",")
		for i := range vals {
			vals[i] = strings.TrimSpace(vals[i])
		}
		st.Fields[name] = vals
	}

	var err error
	f := st.Fields
	st.MPos, err = floats3(f["MPos"])
	if err != nil {
		return Status{}, fmt.Errorf("MPos: %w", err)
	}
	st.WPos, err = floats3(f["WPos"])
	if err != nil {
		return Status{}, fmt.Errorf("WPos: %w", err)
	}

	st.Feed = floatAt(f["F"], 0)
	if t := f["T"]; len(t) > 0 {
		st.Tool = intAt(t, 0, -1)
		st.ToolOffset = floatAt(t, 1)
		st.TargetTool = intAt(t, 2, -1)
	}
	if l := f["L"]; len(l) >= 5 {
		st.LaserMode = intAt(l, 0, 0) != 0
		st.LaserOn = intAt(l, 1, 0) != 0
		st.LaserTesting = intAt(l, 2, 0) != 0
		st.LaserPower = floatAt(l, 3)
		st.LaserScale = floatAt(l, 4)
	}
	if p := f["P"]; len(p) >= 3 {
		st.Playing = len(p) < 4 || intAt(p, 3, 0) != 0
		st.Progress = [3]int{intAt(p, 0, 0), intAt(p, 1, 0), intAt(p, 2, 0)}
	}
	st.Model = intAt(f["C"], 0, 0)
	st.Halt = intAt(f["H"], 0, 0)
	return st, nil
}

func floats3(vals []string) ([3]float64, error) {
	var out [3]float64
	if len(vals) < 3 {
		return out, fmt.Errorf("want 3 values, got %d", len(vals))
	}
	for i := range 3 {
		v, err := strconv.ParseFloat(vals[i], 64)
		if err != nil {
			return out, err
		}
		out[i] = v
	}
	return out, nil
}

func floatAt(vals []string, i int) float64 {
	if i >= len(vals) {
		return 0
	}
	v, _ := strconv.ParseFloat(vals[i], 64)
	return v
}

func intAt(vals []string, i, def int) int {
	if i >= len(vals) {
		return def
	}
	v, err := strconv.Atoi(vals[i])
	if err != nil {
		return def
	}
	return v
}

// haltReasons are the firmware's halt reasons (Kernel.h).
var haltReasons = map[int]string{
	1:  "manual halt",
	2:  "homing failed",
	3:  "probe failed",
	4:  "tool calibration failed",
	5:  "ATC homing failed",
	6:  "invalid tool",
	7:  "no tool",
	8:  "tool already loaded",
	9:  "spindle overheated",
	10: "soft limit",
	11: "cover open",
	12: "invalid probe",
	13: "emergency stop",
	14: "power supply overheated",
	15: "not homed",
	16: "crash detected",
	21: "hard limit",
	22: "motor error X",
	23: "motor error Y",
	24: "motor error Z",
	25: "spindle stall",
	26: "SD card error",
}

// HaltReason describes a halt code.
func HaltReason(code int) string {
	if r, ok := haltReasons[code]; ok {
		return r
	}
	return "halt reason " + strconv.Itoa(code)
}

// NeedsReset reports whether a halt code requires a reset rather than an
// unlock to clear.
func NeedsReset(code int) bool { return code >= 21 }
