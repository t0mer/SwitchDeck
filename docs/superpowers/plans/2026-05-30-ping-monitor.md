# Ping Monitor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a 30-second TCP-connect probe to each switch worker; flip status to "offline" after 2 consecutive failures, reset to "online" on success.

**Architecture:** A `pingTicker` is added to the existing `worker` struct alongside the stats/config tickers. `doPing()` does a 3-second TCP dial to `ip:80`, updates a consecutive-failure counter (reset on success), and sets `pingIsDown=true` when the count reaches 2. `Manager.Status()` consults `w.pingIsDown()` above the snapshot check so that an unreachable switch shows "offline" even if it has a cached snapshot.

**Tech Stack:** Go standard library `net` package only — no new dependencies.

---

### Task 1: Add ping state fields to the worker and expose `pingIsDown()`

**Files:**
- Modify: `internal/manager/worker.go`

- [ ] **Step 1: Add ping constants and fields**

In `internal/manager/worker.go`, add directly after the imports block:

```go
const (
	pingInterval  = 30 * time.Second
	pingTimeout   = 3 * time.Second
	pingThreshold = 2
)
```

Add the following fields to the `worker` struct (after the existing `last *models.SwitchSnapshot` field):

```go
pingMu     sync.Mutex
pingConsec int  // consecutive failure count; reset to 0 on success
pingReady  bool // true once at least one probe has completed
pingDown   bool // true when pingConsec >= pingThreshold
```

- [ ] **Step 2: Add `pingIsDown()` accessor**

Add after the existing `lastSnapshot()` method:

```go
// pingIsDown returns true when the switch has failed pingThreshold consecutive
// probes. Returns false until the first probe completes (avoids premature offline).
func (w *worker) pingIsDown() bool {
	w.pingMu.Lock()
	defer w.pingMu.Unlock()
	return w.pingReady && w.pingDown
}
```

- [ ] **Step 3: Build**

```bash
go build ./internal/manager/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/manager/worker.go
git commit -m "feat(ping): add ping state fields and pingIsDown accessor to worker"
```

---

### Task 2: Implement `doPing()` with failure-count logic

**Files:**
- Modify: `internal/manager/worker.go`
- Test: `internal/manager/worker_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/manager/worker_test.go`:

```go
func TestPingSuccess(t *testing.T) {
	// Start a TCP listener so the dial succeeds.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	host, port, _ := net.SplitHostPort(ln.Addr().String())
	cfg := models.SwitchConfig{
		ID: "ping-ok", IP: host,
		Username: "u", Password: "p",
		PollStatsSecs: 60, PollConfigSecs: 300,
	}
	// Override the port used by doPing via a test helper (see Step 3).
	w := newTestWorker(cfg, port)
	ctx := context.Background()
	w.doPing(ctx)

	if w.pingIsDown() {
		t.Error("expected pingIsDown=false after successful probe")
	}
}

func TestPingOfflineAfterTwoFailures(t *testing.T) {
	// Use a port nothing listens on so dial fails fast.
	cfg := models.SwitchConfig{
		ID: "ping-fail", IP: "127.0.0.1",
		Username: "u", Password: "p",
		PollStatsSecs: 60, PollConfigSecs: 300,
	}
	w := newTestWorker(cfg, "19999") // nothing listening
	ctx := context.Background()

	w.doPing(ctx)
	if w.pingIsDown() {
		t.Error("should not be offline after only one failure")
	}
	w.doPing(ctx)
	if !w.pingIsDown() {
		t.Error("expected pingIsDown=true after two failures")
	}
}

func TestPingRecovery(t *testing.T) {
	cfg := models.SwitchConfig{
		ID: "ping-recover", IP: "127.0.0.1",
		Username: "u", Password: "p",
		PollStatsSecs: 60, PollConfigSecs: 300,
	}
	w := newTestWorker(cfg, "19999")
	ctx := context.Background()

	// Drive to offline.
	w.doPing(ctx)
	w.doPing(ctx)
	if !w.pingIsDown() {
		t.Fatal("setup: expected offline after two failures")
	}

	// Now let a real listener recover it.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	w2 := newTestWorker(cfg, port)
	// Copy failure state onto w2 to simulate recovery from offline.
	w2.pingMu.Lock()
	w2.pingConsec = pingThreshold
	w2.pingReady = true
	w2.pingDown = true
	w2.pingMu.Unlock()

	w2.doPing(ctx)
	if w2.pingIsDown() {
		t.Error("expected pingIsDown=false after successful probe (recovery)")
	}
}
```

Also add a package-level helper at the top of the test file (after the existing imports):

```go
// newTestWorker creates a worker whose doPing uses overridePort instead of ":80".
func newTestWorker(cfg models.SwitchConfig, overridePort string) *manager.TestWorker {
	return manager.NewTestWorker(cfg, overridePort)
}
```

Add `"net"` to the test file's import block.

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/manager/... -run "TestPing" -v
```

Expected: compile error — `doPing`, `newTestWorker`, `NewTestWorker`, `pingThreshold` not yet exported.

- [ ] **Step 3: Implement `doPing()` and a test-only constructor**

Add `doPing` to `internal/manager/worker.go`:

```go
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
```

Add `pingPort string` field to the `worker` struct (after `pingDown bool`):

```go
pingPort string // management TCP port to probe; normally "80"
```

Update `newWorker` to initialise it:

```go
func newWorker(cfg models.SwitchConfig, client switchclient.Client, onSnap SnapshotFunc, onErr ErrorFunc) *worker {
	return &worker{
		cfg:      cfg,
		client:   client,
		onSnap:   onSnap,
		onErr:    onErr,
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
		pingPort: "80",
	}
}
```

Add `"net"` to the `worker.go` imports.

Add the exported test helper at the bottom of `worker.go` (inside a `//go:build` guard so it only compiles in tests — or export via a separate `export_test.go` file):

