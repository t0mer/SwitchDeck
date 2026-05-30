package manager

import (
	"context"

	"github.com/t0mer/SwitchDeck/internal/models"
)

// PingThreshold exposes the package-level constant for tests.
const PingThreshold = pingThreshold

// TestWorker exposes worker internals for white-box tests.
type TestWorker struct {
	*worker
}

// NewTestWorker creates a worker with a custom probe port for testing.
func NewTestWorker(cfg models.SwitchConfig, probePort string) *TestWorker {
	w := newWorker(cfg, nil, func(*models.SwitchSnapshot, bool) {}, func(string, error) {})
	w.pingPort = probePort
	return &TestWorker{w}
}

// DoPing exposes doPing for tests.
func (tw *TestWorker) DoPing(ctx context.Context) {
	tw.worker.doPing(ctx)
}

// PingIsDown exposes pingIsDown for tests.
func (tw *TestWorker) PingIsDown() bool {
	return tw.worker.pingIsDown()
}

// SetWorkerOffline drives a worker's ping state to offline for testing recovery.
func SetWorkerOffline(tw *TestWorker) {
	tw.pingMu.Lock()
	tw.pingConsec = pingThreshold
	tw.pingReady = true
	tw.pingDown = true
	tw.pingMu.Unlock()
}
