//go:build unix

package machine

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// reusePort lets several programs (e.g. this and Carvera Controller)
// listen for discovery broadcasts at the same time.
func reusePort(_, _ string, rc syscall.RawConn) error {
	var serr error
	err := rc.Control(func(fd uintptr) {
		serr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
		if serr != nil {
			return
		}
		serr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	})
	if err != nil {
		return err
	}
	return serr
}
