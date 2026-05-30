package tplink_test

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	var gotPath, gotMethod, gotPortID, gotState, gotSpeed, gotFC string
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotPortID = r.URL.Query().Get("portid")
		gotState = r.URL.Query().Get("state")
		gotSpeed = r.URL.Query().Get("speed")
		gotFC = r.URL.Query().Get("flowcontrol")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := c.SetPort(context.Background(), 3, models.PortConfig{
		Enabled:     false,
		Speed:       models.PortSpeed100M,
		FlowControl: true,
	})
	if err != nil {
		t.Fatalf("SetPort: %v", err)
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method: got %q, want GET", gotMethod)
	}
	if gotPath != "/port_setting.cgi" {
		t.Errorf("path: got %q, want /port_setting.cgi", gotPath)
	}
	if gotPortID != "3" {
		t.Errorf("portid: got %q, want 3", gotPortID)
	}
	if gotState != "0" {
		t.Errorf("state: got %q, want 0 (disabled)", gotState)
	}
	if gotSpeed != "5" {
		t.Errorf("speed: got %q, want 5 (100MFull)", gotSpeed)
	}
	if gotFC != "1" {
		t.Errorf("flowcontrol: got %q, want 1 (on)", gotFC)
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
	var gotPath, gotOp string
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			gotPath = r.URL.Path
			r.ParseForm()
			gotOp = r.FormValue("reboot_op")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := c.Reboot(context.Background()); err != nil {
		t.Fatalf("Reboot: %v", err)
	}
	if gotPath != "/reboot.cgi" {
		t.Errorf("path: got %q, want /reboot.cgi", gotPath)
	}
	if gotOp != "reboot" {
		t.Errorf("reboot_op: got %q, want reboot", gotOp)
	}
}
