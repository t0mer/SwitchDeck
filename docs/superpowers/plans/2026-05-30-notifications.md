# Notifications Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a notification system that alerts configured channels (Shoutrrr, GreenAPI, WhatsApp Web) when a switch goes offline (2 consecutive ping failures) or comes back online.

**Architecture:** A new `internal/notification/` package owns the data model, SQLite CRUD, provider senders, and the fan-out service. The worker gains a `pingChangeFn` callback that fires on offline↔online transitions; the Manager wires it to `notification.Service.NotifyPingChange`. REST handlers and the Settings UI round out the feature.

**Tech Stack:** Go stdlib + `github.com/containrrr/shoutrrr` (Shoutrrr provider), raw `net/http` for GreenAPI and WhatsApp Web, `modernc.org/sqlite` (existing), AES-256-GCM via `internal/store.Encrypt/Decrypt` (existing).

---

## File map

| File | Action | Purpose |
|---|---|---|
| `internal/notification/channel.go` | Create | Channel model, config structs, provider constants |
| `internal/notification/store.go` | Create | CRUD + ListEnabled |
| `internal/notification/store_test.go` | Create | CRUD + ListEnabled tests |
| `internal/notification/sender.go` | Create | Sender interface + NewSender factory |
| `internal/notification/sender_test.go` | Create | Sender unit tests |
| `internal/notification/service.go` | Create | NotifyPingChange fan-out |
| `internal/manager/worker.go` | Modify | Add pingChangeFn, pingWasDown, transition detection |
| `internal/manager/export_test.go` | Modify | Export SetPingChangeFn helper |
| `internal/manager/worker_test.go` | Modify | Tests for transition detection |
| `internal/manager/manager.go` | Modify | Accept *notification.Service, wire pingChangeFn |
| `internal/store/schema.go` | Modify | Add notification_channels table |
| `internal/api/handlers/handlers.go` | Modify | Add NotifStore field |
| `internal/api/handlers/notifications.go` | Create | CRUD + test handlers |
| `internal/server/server.go` | Modify | Mount notification routes |
| `cmd/switchdeck/main.go` | Modify | Wire notification service |
| `internal/webui/templates/settings.html` | Modify | Add Notifications section + modal |
| `internal/webui/static/js/app.js` | Modify | Notification channel list + form JS |
| `go.mod` / `go.sum` | Modify | Add containrrr/shoutrrr |

---

### Task 1: Add shoutrrr dependency and create channel model

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/notification/channel.go`

- [ ] **Step 1: Add shoutrrr dependency**

```bash
cd /opt/dev/SwitchDeck
go get github.com/containrrr/shoutrrr@latest
```

Expected: `go.mod` and `go.sum` updated, no errors.

- [ ] **Step 2: Create `internal/notification/channel.go`**

```go
package notification

import "time"

const (
	ProviderShoutrrr   = "shoutrrr"
	ProviderGreenAPI   = "greenapi"
	ProviderWhatsApp   = "whatsapp_web"
)

