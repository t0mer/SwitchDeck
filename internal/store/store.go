package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"github.com/t0mer/SwitchDeck/internal/models"
)

// ApiToken is a named bearer token for external API access.
type ApiToken struct {
	ID        string
	Name      string
	Expiry    int64 // unix timestamp; 0 = never
	CreatedAt time.Time
}

// Store persists switch inventory, snapshots, settings, and API tokens.
type Store interface {
	EncryptionKey() ([]byte, error)
	AddSwitch(ctx context.Context, cfg models.SwitchConfig, encKey []byte) error
	GetSwitch(ctx context.Context, id string, encKey []byte) (*models.SwitchConfig, error)
	ListSwitches(ctx context.Context, encKey []byte) ([]models.SwitchConfig, error)
	UpdateSwitch(ctx context.Context, cfg models.SwitchConfig, encKey []byte) error
	DeleteSwitch(ctx context.Context, id string) error
	UpsertSnapshot(ctx context.Context, snap *models.SwitchSnapshot) error
	LatestSnapshot(ctx context.Context, switchID string) (*models.SwitchSnapshot, error)
	GetSetting(ctx context.Context, key string) (string, error)
	SetSetting(ctx context.Context, key, value string) error
	// API token management
	CreateApiToken(ctx context.Context, name, plaintext string, expiry int64) (*ApiToken, error)
	ListApiTokens(ctx context.Context) ([]ApiToken, error)
	DeleteApiToken(ctx context.Context, id string) error
	MatchApiToken(ctx context.Context, plaintext string) (*ApiToken, error)
	Close() error
	DB() *sql.DB
}

// SQLiteStore is the SQLite-backed Store implementation.
type SQLiteStore struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database and applies the schema.
func Open(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path+"?_journal=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	st := &SQLiteStore{db: db}
	// Generate encryption key on first run
	if _, err := st.GetSetting(context.Background(), "encryption_key"); err != nil {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generate encryption key: %w", err)
		}
		encoded := base64.StdEncoding.EncodeToString(key)
		if err := st.SetSetting(context.Background(), "encryption_key", encoded); err != nil {
			return nil, fmt.Errorf("store encryption key: %w", err)
		}
	}
	return st, nil
}

// EncryptionKey returns the 32-byte AES key from settings.
func (s *SQLiteStore) EncryptionKey() ([]byte, error) {
	val, err := s.GetSetting(context.Background(), "encryption_key")
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(val)
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for use by sub-packages.
func (s *SQLiteStore) DB() *sql.DB { return s.db }

func (s *SQLiteStore) GetSetting(ctx context.Context, key string) (string, error) {
	var val string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("setting %q not found", key)
	}
	return val, err
}

func (s *SQLiteStore) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		key, value)
	return err
}

func (s *SQLiteStore) AddSwitch(ctx context.Context, cfg models.SwitchConfig, encKey []byte) error {
	enc, err := Encrypt(encKey, []byte(cfg.Password))
	if err != nil {
		return fmt.Errorf("encrypt password: %w", err)
	}
	t := nowUnix()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO switches(id,name,ip,username,password_enc,insecure_tls,enabled,
			poll_stats_secs,poll_config_secs,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		cfg.ID, cfg.Name, cfg.IP, cfg.Username, enc,
		boolInt(cfg.InsecureTLS), boolInt(cfg.Enabled),
		cfg.PollStatsSecs, cfg.PollConfigSecs, t, t)
	return err
}

func (s *SQLiteStore) GetSwitch(ctx context.Context, id string, encKey []byte) (*models.SwitchConfig, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id,name,ip,username,password_enc,insecure_tls,enabled,
			poll_stats_secs,poll_config_secs
		FROM switches WHERE id=?`, id)
	return scanSwitch(row, encKey)
}

func (s *SQLiteStore) ListSwitches(ctx context.Context, encKey []byte) ([]models.SwitchConfig, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,name,ip,username,password_enc,insecure_tls,enabled,
			poll_stats_secs,poll_config_secs
		FROM switches ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []models.SwitchConfig
	for rows.Next() {
		cfg, err := scanSwitch(rows, encKey)
		if err != nil {
			return nil, err
		}
		list = append(list, *cfg)
	}
	return list, rows.Err()
}

func (s *SQLiteStore) UpdateSwitch(ctx context.Context, cfg models.SwitchConfig, encKey []byte) error {
	enc, err := Encrypt(encKey, []byte(cfg.Password))
	if err != nil {
		return fmt.Errorf("encrypt password: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE switches SET name=?,ip=?,username=?,password_enc=?,insecure_tls=?,
			enabled=?,poll_stats_secs=?,poll_config_secs=?,updated_at=?
		WHERE id=?`,
		cfg.Name, cfg.IP, cfg.Username, enc,
		boolInt(cfg.InsecureTLS), boolInt(cfg.Enabled),
		cfg.PollStatsSecs, cfg.PollConfigSecs, nowUnix(), cfg.ID)
	return err
}

