package machine

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeMachine is a minimal framed-protocol peer. handle is called for
// every frame received and may reply using send.
type fakeMachine struct {
	t      *testing.T
	ln     net.Listener
	handle func(f Frame, send func(Frame))

	mu       sync.Mutex
	received []Frame
}

func newFakeMachine(t *testing.T, handle func(f Frame, send func(Frame))) (*fakeMachine, *Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	m := &fakeMachine{t: t, ln: ln, handle: handle}
	go m.serve()

	c, err := Dial(context.Background(), ln.Addr().String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c.Close()
		ln.Close()
	})
	return m, c
}

func (m *fakeMachine) serve() {
	nc, err := m.ln.Accept()
	if err != nil {
		return
	}
	defer nc.Close()

	var wmu sync.Mutex
	send := func(f Frame) {
		b, _ := f.MarshalBinary()
		wmu.Lock()
		defer wmu.Unlock()
		nc.Write(b)
	}

	fr := NewFrameReader(nc)
	for {
		f, err := fr.Next()
		if err != nil {
			return
		}
		m.mu.Lock()
		m.received = append(m.received, f)
		m.mu.Unlock()
		m.handle(f, send)
	}
}

func (m *fakeMachine) commands() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []string{}
	for _, f := range m.received {
		if f.Type == TypeCtrlMulti {
			out = append(out, string(f.Payload))
		}
	}
	return out
}

func text(s string) Frame { return Frame{Type: TypeNormalInfo, Payload: []byte(s)} }

func TestCommandQuietAndOK(t *testing.T) {
	_, c := newFakeMachine(t, func(f Frame, send func(Frame)) {
		switch string(f.Payload) {
		case "version":
			// Fragmented line, no "ok" (like the Air).
			send(text("version = "))
			send(text("1.0.5\r\n"))
		case "$G":
			// Terminated with "ok" (like the Z1).
			send(text("[G0 G54 G17 G21 G90]\n"))
			send(text("ok\n"))
		}
	})
	ctx := context.Background()

	got, err := c.Command(ctx, "version")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "version = 1.0.5" {
		t.Fatalf("version: %q", got)
	}

	start := time.Now()
	got, err = c.Command(ctx, "$G")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !strings.HasPrefix(got[0], "[G0") {
		t.Fatalf("$G: %q", got)
	}
	if time.Since(start) > replyTimeout/2 {
		t.Fatal("ok should end the reply early")
	}

	got, err = c.Command(ctx, "G90")
	if err != nil || len(got) != 0 {
		t.Fatalf("silent command: %q %v", got, err)
	}
}

