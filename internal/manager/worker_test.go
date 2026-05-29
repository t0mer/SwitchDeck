package manager_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/t0mer/SwitchDeck/internal/manager"
	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/internal/switchclient"
)

// mockClient counts calls to GetSnapshot and RefreshStats.
type mockClient struct {
	snapshots atomic.Int64
	statsOnly atomic.Int64
}

func (m *mockClient) Login(_ context.Context, _, _, _ string) error { return nil }
func (m *mockClient) Logout(_ context.Context) error                { return nil }
func (m *mockClient) GetSnapshot(_ context.Context) (*models.SwitchSnapshot, error) {
	m.snapshots.Add(1)
	return &models.SwitchSnapshot{Switch: models.Switch{ID: "sw-1"}}, nil
}
func (m *mockClient) RefreshStats(_ context.Context) ([]models.PortStats, error) {
	m.statsOnly.Add(1)
	return nil, nil
}
func (m *mockClient) SetPort(_ context.Context, _ int, _ models.PortConfig) error { return nil }
func (m *mockClient) ResetPortCounters(_ context.Context) error                    { return nil }
func (m *mockClient) SetVLANs(_ context.Context, _ []models.VLAN) error            { return nil }
func (m *mockClient) SetQoS(_ context.Context, _ models.QoSStatus) error           { return nil }
func (m *mockClient) SetBandwidth(_ context.Context, _ []models.BandwidthControl) error {
	return nil
}
func (m *mockClient) SetStormControl(_ context.Context, _ []models.StormControl) error { return nil }
func (m *mockClient) SetPortMirror(_ context.Context, _ models.PortMirror) error       { return nil }
func (m *mockClient) SetLAG(_ context.Context, _ []models.LAGGroup) error              { return nil }
func (m *mockClient) SetIGMP(_ context.Context, _ bool) error                          { return nil }
func (m *mockClient) SetLoopPrevention(_ context.Context, _ bool) error                { return nil }
func (m *mockClient) Reboot(_ context.Context) error                                   { return nil }

// compile-time check that mockClient satisfies the interface
var _ switchclient.Client = (*mockClient)(nil)

func TestManagerAddRemove(t *testing.T) {
	mc := &mockClient{}
	cfg := models.SwitchConfig{
		ID:             "sw-1",
		IP:             "1.2.3.4",
		Username:       "admin",
		Password:       "pass",
		Enabled:        true,
		PollStatsSecs:  60,
		PollConfigSecs: 300,
	}

	clientFactory := func(_ bool) switchclient.Client { return mc }
	mgr := manager.New(clientFactory)

	if err := mgr.Add(cfg); err != nil {
		t.Fatalf("Add: %v", err)
	}
	time.Sleep(200 * time.Millisecond) // let initial collection run

	if mc.snapshots.Load() < 1 {
		t.Error("expected at least one snapshot after Add")
	}

	c, err := mgr.GetClient("sw-1")
	if err != nil {
		t.Fatalf("GetClient: %v", err)
	}
	if c == nil {
		t.Error("expected non-nil client")
	}

	if err := mgr.Remove("sw-1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if _, err := mgr.GetClient("sw-1"); err == nil {
		t.Error("expected error after Remove, got nil")
	}
}

func TestManagerDuplicateAdd(t *testing.T) {
	mc := &mockClient{}
	cfg := models.SwitchConfig{
		ID:             "sw-1",
		IP:             "1.2.3.4",
		Username:       "admin",
		Password:       "pass",
		Enabled:        true,
		PollStatsSecs:  60,
		PollConfigSecs: 300,
	}
	clientFactory := func(_ bool) switchclient.Client { return mc }
	mgr := manager.New(clientFactory)

	mgr.Add(cfg)
	if err := mgr.Add(cfg); err == nil {
		t.Error("expected error on duplicate Add, got nil")
	}
	mgr.Remove("sw-1")
}

func TestManagerStatus(t *testing.T) {
	mc := &mockClient{}
	cfg := models.SwitchConfig{
		ID: "sw-1", IP: "1.2.3.4",
		Username: "admin", Password: "pass", Enabled: true,
		PollStatsSecs: 60, PollConfigSecs: 300,
	}
	clientFactory := func(_ bool) switchclient.Client { return mc }
	mgr := manager.New(clientFactory)

	// Unknown before add
	if status := mgr.Status("sw-1"); status != models.SwitchStatusUnknown {
		t.Errorf("status before add: got %v, want unknown", status)
	}

	mgr.Add(cfg)
	time.Sleep(200 * time.Millisecond)

	// Online after first collection
	if status := mgr.Status("sw-1"); status != models.SwitchStatusOnline {
		t.Errorf("status after add: got %v, want online", status)
	}
	mgr.Remove("sw-1")
}
