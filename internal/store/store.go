package store

import (
	"context"

	"github.com/t0mer/SwitchDeck/internal/models"
)

// Store persists switch inventory and collected state.
type Store interface {
	ListSwitches(ctx context.Context) ([]models.Switch, error)
	UpsertSnapshot(ctx context.Context, snap *models.SwitchSnapshot) error
	LatestSnapshot(ctx context.Context, switchID string) (*models.SwitchSnapshot, error)
	Close() error
}

// SQLiteStore is the SQLite-backed implementation of Store.
type SQLiteStore struct {
	dbPath string
}

// NewSQLiteStore opens (or creates) the SQLite database at dbPath.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	return &SQLiteStore{dbPath: dbPath}, nil
}

func (s *SQLiteStore) ListSwitches(ctx context.Context) ([]models.Switch, error) {
	return nil, nil
}

func (s *SQLiteStore) UpsertSnapshot(ctx context.Context, snap *models.SwitchSnapshot) error {
	return nil
}

func (s *SQLiteStore) LatestSnapshot(ctx context.Context, switchID string) (*models.SwitchSnapshot, error) {
	return nil, nil
}

func (s *SQLiteStore) Close() error {
	return nil
}
