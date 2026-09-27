package api

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/borud/laserlabel/pkg/fonts"
	"github.com/borud/laserlabel/pkg/machine"
	"golang.org/x/image/font/gofont/goregular"
)

// fakeMachine answers status requests, a few commands and uploads.
type fakeMachine struct {
	ln net.Listener

	mu        sync.Mutex
	state     string
	laserMode bool
	yMin      string
	commands  []string
	realtime  []byte
	uploaded  []byte
}

func newFakeMachine(t *testing.T) *fakeMachine {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	m := &fakeMachine{ln: ln, state: "Idle", laserMode: true, yMin: "-212.0"}
	t.Cleanup(func() { ln.Close() })
	go m.serve()
	return m
}

func (m *fakeMachine) serve() {
	nc, err := m.ln.Accept()
	if err != nil {
		return
	}
	defer nc.Close()

	var wmu sync.Mutex
	send := func(f machine.Frame) {
		b, _ := f.MarshalBinary()
		wmu.Lock()
		defer wmu.Unlock()
		nc.Write(b)
	}

	var upload bytes.Buffer
	var packets, next int
	fr := machine.NewFrameReader(nc)
	for {
		f, err := fr.Next()
		if err != nil {
			return
		}

		switch f.Type {
		case machine.TypeCtrlSingle:
			m.mu.Lock()
			m.realtime = append(m.realtime, f.Payload[0])
			st, laser := m.state, 0
			if m.laserMode {
				laser = 1
			}
			m.mu.Unlock()
			if f.Payload[0] != '?' {
				continue
			}
			send(machine.Frame{Type: machine.TypeStatus, Payload: fmt.Appendf(nil, "<%s|MPos:0,0,0|WPos:1,2,3|T:8888,0,-1|L:%d,0,0,0,100>\n", st, laser)})
		case machine.TypeCtrlMulti:
			cmd := string(f.Payload)
			m.mu.Lock()
			m.commands = append(m.commands, cmd)
			switch cmd {
			case "M322":
				m.laserMode = false
			case "M321":
				m.laserMode = true
			}
			m.mu.Unlock()
			switch cmd {
			case "version":
				send(machine.Frame{Type: machine.TypeNormalInfo, Payload: []byte("version = 1.0.5\n")})
			case "config-get sd soft_endstop.x_min":
				send(machine.Frame{Type: machine.TypeNormalInfo, Payload: []byte("sd: soft_endstop.x_min is set to -302.0\n")})
			case "config-get sd soft_endstop.y_min":
				m.mu.Lock()
				y := m.yMin
				m.mu.Unlock()
				send(machine.Frame{Type: machine.TypeNormalInfo, Payload: []byte("sd: soft_endstop.y_min is set to " + y + "\n")})
			case "$#":
				send(machine.Frame{Type: machine.TypeNormalInfo, Payload: []byte("[G54:-148.0550,-112.0550,-84.6359,40.0000,0.0000]\n[G55:0,0,0,0,0]\n")})
			}
		case machine.TypeFileMD5:
			upload.Reset()
			send(machine.Frame{Type: machine.TypeFileView})
		case machine.TypeFileView:
			packets = int(binary.BigEndian.Uint32(f.Payload))
			next = 1
			send(machine.Frame{Type: machine.TypeFileData, Payload: binary.BigEndian.AppendUint32(nil, 1)})
		case machine.TypeFileData:
			upload.Write(f.Payload[4:])
			next++
			if next <= packets {
				send(machine.Frame{Type: machine.TypeFileData, Payload: binary.BigEndian.AppendUint32(nil, uint32(next))})
				continue
			}
			m.mu.Lock()
			m.uploaded = bytes.Clone(upload.Bytes())
			m.mu.Unlock()
			send(machine.Frame{Type: machine.TypeFileEnd})
		}
	}
}

func (m *fakeMachine) setState(s string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = s
}

func (m *fakeMachine) sent() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.commands...)
}

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	cat := fonts.NewCatalog(nil, nil)
	if _, err := cat.AddData("go", goregular.TTF); err != nil {
		t.Fatal(err)
	}

	s := New(Config{Catalog: cat})
	s.machine.interval = 20 * time.Millisecond
	ts := httptest.NewServer(s)
	t.Cleanup(func() {
		ts.Close()
		s.Close()
	})
	return ts
}

func call(t *testing.T, ts *httptest.Server, method, path string, body any, out any) int {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	var req *http.Request
	var err error
	if rd != nil {
		req, err = http.NewRequest(method, ts.URL+path, rd)
	} else {
		req, err = http.NewRequest(method, ts.URL+path, nil)
	}
	if err != nil {
		t.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: decode: %v", method, path, err)
		}
	}
	return resp.StatusCode
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