// Channel is a configured notification destination.
type Channel struct {
	ID            string
	Name          string
	Provider      string
	Config        string // decrypted JSON, plaintext in memory only
	Enabled       bool
	NotifyOffline bool
	NotifyOnline  bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ShoutrrrConfig holds the single URL for a Shoutrrr channel.
type ShoutrrrConfig struct {
	URL string `json:"url"`
}

// GreenAPIConfig holds credentials for the GreenAPI WhatsApp provider.
type GreenAPIConfig struct {
	InstanceID string `json:"instance_id"`
	Token      string `json:"token"`
	Recipient  string `json:"recipient"`
	APIURL     string `json:"api_url,omitempty"` // defaults to https://api.green-api.com
}

// WhatsAppConfig holds connection details for a self-hosted WhatsApp Web instance.
type WhatsAppConfig struct {
	BaseURL   string `json:"base_url"`
	Recipient string `json:"recipient"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
}
```

- [ ] **Step 3: Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/notification/channel.go
git commit -m "feat(notifications): add shoutrrr dep and channel model"
```

---

### Task 2: Add notification_channels table to schema

**Files:**
- Modify: `internal/store/schema.go`

- [ ] **Step 1: Append table to schema constant**

In `internal/store/schema.go`, add to the `schema` const string before the closing backtick:

```sql

CREATE TABLE IF NOT EXISTS notification_channels (
	id             TEXT PRIMARY KEY,
	name           TEXT NOT NULL UNIQUE,
	provider       TEXT NOT NULL,
	config_enc     BLOB NOT NULL,
	enabled        INTEGER NOT NULL DEFAULT 1,
	notify_offline INTEGER NOT NULL DEFAULT 1,
	notify_online  INTEGER NOT NULL DEFAULT 1,
	created_at     INTEGER NOT NULL,
	updated_at     INTEGER NOT NULL
);
```

- [ ] **Step 2: Build and run existing tests**

```bash
go build ./... && go test ./internal/store/...
```

Expected: all pass (schema is applied via `IF NOT EXISTS`, no migration needed).

- [ ] **Step 3: Commit**

```bash
git add internal/store/schema.go
git commit -m "feat(notifications): add notification_channels table"
```

---

### Task 3: Implement notification store with TDD

**Files:**
- Create: `internal/notification/store.go`
- Create: `internal/notification/store_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/notification/store_test.go`:

```go
package notification_test

import (
	"context"
	"os"
	"testing"

	"github.com/t0mer/SwitchDeck/internal/notification"
	"github.com/t0mer/SwitchDeck/internal/store"
)

func openTestStore(t *testing.T) (*store.SQLiteStore, []byte, func()) {
	t.Helper()
	f, err := os.CreateTemp("", "notif-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	st, err := store.Open(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	key, err := st.EncryptionKey()
	if err != nil {
		t.Fatal(err)
	}
	return st, key, func() {
		st.Close()
		os.Remove(f.Name())
	}
}

func TestChannelCRUD(t *testing.T) {
	st, key, cleanup := openTestStore(t)
	defer cleanup()
	ns := notification.NewStore(st.DB(), key)
	ctx := context.Background()

	ch := notification.Channel{
		Name:          "test-slack",
		Provider:      notification.ProviderShoutrrr,
		Config:        `{"url":"slack://token@channel"}`,
		Enabled:       true,
		NotifyOffline: true,
		NotifyOnline:  false,
	}
	created, err := ns.Create(ctx, ch)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty ID")
	}

	got, err := ns.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Config != ch.Config {
		t.Errorf("Config: got %q, want %q", got.Config, ch.Config)
	}

	created.NotifyOnline = true
	if err := ns.Update(ctx, *created); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got2, _ := ns.Get(ctx, created.ID)
	if !got2.NotifyOnline {
		t.Error("expected NotifyOnline=true after update")
	}

	if err := ns.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := ns.Get(ctx, created.ID); err == nil {
		t.Error("expected error after delete")
	}
}

func TestListEnabled(t *testing.T) {
	st, key, cleanup := openTestStore(t)
	defer cleanup()
	ns := notification.NewStore(st.DB(), key)
	ctx := context.Background()

	// channel that wants offline only
	ns.Create(ctx, notification.Channel{
		Name: "offline-only", Provider: notification.ProviderShoutrrr,
		Config: `{"url":"slack://a@b"}`, Enabled: true,
		NotifyOffline: true, NotifyOnline: false,
	})
	// channel that wants online only
	ns.Create(ctx, notification.Channel{
		Name: "online-only", Provider: notification.ProviderShoutrrr,
		Config: `{"url":"slack://c@d"}`, Enabled: true,
		NotifyOffline: false, NotifyOnline: true,
	})
	// disabled channel
	ns.Create(ctx, notification.Channel{
		Name: "disabled", Provider: notification.ProviderShoutrrr,
		Config: `{"url":"slack://e@f"}`, Enabled: false,
		NotifyOffline: true, NotifyOnline: true,
	})

	offline, err := ns.ListEnabled(ctx, true, false)
	if err != nil {
		t.Fatalf("ListEnabled offline: %v", err)
	}
	if len(offline) != 1 || offline[0].Name != "offline-only" {
		t.Errorf("offline list: got %v", offline)
	}

	online, _ := ns.ListEnabled(ctx, false, true)
	if len(online) != 1 || online[0].Name != "online-only" {
		t.Errorf("online list: got %v", online)
	}
}

func TestDuplicateName(t *testing.T) {
	st, key, cleanup := openTestStore(t)
	defer cleanup()
	ns := notification.NewStore(st.DB(), key)
	ctx := context.Background()

	ch := notification.Channel{Name: "dup", Provider: notification.ProviderShoutrrr,
		Config: `{"url":"slack://x@y"}`, Enabled: true, NotifyOffline: true}
	ns.Create(ctx, ch)
	_, err := ns.Create(ctx, ch)
	if err == nil {
		t.Error("expected error on duplicate name")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/notification/... -v 2>&1 | head -20
```

Expected: compile error — `notification.NewStore`, `notification.Store` not defined.

- [ ] **Step 3: Create `internal/notification/store.go`**

```go
package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/t0mer/SwitchDeck/internal/store"
)

// Store persists notification channels.
type Store struct {
	db     *sql.DB
	encKey []byte
}

// NewStore creates a Store backed by the given database and encryption key.
func NewStore(db *sql.DB, encKey []byte) *Store {
	return &Store{db: db, encKey: encKey}
}

func (s *Store) Create(ctx context.Context, ch Channel) (*Channel, error) {
	enc, err := encryptConfig(s.encKey, ch.Config)
	if err != nil {
		return nil, err
	}
	ch.ID = uuid.New().String()
	now := time.Now().Unix()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO notification_channels
			(id, name, provider, config_enc, enabled, notify_offline, notify_online, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		ch.ID, ch.Name, ch.Provider, enc,
		boolInt(ch.Enabled), boolInt(ch.NotifyOffline), boolInt(ch.NotifyOnline),
		now, now)
	if err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}
	ch.CreatedAt = time.Unix(now, 0)
	ch.UpdatedAt = ch.CreatedAt
	return &ch, nil
}

func (s *Store) Get(ctx context.Context, id string) (*Channel, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, provider, config_enc, enabled, notify_offline, notify_online, created_at, updated_at
		FROM notification_channels WHERE id = ?`, id)
	return scanChannel(row, s.encKey)
}

func (s *Store) List(ctx context.Context) ([]Channel, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, provider, config_enc, enabled, notify_offline, notify_online, created_at, updated_at
		FROM notification_channels ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		ch, err := scanChannel(rows, s.encKey)
		if err != nil {
			return nil, err
		}
		out = append(out, *ch)
	}
	return out, rows.Err()
}

func (s *Store) Update(ctx context.Context, ch Channel) error {
	now := time.Now().Unix()
	if ch.Config != "" {
		enc, err := encryptConfig(s.encKey, ch.Config)
		if err != nil {
			return err
		}
		_, err = s.db.ExecContext(ctx, `
			UPDATE notification_channels SET
				name=?, provider=?, config_enc=?, enabled=?,
				notify_offline=?, notify_online=?, updated_at=?
			WHERE id=?`,
			ch.Name, ch.Provider, enc,
			boolInt(ch.Enabled), boolInt(ch.NotifyOffline), boolInt(ch.NotifyOnline),
			now, ch.ID)
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE notification_channels SET
			name=?, provider=?, enabled=?,
			notify_offline=?, notify_online=?, updated_at=?
		WHERE id=?`,
		ch.Name, ch.Provider,
		boolInt(ch.Enabled), boolInt(ch.NotifyOffline), boolInt(ch.NotifyOnline),
		now, ch.ID)
	return err
}

func (s *Store) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM notification_channels WHERE id=?`, id)
	return err
}

// ListEnabled returns channels where enabled=1 and the relevant event flag is set.
// Pass offline=true to get channels for offline events, online=true for online events.
func (s *Store) ListEnabled(ctx context.Context, offline, online bool) ([]Channel, error) {
	col := "notify_online"
	if offline {
		col = "notify_offline"
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, provider, config_enc, enabled, notify_offline, notify_online, created_at, updated_at
		FROM notification_channels WHERE enabled=1 AND `+col+`=1 ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Channel
	for rows.Next() {
		ch, err := scanChannel(rows, s.encKey)
		if err != nil {
			return nil, err
		}
		out = append(out, *ch)
	}
	return out, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanChannel(row scanner, encKey []byte) (*Channel, error) {
	var ch Channel
	var enc []byte
	var enabled, offline, online int
	var ca, ua int64
	err := row.Scan(&ch.ID, &ch.Name, &ch.Provider, &enc,
		&enabled, &offline, &online, &ca, &ua)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("channel not found")
	}
	if err != nil {
		return nil, err
	}
	ch.Enabled = enabled == 1
	ch.NotifyOffline = offline == 1
	ch.NotifyOnline = online == 1
	ch.CreatedAt = time.Unix(ca, 0)
	ch.UpdatedAt = time.Unix(ua, 0)
	plain, err := store.Decrypt(encKey, enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt config: %w", err)
	}
	ch.Config = string(plain)
	return &ch, nil
}

func encryptConfig(key []byte, config string) ([]byte, error) {
	return store.Encrypt(key, []byte(config))
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// DB returns the underlying *sql.DB so callers can pass it to NewStore.
func (s *Store) DB() *sql.DB { return s.db }
```

We also need to expose `DB()` on `SQLiteStore` in `internal/store/store.go`. Add at the end of that file:

```go
// DB returns the underlying *sql.DB for use by sub-stores.
func (s *SQLiteStore) DB() *sql.DB { return s.db }
```

In `internal/store/store.go`, add `DB() *sql.DB` to the `Store` interface (the list of method signatures starting with `EncryptionKey()`):

```go
DB() *sql.DB
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/notification/... -v -run "TestChannel|TestList|TestDuplicate"
```

Expected: all three tests PASS.

- [ ] **Step 5: Run full suite**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/notification/store.go internal/notification/store_test.go internal/store/store.go
git commit -m "feat(notifications): implement notification store with CRUD and ListEnabled"
```

---

### Task 4: Implement senders with TDD

**Files:**
- Create: `internal/notification/sender.go`
- Create: `internal/notification/sender_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/notification/sender_test.go`:

```go
package notification_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/t0mer/SwitchDeck/internal/notification"
)

