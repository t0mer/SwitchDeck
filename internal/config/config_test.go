package config_test

import (
	"testing"

	"github.com/t0mer/SwitchDeck/internal/config"
)

func TestDefaultConfig(t *testing.T) {
	cfg := config.Default()
	if cfg.Port == 0 {
		t.Fatal("expected non-zero default port")
	}
	if cfg.LogLevel == "" {
		t.Fatal("expected non-empty log level")
	}
}
