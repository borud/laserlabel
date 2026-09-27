// Package machine talks to Makera machines (Carvera Air, Z1) over the
// Makera binary framed protocol on TCP port 2222.
//
// Frame layout, big-endian:
//
//	0x8668 | length(2) | type(1) | payload | crc16(2) | 0x55AA
//
// where length = 1 + len(payload) + 2 and the CRC-16/CCITT covers the
// length, type and payload.
package machine

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Frame delimiters.
const (
	frameHeader = 0x8668
	frameFooter = 0x55AA

	// maxFrameLength bounds the length field, as in the Makera controller.
	maxFrameLength = 8200
)

// Frame types.
const (
	TypeCtrlSingle = 0xA1
	TypeCtrlMulti  = 0xA2
	TypeFileStart  = 0xB0
	TypeFileMD5    = 0xB1
	TypeFileView   = 0xB2
	TypeFileData   = 0xB3
	TypeFileEnd    = 0xB4
	TypeFileCancel = 0xB5
	TypeFileRetry  = 0xB6
	TypeStatus     = 0x81
	TypeDiag       = 0x82
	TypeLoadInfo   = 0x83
	TypeLoadFinish = 0x84
	TypeLoadError  = 0x85
	TypeNormalInfo = 0x90
)

// ErrFrameTooLarge is returned when encoding a frame whose payload
// exceeds the protocol limit.
var ErrFrameTooLarge = errors.New("frame too large")

// Frame is a single protocol frame.
type Frame struct {
	Type    byte
	Payload []byte
}

// MarshalBinary encodes the frame for the wire.
func (f Frame) MarshalBinary() ([]byte, error) {
	length := 1 + len(f.Payload) + 2
	if length > maxFrameLength {
		return nil, fmt.Errorf("%w: %d bytes", ErrFrameTooLarge, len(f.Payload))
	}

	b := make([]byte, 0, 2+length+2)
	b = binary.BigEndian.AppendUint16(b, frameHeader)
	b = binary.BigEndian.AppendUint16(b, uint16(length))
	b = append(b, f.Type)
	b = append(b, f.Payload...)
	b = binary.BigEndian.AppendUint16(b, crc16(b[2:]))
	b = binary.BigEndian.AppendUint16(b, frameFooter)
	return b, nil
}

// FrameReader reads frames from a byte stream, skipping garbage and
// frames with bad checksums.
type FrameReader struct {
	r *bufio.Reader
}

// NewFrameReader returns a FrameReader reading from r.
func NewFrameReader(r io.Reader) *FrameReader {
	return &FrameReader{r: bufio.NewReaderSize(r, 2*maxFrameLength)}
}

// Next returns the next valid frame.
func (fr *FrameReader) Next() (Frame, error) {
	for {
		err := fr.syncHeader()
		if err != nil {
			return Frame{}, err
		}

		var lb [2]byte
		if _, err := io.ReadFull(fr.r, lb[:]); err != nil {
			return Frame{}, err
		}
		length := int(binary.BigEndian.Uint16(lb[:]))
		if length < 3 || length > maxFrameLength {
			continue
		}

		body := make([]byte, length+2)
		if _, err := io.ReadFull(fr.r, body); err != nil {
			return Frame{}, err
		}
		if binary.BigEndian.Uint16(body[length:]) != frameFooter {
			continue
		}

		crcData := append(lb[:], body[:length-2]...)
		if crc16(crcData) != binary.BigEndian.Uint16(body[length-2:length]) {
			continue
		}
		return Frame{Type: body[0], Payload: body[1 : length-2]}, nil
	}
}

// syncHeader consumes bytes up to and including the next frame header.
func (fr *FrameReader) syncHeader() error {
	var prev byte
	for {
		b, err := fr.r.ReadByte()
		if err != nil {
			return err
		}
		if uint16(prev)<<8|uint16(b) == frameHeader {
			return nil
		}
		prev = b
	}
}

// crc16 computes CRC-16/CCITT (XModem variant, polynomial 0x1021, init 0).
func crc16(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
				continue
			}
			crc <<= 1
		}
	}
	return crc
}
