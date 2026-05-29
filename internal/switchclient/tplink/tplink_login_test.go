package tplink_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/t0mer/SwitchDeck/internal/switchclient/tplink"
)

func TestLoginHandlesTCPRST(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	defer ln.Close()

	c := tplink.New(false)
	err = c.Login(context.Background(), "http://"+ln.Addr().String(), "admin", "pass")
	if err != nil {
		t.Errorf("expected nil error for RST-like close, got: %v", err)
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
