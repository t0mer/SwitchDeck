# Notifications Design

Date: 2026-05-30
Scope: Per-channel notification system for switch offline/online events. Three providers: Shoutrrr, GreenAPI, WhatsApp Web.

---

## Goals

- Send a notification when a switch goes offline (2 consecutive ping failures).
- Send a notification when a switch comes back online.
- Let the operator configure any number of notification channels, each with independent online/offline toggles and an enabled flag.
- Store channel credentials encrypted at rest (AES-256-GCM, same key as switch passwords).
- Sending is best-effort: errors are logged, never block the ping loop.

---

## Events

| Event | Trigger | `notify_offline` | `notify_online` |
|---|---|---|---|
| Switch offline | `pingDown` flips true (2nd consecutive failure) | ✓ channel receives it | — |
| Switch online | `pingDown` flips false (recovery) | — | ✓ channel receives it |

Transitions only — the callback fires once per edge, not on every probe.

---

## Providers

Three providers, identical to the AGHSync reference:

1. **Shoutrrr** (`shoutrrr`) — `github.com/containrrr/shoutrrr`. Single URL field covers Slack, Discord, Telegram, Gotify, SMTP, ntfy, and more.
2. **GreenAPI** (`greenapi`) — WhatsApp via cloud API. Fields: `instance_id`, `token`, `recipient` (digits only, no `+`), `api_url` (optional, defaults to `https://api.green-api.com`).
3. **WhatsApp Web** (`whatsapp_web`) — self-hosted multidevice. Fields: `base_url`, `recipient`, `username` (optional), `password` (optional).

---

## Data Layer

### SQLite table

```sql
CREATE TABLE notification_channels (
  id              TEXT PRIMARY KEY,
  name            TEXT NOT NULL UNIQUE,
  provider        TEXT NOT NULL,        -- "shoutrrr" | "greenapi" | "whatsapp_web"
  config_enc      BLOB NOT NULL,        -- AES-256-GCM(JSON(provider config))
  enabled         INTEGER NOT NULL DEFAULT 1,
  notify_offline  INTEGER NOT NULL DEFAULT 1,
  notify_online   INTEGER NOT NULL DEFAULT 1,
  created_at      INTEGER NOT NULL,
  updated_at      INTEGER NOT NULL
);
```

### Config structs (serialised to JSON before encryption)

```go
type ShoutrrrConfig struct {
    URL string `json:"url"`
}
type GreenAPIConfig struct {
    InstanceID string `json:"instance_id"`
    Token      string `json:"token"`
    Recipient  string `json:"recipient"`
    APIURL     string `json:"api_url,omitempty"` // defaults to https://api.green-api.com
}
type WhatsAppConfig struct {
    BaseURL   string `json:"base_url"`
    Recipient string `json:"recipient"`
    Username  string `json:"username,omitempty"`
    Password  string `json:"password,omitempty"`
}
```

### Channel model

```go
type Channel struct {
    ID             string
    Name           string
    Provider       string
    Config         string // decrypted JSON, plaintext in memory only
    Enabled        bool
    NotifyOffline  bool
    NotifyOnline   bool
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

---

## Package: `internal/notification/`

Four files, each with one responsibility:

| File | Responsibility |
|---|---|
| `channel.go` | `Channel` model, config structs, provider constants |
| `store.go` | CRUD against `notification_channels`; `ListEnabled(offline, online bool)` |
| `sender.go` | `Sender` interface; `NewSender(ch)` factory for three providers |
| `service.go` | `Service.NotifyPingChange(name, ip string, online bool)` — fan-out |

### store.go

- Uses the store's existing `EncKey` for AES-256-GCM (same helper used for switch passwords).
- `ListEnabled(offline bool, online bool)` — returns channels where `enabled=1` AND the relevant toggle (`notify_offline=1` or `notify_online=1`) matches the event.
- `Update`: if `Config == ""` in the request, preserves the existing `config_enc` (allows updating toggles without re-entering credentials).

### sender.go

```go
type Sender interface {
    Send(ctx context.Context, message string) error
}
```

`NewSender(ch *Channel) Sender` switches on `ch.Provider`:
- `shoutrrr` → unmarshals `ShoutrrrConfig`, calls `shoutrrr.Send(url, msg)`
- `greenapi` → unmarshals `GreenAPIConfig`; POST `{apiURL}/waInstance{instanceID}/sendMessage/{token}` with body `{"chatId":"{recipient}@c.us","message":"..."}`. Appends `@c.us` if recipient doesn't already contain `@`. Trims all fields.
- `whatsapp_web` → unmarshals `WhatsAppConfig`; POST `{baseURL}/send/message` with body `{"phone":..., "message":...}`, optional Basic Auth.

### service.go

```go
type Service struct {
    store  NotificationStore
    encKey []byte
}

func (s *Service) NotifyPingChange(switchID, name, ip string, online bool)
```

1. Calls `store.ListEnabled(!online, online)` — offline event → `ListEnabled(true, false)`, online event → `ListEnabled(false, true)`.
2. Builds message (see below).
3. For each channel: `NewSender(ch).Send(ctx, msg)` — logs errors, never returns them.
4. Uses a 10-second context per send.

### Message format

```
🔴 Switch Offline: Salon (192.168.0.231)
Two consecutive ping failures — switch may be unreachable.

