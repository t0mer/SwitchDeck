package config

// Config holds all runtime configuration for SwitchDeck.
type Config struct {
	Port     int
	LogLevel string
	DataDir  string
	DBPath   string
}

// Default returns a Config populated with sensible defaults.
func Default() *Config {
	return &Config{
		Port:     8080,
		LogLevel: "info",
		DataDir:  "/data/switchdeck",
		DBPath:   "/data/switchdeck/switchdeck.db",
	}
}
