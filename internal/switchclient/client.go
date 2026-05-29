package switchclient

import (
	"context"

	"github.com/t0mer/SwitchDeck/internal/models"
)

// Client is the contract every switch vendor must implement.
type Client interface {
	Login(ctx context.Context, url, username, password string) error
	Logout(ctx context.Context) error
	GetSnapshot(ctx context.Context) (*models.SwitchSnapshot, error)
	GetSystemInfo(ctx context.Context) (*models.Switch, error)
	GetPorts(ctx context.Context) ([]models.Port, error)
	GetPortStats(ctx context.Context) ([]models.PortStats, error)
	GetVLANs(ctx context.Context) ([]models.VLAN, error)
	GetPoE(ctx context.Context) (*models.PoEStatus, error)
	GetMACTable(ctx context.Context) ([]models.MACEntry, error)
	GetLLDP(ctx context.Context) ([]models.LLDPNeighbor, error)
}