var labelBody = map[string]any{
	"label": map[string]any{
		"lines": []map[string]any{{"text": "Jam", "fontID": "go", "sizeMM": 8, "align": "center"}},
	},
	"workpiece": map[string]any{"widthMM": 62, "heightMM": 62, "marginMM": 4},
	"settings":  map[string]any{"power": 0.4, "feed": 1000, "passes": 1, "mode": "fill", "hatchIntervalMM": 0.2, "bidirectional": true},
}

var gridBody = map[string]any{
	"powers": []float64{0.2, 0.4}, "feeds": []float64{500, 1000},
	"cellMM": 5, "gapMM": 2, "hatchIntervalMM": 0.5,
}

func TestFontsAndPreview(t *testing.T) {
	ts := testServer(t)

	var fontList []map[string]any
	if code := call(t, ts, "GET", "/api/fonts", nil, &fontList); code != 200 || len(fontList) != 1 {
		t.Fatalf("fonts: %d %v", code, fontList)
	}

	var pv preview
	if code := call(t, ts, "POST", "/api/preview/label", labelBody, &pv); code != 200 {
		t.Fatalf("preview label: %d", code)
	}
	if len(pv.Burn) == 0 || pv.Workpiece == nil || pv.Printable == nil || !strings.Contains(pv.GCode, "M321") {
		t.Fatalf("preview: %d burn segments, workpiece %v", len(pv.Burn), pv.Workpiece)
	}
	if pv.EstimateSec <= 0 || len(pv.Warnings) != 0 {
		t.Fatalf("estimate %v warnings %v", pv.EstimateSec, pv.Warnings)
	}

	if code := call(t, ts, "POST", "/api/preview/testgrid", gridBody, &pv); code != 200 || len(pv.Burn) == 0 {
		t.Fatalf("preview grid: %d", code)
	}
}

func TestPreviewValidation(t *testing.T) {
	ts := testServer(t)

	bad := map[string]any{"powers": []float64{1.5}, "feeds": []float64{500}, "cellMM": 5, "hatchIntervalMM": 0.5}
	var e errorResponse
	if code := call(t, ts, "POST", "/api/preview/testgrid", bad, &e); code != 400 || !strings.Contains(e.Error, "power") {
		t.Fatalf("got %d %q", code, e.Error)
	}

	unknownFont := map[string]any{
		"label":     map[string]any{"lines": []map[string]any{{"text": "x", "fontID": "nope", "sizeMM": 5}}},
		"workpiece": map[string]any{"widthMM": 62, "heightMM": 62},
		"settings":  map[string]any{"power": 0.4, "feed": 1000, "hatchIntervalMM": 0.1},
	}
	if code := call(t, ts, "POST", "/api/preview/label", unknownFont, &e); code != 400 {
		t.Fatalf("unknown font: %d %q", code, e.Error)
	}
}

func TestMachineNotConnected(t *testing.T) {
	ts := testServer(t)
	var e errorResponse
	if code := call(t, ts, "POST", "/api/machine/cmd", map[string]string{"line": "version"}, &e); code != 409 {
		t.Fatalf("got %d %q", code, e.Error)
	}
	if code := call(t, ts, "POST", "/api/job/testgrid", gridBody, &e); code != 409 {
		t.Fatalf("got %d %q", code, e.Error)
	}
}