func TestGreenAPISender(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := notification.GreenAPIConfig{
		InstanceID: "inst1",
		Token:      "tok1",
		Recipient:  "972501234567",
		APIURL:     srv.URL,
	}
	cfgJSON, _ := json.Marshal(cfg)
	ch := &notification.Channel{
		Provider: notification.ProviderGreenAPI,
		Config:   string(cfgJSON),
	}
	sender, err := notification.NewSender(ch)
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}
	if err := sender.Send(context.Background(), "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotBody["chatId"] != "972501234567@c.us" {
		t.Errorf("chatId: got %q, want 972501234567@c.us", gotBody["chatId"])
	}
	if gotBody["message"] != "hello" {
		t.Errorf("message: got %q, want hello", gotBody["message"])
	}
}

func TestGreenAPIAppendsAtCUs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Recipient already has @c.us — must not double-append.
	cfg := notification.GreenAPIConfig{
		InstanceID: "i", Token: "t",
		Recipient: "972501234567@c.us", APIURL: srv.URL,
	}
	cfgJSON, _ := json.Marshal(cfg)
	ch := &notification.Channel{Provider: notification.ProviderGreenAPI, Config: string(cfgJSON)}
	sender, _ := notification.NewSender(ch)
	if err := sender.Send(context.Background(), "x"); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestWhatsAppSender(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := notification.WhatsAppConfig{BaseURL: srv.URL, Recipient: "972501234567"}
	cfgJSON, _ := json.Marshal(cfg)
	ch := &notification.Channel{Provider: notification.ProviderWhatsApp, Config: string(cfgJSON)}
	sender, err := notification.NewSender(ch)
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}
	if err := sender.Send(context.Background(), "world"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotBody["message"] != "world" {
		t.Errorf("message: got %q, want world", gotBody["message"])
	}
}

func TestUnknownProvider(t *testing.T) {
	ch := &notification.Channel{Provider: "unknown", Config: "{}"}
	_, err := notification.NewSender(ch)
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/notification/... -run "TestGreen|TestWhatsApp|TestUnknown" -v 2>&1 | head -10
```

Expected: compile error — `notification.NewSender` not defined.

- [ ] **Step 3: Create `internal/notification/sender.go`**

```go
package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/containrrr/shoutrrr"
)

// Sender sends a notification message to a single channel.
type Sender interface {
	Send(ctx context.Context, message string) error
}

// NewSender constructs a Sender for the given channel's provider.
func NewSender(ch *Channel) (Sender, error) {
	switch ch.Provider {
	case ProviderShoutrrr:
		var cfg ShoutrrrConfig
		if err := json.Unmarshal([]byte(ch.Config), &cfg); err != nil {
			return nil, fmt.Errorf("shoutrrr config: %w", err)
		}
		return &shoutrrrSender{url: strings.TrimSpace(cfg.URL)}, nil
	case ProviderGreenAPI:
		var cfg GreenAPIConfig
		if err := json.Unmarshal([]byte(ch.Config), &cfg); err != nil {
			return nil, fmt.Errorf("greenapi config: %w", err)
		}
		apiURL := strings.TrimSpace(cfg.APIURL)
		if apiURL == "" {
			apiURL = "https://api.green-api.com"
		}
		return &greenAPISender{
			instanceID: strings.TrimSpace(cfg.InstanceID),
			token:      strings.TrimSpace(cfg.Token),
			recipient:  strings.TrimSpace(cfg.Recipient),
			apiURL:     apiURL,
		}, nil
	case ProviderWhatsApp:
		var cfg WhatsAppConfig
		if err := json.Unmarshal([]byte(ch.Config), &cfg); err != nil {
			return nil, fmt.Errorf("whatsapp config: %w", err)
		}
		return &whatsappSender{
			baseURL:   strings.TrimSpace(cfg.BaseURL),
			recipient: strings.TrimSpace(cfg.Recipient),
			username:  cfg.Username,
			password:  cfg.Password,
		}, nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", ch.Provider)
	}
}

