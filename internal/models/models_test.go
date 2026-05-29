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