func (s *SQLiteStore) DeleteSwitch(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM switches WHERE id=?`, id)
	return err
}

func (s *SQLiteStore) UpsertSnapshot(ctx context.Context, snap *models.SwitchSnapshot) error {
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO snapshots(switch_id, collected_at, data) VALUES(?,?,?)
		ON CONFLICT(switch_id) DO UPDATE SET collected_at=excluded.collected_at, data=excluded.data`,
		snap.Switch.ID, snap.CollectedAt.Unix(), data)
	return err
}

func (s *SQLiteStore) LatestSnapshot(ctx context.Context, switchID string) (*models.SwitchSnapshot, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT data FROM snapshots WHERE switch_id=?`, switchID).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no snapshot for switch %s", switchID)
	}
	if err != nil {
		return nil, err
	}
	var snap models.SwitchSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot: %w", err)
	}
	return &snap, nil
}

// Ensure SQLiteStore implements Store at compile time.
var _ Store = (*SQLiteStore)(nil)

type scanner interface {
	Scan(dest ...any) error
}

func scanSwitch(row scanner, encKey []byte) (*models.SwitchConfig, error) {
	var cfg models.SwitchConfig
	var enc []byte
	var insecure, enabled int
	err := row.Scan(&cfg.ID, &cfg.Name, &cfg.IP, &cfg.Username, &enc,
		&insecure, &enabled, &cfg.PollStatsSecs, &cfg.PollConfigSecs)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("switch not found")
	}
	if err != nil {
		return nil, err
	}
	cfg.InsecureTLS = insecure == 1
	cfg.Enabled = enabled == 1
	plain, err := Decrypt(encKey, enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt password: %w", err)
	}
	cfg.Password = string(plain)
	return &cfg, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nowUnix() int64 { return time.Now().Unix() }

// ── API tokens ────────────────────────────────────────────────────────────

func hashToken(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(h[:])
}

func (s *SQLiteStore) CreateApiToken(ctx context.Context, name, plaintext string, expiry int64) (*ApiToken, error) {
	id := uuid.New().String()
	now := nowUnix()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO api_tokens (id, name, token_hash, expiry, created_at) VALUES (?,?,?,?,?)`,
		id, name, hashToken(plaintext), expiry, now)
	if err != nil {
		return nil, fmt.Errorf("create api token: %w", err)
	}
	return &ApiToken{ID: id, Name: name, Expiry: expiry, CreatedAt: time.Unix(now, 0)}, nil
}

func (s *SQLiteStore) ListApiTokens(ctx context.Context) ([]ApiToken, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, expiry, created_at FROM api_tokens ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ApiToken
	for rows.Next() {
		var t ApiToken
		var ca int64
		if err := rows.Scan(&t.ID, &t.Name, &t.Expiry, &ca); err != nil {
			return nil, err
		}
		t.CreatedAt = time.Unix(ca, 0)
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) DeleteApiToken(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id=?`, id)
	return err
}

// MatchApiToken checks whether plaintext matches any stored token that hasn't
// expired. Returns the matching token on success.
func (s *SQLiteStore) MatchApiToken(ctx context.Context, plaintext string) (*ApiToken, error) {
	h := hashToken(plaintext)
	var t ApiToken
	var ca int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, expiry, created_at FROM api_tokens WHERE token_hash=?`, h).
		Scan(&t.ID, &t.Name, &t.Expiry, &ca)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("token not found")
	}
	if err != nil {
		return nil, err
	}
	t.CreatedAt = time.Unix(ca, 0)
	if t.Expiry != 0 && time.Now().Unix() > t.Expiry {
		return nil, fmt.Errorf("token expired")
	}
	return &t, nil
}