// ── Shoutrrr ──────────────────────────────────────────────────────────────

type shoutrrrSender struct{ url string }

func (s *shoutrrrSender) Send(_ context.Context, message string) error {
	return shoutrrr.Send(s.url, message)
}

// ── GreenAPI ──────────────────────────────────────────────────────────────

type greenAPISender struct {
	instanceID, token, recipient, apiURL string
}

func (s *greenAPISender) Send(ctx context.Context, message string) error {
	chatID := s.recipient
	if !strings.Contains(chatID, "@") {
		chatID += "@c.us"
	}
	endpoint := fmt.Sprintf("%s/waInstance%s/sendMessage/%s", s.apiURL, s.instanceID, s.token)
	body, _ := json.Marshal(map[string]string{"chatId": chatID, "message": message})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("greenapi: unexpected status %d", resp.StatusCode)
	}
	return nil
}

// ── WhatsApp Web ──────────────────────────────────────────────────────────

type whatsappSender struct {
	baseURL, recipient, username, password string
}

func (s *whatsappSender) Send(ctx context.Context, message string) error {
	body, _ := json.Marshal(map[string]string{"phone": s.recipient, "message": message})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.baseURL+"/send/message", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.username != "" {
		req.SetBasicAuth(s.username, s.password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("whatsapp: unexpected status %d", resp.StatusCode)
	}
	return nil
}
```

- [ ] **Step 4: Run sender tests**

```bash
go test ./internal/notification/... -run "TestGreen|TestWhatsApp|TestUnknown" -v
```

Expected: all four tests PASS.

- [ ] **Step 5: Run full suite**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/notification/sender.go internal/notification/sender_test.go
git commit -m "feat(notifications): implement three-provider sender (shoutrrr, greenapi, whatsapp)"
```

---

### Task 5: Implement notification service

**Files:**
- Create: `internal/notification/service.go`

- [ ] **Step 1: Create `internal/notification/service.go`**

```go
package notification

import (
	"context"
	"fmt"
	"log"
	"time"
)

// EventStore is the subset of Store methods used by Service.
type EventStore interface {
	ListEnabled(ctx context.Context, offline, online bool) ([]Channel, error)
}

// Service fans out offline/online notifications to all relevant channels.
type Service struct {
	store EventStore
}

// NewService creates a Service backed by the given store.
func NewService(s EventStore) *Service {
	return &Service{store: s}
}

// NotifyPingChange sends a notification when a switch goes offline or comes back online.
// switchID is reserved for future per-switch filtering; name and ip are used in the message.
// Sending is best-effort: errors are logged and never returned.
func (svc *Service) NotifyPingChange(switchID, name, ip string, online bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	channels, err := svc.store.ListEnabled(ctx, !online, online)
	if err != nil {
		log.Printf("notifications: list channels: %v", err)
		return
	}
	if len(channels) == 0 {
		return
	}

	msg := buildMessage(name, ip, online)
	for _, ch := range channels {
		ch := ch
		sender, err := NewSender(&ch)
		if err != nil {
			log.Printf("notifications[%s]: build sender: %v", ch.Name, err)
			continue
		}
		if err := sender.Send(ctx, msg); err != nil {
			log.Printf("notifications[%s]: send: %v", ch.Name, err)
		}
	}
}

func buildMessage(name, ip string, online bool) string {
	if online {
		return fmt.Sprintf("🟢 Switch Online: %s (%s)\nSwitch is reachable again.", name, ip)
	}
	return fmt.Sprintf("🔴 Switch Offline: %s (%s)\nTwo consecutive ping failures — switch may be unreachable.", name, ip)
}
```

- [ ] **Step 2: Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/notification/service.go
git commit -m "feat(notifications): implement NotifyPingChange fan-out service"
```

---

### Task 6: Add ping transition detection to worker

**Files:**
- Modify: `internal/manager/worker.go`
- Modify: `internal/manager/export_test.go`
- Modify: `internal/manager/worker_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/manager/worker_test.go`:

```go
func TestPingTransitionCallback(t *testing.T) {
	cfg := models.SwitchConfig{
		ID: "cb-test", IP: "127.0.0.1",
		Username: "u", Password: "p",
		PollStatsSecs: 60, PollConfigSecs: 300,
	}
	w := manager.NewTestWorker(cfg, "19999") // nothing listening

	var events []bool
	manager.SetPingChangeFn(w, func(_ string, online bool) {
		events = append(events, online)
	})

	ctx := context.Background()

	// First failure — not yet at threshold, no transition yet
	w.DoPing(ctx)
	if len(events) != 0 {
		t.Errorf("expected 0 events after 1 failure, got %d", len(events))
	}

	// Second failure — crosses threshold, fires offline (false)
	w.DoPing(ctx)
	if len(events) != 1 || events[0] != false {
		t.Errorf("expected offline event after 2 failures, got %v", events)
	}

	// Third failure — already offline, no new event
	w.DoPing(ctx)
	if len(events) != 1 {
		t.Errorf("expected no new event on 3rd failure, got %d events", len(events))
	}

	// Recovery: start a listener and use a new worker pre-seeded as offline
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	w2 := manager.NewTestWorker(cfg, port)
	manager.SetWorkerOffline(w2)

	var events2 []bool
	manager.SetPingChangeFn(w2, func(_ string, online bool) {
		events2 = append(events2, online)
	})
	w2.DoPing(ctx)
	if len(events2) != 1 || events2[0] != true {
		t.Errorf("expected online event on recovery, got %v", events2)
	}
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
go test ./internal/manager/... -run TestPingTransitionCallback -v 2>&1 | head -10
```

Expected: compile error — `manager.SetPingChangeFn` not defined.

- [ ] **Step 3: Add pingChangeFn and pingWasDown to worker struct**

In `internal/manager/worker.go`, add to the worker struct after the `pingPort string` field:

```go
pingChangeFn func(id string, online bool) // called on offline↔online transition
pingWasDown  bool                          // previous pingDown value; tracks transitions
```

Update `newWorker` to initialise `pingChangeFn` to a no-op:

```go
func newWorker(cfg models.SwitchConfig, client switchclient.Client, onSnap SnapshotFunc, onErr ErrorFunc) *worker {
	return &worker{
		cfg:          cfg,
		client:       client,
		onSnap:       onSnap,
		onErr:        onErr,
		stop:         make(chan struct{}),
		stopped:      make(chan struct{}),
		pingPort:     "80",
		pingChangeFn: func(string, bool) {},
	}
}
```

At the end of `doPing`, inside the `pingMu.Lock()` block, after updating `pingDown`, add transition detection. Replace the entire `doPing` method body:

```go
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
	} else {
		conn.Close()
		w.pingConsec = 0
		w.pingDown = false
	}
	// Fire transition callback when pingDown flips.
	if w.pingDown != w.pingWasDown {
		w.pingWasDown = w.pingDown
		fn := w.pingChangeFn
		go fn(w.cfg.ID, !w.pingDown) // online=true means came back up
	}
}
```

- [ ] **Step 4: Add SetPingChangeFn to export_test.go**

Append to `internal/manager/export_test.go`:

```go
// SetPingChangeFn sets the pingChangeFn on a worker for testing.
func SetPingChangeFn(w *TestWorker, fn func(id string, online bool)) {
	w.pingMu.Lock()
	w.pingChangeFn = fn
	w.pingMu.Unlock()
}
```

- [ ] **Step 5: Run transition test**

```bash
go test ./internal/manager/... -run TestPingTransitionCallback -v
```

Expected: PASS.

- [ ] **Step 6: Run full suite**

```bash
go test ./...
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/manager/worker.go internal/manager/export_test.go internal/manager/worker_test.go
git commit -m "feat(notifications): add pingChangeFn transition detection to worker"
```

---

### Task 7: Wire notification service into Manager

**Files:**
- Modify: `internal/manager/manager.go`

- [ ] **Step 1: Add NotificationService interface and notifSvc field to Manager**

In `internal/manager/manager.go`, add at the top of the file (after imports):

```go
// NotificationService is called when a switch transitions between online and offline.
type NotificationService interface {
	NotifyPingChange(switchID, name, ip string, online bool)
}
```

Add `notifSvc NotificationService` to the `Manager` struct (after `errFunc`).

Update `New` to accept an optional notification service via a setter:

```go
// SetNotificationService registers the service that receives ping change events.
// Must be called before LoadFromStore or Add.
func (m *Manager) SetNotificationService(svc NotificationService) {
	m.notifSvc = svc
}
```

- [ ] **Step 2: Wire pingChangeFn in Add()**

In `Manager.Add()`, after `w := newWorker(...)`, add:

```go
if m.notifSvc != nil {
	svc := m.notifSvc
	cfg := cfg
	w.pingChangeFn = func(id string, online bool) {
		svc.NotifyPingChange(id, cfg.Name, cfg.IP, online)
	}
}
```

- [ ] **Step 3: Build and run full suite**

```bash
go build ./... && go test ./...
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add internal/manager/manager.go
git commit -m "feat(notifications): wire notification service into Manager via pingChangeFn"
```

---

### Task 8: REST API handlers for notification channels

**Files:**
- Modify: `internal/api/handlers/handlers.go`
- Create: `internal/api/handlers/notifications.go`
- Modify: `internal/server/server.go`

- [ ] **Step 1: Add NotifStore to Handlers**

In `internal/api/handlers/handlers.go`, add `NotifStore` field and update `New`:

```go
package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/t0mer/SwitchDeck/internal/manager"
	"github.com/t0mer/SwitchDeck/internal/notification"
	"github.com/t0mer/SwitchDeck/internal/store"
)

