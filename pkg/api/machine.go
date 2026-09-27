package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/borud/laserlabel/pkg/machine"
)

const (
	statusInterval = time.Second
	logLines       = 200
	jobDir         = "/sd/gcodes"
)

// Errors reported to clients.
var (
	ErrNotConnected = errors.New("not connected to a machine")
	ErrBusy         = errors.New("machine is busy")
)

// Job phases.
const (
	PhaseUploading = "uploading"
	PhasePlaying   = "playing"
	PhaseDone      = "done"
	PhaseFailed    = "failed"
)

// Job describes the most recent job started from the server.
type Job struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Phase   string    `json:"phase"`
	Sent    int       `json:"sent"`
	Total   int       `json:"total"`
	Error   string    `json:"error,omitempty"`
	Started time.Time `json:"started"`

	// running is set once the machine has been seen executing the job, so
	// that an idle report right after play doesn't end the job.
	running bool
}

// MachineState is a snapshot of the machine connection for clients.
type MachineState struct {
	Connected bool            `json:"connected"`
	Addr      string          `json:"addr"`
	Error     string          `json:"error,omitempty"`
	Status    *machine.Status `json:"status"`

	// Homed is set once the machine has been seen homing since connecting.
	// The firmware doesn't report whether it is homed.
	Homed      bool   `json:"homed"`
	Halt       string `json:"halt,omitempty"`
	NeedsReset bool   `json:"needsReset"`

	// OriginSet is set once the work origin has been set, nudged or
	// explicitly accepted since connecting. Probing is refused until then,
	// so a stale origin from an earlier job isn't used by accident.
	OriginSet bool            `json:"originSet"`
	Limits    *machine.Limits `json:"limits"`

	// Offsets is the G54 origin in machine coordinates, refreshed after
	// origin changes and whenever the machine becomes idle.
	Offsets *machine.Offsets `json:"offsets"`

	// Traced and ZProbed record that the outline was traced and Z probed
	// (and completed) for the current origin.
	Traced  bool `json:"traced"`
	ZProbed bool `json:"zProbed"`

	// LabelsDone counts completed label jobs since connecting or the last
	// counter reset.
	LabelsDone int `json:"labelsDone"`

	Job       *Job            `json:"job"`
	Log       []string        `json:"log"`
}

// machineManager owns the single connection to the machine, polls its
// status and runs jobs.
type machineManager struct {
	logger   *slog.Logger
	interval time.Duration

	mu     sync.Mutex
	conn   *machine.Conn
	addr   string
	status *machine.Status
	err    string
	job    *Job
	log    []string
	cancel context.CancelFunc
	homed  bool
	homing bool

	originSet bool
	limits    *machine.Limits
	offsets   *machine.Offsets

	traced, zProbed             bool
	pendingTrace, pendingZProbe bool
	wasBusy                     bool
	labelsDone                  int
}

func newMachineManager(logger *slog.Logger) *machineManager {
	return &machineManager{logger: logger, interval: statusInterval, log: []string{}}
}

// Connect connects to addr, or discovers a machine (optionally by name)
// when addr is empty. Any existing connection is closed first.
func (m *machineManager) Connect(ctx context.Context, addr, name string) error {
	m.Disconnect()

	if addr == "" {
		a, err := discoverOne(ctx, name)
		if err != nil {
			return err
		}
		addr = a.Addr()
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "2222")
	}

	c, err := machine.Dial(ctx, addr, m.logger)
	if err != nil {
		m.setError(err)
		return err
	}

	pollCtx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.conn, m.addr, m.err, m.status, m.cancel = c, addr, "", nil, cancel
	m.homed, m.homing, m.originSet, m.limits = false, false, false, nil
	m.offsets, m.traced, m.zProbed, m.pendingTrace, m.pendingZProbe = nil, false, false, false, false
	m.wasBusy, m.labelsDone = false, 0
	m.mu.Unlock()

	m.addLog("connected to " + addr)
	go m.poll(pollCtx, c)
	go m.refreshOffsets(pollCtx)
	return nil
}

// Disconnect closes the connection, if any.
func (m *machineManager) Disconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn == nil {
		return
	}
	m.cancel()
	m.conn.Close()
	m.conn, m.status = nil, nil
}

// State returns a snapshot for clients.
func (m *machineManager) State() MachineState {
	m.mu.Lock()
	defer m.mu.Unlock()

	st := MachineState{
		Connected: m.conn != nil,
		Addr:      m.addr,
		Error:     m.err,
		Status:    m.status,
		Log:       append([]string{}, m.log...),
		Homed:     m.homed,
		OriginSet:  m.originSet,
		Limits:     m.limits,
		Offsets:    m.offsets,
		Traced:     m.traced,
		ZProbed:    m.zProbed,
		LabelsDone: m.labelsDone,
	}
	if m.status != nil && m.status.Halt != 0 {
		st.Halt = machine.HaltReason(m.status.Halt)
		st.NeedsReset = machine.NeedsReset(m.status.Halt)
	}
	if m.job != nil {
		j := *m.job
		st.Job = &j
	}
	return st
}

// Command sends a line and returns the reply.
func (m *machineManager) Command(ctx context.Context, line string) ([]string, error) {
	c, err := m.connection()
	if err != nil {
		return nil, err
	}

	m.addLog("> " + line)
	lines, err := c.Command(ctx, line)
	for _, l := range lines {
		m.addLog(l)
	}
	return lines, err
}

