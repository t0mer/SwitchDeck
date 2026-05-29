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
	var gotPath, gotContentType, gotPortID, gotState, gotSpeed, gotFC string
	c, srv := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			gotPath = r.URL.Path
			gotContentType = r.Header.Get("Content-Type")
			r.ParseMultipartForm(1 << 20)
			if r.MultipartForm != nil {
				gotPortID = r.MultipartForm.Value["portid"][0]
				gotState = r.MultipartForm.Value["state"][0]
				gotSpeed = r.MultipartForm.Value["speed"][0]
				gotFC = r.MultipartForm.Value["flowcontrol"][0]
			}
		}
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
	if gotPath != "/port_setting.cgi" {
		t.Errorf("path: got %q, want /port_setting.cgi", gotPath)
	}
	if !strings.Contains(gotContentType, "multipart/form-data") {
		t.Errorf("content-type: got %q, want multipart/form-data", gotContentType)
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
