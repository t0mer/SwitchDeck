package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/t0mer/SwitchDeck/internal/models"
	"github.com/t0mer/SwitchDeck/internal/store"
)

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	plaintext := "super-secret-password"

	encrypted, err := store.Encrypt(key, []byte(plaintext))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if len(encrypted) == 0 {
		t.Fatal("encrypted should be non-empty")
	}
	if string(encrypted) == plaintext {
		t.Fatal("encrypted should not equal plaintext")
	}

	decrypted, err := store.Decrypt(key, encrypted)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(decrypted) != plaintext {
		t.Errorf("Decrypt: got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptNonDeterministic(t *testing.T) {
	key := make([]byte, 32)
	a, _ := store.Encrypt(key, []byte("hello"))
	b, _ := store.Encrypt(key, []byte("hello"))
	if string(a) == string(b) {
		t.Error("two encryptions of same plaintext should differ (random nonce)")
	}
}

func TestOpenCreatesSchema(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	key, err := st.EncryptionKey()
	if err != nil {
		t.Fatalf("EncryptionKey: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}

	// Idempotent: open again should return same key
	st2, err := store.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer st2.Close()
	key2, _ := st2.EncryptionKey()
	if len(key2) != 32 {
		t.Fatalf("second key should also be 32 bytes")
	}
}

func TestSwitchCRUD(t *testing.T) {
	st, _ := store.Open(t.TempDir() + "/test.db")
	defer st.Close()
	key, _ := st.EncryptionKey()
	ctx := context.Background()

	cfg := models.SwitchConfig{
		ID:             "sw-1",
		Name:           "Test Switch",
		IP:             "192.168.1.10",
		Username:       "admin",
		Password:       "secret",
		InsecureTLS:    true,
		Enabled:        true,
		PollStatsSecs:  60,
		PollConfigSecs: 300,
	}

	if err := st.AddSwitch(ctx, cfg, key); err != nil {
		t.Fatalf("AddSwitch: %v", err)
	}

	got, err := st.GetSwitch(ctx, "sw-1", key)
	if err != nil {
		t.Fatalf("GetSwitch: %v", err)
	}
	if got.Name != "Test Switch" {
		t.Errorf("Name: got %q, want %q", got.Name, "Test Switch")
	}
	if got.Password != "secret" {
		t.Errorf("Password not decrypted correctly: %q", got.Password)
	}

	list, err := st.ListSwitches(ctx, key)
	if err != nil {
		t.Fatalf("ListSwitches: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 switch, got %d", len(list))
	}

	cfg.Name = "Renamed"
	if err := st.UpdateSwitch(ctx, cfg, key); err != nil {
		t.Fatalf("UpdateSwitch: %v", err)
	}
	got2, _ := st.GetSwitch(ctx, "sw-1", key)
	if got2.Name != "Renamed" {
		t.Errorf("after update, Name: got %q, want Renamed", got2.Name)
	}

	if err := st.DeleteSwitch(ctx, "sw-1"); err != nil {
		t.Fatalf("DeleteSwitch: %v", err)
	}
	list2, _ := st.ListSwitches(ctx, key)
	if len(list2) != 0 {
		t.Errorf("expected 0 switches after delete, got %d", len(list2))
	}
}

func TestSnapshotRoundtrip(t *testing.T) {
	st, _ := store.Open(t.TempDir() + "/test.db")
	defer st.Close()
	key, _ := st.EncryptionKey()
	ctx := context.Background()

	st.AddSwitch(ctx, models.SwitchConfig{
		ID: "sw-1", Name: "T", IP: "1.2.3.4",
		Username: "a", Password: "b", Enabled: true,
		PollStatsSecs: 60, PollConfigSecs: 300,
	}, key)

	snap := &models.SwitchSnapshot{
		Switch:      models.Switch{ID: "sw-1", Name: "Test"},
		Ports:       []models.Port{{Number: 1, Enabled: true}},
		CollectedAt: time.Now().Truncate(time.Second),
	}
	if err := st.UpsertSnapshot(ctx, snap); err != nil {
		t.Fatalf("UpsertSnapshot: %v", err)
	}

	got, err := st.LatestSnapshot(ctx, "sw-1")
	if err != nil {
		t.Fatalf("LatestSnapshot: %v", err)
	}
	if got.Switch.Name != "Test" {
		t.Errorf("Switch.Name: got %q, want Test", got.Switch.Name)
	}
	if len(got.Ports) != 1 || !got.Ports[0].Enabled {
		t.Errorf("Ports not restored correctly: %+v", got.Ports)
	}
}

func TestSettings(t *testing.T) {
	st, _ := store.Open(t.TempDir() + "/test.db")
	defer st.Close()
	ctx := context.Background()

	if err := st.SetSetting(ctx, "foo", "bar"); err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	val, err := st.GetSetting(ctx, "foo")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if val != "bar" {
		t.Errorf("got %q, want bar", val)
	}
}