// Handlers holds shared dependencies for all HTTP handlers.
type Handlers struct {
	Manager    *manager.Manager
	Store      store.Store
	EncKey     []byte
	NotifStore *notification.Store
}

// New creates a Handlers instance.
func New(mgr *manager.Manager, st store.Store, encKey []byte, ns *notification.Store) *Handlers {
	return &Handlers{Manager: mgr, Store: st, EncKey: encKey, NotifStore: ns}
}
```

- [ ] **Step 2: Create `internal/api/handlers/notifications.go`**

```go
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/t0mer/SwitchDeck/internal/notification"
)

type channelRequest struct {
	Name          string `json:"name"`
	Provider      string `json:"provider"`
	Config        string `json:"config"`
	Enabled       bool   `json:"enabled"`
	NotifyOffline bool   `json:"notify_offline"`
	NotifyOnline  bool   `json:"notify_online"`
}

func validateProvider(p string) bool {
	return p == notification.ProviderShoutrrr ||
		p == notification.ProviderGreenAPI ||
		p == notification.ProviderWhatsApp
}

// ListNotifications handles GET /api/v1/notifications.
func (h *Handlers) ListNotifications(w http.ResponseWriter, r *http.Request) {
	channels, err := h.NotifStore.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if channels == nil {
		channels = []notification.Channel{}
	}
	writeJSON(w, http.StatusOK, channels)
}

// CreateNotification handles POST /api/v1/notifications.
func (h *Handlers) CreateNotification(w http.ResponseWriter, r *http.Request) {
	var req channelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" || !validateProvider(req.Provider) || req.Config == "" {
		writeError(w, http.StatusBadRequest, "name, provider, and config are required; provider must be shoutrrr|greenapi|whatsapp_web")
		return
	}
	ch, err := h.NotifStore.Create(r.Context(), notification.Channel{
		Name: req.Name, Provider: req.Provider, Config: req.Config,
		Enabled: req.Enabled, NotifyOffline: req.NotifyOffline, NotifyOnline: req.NotifyOnline,
	})
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, ch)
}

