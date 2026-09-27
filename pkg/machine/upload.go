package machine

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	// uploadPacketSize is the data size per frame used over WiFi.
	uploadPacketSize = 8192

	// uploadTimeout is how long to wait for the machine during a transfer.
	uploadTimeout = 10 * time.Second
)

// Upload writes data to path on the machine's SD card, e.g.
// "/sd/gcodes/laserlabel/current.nc". The machine drives the transfer by
// requesting the file size and then each packet in turn. progress, if not
// nil, is called with the number of bytes sent so far.
func (c *Conn) Upload(ctx context.Context, path string, data []byte, progress func(sent int)) error {
	c.cmdMu.Lock()
	defer c.cmdMu.Unlock()

	sum := md5.Sum(data)
	digest := []byte(hex.EncodeToString(sum[:]))

	drain(c.file)
	start := Frame{Type: TypeFileStart, Payload: []byte(escape("upload "+escapePath(path)) + "\n")}
	if err := c.writeFrame(start); err != nil {
		return err
	}

	last := Frame{Type: TypeFileMD5, Payload: digest}
	if err := c.writeFrame(last); err != nil {
		return err
	}

	packets := (len(data) + uploadPacketSize - 1) / uploadPacketSize
	timer := time.NewTimer(uploadTimeout)
	defer timer.Stop()
	for {
		var f Frame
		select {
		case f = <-c.file:
			timer.Reset(uploadTimeout)
		case <-timer.C:
			_ = c.writeFrame(Frame{Type: TypeFileCancel})
			return fmt.Errorf("upload %s: machine timed out", path)
		case <-c.done:
			return c.err()
		case <-ctx.Done():
			_ = c.writeFrame(Frame{Type: TypeFileCancel})
			return ctx.Err()
		}

		switch f.Type {
		case TypeFileMD5:
			last = Frame{Type: TypeFileMD5, Payload: digest}
		case TypeFileRetry:
		case TypeFileView:
			p := binary.BigEndian.AppendUint32(nil, uint32(packets))
			p = binary.BigEndian.AppendUint16(p, uploadPacketSize)
			last = Frame{Type: TypeFileView, Payload: p}
		case TypeFileData:
			if len(f.Payload) < 4 {
				continue
			}
			seq := int(binary.BigEndian.Uint32(f.Payload))
			if seq < 1 || seq > packets {
				_ = c.writeFrame(Frame{Type: TypeFileCancel})
				return fmt.Errorf("upload %s: machine requested packet %d of %d", path, seq, packets)
			}
			from := (seq - 1) * uploadPacketSize
			to := min(from+uploadPacketSize, len(data))
			last = Frame{Type: TypeFileData, Payload: append(binary.BigEndian.AppendUint32(nil, uint32(seq)), data[from:to]...)}
			if progress != nil {
				progress(to)
			}
		case TypeFileEnd:
			return nil
		case TypeFileCancel:
			return fmt.Errorf("upload %s: cancelled by machine", path)
		default:
			continue
		}

		if err := c.writeFrame(last); err != nil {
			return err
		}
	}
}
