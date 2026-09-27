package machine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"
)

// Timing of command replies. The Carvera Air never sends "ok", so replies
// are collected until the machine has been quiet for a while.
const (
	replyTimeout  = time.Second
	replyQuiet    = 200 * time.Millisecond
	statusTimeout = 2 * time.Second
	listTimeout   = 5 * time.Second
	pollInterval  = 250 * time.Millisecond
)

// ErrClosed is returned when the connection has been closed.
var ErrClosed = errors.New("connection closed")

// Conn is a connection to a machine.
type Conn struct {
	nc     net.Conn
	logger *slog.Logger

	writeMu  sync.Mutex
	cmdMu    sync.Mutex
	statusMu sync.Mutex

	lines  chan string
	status chan string
	load   chan Frame
	file   chan Frame

	done    chan struct{}
	readErr error
}

// Dial connects to the machine at addr (host:port).
func Dial(ctx context.Context, addr string, logger *slog.Logger) (*Conn, error) {
	if logger == nil {
		logger = slog.Default()
	}

	var d net.Dialer
	nc, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	return newConn(nc, logger), nil
}

func newConn(nc net.Conn, logger *slog.Logger) *Conn {
	c := &Conn{
		nc:     nc,
		logger: logger,
		lines:  make(chan string, 256),
		status: make(chan string, 8),
		load:   make(chan Frame, 64),
		file:   make(chan Frame, 16),
		done:   make(chan struct{}),
	}
	go c.readLoop()
	return c
}

// Close closes the connection.
func (c *Conn) Close() error {
	return c.nc.Close()
}

// Done is closed when the connection is lost.
func (c *Conn) Done() <-chan struct{} { return c.done }

// Send sends a command line without waiting for any reply.
func (c *Conn) Send(line string) error {
	return c.writeFrame(Frame{Type: TypeCtrlMulti, Payload: []byte(escape(line))})
}

// Command sends a command line and returns the reply lines, excluding
// any "ok".
func (c *Conn) Command(ctx context.Context, line string) ([]string, error) {
	c.cmdMu.Lock()
	defer c.cmdMu.Unlock()

	drain(c.lines)
	if err := c.Send(line); err != nil {
		return nil, err
	}

	replies := []string{}
	timer := time.NewTimer(replyTimeout)
	defer timer.Stop()
	for {
		select {
		case l := <-c.lines:
			if l == "ok" {
				return replies, nil
			}
			replies = append(replies, l)
			timer.Reset(replyQuiet)
		case <-timer.C:
			return replies, nil
		case <-c.done:
			return replies, c.err()
		case <-ctx.Done():
			return replies, ctx.Err()
		}
	}
}

// Realtime sends a single realtime control character.
func (c *Conn) Realtime(ch byte) error {
	return c.writeFrame(Frame{Type: TypeCtrlSingle, Payload: []byte{ch}})
}

// Status requests and returns a status report.
func (c *Conn) Status(ctx context.Context) (Status, error) {
	c.statusMu.Lock()
	defer c.statusMu.Unlock()

	drain(c.status)
	if err := c.Realtime('?'); err != nil {
		return Status{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, statusTimeout)
	defer cancel()
	select {
	case s := <-c.status:
		return ParseStatus(s)
	case <-c.done:
		return Status{}, c.err()
	case <-ctx.Done():
		return Status{}, fmt.Errorf("status: %w", ctx.Err())
	}
}

// WaitIdle waits for the machine to finish what it is doing. It waits at
// least settle before the first poll, so that a just-sent command has time
// to start, then until two consecutive polls report an idle machine.
func (c *Conn) WaitIdle(ctx context.Context, settle time.Duration) (Status, error) {
	select {
	case <-time.After(settle):
	case <-ctx.Done():
		return Status{}, ctx.Err()
	}

	idle := 0
	for {
		st, err := c.Status(ctx)
		if err != nil {
			return Status{}, err
		}
		if st.State == StateAlarm {
			return st, fmt.Errorf("machine in alarm state")
		}

		idle++
		if st.State != StateIdle || st.Playing {
			idle = 0
		}
		if idle >= 2 {
			return st, nil
		}

		select {
		case <-time.After(pollInterval):
		case <-ctx.Done():
			return Status{}, ctx.Err()
		}
	}
}

// FileEntry is a file or directory on the machine's SD card.
type FileEntry struct {
	Name  string `json:"name"`
	Dir   bool   `json:"dir"`
	Size  int64  `json:"size"`
	MTime string `json:"mtime"`
}

// List lists a directory on the machine, e.g. "/sd/gcodes".
func (c *Conn) List(ctx context.Context, dir string) ([]FileEntry, error) {
	c.cmdMu.Lock()
	defer c.cmdMu.Unlock()

	drain(c.load)
	if err := c.Send("ls -e -s " + escapePath(dir)); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, listTimeout)
	defer cancel()

	var buf bytes.Buffer
	for {
		select {
		case f := <-c.load:
			switch f.Type {
			case TypeLoadInfo:
				buf.Write(f.Payload)
			case TypeLoadFinish:
				return parseListing(buf.String()), nil
			case TypeLoadError:
				return nil, fmt.Errorf("list %s: %s", dir, strings.TrimSpace(string(f.Payload)))
			}
		case <-c.done:
			return nil, c.err()
		case <-ctx.Done():
			return nil, fmt.Errorf("list %s: %w", dir, ctx.Err())
		}
	}
}

