package manager

import (
	"context"
	"fmt"
	"sync"

	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/internal/store"
	"github.com/t0mer/SwitchDeck/internal/switchclient"
)

// ClientFactory creates a switch client. insecure controls TLS verification.
type ClientFactory func(insecure bool) switchclient.Client

// Manager orchestrates the collection pool across all registered switches.
type Manager struct {
	factory    ClientFactory
	mu         sync.RWMutex
	workers    map[string]*worker
	collecting sync.Map // string → struct{}: switches currently collecting
	snapFunc   SnapshotFunc
	errFunc    ErrorFunc
}

// New creates a Manager. factory is called when a switch is added.
func New(factory ClientFactory) *Manager {
	m := &Manager{
		factory:  factory,
		workers:  make(map[string]*worker),
		snapFunc: func(*models.SwitchSnapshot, bool) {},
		errFunc:  func(string, error) {},
	}
	return m
}

// SetSnapshotHandler registers a callback invoked after each successful collection.
func (m *Manager) SetSnapshotHandler(fn SnapshotFunc) {
	m.mu.Lock()
	m.snapFunc = fn
	m.mu.Unlock()
}

// SetErrorHandler registers a callback invoked on collection errors.
func (m *Manager) SetErrorHandler(fn ErrorFunc) {
	m.mu.Lock()
	m.errFunc = fn
	m.mu.Unlock()
}

// LoadFromStore reads all enabled switches from the store and starts their workers.
func (m *Manager) LoadFromStore(ctx context.Context, st store.Store, encKey []byte) error {
	cfgs, err := st.ListSwitches(ctx, encKey)
	if err != nil {
		return fmt.Errorf("list switches: %w", err)
	}
	for _, cfg := range cfgs {
		if !cfg.Enabled {
			continue
		}
		if err := m.Add(cfg); err != nil {
			return fmt.Errorf("add switch %s: %w", cfg.ID, err)
		}
	}
	return nil
}

// Add adds a switch to the pool and starts its collection worker.
func (m *Manager) Add(cfg models.SwitchConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.workers[cfg.ID]; exists {
		return fmt.Errorf("switch %s already in pool", cfg.ID)
	}
	client := m.factory(cfg.InsecureTLS)
	w := newWorker(cfg, client, m.snapFunc, m.errFunc)
	m.workers[cfg.ID] = w
	w.start()
	return nil
}

// Remove stops the switch worker and removes it from the pool.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	w, ok := m.workers[id]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("switch %s not in pool", id)
	}
	delete(m.workers, id)
	m.mu.Unlock()
	w.stopAndWait()
	return nil
}

// Update stops and re-creates a switch worker with new config.
func (m *Manager) Update(cfg models.SwitchConfig) error {
	if err := m.Remove(cfg.ID); err != nil {
		return err
	}
	return m.Add(cfg)
}

// GetClient returns the switch client for use by API action handlers.
func (m *Manager) GetClient(id string) (switchclient.Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.workers[id]
	if !ok {
		return nil, fmt.Errorf("switch %s not in pool", id)
	}
	return w.client, nil
}

// CollectNow performs an immediate full collection for a switch using a fresh
// authenticated client session independent of the background worker.
// On success it updates the worker's cached last snapshot.
func (m *Manager) CollectNow(ctx context.Context, id string) (*models.SwitchSnapshot, error) {
	m.mu.RLock()
	w, ok := m.workers[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("switch %s not in pool", id)
	}
	m.collecting.Store(id, struct{}{})
	defer m.collecting.Delete(id)

	client := m.factory(w.cfg.InsecureTLS)
	if err := client.Login(ctx, "http://"+w.cfg.IP, w.cfg.Username, w.cfg.Password); err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}
	snap, err := client.GetSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	snap.Switch.ID = id
	w.mu.Lock()
	w.last = snap
	w.mu.Unlock()
	return snap, nil
}

// LastSnapshot returns the most recently collected snapshot for a switch.
func (m *Manager) LastSnapshot(id string) (*models.SwitchSnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.workers[id]
	if !ok {
		return nil, fmt.Errorf("switch %s not in pool", id)
	}
	snap := w.lastSnapshot()
	if snap == nil {
		return nil, fmt.Errorf("no snapshot yet for switch %s", id)
	}
	return snap, nil
}

// Status returns the runtime reachability status of a switch.
func (m *Manager) Status(id string) models.SwitchStatus {
	if _, collecting := m.collecting.Load(id); collecting {
		return models.SwitchStatusCollecting
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.workers[id]
	if !ok {
		return models.SwitchStatusUnknown
	}
	if w.lastSnapshot() != nil {
		return models.SwitchStatusOnline
	}
	return models.SwitchStatusUnknown
}
