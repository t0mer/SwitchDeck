# SwitchDeck

SwitchDeck is a self-hosted central management portal for **TP-Link smart switches** on a local network. Instead of logging into each switch's web UI separately, SwitchDeck gives you a single dashboard to monitor and manage all your switches in one place.

Built in Go with an embedded web UI — a single binary, no external dependencies.

## Features

- **Multi-switch dashboard** — monitor all switches at a glance; per-switch port LED strip shows live link state for each port
- **Real-time status** — online/offline detection with automatic background polling; data refreshes every 60 seconds
- **Port management** — view port status, speed, duplex, and description; enable or disable individual ports
- **Port statistics** — per-port RX/TX byte and packet counters, error and drop counts
- **VLAN management** — view and manage 802.1Q VLAN configuration
- **System information** — model, hardware and firmware version, MAC address, last-collected timestamp
- **Notifications** — send alerts when a switch goes offline or comes back online; supports Shoutrrr (Slack, Discord, Telegram, SMTP, ntfy, Gotify, and more), GreenAPI (WhatsApp Cloud), and WhatsApp Web (self-hosted)
- **Authentication** — optional username/password login with argon2id-hashed passwords and HMAC-SHA256 signed session cookies; CLI `--reset-password` flag for credential recovery
- **API tokens** — named bearer tokens for external API access (e.g. Home Assistant, scripts); each token is stored as a SHA-256 hash and shown in plaintext only once
- **Dark / light mode** — system-preference-aware toggle; preference persisted in `localStorage`
- **Responsive layout** — collapsible sidebar on desktop; fixed bottom tab bar on mobile
- **Docker-ready** — multi-arch image (`amd64`, `arm64`, `armv7`) published to Docker Hub
