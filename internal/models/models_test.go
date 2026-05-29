package models_test

import (
	"testing"
	"time"

	"github.com/t0mer/SwitchDeck/internal/models"
)

func TestSwitchDefaults(t *testing.T) {
	sw := models.Switch{
		ID:   "sw-1",
		Name: "Core Switch",
		IP:   "192.168.1.1",
	}
	if sw.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if sw.Name == "" {
		t.Fatal("expected non-empty Name")
	}
}

func TestPortStatus(t *testing.T) {
	p := models.Port{
		Number: 1,
		Status: models.PortStatusUp,
		Speed:  models.PortSpeed1G,
		Duplex: models.DuplexFull,
	}
	if p.Number != 1 {
		t.Fatalf("expected port 1, got %d", p.Number)
	}
}

func TestPoEPort(t *testing.T) {
	poe := models.PoEPort{
		PortNumber:  2,
		Enabled:     true,
		Priority:    models.PoEPriorityHigh,
		PowerWatts:  15.4,
		LastUpdated: time.Now(),
	}
	if !poe.Enabled {
		t.Fatal("expected PoE enabled")
	}
}

func TestPortMirror(t *testing.T) {
	m := models.PortMirror{
		Enabled:     true,
		DestPort:    1,
		Mode:        models.MirrorBoth,
		SourcePorts: []int{2, 3},
	}
	if len(m.SourcePorts) != 2 {
		t.Fatalf("expected 2 source ports, got %d", len(m.SourcePorts))
	}
}

func TestSwitchConfig(t *testing.T) {
	cfg := models.SwitchConfig{
		ID:             "sw-1",
		Name:           "Test Switch",
		IP:             "192.168.1.10",
		Username:       "admin",
		Password:       "secret",
		InsecureTLS:    true,
		PollStatsSecs:  60,
		PollConfigSecs: 300,
	}
	if cfg.PollStatsSecs != 60 {
		t.Fatalf("expected 60, got %d", cfg.PollStatsSecs)
	}
}
