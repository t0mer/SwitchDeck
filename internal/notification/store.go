package notification

import (
	"context"
	"database/sql"
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
