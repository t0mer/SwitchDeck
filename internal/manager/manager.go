package manager

import (
	"context"
	"fmt"
	"sync"

	"github.com/t0mer/SwitchDeck/internal/collector"
	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/internal/store"
	"github.com/t0mer/SwitchDeck/internal/switchclient"
)

// SwitchEntry pairs a Switch definition with its client.
type SwitchEntry struct {
	Switch models.Switch
	Client switchclient.Client
}

// Manager orchestrates collection and actions across all registered switches.
type Manager struct {
	switches []SwitchEntry
	store    store.Store
}

// New creates a Manager with the given switches and persistence store.
func New(switches []SwitchEntry, s store.Store) *Manager {
	return &Manager{switches: switches, store: s}
}

// CollectAll collects a snapshot from every switch concurrently.
// Errors are aggregated; partial success is returned alongside a combined error.
func (m *Manager) CollectAll(ctx context.Context) ([]*models.SwitchSnapshot, error) {
	type result struct {
		snap *models.SwitchSnapshot
		err  error
	}

	results := make([]result, len(m.switches))
	var wg sync.WaitGroup

	for i, entry := range m.switches {
		wg.Add(1)
		go func(idx int, e SwitchEntry) {
			defer wg.Done()
			c := collector.New(e.Client)
			snap, err := c.Collect(ctx)
			if err == nil && snap != nil {
				snap.Switch = e.Switch
			}
			results[idx] = result{snap: snap, err: err}
		}(i, entry)
	}

	wg.Wait()

	var snaps []*models.SwitchSnapshot
	var errs []error
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
		} else {
			snaps = append(snaps, r.snap)
		}
	}

	if len(errs) > 0 {
		return snaps, fmt.Errorf("collection errors: %v", errs)
	}
	return snaps, nil
}
