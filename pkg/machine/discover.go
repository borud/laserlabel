package machine

import (
	"context"
	"net"
	"strconv"
	"strings"
	"time"
)

// discoveryPort is the UDP port machines broadcast their presence on.
const discoveryPort = 3333

// Announcement is a machine's UDP presence broadcast, e.g.
// "CARVERA_AIR_00000,192.168.1.50,2222,0".
type Announcement struct {
	Name  string `json:"name"`
	IP    string `json:"ip"`
	Port  int    `json:"port"`
	Busy  bool   `json:"busy"`
	State string `json:"state"`
}

// Addr returns the machine's TCP address.
func (a Announcement) Addr() string {
	return net.JoinHostPort(a.IP, strconv.Itoa(a.Port))
}

// ParseAnnouncement parses a presence broadcast.
func ParseAnnouncement(b []byte) (Announcement, bool) {
	f := strings.Split(strings.TrimSpace(string(b)), ",")
	if len(f) < 4 {
		return Announcement{}, false
	}
	port, err := strconv.Atoi(f[2])
	if err != nil {
		return Announcement{}, false
	}

	a := Announcement{Name: f[0], IP: f[1], Port: port, Busy: f[3] != "0"}
	if len(f) > 4 {
		a.State = f[4]
	}
	return a, true
}

// Discover listens for machine broadcasts for the given duration, or
// until ctx is done, and returns the machines seen.
func Discover(ctx context.Context, d time.Duration) ([]Announcement, error) {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()

	seen := map[string]bool{}
	found := []Announcement{}
	err := Watch(ctx, func(a Announcement) {
		if seen[a.Name] {
			return
		}
		seen[a.Name] = true
		found = append(found, a)
	})
	return found, err
}

// Watch listens for machine broadcasts and calls f for each one until ctx
// is done. It returns nil when ctx ends.
func Watch(ctx context.Context, f func(Announcement)) error {
	lc := net.ListenConfig{Control: reusePort}
	pc, err := lc.ListenPacket(ctx, "udp4", ":"+strconv.Itoa(discoveryPort))
	if err != nil {
		return err
	}
	defer pc.Close()

	go func() {
		<-ctx.Done()
		pc.Close()
	}()

	buf := make([]byte, 1024)
	for {
		n, _, err := pc.ReadFrom(buf)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			return err
		}
		if a, ok := ParseAnnouncement(buf[:n]); ok {
			f(a)
		}
	}
}
