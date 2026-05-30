# Ping Monitor Design

Date: 2026-05-30
Scope: Per-switch TCP-connect probe every 30 s; offline status after 2 consecutive failures.

---

## Goals

- Detect switch reachability independently of the data-collection cycle.
- Flip a switch to "offline" after 2 consecutive probe failures to avoid false
  positives from transient packet loss.
- Expose the result through the existing `Status()` path so the UI badge updates
  automatically with no additional changes.
- Lay the groundwork for offline notifications (out of scope here).

---

## Probe method

TCP-connect to `<switch-ip>:80` with a 3-second dial timeout. No HTTP request
is sent — a successful TCP handshake is sufficient to confirm reachability. The
connection is closed immediately on success.

Port 80 is chosen because it is always open on the TL-SG108E management
interface and requires no authentication.

---

## Architecture

All changes live inside `internal/manager/worker.go` and
`internal/manager/manager.go`. No other packages are touched.

### Worker fields added

```go
pingTicker  *time.Ticker  // 30 s interval
pingMu      sync.Mutex    // guards the three fields below
pingConsec  int           // consecutive failure count; reset to 0 on success
pingReady   bool          // true once at least one probe has completed
pingIsDown  bool          // true when pingConsec >= 2
```

### Worker behaviour

1. **Immediate first probe** — `go w.doPing(ctx)` is called once before the
   ticker loop starts, so the first result arrives within 3 s of worker startup.
2. **Ticker** — `pingTicker = time.NewTicker(30 * time.Second)`.  
   Each tick fires `go w.doPing(ctx)` so probes never block the collect loop.
3. **doPing logic**:
   - `net.DialTimeout("tcp", w.cfg.IP+":80", 3*time.Second)`
   - **Success**: close conn, acquire `pingMu`, reset `pingConsec = 0`,
     `pingReady = true`, `pingIsDown = false`.
   - **Failure**: acquire `pingMu`, increment `pingConsec`,
     `pingReady = true`; if `pingConsec >= 2` → `pingIsDown = true`.
4. **Ticker stopped** in `defer` alongside the existing tickers when the worker
   stops.

### Manager.Status() priority (updated)

```
1. collecting map entry present → "collecting"
2. w.pingIsDown() == true       → "offline"
3. w.lastSnapshot() != nil      → "online"
4. default                       → "unknown"
```

`pingIsDown()` acquires `pingMu` and returns the `pingIsDown` field.  
If `pingReady` is false (no probe completed yet) the method returns false so the
switch is not prematurely marked offline on first startup.

---

## Constants

| Constant | Value | Rationale |
|---|---|---|
| `pingInterval` | 30 s | Frequent enough to detect failures promptly; light enough not to overload the switch. |
| `pingTimeout` | 3 s | Generous dial timeout for a LAN device; well below the 30 s interval. |
| `pingThreshold` | 2 | Requires two consecutive failures before reporting offline; absorbs single-packet transients. |

---

## Error handling

- Probe errors are logged at debug level: `ping[<id>]: <err>`.
- A probe failure never blocks or panics the worker.
- Recovery: any successful probe resets the failure counter immediately, so the
  switch returns to "online" on the next successful probe after an outage ends.

---

## What this does NOT include

- Persistence of ping history (runtime state only).
- Push notifications on status change (future work).
- Any API or UI additions — the existing `Status()` → `switchResponse.status`
  → badge pipeline already handles "offline".

---

## Files changed

| File | Change |
|---|---|
| `internal/manager/worker.go` | Add ping fields, `doPing()`, `pingTicker` in `run()` |
| `internal/manager/manager.go` | Update `Status()` priority order |