Create `internal/manager/export_test.go`:

```go
package manager

import "github.com/t0mer/SwitchDeck/internal/models"

// TestWorker exposes worker internals for white-box tests.
type TestWorker = worker

// NewTestWorker creates a worker with a custom probe port for testing.
func NewTestWorker(cfg models.SwitchConfig, probePort string) *worker {
	w := newWorker(cfg, nil, func(*models.SwitchSnapshot, bool) {}, func(string, error) {})
	w.pingPort = probePort
	return w
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/manager/... -run "TestPing" -v
```

Expected:
```
--- PASS: TestPingSuccess
--- PASS: TestPingOfflineAfterTwoFailures
--- PASS: TestPingRecovery
```

- [ ] **Step 5: Run full suite to check no regressions**

```bash
go test ./internal/manager/... -v
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/manager/worker.go internal/manager/export_test.go internal/manager/worker_test.go
git commit -m "feat(ping): implement doPing with consecutive-failure threshold"
```

---

### Task 3: Start the ping ticker in `worker.run()`

**Files:**
- Modify: `internal/manager/worker.go`

- [ ] **Step 1: Add ping ticker to `run()`**

Replace the ticker setup section in `worker.run()` (the block that creates `statsTicker`, `configTicker`, and starts the `for` loop) with:

```go
statsDur := time.Duration(w.cfg.PollStatsSecs) * time.Second
configDur := time.Duration(w.cfg.PollConfigSecs) * time.Second

statsTicker  := time.NewTicker(statsDur)
configTicker := time.NewTicker(configDur)
pingTicker   := time.NewTicker(pingInterval)
defer statsTicker.Stop()
defer configTicker.Stop()
defer pingTicker.Stop()

// First probe fires immediately so status is known within seconds of startup.
go w.doPing(ctx)

for {
    select {
    case <-w.stop:
        return
    case <-configTicker.C:
        w.collectFull(ctx)
    case <-statsTicker.C:
        w.collectStats(ctx)
    case <-pingTicker.C:
        go w.doPing(ctx)
    }
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/manager/...
```

Expected: no errors.

- [ ] **Step 3: Run full suite**

```bash
go test ./internal/manager/... -v
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add internal/manager/worker.go
git commit -m "feat(ping): start ping ticker in worker.run() with immediate first probe"
```

---

### Task 4: Update `Manager.Status()` to consult ping state

**Files:**
- Modify: `internal/manager/manager.go`
- Test: `internal/manager/worker_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/manager/worker_test.go`:

```go
func TestManagerStatusOfflineWhenPingFails(t *testing.T) {
	mc := &mockClient{}
	cfg := models.SwitchConfig{
		ID: "sw-ping", IP: "127.0.0.1",
		Username: "admin", Password: "pass", Enabled: true,
		PollStatsSecs: 60, PollConfigSecs: 300,
	}
	clientFactory := func(_ bool) switchclient.Client { return mc }
	mgr := manager.New(clientFactory)
	mgr.Add(cfg)
	time.Sleep(300 * time.Millisecond) // let initial collection + ping run

	// At this point the switch is "online" (snapshot collected, ping to
	// 127.0.0.1:80 will fail because nothing listens → but threshold is 2).
	// Manually drive the worker to offline via the test hook.
	manager.DriveWorkerOffline(mgr, "sw-ping")

	if status := mgr.Status("sw-ping"); status != models.SwitchStatusOffline {
		t.Errorf("expected offline, got %v", status)
	}
	mgr.Remove("sw-ping")
}
```

Add the test hook to `internal/manager/export_test.go`:

```go
// DriveWorkerOffline forces the named worker's ping state to offline
// (pingConsec >= pingThreshold) for testing Manager.Status().
func DriveWorkerOffline(m *Manager, id string) {
	m.mu.RLock()
	w, ok := m.workers[id]
	m.mu.RUnlock()
	if !ok {
		return
	}
	w.pingMu.Lock()
	w.pingConsec = pingThreshold
	w.pingReady = true
	w.pingDown = true
	w.pingMu.Unlock()
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./internal/manager/... -run "TestManagerStatusOfflineWhenPingFails" -v
```

Expected: FAIL — `got online, want offline`.

- [ ] **Step 3: Update `Manager.Status()` priority**

Replace the body of `Status()` in `internal/manager/manager.go`:

```go
// Status returns the runtime reachability status of a switch.
func (m *Manager) Status(id string) models.SwitchStatus {
	if _, ok := m.collecting.Load(id); ok {
		return models.SwitchStatusCollecting
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.workers[id]
	if !ok {
		return models.SwitchStatusUnknown
	}
	if w.pingIsDown() {
		return models.SwitchStatusOffline
	}
	if w.lastSnapshot() != nil {
		return models.SwitchStatusOnline
	}
	return models.SwitchStatusUnknown
}
```

- [ ] **Step 4: Run all tests**

```bash
go test ./... -v 2>&1 | grep -E "PASS|FAIL|---"
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/manager/manager.go internal/manager/export_test.go internal/manager/worker_test.go
git commit -m "feat(ping): update Manager.Status() to return offline on ping failure"
```

---

### Task 5: Push

- [ ] **Step 1: Final build + test**

```bash
go build ./... && go test ./...
```

Expected: BUILD OK, all tests pass.

- [ ] **Step 2: Push**

```bash
git push
```
