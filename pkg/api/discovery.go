package api

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/borud/laserlabel/pkg/machine"
)

const (
	// machineExpiry is how long a machine stays listed after its last
	// broadcast.
	machineExpiry = 30 * time.Second

	// watchRetry is the delay before restarting a failed listener.
	watchRetry = 5 * time.Second
)

// SeenMachine is a machine heard on the network.
type SeenMachine struct {
	machine.Announcement
	LastSeen time.Time `json:"lastSeen"`
}

// discovery keeps track of machines broadcasting on the network.
type discovery struct {
	logger *slog.Logger

	mu   sync.Mutex
	seen map[string]SeenMachine
}

func newDiscovery(logger *slog.Logger) *discovery {
	return &discovery{logger: logger, seen: map[string]SeenMachine{}}
}

// run listens for broadcasts until ctx is done, restarting the listener
// if it fails (for instance if the port is briefly taken).
func (d *discovery) run(ctx context.Context) {
	for {
		err := machine.Watch(ctx, d.add)
		if ctx.Err() != nil {
			return
		}
		d.logger.Warn("machine discovery failed, retrying", "err", err)

		select {
		case <-ctx.Done():
			return
		case <-time.After(watchRetry):
		}
	}
}

func (d *discovery) add(a machine.Announcement) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.seen[a.Name] = SeenMachine{Announcement: a, LastSeen: time.Now()}
}

// list returns recently seen machines sorted by name.
func (d *discovery) list() []SeenMachine {
	d.mu.Lock()
	defer d.mu.Unlock()

	out := []SeenMachine{}
	for name, m := range d.seen {
		if time.Since(m.LastSeen) > machineExpiry {
			delete(d.seen, name)
			continue
		}
		out = append(out, m)
	}
	slices.SortFunc(out, func(a, b SeenMachine) int { return strings.Compare(a.Name, b.Name) })
	return out
}
