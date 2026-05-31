# Authentication Design

Date: 2026-05-31
Scope: Username/password session auth for the web UI + enhanced API bearer token with rotation, expiry, and copy-once UX. CLI `--reset-password` flag.

---

## Goals

- Protect the web UI with a username + password login page and session cookie.
- Keep the existing bearer token for external / programmatic API access; add rotation and expiry.
- Allow the admin to reset credentials from the CLI without needing the UI.
- Auth is opt-in (toggled in Settings); when disabled the app is fully open as before.

---

## Storage

All state lives in the existing `settings` SQLite table. No schema changes.

| Key | Value type | Description |
|---|---|---|
| `auth_enabled` | `"true"` / `"false"` | Master on/off switch (existing) |
| `auth_username` | string | Admin username, plaintext |
| `auth_password_hash` | string | argon2id-encoded hash of the password |
| `session_secret` | string | 32 random bytes, base64 — generated on first use |
| `auth_token` | string | Bearer token, plaintext (existing; local SQLite is acceptable) |
| `auth_token_expiry` | string | Unix timestamp; `"0"` = never expires |

Password is **never** stored. `auth_token` is stored plaintext; it is shown exactly once in the UI (rotate modal) and never returned by any API endpoint thereafter.

---

## Password hashing

- Algorithm: **argon2id** via `golang.org/x/crypto/argon2`
- Parameters: memory 64 MiB, time 1, threads 4, key 32 bytes, salt 16 random bytes
- Encoded format: `$argon2id$v=19$m=65536,t=1,p=4$<salt-base64>$<hash-base64>`
- Comparison: constant-time via `subtle.ConstantTimeCompare`

---

## Session cookie

- Name: `sd_session`
- Value: `base64url(username|unixExpiry) + "." + base64url(HMAC-SHA256(secret, payload))`
- TTL: 24 hours
- Flags: `HttpOnly`, `SameSite=Lax`, `Path=/`
- `session_secret` is auto-generated (32 random bytes) on first login if not present and stored in the settings table. It persists across restarts; rotating it invalidates all existing sessions.

---

## New routes (always public — exempt from auth middleware)

| Method | Path | Description |
|---|---|---|
| `GET` | `/login` | Login page (HTML) |
| `POST` | `/api/v1/auth/login` | Authenticate; sets `sd_session` cookie on success |
| `POST` | `/api/v1/auth/logout` | Clears `sd_session` cookie |
| `GET` | `/api/v1/auth/session` | Returns `{auth_enabled, authenticated}` |

`POST /api/v1/auth/login` body: `{username, password}` JSON.
On success: `200 {"status":"ok"}`, sets cookie.
On failure: `401 {"error":"invalid credentials"}`.

---

## Middleware

Two modes, one implementation (`internal/api/middleware/auth.go` — replaces current bearer-only middleware):

### API mode (`/api/v1/*` except the three auth routes above)
- Auth disabled → pass through
- Auth enabled → accept **either**:
  - Valid `sd_session` cookie (HMAC verified, not expired)
  - Valid `Authorization: Bearer <token>` header (constant-time compare, expiry checked if non-zero)
- On failure: `401 {"error":"unauthorized"}`

### UI mode (`/`, `/switches/*`, `/settings`, `/login` is exempt)
- Auth disabled → pass through
- Auth enabled + valid session → pass through
- Auth enabled + no/invalid session → `302` redirect to `/login?next=<original-path>`

### Bootstrap rule
If `auth_enabled=true` but `auth_username` is empty (first-time setup), all requests pass through to let the user set credentials via the Settings page.

---

## Settings UI changes

The existing **Authentication** section is replaced by two sub-sections within a single settings card:

### Web Access
- **Username** input (pre-filled with current username)
- **New password** input (type=password)
- **Confirm password** input (type=password)
- Save button — argon2id-hashes the password, stores both
- Validation: passwords must match and be ≥ 8 characters

### API Token
- Token field: masked (`••••••••`) with a **Show / Copy** icon button
  - Clicking reveals the full token in the field and copies it to the clipboard
  - The token is returned by `GET /api/v1/settings` **only** immediately after rotation (one-time response field `token_plaintext`); subsequent GETs omit it
- **Expiry** selector: "Never" or a date picker → stored as Unix timestamp
- **Rotate** button → opens a modal:
  1. Generates a new 32-byte random token server-side
  2. Displays it in a read-only field with a **Copy** button
  3. "I've copied it" confirm button saves the new token and closes the modal
  4. Cancel leaves the old token unchanged
- `GET /api/v1/settings` response: `{auth_enabled, username_set, token_set, token_expiry}` — never returns the plaintext token except in the rotate response

---

## API: settings endpoint changes

`GET /api/v1/settings` returns:
```json
{
  "auth_enabled": true,
  "username_set": true,
  "token_set": true,
  "token_expiry": 0
}
```

`PUT /api/v1/settings` accepts (all optional):
```json
{
  "auth_enabled": true,
  "username": "admin",
  "password": "...",
  "token_expiry": 1800000000
}
```

`POST /api/v1/settings/rotate-token` returns (one-time):
```json
{
  "status": "ok",
  "token": "<32-byte-hex>"
}
```
The token is stored server-side immediately; if the client loses it, they must rotate again.

---

## `--reset-password` CLI flag

When `--reset-password` is passed:
1. Parse `--data` flag for the DB path (default `/data/switchdeck/switchdeck.db`)
2. Open SQLite DB
3. Prompt interactively using `golang.org/x/term`:
   ```
   New username: 
   New password: (hidden)
   Confirm password: (hidden)
   ```
4. Validate: passwords match, ≥ 8 characters
5. Hash with argon2id, write `auth_username` and `auth_password_hash` to settings table
6. Print `✓ Credentials updated. Restart the server to apply.`
7. Exit 0

If the DB file doesn't exist yet, print an error and exit 1.

---

## Login page (`/login`)

- New template `internal/webui/templates/login.html`
- Full-page centered card, no sidebar
- Username + password form, `POST /api/v1/auth/login` via JS fetch
- On success: `window.location.href = next || '/'`
- On failure: inline error badge "Invalid username or password"
- Reads `data-theme` from `<html>` on load (same dark/light logic as the rest of the app)
- Redirect parameter: `/login?next=/settings` — the auth middleware appends the original path

---

## Files changed / created

| File | Action |
|---|---|
| `internal/api/middleware/auth.go` | Rewrite — session cookie + bearer dual-mode |
| `internal/api/middleware/auth_test.go` | Rewrite tests |
| `internal/api/handlers/auth.go` | New — Login, Logout, Session handlers |
| `internal/api/handlers/settings.go` | Update — new fields, rotate-token endpoint |
| `internal/server/server.go` | Mount new auth routes, apply dual middleware |
| `cmd/switchdeck/main.go` | Add `--reset-password` branch |
| `internal/webui/handler.go` | Add `Login` handler for `GET /login` |
| `internal/webui/templates/login.html` | New — login page |
| `internal/webui/static/js/app.js` | Update settings UI (Web Access + Token sections) |
| `internal/webui/templates/settings.html` | Update auth section HTML |
| `go.mod` / `go.sum` | Add `golang.org/x/crypto` (argon2id) |

---

## What this does NOT include

- Multi-user support (single admin only)
- Password strength meter
- Account lockout after N failed attempts
- HTTPS enforcement (assumed to be handled by a reverse proxy)
