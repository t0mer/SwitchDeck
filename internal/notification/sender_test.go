package notification_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/t0mer/SwitchDeck/internal/notification"
)

func TestGreenAPISender(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := notification.GreenAPIConfig{
		InstanceID: "inst1", Token: "tok1",
		Recipient: "972501234567", APIURL: srv.URL,
	}
	cfgJSON, _ := json.Marshal(cfg)
	ch := &notification.Channel{Provider: notification.ProviderGreenAPI, Config: string(cfgJSON)}
	sender, err := notification.NewSender(ch)
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}
	if err := sender.Send(context.Background(), "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotBody["chatId"] != "972501234567@c.us" {
		t.Errorf("chatId: got %q, want 972501234567@c.us", gotBody["chatId"])
	}
	if gotBody["message"] != "hello" {
		t.Errorf("message: got %q, want hello", gotBody["message"])
	}
}

func TestGreenAPIAppendsAtCUs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Recipient already has @c.us — must not double-append.
	cfg := notification.GreenAPIConfig{
		InstanceID: "i", Token: "t",
		Recipient: "972501234567@c.us", APIURL: srv.URL,
	}
	cfgJSON, _ := json.Marshal(cfg)
	ch := &notification.Channel{Provider: notification.ProviderGreenAPI, Config: string(cfgJSON)}
	sender, _ := notification.NewSender(ch)
	if err := sender.Send(context.Background(), "x"); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestWhatsAppSender(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := notification.WhatsAppConfig{BaseURL: srv.URL, Recipient: "972501234567"}
	cfgJSON, _ := json.Marshal(cfg)
	ch := &notification.Channel{Provider: notification.ProviderWhatsApp, Config: string(cfgJSON)}
	sender, err := notification.NewSender(ch)
	if err != nil {
		t.Fatalf("NewSender: %v", err)
	}
	if err := sender.Send(context.Background(), "world"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotBody["message"] != "world" {
		t.Errorf("message: got %q, want world", gotBody["message"])
	}
}

func TestUnknownProvider(t *testing.T) {
	ch := &notification.Channel{Provider: "unknown", Config: "{}"}
	_, err := notification.NewSender(ch)
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}