// Jog moves one axis by mm relative to the current position. It is
// refused while the machine is not idle, so held buttons don't queue up
// moves.
func (m *machineManager) Jog(axis string, mm float64) error {
	c, err := m.connection()
	if err != nil {
		return err
	}
	if !m.idle() {
		return ErrBusy
	}

	switch axis {
	case "X", "Y", "Z":
	default:
		return fmt.Errorf("%w: invalid axis %q", errBadRequest, axis)
	}

	if err := m.send(fmt.Sprintf("G91 G0 %s%.3f", axis, mm)); err != nil {
		return err
	}
	return c.Send("G90")
}

// Realtime sends a realtime control character (hold, resume).
func (m *machineManager) Realtime(ch byte) error {
	c, err := m.connection()
	if err != nil {
		return err
	}
	return c.Realtime(ch)
}

// Abort aborts the running job.
func (m *machineManager) Abort() error {
	c, err := m.connection()
	if err != nil {
		return err
	}
	m.addLog("> abort")
	return c.Abort()
}

// StartJob uploads the program and plays it in the background.
func (m *machineManager) StartJob(name, program string) error {
	c, err := m.connection()
	if err != nil {
		return err
	}

	m.mu.Lock()
	busy := m.job != nil && (m.job.Phase == PhaseUploading || m.job.Phase == PhasePlaying)
	if busy || (m.status != nil && (m.status.Playing || m.status.State != machine.StateIdle)) {
		m.mu.Unlock()
		return ErrBusy
	}
	job := &Job{
		Name:    name,
		Path:    jobDir + "/laserlabel-" + name + ".nc",
		Phase:   PhaseUploading,
		Total:   len(program),
		Started: time.Now(),
	}
	m.job = job
	m.mu.Unlock()

	go m.runJob(c, job.Path, []byte(program))
	return nil
}

func (m *machineManager) runJob(c *machine.Conn, path string, data []byte) {
	ctx := context.Background()
	m.addLog("uploading " + path)

	err := c.Upload(ctx, path, data, func(n int) {
		m.updateJob(func(j *Job) { j.Sent = n })
	})
	if err != nil {
		m.failJob(err)
		return
	}

	// Mark the job as playing before sending play: the machine may start
	// while we are still collecting the reply.
	m.updateJob(func(j *Job) { j.Phase = PhasePlaying })
	m.addLog("> play " + path)
	lines, err := c.Play(ctx, path)
	for _, l := range lines {
		m.addLog(l)
	}
	if err != nil {
		m.failJob(err)
	}
}

func (m *machineManager) poll(ctx context.Context, c *machine.Conn) {
	t := time.NewTicker(m.interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.Done():
			m.lost(c)
			return
		case <-t.C:
		}

		st, err := c.Status(ctx)
		if err != nil {
			m.logger.Debug("status poll failed", "err", err)
			continue
		}

		m.mu.Lock()
		m.status = &st
		switch {
		case st.State == machine.StateHome:
			m.homing = true
		case m.homing && st.State == machine.StateIdle:
			m.homing, m.homed = false, true
		case st.State == machine.StateAlarm:
			m.homing = false
		}
		busy := st.Playing || st.State != machine.StateIdle
		if j := m.job; j != nil && j.Phase == PhasePlaying {
			if busy {
				j.running = true
			}
			if j.running && !busy {
				j.Phase = PhaseDone
				if j.Name == "label" {
					m.labelsDone++
				}
			}
		}
		finished := m.wasBusy && !busy
		m.wasBusy = busy
		switch {
		case st.State == machine.StateAlarm:
			m.pendingTrace, m.pendingZProbe = false, false
		case finished:
			m.traced = m.traced || m.pendingTrace
			m.zProbed = m.zProbed || m.pendingZProbe
			m.pendingTrace, m.pendingZProbe = false, false
		}
		m.mu.Unlock()

		if finished {
			go m.refreshOffsets(ctx)
		}
	}
}

// refreshOffsets reads the G54 offsets into the state.
func (m *machineManager) refreshOffsets(ctx context.Context) {
	o, err := m.Offsets(ctx)
	if err != nil {
		m.logger.Debug("reading offsets failed", "err", err)
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.offsets = &o
}

// lost clears the connection c after it dropped, unless it has already
// been replaced.
func (m *machineManager) lost(c *machine.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn != c {
		return
	}
	m.cancel()
	m.conn, m.status, m.err = nil, nil, "connection lost"
}

func (m *machineManager) connection() (*machine.Conn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn == nil {
		return nil, ErrNotConnected
	}
	return m.conn, nil
}

func (m *machineManager) idle() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status != nil && m.status.State == machine.StateIdle && !m.status.Playing
}

func (m *machineManager) updateJob(f func(*Job)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.job != nil {
		f(m.job)
	}
}

func (m *machineManager) failJob(err error) {
	m.addLog("job failed: " + err.Error())
	m.updateJob(func(j *Job) {
		j.Phase = PhaseFailed
		j.Error = err.Error()
	})
}

func (m *machineManager) setError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err.Error()
}

func (m *machineManager) addLog(line string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.log = append(m.log, time.Now().Format("15:04:05 ")+line)
	if len(m.log) > logLines {
		m.log = m.log[len(m.log)-logLines:]
	}
}

func discoverOne(ctx context.Context, name string) (machine.Announcement, error) {
	found, err := machine.Discover(ctx, 3*time.Second)
	if err != nil {
		return machine.Announcement{}, err
	}

	var matches []machine.Announcement
	for _, a := range found {
		if name == "" || strings.EqualFold(a.Name, name) {
			matches = append(matches, a)
		}
	}
	switch len(matches) {
	case 0:
		return machine.Announcement{}, errors.New("no machine found")
	case 1:
		return matches[0], nil
	}

	names := make([]string, 0, len(matches))
	for _, a := range matches {
		names = append(names, a.Name)
	}
	return machine.Announcement{}, fmt.Errorf("several machines found, pick one: %s", strings.Join(names, ", "))
}
