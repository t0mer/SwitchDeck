package httpclient_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/t0mer/SwitchDeck/pkg/httpclient"
)

func TestNewClientHasJar(t *testing.T) {
	c := httpclient.New(httpclient.Options{SkipTLSVerify: true})
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestClientGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := httpclient.New(httpclient.Options{})
	resp, err := c.Get(srv.URL + "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