🟢 Switch Online: Salon (192.168.0.231)
Switch is reachable again.
```

---

## Worker Integration

### Transition detection

Add to `worker`:
```go
pingChangeFn func(id string, online bool) // nil = no-op
pingWasDown  bool                          // previous probe state
```

In `doPing`, after updating `pingDown`, compare to `pingWasDown`:
- If `pingDown` changed from false → true: call `pingChangeFn(id, false)` (went offline)
- If `pingDown` changed from true → false: call `pingChangeFn(id, true)` (came back online)
- Update `pingWasDown` to reflect new state.

`pingChangeFn` is guarded by `pingMu` to avoid races.

### Manager wiring

`Manager` receives `*notification.Service` (optional, nil = notifications disabled). When `Add()` is called, it registers a `pingChangeFn` on the worker:

```go
w.pingChangeFn = func(id string, online bool) {
    cfg := w.cfg
    svc.NotifyPingChange(id, cfg.Name, cfg.IP, online)
}
```

`LoadFromStore` propagates the service through to each worker on startup.

---

## REST API

All routes under `/api/v1/notifications`, protected by the existing auth middleware.

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/notifications` | List all channels (config decrypted) |
| POST | `/api/v1/notifications` | Add channel |
| PUT | `/api/v1/notifications/{id}` | Update channel (empty config = keep existing) |
| DELETE | `/api/v1/notifications/{id}` | Delete channel |
| POST | `/api/v1/notifications/test` | Send test message (transient, not saved) |

**Request body** (POST/PUT):
```json
{
  "name": "My Slack",
  "provider": "shoutrrr",
  "config": "{\"url\":\"slack://...\"}",
  "enabled": true,
  "notify_offline": true,
  "notify_online": true
}
```

**Test endpoint**: accepts `provider` + `config` only; sends `"🧪 SwitchDeck test notification"` with a 15-second context; returns `{"status":"ok"}` or `{"error":"..."}`.

**Errors**: 409 on duplicate name, 404 on not found, 400 on invalid provider.

---

## Settings UI

Added to `internal/webui/templates/settings.html` and `internal/webui/static/js/app.js`.

### Channel list table

Below the Authentication section, a new `settings-section` titled **Notifications**:
- Table columns: Name | Provider | Offline | Online | Enabled | Actions
- Provider shown as a badge (text label)
- Offline/Online as read-only badges (green/grey)
- Enabled as an inline toggle
- Actions: Edit (opens modal pre-populated), Delete (with confirm)

### Add / Edit modal

Fields:
- **Name** (text, required)
- **Provider** selector: Shoutrrr / GreenAPI / WhatsApp Web
- Provider-conditional fields:
  - Shoutrrr: **URL** (placeholder: `slack://token@channel`)
  - GreenAPI: **Instance ID**, **Token**, **Recipient Phone**, **API URL** (optional)
  - WhatsApp Web: **Base URL**, **Recipient Phone**, **Username** (opt), **Password** (opt)
- **Notify when offline** toggle (default on)
- **Notify when back online** toggle (default on)
- **Enabled** toggle (default on)
- **Send Test** button — `POST /api/v1/notifications/test`; shows inline "✓ Sent" or error
- Save / Cancel

Password/token fields are `type="password"`. Config is never returned masked — it's always the full decrypted value so the Edit form can pre-populate correctly (credentials stay in the browser's own session scope).

---

## Files Changed / Created

| File | Change |
|---|---|
| `internal/notification/channel.go` | New — model + config structs + constants |
| `internal/notification/store.go` | New — CRUD + ListEnabled |
| `internal/notification/sender.go` | New — Sender interface + factory |
| `internal/notification/service.go` | New — NotifyPingChange fan-out |
| `internal/notification/store_test.go` | New — CRUD + ListEnabled tests |
| `internal/notification/sender_test.go` | New — sender unit tests |
| `internal/manager/worker.go` | Add `pingChangeFn`, `pingWasDown`, transition detection |
| `internal/manager/manager.go` | Accept `*notification.Service`, wire pingChangeFn |
| `internal/api/handlers/notifications.go` | New — CRUD + test handlers |
| `internal/api/handlers/handlers.go` | Add `NotificationStore` dependency |
| `internal/server/server.go` | Mount notification routes |
| `internal/store/schema.go` | Add `notification_channels` table |
| `cmd/switchdeck/main.go` | Wire notification service into manager + handlers |
| `internal/webui/templates/settings.html` | Add Notifications section + modal |
| `internal/webui/static/js/app.js` | Add notification channel list + form JS |
| `go.mod` / `go.sum` | Add `containrrr/shoutrrr` |

---

## What this does NOT include

- Per-switch notification overrides (all channels apply to all switches).
- Rate limiting / de-duplication (a switch that flaps rapidly will send many messages).
- Historical notification log.
- Notification on collection failure (out of scope — events are ping-based only).