func TestMachineFlow(t *testing.T) {
	fm := newFakeMachine(t)
	ts := testServer(t)

	var st MachineState
	if code := call(t, ts, "POST", "/api/machine/connect", map[string]string{"host": fm.ln.Addr().String()}, &st); code != 200 || !st.Connected {
		t.Fatalf("connect: %d %+v", code, st)
	}
	waitFor(t, "status", func() bool {
		call(t, ts, "GET", "/api/machine/state", nil, &st)
		return st.Status != nil && st.Status.State == machine.StateIdle
	})

	var reply map[string][]string
	if code := call(t, ts, "POST", "/api/machine/cmd", map[string]string{"line": "version"}, &reply); code != 200 || reply["lines"][0] != "version = 1.0.5" {
		t.Fatalf("cmd: %d %v", code, reply)
	}

	if code := call(t, ts, "POST", "/api/machine/jog", map[string]any{"axis": "x", "mm": 1}, &st); code != 200 {
		t.Fatalf("jog: %d", code)
	}
	waitFor(t, "jog commands", func() bool {
		cmds := fm.sent()
		i := slices.Index(cmds, "G91 G0 X1.000")
		return i >= 0 && i+1 < len(cmds) && cmds[i+1] == "G90"
	})

	var e errorResponse
	if code := call(t, ts, "POST", "/api/machine/jog", map[string]any{"axis": "Q", "mm": 1}, &e); code != 400 {
		t.Fatalf("bad axis: %d", code)
	}

	// Jobs upload the program and play it.
	if code := call(t, ts, "POST", "/api/job/testgrid", gridBody, &st); code != 202 {
		t.Fatalf("job: %d", code)
	}
	waitFor(t, "play", func() bool {
		cmds := fm.sent()
		return len(cmds) > 0 && cmds[len(cmds)-1] == "play /sd/gcodes/laserlabel-testgrid.nc"
	})
	fm.mu.Lock()
	uploaded := string(fm.uploaded)
	fm.mu.Unlock()
	if !strings.HasPrefix(uploaded, "; laserlabel test grid") || !strings.Contains(uploaded, "M322") {
		t.Fatalf("uploaded program: %.60q", uploaded)
	}

	// While the machine runs, jogging and new jobs are refused.
	fm.setState("Run")
	waitFor(t, "run state", func() bool {
		call(t, ts, "GET", "/api/machine/state", nil, &st)
		return st.Status.State == machine.StateRun
	})
	if code := call(t, ts, "POST", "/api/machine/jog", map[string]any{"axis": "Y", "mm": 1}, &e); code != 409 {
		t.Fatalf("jog while running: %d", code)
	}
	if code := call(t, ts, "POST", "/api/job/label", labelBody, &e); code != 409 {
		t.Fatalf("job while running: %d", code)
	}

	fm.setState("Idle")
	waitFor(t, "job done", func() bool {
		call(t, ts, "GET", "/api/machine/state", nil, &st)
		return st.Job != nil && st.Job.Phase == PhaseDone
	})

	if code := call(t, ts, "POST", "/api/machine/abort", nil, &st); code != 200 {
		t.Fatalf("abort: %d", code)
	}
	if code := call(t, ts, "POST", "/api/machine/disconnect", nil, &st); code != 200 || st.Connected {
		t.Fatalf("disconnect: %d %+v", code, st)
	}
}

func connectFake(t *testing.T) (*fakeMachine, *httptest.Server) {
	t.Helper()
	fm := newFakeMachine(t)
	ts := testServer(t)

	var st MachineState
	if code := call(t, ts, "POST", "/api/machine/connect", map[string]string{"host": fm.ln.Addr().String()}, &st); code != 200 {
		t.Fatalf("connect: %d", code)
	}
	waitFor(t, "status", func() bool {
		call(t, ts, "GET", "/api/machine/state", nil, &st)
		return st.Status != nil
	})
	return fm, ts
}

func TestHomeUnlockReset(t *testing.T) {
	fm, ts := connectFake(t)

	var st MachineState
	for _, p := range []string{"/api/machine/home", "/api/machine/unlock", "/api/machine/reset"} {
		if code := call(t, ts, "POST", p, nil, &st); code != 200 {
			t.Fatalf("%s: %d", p, code)
		}
	}
	waitFor(t, "commands", func() bool {
		cmds := fm.sent()
		return slices.Contains(cmds, "$H") && slices.Contains(cmds, "$X")
	})
	waitFor(t, "soft reset", func() bool {
		fm.mu.Lock()
		defer fm.mu.Unlock()
		return slices.Contains(fm.realtime, 0x18)
	})

	if st.Homed {
		t.Fatal("homed before homing was seen")
	}
	fm.setState("Home")
	waitFor(t, "homing", func() bool {
		call(t, ts, "GET", "/api/machine/state", nil, &st)
		return st.Status.State == machine.StateHome
	})
	fm.setState("Idle")
	waitFor(t, "homed", func() bool {
		call(t, ts, "GET", "/api/machine/state", nil, &st)
		return st.Homed
	})
}