// Play starts playing a file on the machine's SD card and returns any
// reply lines.
func (c *Conn) Play(ctx context.Context, path string) ([]string, error) {
	return c.Command(ctx, "play "+escapePath(path))
}

// Abort aborts the running job.
func (c *Conn) Abort() error {
	return c.Send("abort")
}

func (c *Conn) writeFrame(f Frame) error {
	b, err := f.MarshalBinary()
	if err != nil {
		return err
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = c.nc.Write(b)
	return err
}

func (c *Conn) readLoop() {
	defer close(c.done)

	fr := NewFrameReader(c.nc)
	var partial []byte
	for {
		f, err := fr.Next()
		if err != nil {
			c.readErr = err
			return
		}

		switch f.Type {
		case TypeNormalInfo:
			partial = append(partial, f.Payload...)
			for {
				i := bytes.IndexByte(partial, '\n')
				if i < 0 {
					break
				}
				line := strings.TrimRight(string(partial[:i]), "\r")
				partial = partial[i+1:]
				c.logger.Debug("machine", "line", line)
				offer(c.lines, line)
			}
		case TypeStatus:
			offer(c.status, string(f.Payload))
		case TypeLoadInfo, TypeLoadFinish, TypeLoadError:
			offer(c.load, f)
		case TypeFileStart, TypeFileMD5, TypeFileView, TypeFileData, TypeFileEnd, TypeFileCancel, TypeFileRetry:
			offer(c.file, f)
		default:
			c.logger.Debug("ignoring frame", "type", fmt.Sprintf("%#x", f.Type), "len", len(f.Payload))
		}
	}
}

func (c *Conn) err() error {
	if c.readErr != nil {
		return fmt.Errorf("%w: %w", ErrClosed, c.readErr)
	}
	return ErrClosed
}

// offer sends v on ch, dropping the oldest value if ch is full.
func offer[T any](ch chan T, v T) {
	for {
		select {
		case ch <- v:
			return
		default:
		}
		select {
		case <-ch:
		default:
		}
	}
}

func drain[T any](ch chan T) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// escape replaces characters that the firmware treats as realtime
// commands, as the Makera controller does.
func escape(s string) string {
	return strings.NewReplacer("?", "\x02", "&", "\x03", "!", "\x04", "~", "\x05").Replace(s)
}

// escapePath encodes spaces in a path, which the firmware expects as 0x01.
func escapePath(p string) string {
	return strings.ReplaceAll(p, " ", "\x01")
}

// parseListing parses "ls -e -s" output: one "name size mtime" per line,
// directories with a trailing slash.
func parseListing(s string) []FileEntry {
	entries := []FileEntry{}
	for _, line := range strings.Split(s, "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) < 3 {
			continue
		}

		var size int64
		fmt.Sscan(f[1], &size)
		name := strings.ReplaceAll(f[0], "\x01", " ")
		entries = append(entries, FileEntry{
			Name:  strings.TrimSuffix(name, "/"),
			Dir:   strings.HasSuffix(name, "/"),
			Size:  size,
			MTime: f[2],
		})
	}
	return entries
}