// UpdateNotification handles PUT /api/v1/notifications/{id}.
func (h *Handlers) UpdateNotification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req channelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if !validateProvider(req.Provider) {
		writeError(w, http.StatusBadRequest, "invalid provider")
		return
	}
	if err := h.NotifStore.Update(r.Context(), notification.Channel{
		ID: id, Name: req.Name, Provider: req.Provider, Config: req.Config,
		Enabled: req.Enabled, NotifyOffline: req.NotifyOffline, NotifyOnline: req.NotifyOnline,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// DeleteNotification handles DELETE /api/v1/notifications/{id}.
func (h *Handlers) DeleteNotification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.NotifStore.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// TestNotification handles POST /api/v1/notifications/test.
func (h *Handlers) TestNotification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
		Config   string `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if !validateProvider(req.Provider) || req.Config == "" {
		writeError(w, http.StatusBadRequest, "provider and config required")
		return
	}
	ch := &notification.Channel{Provider: req.Provider, Config: req.Config}
	sender, err := notification.NewSender(ch)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid config: "+err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := sender.Send(ctx, "🧪 SwitchDeck test notification"); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
```

- [ ] **Step 3: Mount routes in `internal/server/server.go`**

In the `/api/v1` route group, add after the settings routes:

```go
// Notifications
r.Get("/notifications", h.ListNotifications)
r.Post("/notifications", h.CreateNotification)
r.Put("/notifications/{id}", h.UpdateNotification)
r.Delete("/notifications/{id}", h.DeleteNotification)
r.Post("/notifications/test", h.TestNotification)
```

- [ ] **Step 4: Build**

```bash
go build ./...
```

Expected: no errors. (Note: `main.go` will fail because `handlers.New` signature changed — fix in Task 9.)

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers/handlers.go internal/api/handlers/notifications.go internal/server/server.go
git commit -m "feat(notifications): add CRUD + test REST handlers and routes"
```

---

### Task 9: Wire everything in main.go

**Files:**
- Modify: `cmd/switchdeck/main.go`

- [ ] **Step 1: Update main.go**

Replace `cmd/switchdeck/main.go` with:

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	flag "github.com/spf13/pflag"

	"github.com/t0mer/SwitchDeck/internal/api/handlers"
	"github.com/t0mer/SwitchDeck/internal/config"
	"github.com/t0mer/SwitchDeck/internal/manager"
	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/internal/notification"
	"github.com/t0mer/SwitchDeck/internal/server"
	"github.com/t0mer/SwitchDeck/internal/store"
	"github.com/t0mer/SwitchDeck/internal/switchclient"
	"github.com/t0mer/SwitchDeck/internal/switchclient/tplink"
)

var version = "dev"

func main() {
	cfg := config.Default()

	flag.IntVar(&cfg.Port, "port", cfg.Port, "HTTP listening port")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level: debug, info, warning, error")
	flag.StringVar(&cfg.DataDir, "data", cfg.DataDir, "Data directory")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	cfg.DBPath = cfg.DataDir + "/switchdeck.db"

	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	st, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	encKey, err := st.EncryptionKey()
	if err != nil {
		log.Fatalf("encryption key: %v", err)
	}

	notifStore := notification.NewStore(st.DB(), encKey)
	notifSvc := notification.NewService(notifStore)

	clientFactory := func(insecure bool) switchclient.Client {
		return tplink.New(insecure)
	}

	mgr := manager.New(clientFactory)
	mgr.SetSnapshotHandler(func(snap *models.SwitchSnapshot, _ bool) {
		st.UpsertSnapshot(context.Background(), snap)
	})
	mgr.SetNotificationService(notifSvc)

	if err := mgr.LoadFromStore(context.Background(), st, encKey); err != nil {
		log.Fatalf("load switches: %v", err)
	}

	h := handlers.New(mgr, st, encKey, notifStore)
	srv := server.New(cfg, h, st)

	log.Printf("SwitchDeck %s listening on :%d", version, cfg.Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
```

- [ ] **Step 2: Fix handlers test — New() signature changed**

In `internal/api/handlers/switches_test.go`, find the call to `handlers.New(...)` and update it:

```go
// Find the existing line like:
h := handlers.New(mgr, st, encKey)
// Replace with:
h := handlers.New(mgr, st, encKey, nil)
```

- [ ] **Step 3: Build and run full suite**

```bash
go build ./... && go test ./...
```

Expected: all pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/switchdeck/main.go internal/api/handlers/switches_test.go
git commit -m "feat(notifications): wire notification store and service into main.go"
```

---

### Task 10: Settings UI — Notifications section

**Files:**
- Modify: `internal/webui/templates/settings.html`
- Modify: `internal/webui/static/js/app.js`
- Modify: `internal/webui/static/css/app.css`

- [ ] **Step 1: Add Notifications section to settings.html**

In `internal/webui/templates/settings.html`, add after the closing `</form>` tag and before `{{end}}`:

```html
<!-- Notifications section -->
<div class="settings-section mt-4" style="max-width:700px">
  <div class="settings-section-header" style="display:flex;align-items:center;justify-content:space-between">
    <span>Notifications</span>
    <button type="button" class="btn btn-primary btn-sm" id="btn-add-notif">+ Add Channel</button>
  </div>
  <div class="settings-section-body" style="padding:0">
    <div id="notif-list"></div>
  </div>
</div>

<!-- Add / Edit notification modal -->
<div id="notif-modal-backdrop" class="modal-backdrop hidden">
  <div class="modal-box" style="max-width:520px">
    <div class="modal-header">
      <span class="modal-title" id="notif-modal-title">Add Channel</span>
      <button class="modal-close" id="btn-close-notif-modal">&times;</button>
    </div>
    <form id="notif-form">
      <div class="modal-body">
        <div class="form-group">
          <label class="form-label" for="nf-name">Name</label>
          <input id="nf-name" class="form-control" type="text" placeholder="My Slack" required>
        </div>
        <div class="form-group">
          <label class="form-label" for="nf-provider">Provider</label>
          <select id="nf-provider" class="form-control">
            <option value="shoutrrr">Shoutrrr (Slack, Discord, Telegram, SMTP…)</option>
            <option value="greenapi">GreenAPI (WhatsApp Cloud)</option>
            <option value="whatsapp_web">WhatsApp Web (self-hosted)</option>
          </select>
        </div>

        <!-- Shoutrrr fields -->
        <div id="nf-fields-shoutrrr">
          <div class="form-group">
            <label class="form-label" for="nf-url">URL</label>
            <input id="nf-url" class="form-control" type="text" placeholder="slack://token@channel">
          </div>
        </div>

        <!-- GreenAPI fields -->
        <div id="nf-fields-greenapi" class="hidden">
          <div class="form-row">
            <div class="form-group">
              <label class="form-label" for="nf-instance-id">Instance ID</label>
              <input id="nf-instance-id" class="form-control" type="text">
            </div>
            <div class="form-group">
              <label class="form-label" for="nf-ga-token">Token</label>
              <input id="nf-ga-token" class="form-control" type="password">
            </div>
          </div>
          <div class="form-row">
            <div class="form-group">
              <label class="form-label" for="nf-recipient">Recipient Phone</label>
              <input id="nf-recipient" class="form-control" type="text" placeholder="972501234567">
            </div>
            <div class="form-group">
              <label class="form-label" for="nf-api-url">API URL (optional)</label>
              <input id="nf-api-url" class="form-control" type="text" placeholder="https://api.green-api.com">
            </div>
          </div>
        </div>

        <!-- WhatsApp Web fields -->
        <div id="nf-fields-whatsapp_web" class="hidden">
          <div class="form-group">
            <label class="form-label" for="nf-base-url">Base URL</label>
            <input id="nf-base-url" class="form-control" type="text" placeholder="http://localhost:3000">
          </div>
          <div class="form-row">
            <div class="form-group">
              <label class="form-label" for="nf-wa-recipient">Recipient Phone</label>
              <input id="nf-wa-recipient" class="form-control" type="text" placeholder="972501234567">
            </div>
            <div class="form-group">
              <label class="form-label" for="nf-wa-username">Username (optional)</label>
              <input id="nf-wa-username" class="form-control" type="text">
            </div>
          </div>
          <div class="form-group">
            <label class="form-label" for="nf-wa-password">Password (optional)</label>
            <input id="nf-wa-password" class="form-control" type="password">
          </div>
        </div>

        <div class="form-group">
          <label class="toggle-wrap">
            <label class="toggle"><input id="nf-notify-offline" type="checkbox" checked><span class="toggle-track"></span></label>
            Notify when switch goes offline
          </label>
        </div>
        <div class="form-group">
          <label class="toggle-wrap">
            <label class="toggle"><input id="nf-notify-online" type="checkbox" checked><span class="toggle-track"></span></label>
            Notify when switch comes back online
          </label>
        </div>
        <div class="form-group">
          <label class="toggle-wrap">
            <label class="toggle"><input id="nf-enabled" type="checkbox" checked><span class="toggle-track"></span></label>
            Enabled
          </label>
        </div>
        <div id="nf-test-result" class="hidden" style="margin-top:8px"></div>
      </div>
      <div class="modal-footer">
        <button type="button" class="btn btn-secondary btn-sm" id="btn-test-notif">Send Test</button>
        <button type="button" class="btn btn-secondary" id="btn-cancel-notif-modal">Cancel</button>
        <button type="submit" class="btn btn-primary">Save</button>
      </div>
    </form>
  </div>
</div>
```

- [ ] **Step 2: Add CSS for `.mt-4`**

In `internal/webui/static/css/app.css`, in the utilities section, add:

```css
.mt-4 { margin-top: 16px; }
```

(It likely already exists — check and skip if so.)

- [ ] **Step 3: Add notification JS to app.js**

Append to `internal/webui/static/js/app.js`:

```js
// ══════════════════════════════════════════════════════════════════════════
// NOTIFICATIONS
// ══════════════════════════════════════════════════════════════════════════

const notifList = document.getElementById('notif-list');
if (notifList) initNotifications();

async function initNotifications() {
  await loadNotifications();
}

async function loadNotifications() {
  try {
    const channels = await apiGet('/notifications');
    renderNotifList(channels || []);
  } catch (e) {
    const errEl = el('p', 'alert alert-danger');
    errEl.style.margin = '12px';
    errEl.textContent = 'Failed to load: ' + e.message;
    notifList.textContent = '';
    notifList.appendChild(errEl);
  }
}

function renderNotifList(channels) {
  notifList.textContent = '';
  if (!channels.length) {
    const empty = el('p', 'text-muted', 'No notification channels configured.');
    empty.style.padding = '16px';
    notifList.appendChild(empty);
    return;
  }
  const wrapper = el('div', 'table-wrapper');
  const table = el('table');
  const thead = el('thead');
  const hr = el('tr');
  for (const h of ['Name', 'Provider', 'Offline', 'Online', 'Enabled', '']) {
    hr.appendChild(el('th', null, h));
  }
  thead.appendChild(hr); table.appendChild(thead);
  const tbody = el('tbody');
  for (const ch of channels) {
    const row = el('tr');
    row.appendChild(el('td', null, ch.name));
    row.appendChild(el('td', null, ch.provider));
    row.appendChild(el('td', null, ch.notify_offline ? '✓' : '—'));
    row.appendChild(el('td', null, ch.notify_online ? '✓' : '—'));
    row.appendChild(el('td', null, ch.enabled ? '✓' : '—'));
    const actions = el('td');
    const editBtn = el('button', 'btn btn-ghost btn-sm', 'Edit');
    editBtn.addEventListener('click', () => openEditNotifModal(ch));
    const delBtn = el('button', 'btn btn-danger btn-sm', 'Delete');
    delBtn.style.marginLeft = '6px';
    delBtn.addEventListener('click', () => deleteNotif(ch.id, ch.name));
    append(actions, editBtn, delBtn);
    row.appendChild(actions);
    tbody.appendChild(row);
  }
  table.appendChild(tbody);
  wrapper.appendChild(table);
  notifList.appendChild(wrapper);
}

// ── Provider field toggling ───────────────────────────────────────────────

document.getElementById('nf-provider')?.addEventListener('change', function () {
  switchNotifFields(this.value);
});

function switchNotifFields(provider) {
  ['shoutrrr', 'greenapi', 'whatsapp_web'].forEach(p => {
    const el2 = document.getElementById('nf-fields-' + p);
    if (el2) el2.classList.toggle('hidden', p !== provider);
  });
}

// ── Modal helpers ──────────────────────────────────────────────────────────

let editingNotifId = null;

function openAddNotifModal() {
  editingNotifId = null;
  setText('notif-modal-title', 'Add Channel');
  document.getElementById('notif-form').reset();
  switchNotifFields('shoutrrr');
  document.getElementById('nf-test-result').classList.add('hidden');
  document.getElementById('notif-modal-backdrop').classList.remove('hidden');
}

function openEditNotifModal(ch) {
  editingNotifId = ch.id;
  setText('notif-modal-title', 'Edit Channel');
  document.getElementById('nf-name').value = ch.name || '';
  document.getElementById('nf-provider').value = ch.provider || 'shoutrrr';
  switchNotifFields(ch.provider);
  // Populate provider-specific fields from config JSON
  try {
    const cfg = JSON.parse(ch.config || '{}');
    if (ch.provider === 'shoutrrr') {
      document.getElementById('nf-url').value = cfg.url || '';
    } else if (ch.provider === 'greenapi') {
      document.getElementById('nf-instance-id').value = cfg.instance_id || '';
      document.getElementById('nf-ga-token').value = cfg.token || '';
      document.getElementById('nf-recipient').value = cfg.recipient || '';
      document.getElementById('nf-api-url').value = cfg.api_url || '';
    } else if (ch.provider === 'whatsapp_web') {
      document.getElementById('nf-base-url').value = cfg.base_url || '';
      document.getElementById('nf-wa-recipient').value = cfg.recipient || '';
      document.getElementById('nf-wa-username').value = cfg.username || '';
      document.getElementById('nf-wa-password').value = cfg.password || '';
    }
  } catch {}
  document.getElementById('nf-notify-offline').checked = ch.notify_offline;
  document.getElementById('nf-notify-online').checked = ch.notify_online;
  document.getElementById('nf-enabled').checked = ch.enabled;
  document.getElementById('nf-test-result').classList.add('hidden');
  document.getElementById('notif-modal-backdrop').classList.remove('hidden');
}

function closeNotifModal() {
  document.getElementById('notif-modal-backdrop').classList.add('hidden');
  editingNotifId = null;
}

function buildNotifConfig(provider) {
  if (provider === 'shoutrrr') {
    return JSON.stringify({ url: document.getElementById('nf-url').value.trim() });
  }
  if (provider === 'greenapi') {
    return JSON.stringify({
      instance_id: document.getElementById('nf-instance-id').value.trim(),
      token:       document.getElementById('nf-ga-token').value.trim(),
      recipient:   document.getElementById('nf-recipient').value.trim(),
      api_url:     document.getElementById('nf-api-url').value.trim(),
    });
  }
  // whatsapp_web
  return JSON.stringify({
    base_url:  document.getElementById('nf-base-url').value.trim(),
    recipient: document.getElementById('nf-wa-recipient').value.trim(),
    username:  document.getElementById('nf-wa-username').value.trim(),
    password:  document.getElementById('nf-wa-password').value.trim(),
  });
}

// ── Event handlers ─────────────────────────────────────────────────────────

document.getElementById('btn-add-notif')?.addEventListener('click', openAddNotifModal);
document.getElementById('btn-close-notif-modal')?.addEventListener('click', closeNotifModal);
document.getElementById('btn-cancel-notif-modal')?.addEventListener('click', closeNotifModal);
document.getElementById('notif-modal-backdrop')?.addEventListener('click', e => {
  if (e.target === e.currentTarget) closeNotifModal();
});

document.getElementById('notif-form')?.addEventListener('submit', async e => {
  e.preventDefault();
  const btn = e.target.querySelector('[type=submit]');
  btn.disabled = true;
  const provider = document.getElementById('nf-provider').value;
  const body = {
    name:           document.getElementById('nf-name').value.trim(),
    provider,
    config:         buildNotifConfig(provider),
    notify_offline: document.getElementById('nf-notify-offline').checked,
    notify_online:  document.getElementById('nf-notify-online').checked,
    enabled:        document.getElementById('nf-enabled').checked,
  };
  try {
    if (editingNotifId) {
      await apiPut(`/notifications/${editingNotifId}`, body);
      toast('Channel updated');
    } else {
      await apiPost('/notifications', body);
      toast('Channel added');
    }
    closeNotifModal();
    await loadNotifications();
  } catch (err) {
    toast(err.message, 'danger');
  } finally {
    btn.disabled = false;
  }
});

document.getElementById('btn-test-notif')?.addEventListener('click', async function () {
  const provider = document.getElementById('nf-provider').value;
  const resultEl = document.getElementById('nf-test-result');
  this.disabled = true;
  this.textContent = 'Sending…';
  resultEl.className = 'hidden';
  try {
    await apiPost('/notifications/test', {
      provider,
      config: buildNotifConfig(provider),
    });
    resultEl.className = 'alert alert-success';
    resultEl.textContent = '✓ Test message sent successfully';
  } catch (err) {
    resultEl.className = 'alert alert-danger';
    resultEl.textContent = 'Error: ' + err.message;
  } finally {
    this.disabled = false;
    this.textContent = 'Send Test';
  }
});

async function deleteNotif(id, name) {
  if (!confirm(`Delete channel "${name}"?`)) return;
  try {
    await apiDel(`/notifications/${id}`);
    toast('Channel deleted');
    await loadNotifications();
  } catch (e) {
    toast('Delete failed: ' + e.message, 'danger');
  }
}
```

- [ ] **Step 4: Build**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/webui/templates/settings.html internal/webui/static/js/app.js internal/webui/static/css/app.css
git commit -m "feat(notifications): add Notifications settings UI with add/edit/delete/test"
```

---

### Task 11: Final build, test, push

- [ ] **Step 1: Full build and test**

```bash
go build ./... && go test ./...
```

Expected: BUILD OK, all tests pass.

- [ ] **Step 2: Push**

```bash
git push -u origin notifications
```