func TestOrigin(t *testing.T) {
	fm, ts := connectFake(t)

	var st MachineState
	if code := call(t, ts, "POST", "/api/machine/origin", map[string]any{"here": true}, &st); code != 200 {
		t.Fatalf("origin here: %d", code)
	}
	if st.Offsets == nil || st.Offsets.X != -148.055 || !st.OriginSet {
		t.Fatalf("state %+v", st)
	}
	if code := call(t, ts, "POST", "/api/machine/origin", map[string]any{"axes": "Y"}, &st); code != 200 {
		t.Fatalf("zero Y: %d", code)
	}
	if code := call(t, ts, "POST", "/api/machine/origin", map[string]any{"dx": 0.5, "dy": -1}, &st); code != 200 {
		t.Fatalf("nudge: %d", code)
	}

	cmds := fm.sent()
	for _, want := range []string{"G10 L20 P1 X0 Y0", "G10 L20 P1 Y0", "G10 L2 P1 X-147.555 Y-113.055"} {
		if !slices.Contains(cmds, want) {
			t.Fatalf("missing %q in %q", want, cmds)
		}
	}

	// Nudging with retrace sends a margin-only M495 afterwards.
	body := map[string]any{"dx": 0.1, "retrace": map[string]any{"widthMM": 62, "heightMM": 62, "marginMM": 4}}
	if code := call(t, ts, "POST", "/api/machine/origin", body, &st); code != 200 {
		t.Fatalf("nudge with retrace: %d", code)
	}
	cmds = fm.sent()
	if last := cmds[len(cmds)-1]; last != "M495 X-31.000 Y-31.000 C31.000 D31.000" {
		t.Fatalf("last command %q", last)
	}

	var e errorResponse
	if code := call(t, ts, "POST", "/api/machine/origin", map[string]any{"dx": 50}, &e); code != 400 {
		t.Fatalf("large nudge: %d", code)
	}
	if code := call(t, ts, "POST", "/api/machine/origin", map[string]any{"axes": "Z"}, &e); code != 400 {
		t.Fatalf("zero Z: %d", code)
	}
}

func TestTraceCompletion(t *testing.T) {
	fm, ts := connectFake(t)

	var st MachineState
	call(t, ts, "POST", "/api/machine/origin", map[string]any{"accept": true}, &st)
	body := map[string]any{"widthMM": 62, "heightMM": 62, "marginMM": 4, "outline": true}
	if code := call(t, ts, "POST", "/api/machine/probe", body, &st); code != 200 {
		t.Fatalf("probe: %d", code)
	}

	// The trace only counts once the machine has run and returned to idle.
	call(t, ts, "GET", "/api/machine/state", nil, &st)
	if st.Traced {
		t.Fatal("traced before running")
	}
	fm.setState("Run")
	waitFor(t, "run", func() bool {
		call(t, ts, "GET", "/api/machine/state", nil, &st)
		return st.Status.State == machine.StateRun
	})
	fm.setState("Idle")
	waitFor(t, "traced", func() bool {
		call(t, ts, "GET", "/api/machine/state", nil, &st)
		return st.Traced
	})
	if st.ZProbed {
		t.Fatal("Z marked probed after trace only")
	}

	// Moving the origin invalidates the trace.
	call(t, ts, "POST", "/api/machine/origin", map[string]any{"dx": 1}, &st)
	if st.Traced {
		t.Fatal("trace still valid after nudge")
	}
}

func TestProbeAndTool(t *testing.T) {
	fm, ts := connectFake(t)

	var st MachineState
	var e errorResponse
	body := map[string]any{"widthMM": 62, "heightMM": 50, "outline": true, "z": true}
	if code := call(t, ts, "POST", "/api/machine/probe", body, &e); code != 400 || !strings.Contains(e.Error, "origin") {
		t.Fatalf("probe before origin set: %d %q", code, e.Error)
	}

	if code := call(t, ts, "POST", "/api/machine/origin", map[string]any{"accept": true}, &st); code != 200 {
		t.Fatalf("accept origin: %d", code)
	}
	if code := call(t, ts, "POST", "/api/machine/probe", body, &st); code != 200 {
		t.Fatalf("probe: %d", code)
	}
	call(t, ts, "GET", "/api/machine/state", nil, &st)
	if !st.OriginSet || st.Limits == nil || st.Limits.YMin != -212 || st.Limits.Source != "machine" {
		t.Fatalf("state %+v", st)
	}

	cmds := fm.sent()
	m322 := slices.Index(cmds, "M322")
	m495 := slices.Index(cmds, "M495 X-31.000 Y-25.000 C31.000 D25.000 O31.000 F25.000")
	if m322 < 0 || m495 < m322 {
		t.Fatalf("commands %q", cmds)
	}

	if code := call(t, ts, "POST", "/api/machine/probe", map[string]any{"widthMM": 62, "heightMM": 62, "z": true}, &st); code != 200 {
		t.Fatalf("probe z: %d", code)
	}
	if !slices.Contains(fm.sent(), "M495 X-31.000 Y-31.000 O31.000 F31.000") {
		t.Fatalf("commands %q", fm.sent())
	}

	if code := call(t, ts, "POST", "/api/machine/probe", map[string]any{"widthMM": 62, "heightMM": 62}, &e); code != 400 {
		t.Fatalf("empty probe: %d", code)
	}

	for _, on := range []bool{true, false} {
		if code := call(t, ts, "POST", "/api/machine/pointer", map[string]bool{"on": on}, &st); code != 200 {
			t.Fatalf("pointer %v: %d", on, code)
		}
	}
	waitFor(t, "pointer commands", func() bool {
		cmds := fm.sent()
		return slices.Contains(cmds, "M831") && slices.Contains(cmds, "M832")
	})

	for _, tool := range []string{"probe", "laser"} {
		if code := call(t, ts, "POST", "/api/machine/tool", map[string]string{"tool": tool}, &st); code != 200 {
			t.Fatalf("tool %s: %d", tool, code)
		}
	}
	cmds = fm.sent()
	if !slices.Contains(cmds, "M6 T0") || cmds[len(cmds)-1] != "M321" {
		t.Fatalf("commands %q", cmds)
	}
}

