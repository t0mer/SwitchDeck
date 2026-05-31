// Package backup provides config export and import for switches, credentials,
// API tokens, and notification channels.
//
// The backup file is plain JSON. Switch passwords and notification channel
// credentials are included in plaintext so the file can be restored on a
// different server with a different encryption key. Store the file securely.
package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/internal/notification"
	"github.com/t0mer/SwitchDeck/internal/store"
)

const version = 1

// settingsKeys are the settings keys that travel with a backup.
// encryption_key and session_secret are server-specific and must never
// be included.
var settingsKeys = []string{
	"auth_enabled",
	"auth_username",
	"auth_password_hash",
}

// File is the top-level backup document.
type File struct {
	Version              int                  `json:"version"`
	CreatedAt            time.Time            `json:"created_at"`
	Settings             map[string]string    `json:"settings"`
	Switches             []models.SwitchConfig `json:"switches"`
	APITokens            []TokenRecord        `json:"api_tokens"`
	NotificationChannels []ChannelRecord      `json:"notification_channels"`
}

// TokenRecord is a token row as it appears in the backup.
// The hash is SHA-256 of the original plaintext, so restored tokens work
// unchanged on the new server.
type TokenRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	TokenHash string    `json:"token_hash"`
	Expiry    int64     `json:"expiry"`
	CreatedAt time.Time `json:"created_at"`
}

// ChannelRecord is a notification channel with its config in plaintext JSON.
type ChannelRecord struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Provider      string    `json:"provider"`
	Config        string    `json:"config"` // plaintext JSON (never encrypted at rest in the backup)
	Enabled       bool      `json:"enabled"`
	NotifyOffline bool      `json:"notify_offline"`
	NotifyOnline  bool      `json:"notify_online"`
	CreatedAt     time.Time `json:"created_at"`
}

// Export builds a backup file from the live database. Switch passwords and
// notification credentials are included in plaintext.
func Export(ctx context.Context, db *sql.DB, st store.Store, encKey []byte, ns *notification.Store) (*File, error) {
	f := &File{
		Version:   version,
		CreatedAt: time.Now().UTC(),
		Settings:  make(map[string]string),
	}

	// settings
	for _, key := range settingsKeys {
		val, err := st.GetSetting(ctx, key)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("read setting %s: %w", key, err)
		}
		if val != "" {
			f.Settings[key] = val
		}
	}

	// switches (passwords decrypted by ListSwitches)
	switches, err := st.ListSwitches(ctx, encKey)
	if err != nil {
		return nil, fmt.Errorf("list switches: %w", err)
	}
	f.Switches = switches

	// api tokens — query directly to include the hash
	rows, err := db.QueryContext(ctx,
		`SELECT id, name, token_hash, expiry, created_at FROM api_tokens ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list tokens: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t TokenRecord
		var ca int64
		if err := rows.Scan(&t.ID, &t.Name, &t.TokenHash, &t.Expiry, &ca); err != nil {
			return nil, fmt.Errorf("scan token: %w", err)
		}
		t.CreatedAt = time.Unix(ca, 0)
		f.APITokens = append(f.APITokens, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tokens: %w", err)
	}

	// notification channels (config decrypted by List)
	channels, err := ns.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	for _, ch := range channels {
		f.NotificationChannels = append(f.NotificationChannels, ChannelRecord{
			ID:            ch.ID,
			Name:          ch.Name,
			Provider:      ch.Provider,
			Config:        ch.Config,
			Enabled:       ch.Enabled,
			NotifyOffline: ch.NotifyOffline,
			NotifyOnline:  ch.NotifyOnline,
			CreatedAt:     ch.CreatedAt,
		})
	}

	return f, nil
}

// Restore wipes the existing switches, API tokens, and notification channels,
// then re-imports everything from f. Credentials are re-encrypted with encKey.
// Settings are written key-by-key so server-specific keys are never touched.
func Restore(ctx context.Context, db *sql.DB, st store.Store, encKey []byte, ns *notification.Store, f *File) error {
	if f.Version != version {
		return fmt.Errorf("unsupported backup version %d (expected %d)", f.Version, version)
	}

	// ── wipe existing data ────────────────────────────────────────────────
	if _, err := db.ExecContext(ctx, `DELETE FROM switches`); err != nil {
		return fmt.Errorf("clear switches: %w", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM api_tokens`); err != nil {
		return fmt.Errorf("clear api_tokens: %w", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM notification_channels`); err != nil {
		return fmt.Errorf("clear notification_channels: %w", err)
	}

	// ── restore settings ──────────────────────────────────────────────────
	for _, key := range settingsKeys {
		if val, ok := f.Settings[key]; ok {
			if err := st.SetSetting(ctx, key, val); err != nil {
				return fmt.Errorf("restore setting %s: %w", key, err)
			}
		}
	}

	// ── restore switches (re-encrypted with this server's key) ────────────
	for _, sw := range f.Switches {
		if err := st.AddSwitch(ctx, sw, encKey); err != nil {
			return fmt.Errorf("restore switch %s: %w", sw.Name, err)
		}
	}

	// ── restore API tokens (hash is portable — no re-hashing needed) ──────
	now := time.Now().Unix()
	for _, t := range f.APITokens {
		ca := t.CreatedAt.Unix()
		if ca == 0 {
			ca = now
		}
		_, err := db.ExecContext(ctx,
			`INSERT INTO api_tokens(id, name, token_hash, expiry, created_at) VALUES (?,?,?,?,?)`,
			t.ID, t.Name, t.TokenHash, t.Expiry, ca,
		)
		if err != nil {
			return fmt.Errorf("restore token %s: %w", t.Name, err)
		}
	}

	// ── restore notification channels (re-encrypted with this server's key)
	for _, rec := range f.NotificationChannels {
		ch := notification.Channel{
			ID:            rec.ID,
			Name:          rec.Name,
			Provider:      rec.Provider,
			Config:        rec.Config,
			Enabled:       rec.Enabled,
			NotifyOffline: rec.NotifyOffline,
			NotifyOnline:  rec.NotifyOnline,
			CreatedAt:     rec.CreatedAt,
		}
		if _, err := ns.Create(ctx, ch); err != nil {
			return fmt.Errorf("restore channel %s: %w", ch.Name, err)
		}
	}

	return nil
}

// Marshal encodes a backup file to JSON bytes.
func Marshal(f *File) ([]byte, error) {
	return json.MarshalIndent(f, "", "  ")
}

// Unmarshal decodes a backup file from JSON bytes.
func Unmarshal(data []byte) (*File, error) {
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return &f, nil
}