func TestCommandEscapes(t *testing.T) {
	m, c := newFakeMachine(t, func(Frame, func(Frame)) {})
	if err := c.Send("echo hi?!"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Play(context.Background(), "/sd/gcodes/my file.nc"); err != nil {
		t.Fatal(err)
	}

	cmds := m.commands()
	if len(cmds) != 2 || cmds[0] != "echo hi\x02\x04" || cmds[1] != "play /sd/gcodes/my\x01file.nc" {
		t.Fatalf("commands %q", cmds)
	}
}

func TestStatusAndWaitIdle(t *testing.T) {
	var mu sync.Mutex
	states := []string{"Run", "Run", "Idle", "Run", "Idle", "Idle"}
	_, c := newFakeMachine(t, func(f Frame, send func(Frame)) {
		if f.Type != TypeCtrlSingle || f.Payload[0] != '?' {
			return
		}
		mu.Lock()
		s := states[0]
		if len(states) > 1 {
			states = states[1:]
		}
		mu.Unlock()
		send(Frame{Type: TypeStatus, Payload: []byte("<" + s + "|MPos:0,0,0|WPos:1,2,3>\n")})
	})

	st, err := c.WaitIdle(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if st.State != StateIdle || st.WPos != [3]float64{1, 2, 3} {
		t.Fatalf("got %+v", st)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(states) != 1 {
		t.Fatalf("WaitIdle returned before two consecutive idle reports (%d left)", len(states))
	}
}

func TestList(t *testing.T) {
	_, c := newFakeMachine(t, func(f Frame, send func(Frame)) {
		if !strings.HasPrefix(string(f.Payload), "ls -e -s") {
			return
		}
		send(Frame{Type: TypeLoadInfo, Payload: []byte("Examples/ 0 20250217150328\ngiraff/ 0 20260102144827\n")})
		send(Frame{Type: TypeLoadInfo, Payload: []byte("my\x01lid.nc 1234 20260927120000\n")})
		send(Frame{Type: TypeLoadFinish, Payload: []byte("Load directory finished.")})
	})

	entries, err := c.List(context.Background(), "/sd/gcodes")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries: %+v", len(entries), entries)
	}
	if !entries[0].Dir || entries[0].Name != "Examples" {
		t.Fatalf("dir entry %+v", entries[0])
	}
	if entries[2].Dir || entries[2].Name != "my lid.nc" || entries[2].Size != 1234 {
		t.Fatalf("file entry %+v", entries[2])
	}
}

// uploadMachine emulates the machine side of a file transfer, requesting
// packets out of order and with one retry.
func uploadMachine(got *bytes.Buffer, digest *string, path *string) func(Frame, func(Frame)) {
	var packets, size, next int
	retried := false
	chunks := map[int][]byte{}

	return func(f Frame, send func(Frame)) {
		switch f.Type {
		case TypeFileStart:
			*path = strings.TrimSpace(string(f.Payload))
		case TypeFileMD5:
			*digest = string(f.Payload)
			send(Frame{Type: TypeFileView})
		case TypeFileView:
			packets = int(binary.BigEndian.Uint32(f.Payload))
			size = int(binary.BigEndian.Uint16(f.Payload[4:]))
			next = 1
			send(Frame{Type: TypeFileData, Payload: binary.BigEndian.AppendUint32(nil, uint32(next))})
		case TypeFileData:
			seq := int(binary.BigEndian.Uint32(f.Payload))
			if seq == 2 && !retried {
				retried = true
				send(Frame{Type: TypeFileRetry})
				return
			}
			chunks[seq] = bytes.Clone(f.Payload[4:])
			if len(f.Payload[4:]) > size {
				panic("chunk too large")
			}
			next++
			if next > packets {
				for i := 1; i <= packets; i++ {
					got.Write(chunks[i])
				}
				send(Frame{Type: TypeFileEnd})
				return
			}
			send(Frame{Type: TypeFileData, Payload: binary.BigEndian.AppendUint32(nil, uint32(next))})
		}
	}
}

func TestUpload(t *testing.T) {
	var got bytes.Buffer
	var digest, path string
	_, c := newFakeMachine(t, uploadMachine(&got, &digest, &path))

	data := bytes.Repeat([]byte("G1 X1 Y1\n"), 3000)
	var sent int
	err := c.Upload(context.Background(), "/sd/gcodes/laserlabel/current.nc", data, func(n int) { sent = n })
	if err != nil {
		t.Fatal(err)
	}

	sum := md5.Sum(data)
	if digest != hex.EncodeToString(sum[:]) {
		t.Fatalf("digest %q", digest)
	}
	if path != "upload /sd/gcodes/laserlabel/current.nc" {
		t.Fatalf("path %q", path)
	}
	if !bytes.Equal(got.Bytes(), data) {
		t.Fatalf("received %d bytes, want %d", got.Len(), len(data))
	}
	if sent != len(data) {
		t.Fatalf("progress %d, want %d", sent, len(data))
	}
}

func TestUploadCancelledByMachine(t *testing.T) {
	_, c := newFakeMachine(t, func(f Frame, send func(Frame)) {
		if f.Type == TypeFileMD5 {
			send(Frame{Type: TypeFileCancel})
		}
	})
	err := c.Upload(context.Background(), "/sd/gcodes/x.nc", []byte("M5\n"), nil)
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("got %v", err)
	}
}