func TestDiscoveryList(t *testing.T) {
	d := newDiscovery(nil)
	d.add(machine.Announcement{Name: "Z1", IP: "10.0.0.2", Port: 2222})
	d.add(machine.Announcement{Name: "AIR", IP: "10.0.0.1", Port: 2222})
	d.seen["OLD"] = SeenMachine{Announcement: machine.Announcement{Name: "OLD"}, LastSeen: time.Now().Add(-time.Hour)}

	got := d.list()
	if len(got) != 2 || got[0].Name != "AIR" || got[1].Addr() != "10.0.0.2:2222" {
		t.Fatalf("got %+v", got)
	}
	if _, ok := d.seen["OLD"]; ok {
		t.Fatal("expired machine not removed")
	}
}

func TestProbeInsideMargin(t *testing.T) {
	_, ts := connectFake(t)
	var st MachineState
	call(t, ts, "POST", "/api/machine/origin", map[string]any{"accept": true}, &st)

	var e errorResponse
	body := map[string]any{"widthMM": 62, "heightMM": 62, "marginMM": 31, "z": true}
	if code := call(t, ts, "POST", "/api/machine/probe", body, &e); code != 400 || !strings.Contains(e.Error, "margin") {
		t.Fatalf("got %d %q", code, e.Error)
	}
}

func TestProbeOutsideLimits(t *testing.T) {
	fm, ts := connectFake(t)
	fm.mu.Lock()
	fm.yMin = "-130.0"
	fm.mu.Unlock()

	var st MachineState
	call(t, ts, "POST", "/api/machine/origin", map[string]any{"accept": true}, &st)

	// G54 Y is -112.055, so a 62 mm workpiece reaches Y -143 < -130.
	var e errorResponse
	body := map[string]any{"widthMM": 62, "heightMM": 62, "outline": true}
	if code := call(t, ts, "POST", "/api/machine/probe", body, &e); code != 400 || !strings.Contains(e.Error, "travel limits") {
		t.Fatalf("got %d %q", code, e.Error)
	}
	for _, c := range fm.sent() {
		if strings.HasPrefix(c, "M495") {
			t.Fatal("M495 sent despite limit violation")
		}
	}
}

func TestGoTo(t *testing.T) {
	fm, ts := connectFake(t)

	var e errorResponse
	if code := call(t, ts, "POST", "/api/machine/goto", map[string]string{"target": "center"}, &e); code != 400 || !strings.Contains(e.Error, "home") {
		t.Fatalf("goto before homing: %d %q", code, e.Error)
	}

	var st MachineState
	fm.setState("Home")
	waitFor(t, "homing", func() bool {
		call(t, ts, "GET", "/api/machine/state", nil, &st)
		return st.Status.State == machine.StateHome
	})
	fm.setState("Idle")
	waitFor(t, "homed", func() bool {
		call(t, ts, "GET", "/api/machine/state", nil, &st)
		return st.Homed
	})

	want := map[string]string{
		"front-left": "G53 G0 X-292.000 Y-202.000",
		"back-right": "G53 G0 X-10.000 Y-10.000",
		"center":     "G53 G0 X-151.000 Y-106.000",
		"origin":     "G90 G0 X0 Y0",
	}
	for target, move := range want {
		if code := call(t, ts, "POST", "/api/machine/goto", map[string]string{"target": target}, &st); code != 200 {
			t.Fatalf("goto %s: %d", target, code)
		}
		waitFor(t, "goto "+target, func() bool {
			cmds := fm.sent()
			i := slices.Index(cmds, move)
			return i >= 1 && cmds[i-1] == "G53 G0 Z-2"
		})
	}

	if code := call(t, ts, "POST", "/api/machine/goto", map[string]string{"target": "moon"}, &e); code != 400 {
		t.Fatalf("bad target: %d", code)
	}
}
