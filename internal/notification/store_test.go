package notification_test

import (
	"context"
	"os"
	"testing"

	"github.com/t0mer/SwitchDeck/internal/notification"
	"github.com/t0mer/SwitchDeck/internal/store"
)

func openTestStore(t *testing.T) (*store.SQLiteStore, []byte, func()) {
	t.Helper()
	f, err := os.CreateTemp("", "notif-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	st, err := store.Open(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	key, err := st.EncryptionKey()
	if err != nil {
		t.Fatal(err)
	}
	return st, key, func() {
		st.Close()
		os.Remove(f.Name())
	}
}

func TestChannelCRUD(t *testing.T) {
	st, key, cleanup := openTestStore(t)
	defer cleanup()
	ns := notification.NewStore(st.DB(), key)
	ctx := context.Background()

	ch := notification.Channel{
		Name:          "test-slack",
		Provider:      notification.ProviderShoutrrr,
		Config:        `{"url":"slack://token@channel"}`,
		Enabled:       true,
		NotifyOffline: true,
		NotifyOnline:  false,
	}
	created, err := ns.Create(ctx, ch)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty ID")
	}

	got, err := ns.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Config != ch.Config {
		t.Errorf("Config: got %q, want %q", got.Config, ch.Config)
	}

	created.NotifyOnline = true
	if err := ns.Update(ctx, *created); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got2, _ := ns.Get(ctx, created.ID)
	if !got2.NotifyOnline {
		t.Error("expected NotifyOnline=true after update")
	}

	if err := ns.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := ns.Get(ctx, created.ID); err == nil {
		t.Error("expected error after delete")
	}
}

func TestListEnabled(t *testing.T) {
	st, key, cleanup := openTestStore(t)
	defer cleanup()
	ns := notification.NewStore(st.DB(), key)
	ctx := context.Background()

	ns.Create(ctx, notification.Channel{
		Name: "offline-only", Provider: notification.ProviderShoutrrr,
		Config: `{"url":"slack://a@b"}`, Enabled: true,
		NotifyOffline: true, NotifyOnline: false,
	})
	ns.Create(ctx, notification.Channel{
		Name: "online-only", Provider: notification.ProviderShoutrrr,
		Config: `{"url":"slack://c@d"}`, Enabled: true,
		NotifyOffline: false, NotifyOnline: true,
	})
	ns.Create(ctx, notification.Channel{
		Name: "disabled", Provider: notification.ProviderShoutrrr,
		Config: `{"url":"slack://e@f"}`, Enabled: false,
		NotifyOffline: true, NotifyOnline: true,
	})

	offline, err := ns.ListEnabled(ctx, true, false)
	if err != nil {
		t.Fatalf("ListEnabled offline: %v", err)
	}
	if len(offline) != 1 || offline[0].Name != "offline-only" {
		t.Errorf("offline list: got %v", offline)
	}

	online, _ := ns.ListEnabled(ctx, false, true)
	if len(online) != 1 || online[0].Name != "online-only" {
		t.Errorf("online list: got %v", online)
	}
}

func TestDuplicateName(t *testing.T) {
	st, key, cleanup := openTestStore(t)
	defer cleanup()
	ns := notification.NewStore(st.DB(), key)
	ctx := context.Background()

	ch := notification.Channel{Name: "dup", Provider: notification.ProviderShoutrrr,
		Config: `{"url":"slack://x@y"}`, Enabled: true, NotifyOffline: true}
	ns.Create(ctx, ch)
	_, err := ns.Create(ctx, ch)
	if err == nil {
		t.Error("expected error on duplicate name")
	}
}
