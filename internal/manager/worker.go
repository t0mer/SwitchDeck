package manager

import (
	"context"
	"log"
	"net"
	"sync"
	"time"

	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/internal/switchclient"
)

const (
	pingInterval  = 30 * time.Second
	pingTimeout   = 3 * time.Second
	pingThreshold = 2
)

// SnapshotFunc is called after each successful collection.
// statsOnly is true when only port stats were refreshed.
type SnapshotFunc func(snap *models.SwitchSnapshot, statsOnly bool)

// ErrorFunc is called when a collection attempt fails.
type ErrorFunc func(switchID string, err error)

// worker runs the collection loop for one switch.
type worker struct {
	cfg     models.SwitchConfig
	client  switchclient.Client
	onSnap  SnapshotFunc
	onErr   ErrorFunc
	stop    chan struct{}
	stopped chan struct{}
	mu      sync.Mutex
	last    *models.SwitchSnapshot

	pingMu     sync.Mutex
	pingConsec int    // consecutive failure count; reset to 0 on success
	pingReady  bool   // true once at least one probe has completed
	pingDown   bool   // true when pingConsec >= pingThreshold
	pingPort   string // management TCP port to probe; normally "80"
}

// newWorker creates a worker. Call start() to begin collection.
func newWorker(cfg models.SwitchConfig, client switchclient.Client, onSnap SnapshotFunc, onErr ErrorFunc) *worker {
	return &worker{
		cfg:     cfg,
		client:  client,
		onSnap:  onSnap,
		onErr:   onErr,
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
		pingPort: "80",
	}
}

// start begins the dual-ticker collection loop in a background goroutine.
func (w *worker) start() {
	go w.run()
}

// stopAndWait signals the worker to stop and waits for it to exit.
func (w *worker) stopAndWait() {
	close(w.stop)
	<-w.stopped
}

// lastSnapshot returns the most recently collected snapshot, or nil.
func (w *worker) lastSnapshot() *models.SwitchSnapshot {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.last
}

// pingIsDown returns true when the switch has failed pingThreshold consecutive
// probes. Returns false until the first probe completes (avoids premature offline).
func (w *worker) pingIsDown() bool {
	w.pingMu.Lock()
	defer w.pingMu.Unlock()
	return w.pingReady && w.pingDown
}

// doPing performs a TCP-connect probe to the switch management port.
// On success the consecutive-failure counter is reset; on failure it is
// incremented and pingDown is set when it reaches pingThreshold.
func (w *worker) doPing(ctx context.Context) {
	addr := w.cfg.IP + ":" + w.pingPort
	conn, err := (&net.Dialer{Timeout: pingTimeout}).DialContext(ctx, "tcp", addr)
	w.pingMu.Lock()
	defer w.pingMu.Unlock()
	w.pingReady = true
	if err != nil {
		log.Printf("ping[%s]: %v", w.cfg.ID, err)
		w.pingConsec++
		if w.pingConsec >= pingThreshold {
			w.pingDown = true
		}
		return
	}
	conn.Close()
	w.pingConsec = 0
	w.pingDown = false
}

func (w *worker) run() {
	defer close(w.stopped)
	ctx := context.Background()

	if err := w.client.Login(ctx, "http://"+w.cfg.IP, w.cfg.Username, w.cfg.Password); err != nil {
		log.Printf("manager[%s]: login: %v", w.cfg.ID, err)
		w.onErr(w.cfg.ID, err)
	} else {
		// Initial full collection immediately on start — only when login succeeded.
		w.collectFull(ctx)
	}

	statsDur := time.Duration(w.cfg.PollStatsSecs) * time.Second
	configDur := time.Duration(w.cfg.PollConfigSecs) * time.Second

	statsTicker := time.NewTicker(statsDur)
	configTicker := time.NewTicker(configDur)
	defer statsTicker.Stop()
	defer configTicker.Stop()

	for {
		select {
		case <-w.stop:
			return
		case <-configTicker.C:
			w.collectFull(ctx)
		case <-statsTicker.C:
			w.collectStats(ctx)
		}
	}
}

func (w *worker) collectFull(ctx context.Context) {
	snap, err := w.client.GetSnapshot(ctx)
	if err != nil {
		log.Printf("manager[%s]: full collect: %v", w.cfg.ID, err)
		w.onErr(w.cfg.ID, err)
		return
	}
	snap.Switch.ID = w.cfg.ID
	w.mu.Lock()
	w.last = snap
	w.mu.Unlock()
	w.onSnap(snap, false)
}

func (w *worker) collectStats(ctx context.Context) {
	stats, err := w.client.RefreshStats(ctx)
	if err != nil {
		log.Printf("manager[%s]: stats refresh: %v", w.cfg.ID, err)
		w.onErr(w.cfg.ID, err)
		return
	}
	w.mu.Lock()
	if w.last != nil && len(stats) > 0 {
		w.last.PortStats = stats
	}
	snap := w.last
	w.mu.Unlock()
	if snap != nil {
		w.onSnap(snap, true)
	}
}
