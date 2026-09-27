package machine

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"testing"
)

// Reference frames produced by the Carvera community controller.
const (
	refVersionCmd   = "8668000aa276657273696f6ecca055aa"
	refVersionReply = "866800139076657273696f6e203d20312e302e350abd7255aa"
)

func TestFrameMarshalMatchesReference(t *testing.T) {
	b, err := Frame{Type: TypeCtrlMulti, Payload: []byte("version")}.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if got := hex.EncodeToString(b); got != refVersionCmd {
		t.Fatalf("got %s, want %s", got, refVersionCmd)
	}

	_, err = Frame{Payload: make([]byte, maxFrameLength)}.MarshalBinary()
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("got %v, want ErrFrameTooLarge", err)
	}
}

func TestFrameReaderResyncs(t *testing.T) {
	good, _ := hex.DecodeString(refVersionReply)
	corrupt := bytes.Clone(good)
	corrupt[8] ^= 0xFF

	var stream []byte
	stream = append(stream, 0x00, 0x86, 0x42)
	stream = append(stream, corrupt...)
	stream = append(stream, good...)

	fr := NewFrameReader(bytes.NewReader(stream))
	f, err := fr.Next()
	if err != nil {
		t.Fatal(err)
	}
	if f.Type != TypeNormalInfo || string(f.Payload) != "version = 1.0.5\n" {
		t.Fatalf("got %#x %q", f.Type, f.Payload)
	}
	if _, err := fr.Next(); err != io.EOF {
		t.Fatalf("got %v, want EOF", err)
	}
}

func TestFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	frames := []Frame{
		{Type: TypeStatus, Payload: []byte("<Idle|MPos:0,0,0|WPos:0,0,0>")},
		{Type: TypeFileData, Payload: bytes.Repeat([]byte{0x86, 0x68, 0x55, 0xAA}, 2048)},
		{Type: TypeFileEnd},
	}
	for _, f := range frames {
		b, err := f.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(b)
	}

	fr := NewFrameReader(&buf)
	for i, want := range frames {
		got, err := fr.Next()
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if got.Type != want.Type || !bytes.Equal(got.Payload, want.Payload) {
			t.Fatalf("frame %d mismatch", i)
		}
	}
}
