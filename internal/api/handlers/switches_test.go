package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/t0mer/SwitchDeck/internal/api/handlers"
	"github.com/t0mer/SwitchDeck/internal/manager"
	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/internal/store"
	"github.com/t0mer/SwitchDeck/internal/switchclient"
)

func noopFactory(_ bool) switchclient.Client { return &noopClient{} }

type noopClient struct{}

func (n *noopClient) Login(_ context.Context, _, _, _ string) error { return nil }
func (n *noopClient) Logout(_ context.Context) error                { return nil }
func (n *noopClient) GetSnapshot(_ context.Context) (*models.SwitchSnapshot, error) {
	return &models.SwitchSnapshot{}, nil
}
func (n *noopClient) RefreshStats(_ context.Context) ([]models.PortStats, error) { return nil, nil }
func (n *noopClient) RefreshPorts(_ context.Context) ([]models.Port, error)      { return nil, nil }
func (n *noopClient) SetPort(_ context.Context, _ int, _ models.PortConfig) error { return nil }
func (n *noopClient) ResetPortCounters(_ context.Context) error                    { return nil }
func (n *noopClient) SetVLANs(_ context.Context, _ []models.VLAN) error            { return nil }
func (n *noopClient) SetQoS(_ context.Context, _ models.QoSStatus) error           { return nil }
func (n *noopClient) SetBandwidth(_ context.Context, _ []models.BandwidthControl) error {
	return nil
}
func (n *noopClient) SetStormControl(_ context.Context, _ []models.StormControl) error { return nil }
func (n *noopClient) SetPortMirror(_ context.Context, _ models.PortMirror) error       { return nil }
func (n *noopClient) SetLAG(_ context.Context, _ []models.LAGGroup) error              { return nil }
func (n *noopClient) SetIGMP(_ context.Context, _ bool) error                          { return nil }
func (n *noopClient) SetLoopPrevention(_ context.Context, _ bool) error                { return nil }
func (n *noopClient) Reboot(_ context.Context) error                                   { return nil }

func setupHandlers(t *testing.T) *handlers.Handlers {
	t.Helper()
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	key, _ := st.EncryptionKey()
	mgr := manager.New(noopFactory)
	return handlers.New(mgr, st, key)
}

func TestListSwitches_empty(t *testing.T) {
	h := setupHandlers(t)
	rec := httptest.NewRecorder()
	h.ListSwitches(rec, httptest.NewRequest(http.MethodGet, "/api/v1/switches", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	var body []any
	json.NewDecoder(rec.Body).Decode(&body)
	if len(body) != 0 {
		t.Errorf("expected empty list, got %v", body)
	}
}

func TestAddAndGetSwitch(t *testing.T) {
	h := setupHandlers(t)
	payload := map[string]any{
		"name": "Test Switch", "ip": "192.168.1.10",
		"username": "admin", "password": "secret",
		"insecure_tls": true,
	}
	body, _ := json.Marshal(payload)
	rec := httptest.NewRecorder()
	h.AddSwitch(rec, httptest.NewRequest(http.MethodPost, "/api/v1/switches", bytes.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("AddSwitch status: got %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	var added map[string]any
	json.NewDecoder(rec.Body).Decode(&added)
	id, _ := added["id"].(string)
	if id == "" {
		t.Fatal("expected non-empty id in response")
	}
}

func TestAddSwitch_validation(t *testing.T) {
	h := setupHandlers(t)
	body, _ := json.Marshal(map[string]any{"name": "no ip"})
	rec := httptest.NewRecorder()
	h.AddSwitch(rec, httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields, got %d", rec.Code)
	}
}
