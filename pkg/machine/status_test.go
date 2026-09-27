package machine

import "testing"

// Status reports captured from real machines.
const (
	airStatus = "<Idle|MPos:-1.0000,-1.0000,-1.0000,0.0000,0.0000|WPos:147.0550,111.0550,99.1266,-40.0000,0.0000|F:0.0,3000.0,100.0|S:0.0,10000.0,100.0,0,14.9,15.2,0,0,0|T:1,-15.491,-1|W:0.00|L:0, 0, 0, 0.0,100.0|C:2,1,0,1>\n"
	z1Status  = "<Idle|MPos:-1.0000,-1.0000,-1.0000,0.0000,0.0000|WPos:185.2900,182.7300,67.8644,0.0000,0.0000|F:0.0,3000.0,100.0|S:0.0,10000.0,100.0,0,25.0,25.8,0,0,0,0|T:3,-4.088,-1|L:0, 0, 0, 0.0,100.0|C:4,1,0,1|E:0,0,0,21,7675|OTA:3,0>\n"
)

func TestParseStatusAir(t *testing.T) {
	st, err := ParseStatus(airStatus)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateIdle || st.Model != 2 {
		t.Fatalf("state %q model %d", st.State, st.Model)
	}
	if st.WPos != [3]float64{147.055, 111.055, 99.1266} || st.MPos[0] != -1 {
		t.Fatalf("positions %v %v", st.MPos, st.WPos)
	}
	if st.Tool != 1 || st.ToolOffset != -15.491 || st.TargetTool != -1 {
		t.Fatalf("tool %d %v %d", st.Tool, st.ToolOffset, st.TargetTool)
	}
	if st.LaserMode || st.LaserScale != 100 || st.Playing {
		t.Fatalf("laser/playing: %+v", st)
	}
}

func TestParseStatusZ1(t *testing.T) {
	st, err := ParseStatus(z1Status)
	if err != nil {
		t.Fatal(err)
	}
	if st.Model != 4 || st.Tool != 3 || st.Fields["OTA"][0] != "3" {
		t.Fatalf("got %+v", st)
	}
}

func TestParseStatusPlayingAndLaser(t *testing.T) {
	st, err := ParseStatus("<Run|MPos:1,2,3|WPos:4,5,6|T:8888,0,-1|L:1,1,0,0.4,100|P:120,35,42,1>")
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateRun || !st.LaserMode || !st.LaserOn || st.LaserPower != 0.4 {
		t.Fatalf("laser: %+v", st)
	}
	if !st.Playing || st.Progress != [3]int{120, 35, 42} || st.Tool != 8888 {
		t.Fatalf("progress: %+v", st)
	}
}

func TestParseStatusHalt(t *testing.T) {
	st, err := ParseStatus("<Alarm|MPos:0,0,0|WPos:0,0,0|H:21>")
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateAlarm || st.Halt != 21 || !NeedsReset(st.Halt) || HaltReason(st.Halt) != "hard limit" {
		t.Fatalf("got %+v", st)
	}
	if NeedsReset(1) || HaltReason(99) != "halt reason 99" {
		t.Fatal("halt helpers")
	}
}

func TestParseStatusErrors(t *testing.T) {
	for _, s := range []string{"", "Idle", "<Idle|WPos:1,2,3>", "<Idle|MPos:a,b,c|WPos:1,2,3>"} {
		if _, err := ParseStatus(s); err == nil {
			t.Errorf("ParseStatus(%q): expected error", s)
		}
	}
}

func TestParseAnnouncement(t *testing.T) {
	a, ok := ParseAnnouncement([]byte("CARVERA_AIR_00000,192.168.1.50,2222,0"))
	if !ok || a.Name != "CARVERA_AIR_00000" || a.Addr() != "192.168.1.50:2222" || a.Busy {
		t.Fatalf("air: %+v", a)
	}
	z, ok := ParseAnnouncement([]byte("Makera_Z1P_000000,192.168.1.51,2222,1,Idle"))
	if !ok || !z.Busy || z.State != "Idle" {
		t.Fatalf("z1: %+v", z)
	}
	if _, ok := ParseAnnouncement([]byte("junk")); ok {
		t.Fatal("parsed junk")
	}
}

func TestParseG54(t *testing.T) {
	// Captured from the Air.
	lines := []string{
		"[G54:-148.0550,-112.0550,-84.6359,40.0000,0.0000]",
		"[G55:0.0000,0.0000,0.0000,0.0000,0.0000]",
		"[TL0:-15.4906]",
	}
	o, err := parseG54(lines)
	if err != nil {
		t.Fatal(err)
	}
	if o != (Offsets{X: -148.055, Y: -112.055, Z: -84.6359}) {
		t.Fatalf("got %+v", o)
	}
	if _, err := parseG54([]string{"ok"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseConfigValue(t *testing.T) {
	v, err := parseConfigValue([]string{"cached: soft_endstop.y_min is set to -212.0"}, "soft_endstop.y_min")
	if err != nil || v != -212 {
		t.Fatalf("got %v %v", v, err)
	}
	// Captured from the Z1.
	v, err = parseConfigValue([]string{"sd: soft_endstop.y_min is set to -206.0"}, "soft_endstop.y_min")
	if err != nil || v != -206 {
		t.Fatalf("got %v %v", v, err)
	}
	if _, err := parseConfigValue([]string{"cached: soft_endstop.y_min is not in config"}, "soft_endstop.y_min"); err == nil {
		t.Fatal("expected error")
	}

	l := Limits{XMin: -302, YMin: -212}
	if !l.Contains(-200, -150, -100, -50) || l.Contains(-217, -214, -155, -152) || l.Contains(-10, -10, 1, -5) {
		t.Fatal("contains")
	}
}
