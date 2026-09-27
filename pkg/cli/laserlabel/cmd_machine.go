package laserlabel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path"
	"strings"
	"time"

	"github.com/borud/laserlabel/pkg/machine"
)

type machineCmd struct {
	Discover machineDiscoverCmd `kong:"cmd,help='find machines on the network'"`
	Status   machineStatusCmd   `kong:"cmd,help='show machine status'"`
	Cmd      machineCmdCmd      `kong:"cmd,help='send a command line and print the reply'"`
	Ls       machineLsCmd       `kong:"cmd,help='list files on the SD card'"`
	Upload   machineUploadCmd   `kong:"cmd,help='upload a file to the SD card'"`
	Play     machinePlayCmd     `kong:"cmd,help='play a file on the SD card and follow progress'"`
}

// target selects the machine to talk to.
type target struct {
	Host string `kong:"short='H',env='LASERLABEL_HOST',help='machine address host[:port]; discovered if empty'"`
	Name string `kong:"env='LASERLABEL_MACHINE',help='machine name to pick when discovering, e.g. CARVERA_AIR_00000'"`
}

type machineDiscoverCmd struct {
	Wait time.Duration `kong:"default='5s',help='how long to listen'"`
}

type machineStatusCmd struct {
	target `kong:"embed"`
	JSON   bool `kong:"name='json',help='print as JSON'"`
}

type machineCmdCmd struct {
	target `kong:"embed"`
	Line   []string `kong:"arg,help='command line, e.g. version or $#'"`
}

type machineLsCmd struct {
	target `kong:"embed"`
	Dir    string `kong:"arg,optional,default='/sd/gcodes',help='directory'"`
}

type machineUploadCmd struct {
	target `kong:"embed"`
	File   string `kong:"arg,type='existingfile',help='local file'"`
	Dest   string `kong:"arg,optional,help='destination path (default /sd/gcodes/<file name>)'"`
}

type machinePlayCmd struct {
	target `kong:"embed"`
	Path   string `kong:"arg,help='file on the SD card, e.g. /sd/gcodes/lid.nc'"`
}

func (m *machineDiscoverCmd) Run(_ *Options) error {
	found, err := machine.Discover(context.Background(), m.Wait)
	if err != nil {
		return err
	}
	for _, a := range found {
		fmt.Printf("%-24s %-21s busy=%v %s\n", a.Name, a.Addr(), a.Busy, a.State)
	}
	return nil
}

func (m *machineStatusCmd) Run(_ *Options) error {
	ctx, cancel := signalContext()
	defer cancel()

	c, err := m.dial(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	st, err := c.Status(ctx)
	if err != nil {
		return err
	}
	if m.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(st)
	}

	fmt.Printf("state:  %s\n", st.State)
	fmt.Printf("work:   X%.3f Y%.3f Z%.3f\n", st.WPos[0], st.WPos[1], st.WPos[2])
	fmt.Printf("mach:   X%.3f Y%.3f Z%.3f\n", st.MPos[0], st.MPos[1], st.MPos[2])
	fmt.Printf("tool:   %d (offset %.3f)\n", st.Tool, st.ToolOffset)
	fmt.Printf("laser:  mode=%v on=%v power=%.2f\n", st.LaserMode, st.LaserOn, st.LaserPower)
	if st.Playing {
		fmt.Printf("play:   line %d, %d%%, %ds\n", st.Progress[0], st.Progress[1], st.Progress[2])
	}
	return nil
}

func (m *machineCmdCmd) Run(_ *Options) error {
	ctx, cancel := signalContext()
	defer cancel()

	c, err := m.dial(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	lines, err := c.Command(ctx, strings.Join(m.Line, " "))
	for _, l := range lines {
		fmt.Println(l)
	}
	return err
}

func (m *machineLsCmd) Run(_ *Options) error {
	ctx, cancel := signalContext()
	defer cancel()

	c, err := m.dial(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	entries, err := c.List(ctx, m.Dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		name := e.Name
		if e.Dir {
			name += "/"
		}
		fmt.Printf("%-40s %10d %s\n", name, e.Size, e.MTime)
	}
	return nil
}

func (m *machineUploadCmd) Run(_ *Options) error {
	ctx, cancel := signalContext()
	defer cancel()

	data, err := os.ReadFile(m.File)
	if err != nil {
		return err
	}
	dest := m.Dest
	if dest == "" {
		dest = path.Join("/sd/gcodes", path.Base(m.File))
	}

	c, err := m.dial(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	start := time.Now()
	err = c.Upload(ctx, dest, data, func(n int) {
		slog.Debug("upload progress", "sent", n, "total", len(data))
	})
	if err != nil {
		return err
	}
	slog.Info("uploaded", "dest", dest, "bytes", len(data), "elapsed", time.Since(start).Round(time.Millisecond))
	return nil
}

func (m *machinePlayCmd) Run(_ *Options) error {
	ctx, cancel := signalContext()
	defer cancel()

	c, err := m.dial(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	lines, err := c.Play(ctx, m.Path)
	if err != nil {
		return err
	}
	for _, l := range lines {
		slog.Info("machine", "reply", l)
	}

	for {
		select {
		case <-ctx.Done():
			slog.Warn("interrupted, aborting job")
			return c.Abort()
		case <-time.After(time.Second):
		}

		st, err := c.Status(ctx)
		if err != nil {
			return err
		}
		if !st.Playing && st.State == machine.StateIdle {
			slog.Info("finished")
			return nil
		}
		slog.Info("playing", "state", st.State, "line", st.Progress[0], "percent", st.Progress[1])
	}
}

// dial connects to the target machine, discovering it if no host is given.
func (t target) dial(ctx context.Context) (*machine.Conn, error) {
	addr := t.Host
	if addr == "" {
		a, err := t.discover(ctx)
		if err != nil {
			return nil, err
		}
		addr = a.Addr()
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, "2222")
	}
	return machine.Dial(ctx, addr, slog.Default())
}

func (t target) discover(ctx context.Context) (machine.Announcement, error) {
	found, err := machine.Discover(ctx, 3*time.Second)
	if err != nil {
		return machine.Announcement{}, err
	}

	var matches []machine.Announcement
	for _, a := range found {
		if t.Name == "" || strings.EqualFold(a.Name, t.Name) {
			matches = append(matches, a)
		}
	}
	switch len(matches) {
	case 0:
		return machine.Announcement{}, fmt.Errorf("no machine found (use --host or --name)")
	case 1:
		return matches[0], nil
	}

	names := make([]string, 0, len(matches))
	for _, a := range matches {
		names = append(names, a.Name)
	}
	return machine.Announcement{}, fmt.Errorf("several machines found, pick one with --name: %s", strings.Join(names, ", "))
}

func signalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}
