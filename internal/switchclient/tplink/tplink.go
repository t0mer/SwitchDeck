package tplink

import (
	"context"
	"fmt"

	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/pkg/httpclient"
)

// TPLink is the TP-Link implementation of switchclient.Client.
type TPLink struct {
	baseURL string
	http    *httpclient.Client
}

// ErrNotImplemented is returned by stub methods pending data collection.
var ErrNotImplemented = fmt.Errorf("not yet implemented")

// New creates a TPLink client with TLS verification disabled for self-signed switch certs.
func New() *TPLink {
	return &TPLink{
		http: httpclient.New(httpclient.Options{SkipTLSVerify: true}),
	}
}

func (t *TPLink) Login(ctx context.Context, url, username, password string) error {
	t.baseURL = url
	return ErrNotImplemented
}

func (t *TPLink) Logout(ctx context.Context) error {
	return ErrNotImplemented
}

func (t *TPLink) GetSnapshot(ctx context.Context) (*models.SwitchSnapshot, error) {
	return nil, ErrNotImplemented
}

func (t *TPLink) GetSystemInfo(ctx context.Context) (*models.Switch, error) {
	return nil, ErrNotImplemented
}

func (t *TPLink) GetPorts(ctx context.Context) ([]models.Port, error) {
	return nil, ErrNotImplemented
}

func (t *TPLink) GetPortStats(ctx context.Context) ([]models.PortStats, error) {
	return nil, ErrNotImplemented
}

func (t *TPLink) GetVLANs(ctx context.Context) ([]models.VLAN, error) {
	return nil, ErrNotImplemented
}

func (t *TPLink) GetPoE(ctx context.Context) (*models.PoEStatus, error) {
	return nil, ErrNotImplemented
}

func (t *TPLink) GetMACTable(ctx context.Context) ([]models.MACEntry, error) {
	return nil, ErrNotImplemented
}

func (t *TPLink) GetLLDP(ctx context.Context) ([]models.LLDPNeighbor, error) {
	return nil, ErrNotImplemented
}
