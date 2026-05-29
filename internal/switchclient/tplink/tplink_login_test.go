package tplink_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/t0mer/SwitchDeck/internal/switchclient/tplink"
)

// rstClose closes c with TCP RST (linger 0) so the peer sees "connection reset by peer"
// rather than a clean FIN that produces EOF.
func rstClose(c net.Conn) {
	if tc, ok := c.(*net.TCPConn); ok {
		tc.SetLinger(0)
	}
	c.Close()
}

func TestLoginHandlesTCPRST(t *testing.T) {
	// Simulate switch behavior: POST /logon.cgi → RST, GET /SystemInfoRpm.htm → data page
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "logon") {
			// Hijack the connection and close it with RST (simulates the switch's behaviour)
			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "no hijack", 500)
				return
			}
			conn, _, _ := hj.Hijack()
			rstClose(conn)
			return
		}
		// For the session validation GET, return a data page (not login form)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<script>var info_ds = {descriStr:["TestSwitch"]};</script>`))
	}))
	defer srv.Close()

	c := tplink.New(false)
	err := c.Login(context.Background(), srv.URL, "admin", "pass")
	if err != nil {
		t.Errorf("expected nil error for RST + valid session, got: %v", err)
	}
}

func TestLoginFailsWhenSessionInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "logon") {
			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "no hijack", 500)
				return
			}
			conn, _, _ := hj.Hijack()
			rstClose(conn)
			return
		}
		// Return login page (session not established — wrong credentials)
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<form action="/logon.cgi"><input name="username"></form>`))
	}))
	defer srv.Close()

	c := tplink.New(false)
	err := c.Login(context.Background(), srv.URL, "admin", "wrongpassword")
	if err == nil {
		t.Error("expected error when session validation returns login page, got nil")
	}
}

func TestLoginSetsBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := tplink.New(false)
	if err := c.Login(context.Background(), srv.URL, "admin", "pass"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
