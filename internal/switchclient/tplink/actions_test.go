package tplink_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/internal/switchclient/tplink"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*tplink.TPLink, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c := tplink.New(false)
	if err := c.Login(context.Background(), srv.URL, "admin", "pass"); err != nil {
		t.Fatalf("login: %v", err)
	}
	return c, srv
}

func TestSetPort(t *testing.T) {
	var gotPath string
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			gotPath = r.URL.Path
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := c.SetPort(context.Background(), 1, models.PortConfig{
		Enabled:     false,
		Speed:       models.PortSpeed100M,
		FlowControl: false,
	})
	if err != nil {
		t.Fatalf("SetPort: %v", err)
	}
	if !strings.Contains(gotPath, "PortSetting") {
		t.Errorf("expected POST to PortSettingRpm.htm, got %s", gotPath)
	}
}

func TestSetPortInvalidNumber(t *testing.T) {
	c := tplink.New(false)
	err := c.SetPort(context.Background(), 0, models.PortConfig{})
	if err == nil {
		t.Error("expected error for port 0")
	}
	err = c.SetPort(context.Background(), 9, models.PortConfig{})
	if err == nil {
		t.Error("expected error for port 9")
	}
}

func TestReboot(t *testing.T) {
	called := false
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "Reboot") {
			called = true
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := c.Reboot(context.Background()); err != nil {
		t.Fatalf("Reboot: %v", err)
	}
	if !called {
		t.Error("expected reboot POST to be called")
	}
}
