package switchclient

import (
	"context"

	"github.com/t0mer/SwitchDeck/internal/models"
)

// Client is the contract every switch vendor must implement.
type Client interface {
	// Session management
	Login(ctx context.Context, url, username, password string) error
	Logout(ctx context.Context) error

	// Data collection
	GetSnapshot(ctx context.Context) (*models.SwitchSnapshot, error)
	RefreshStats(ctx context.Context) ([]models.PortStats, error)

	// Port actions
	SetPort(ctx context.Context, port int, cfg models.PortConfig) error
	ResetPortCounters(ctx context.Context) error

	// VLAN actions
	SetVLANs(ctx context.Context, vlans []models.VLAN) error

	// QoS actions
	SetQoS(ctx context.Context, qos models.QoSStatus) error
	SetBandwidth(ctx context.Context, bw []models.BandwidthControl) error
	SetStormControl(ctx context.Context, sc []models.StormControl) error

	// Network actions
	SetPortMirror(ctx context.Context, m models.PortMirror) error
	SetLAG(ctx context.Context, groups []models.LAGGroup) error
	SetIGMP(ctx context.Context, enabled bool) error
	SetLoopPrevention(ctx context.Context, enabled bool) error

	// Maintenance
	Reboot(ctx context.Context) error
}
