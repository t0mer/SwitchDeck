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
- **Prometheus metrics** — `GET /metrics` exposes per-switch and per-port gauges and counters (port state, speed, TX/RX bytes/packets/errors, PoE watts, STP state, IGMP, QoS, bandwidth limits, storm control, VLAN and LAG counts) read from the in-memory worker cache; no credentials required
- **Backup & Restore** — download a portable JSON backup of all switches (with passwords), auth settings, API tokens, and notification channel credentials; restore on a different server with one click — credentials are re-encrypted with the target server's key automatically
- **Docker-ready** — multi-arch image (`amd64`, `arm64`, `armv7`) published to Docker Hub

## Screenshots

### Dashboard — Dark Mode
![Dashboard dark](assets/screenshots/dashboard-dark.png)

### Dashboard — Light Mode
![Dashboard light](assets/screenshots/dashboard-light.png)

### Switch Detail — Ports
![Switch ports](assets/screenshots/switch-detail-ports.png)

### Switch Detail — Statistics
![Switch statistics](assets/screenshots/switch-detail-stats.png)

### Switch Detail — System Info
![Switch system](assets/screenshots/switch-detail-system.png)

### Add Switch
![Add switch modal](assets/screenshots/add-switch-modal.png)

### Notifications
![Notifications page](assets/screenshots/notifications.png)

### Add Notification Channel
![Add channel modal](assets/screenshots/add-channel-modal.png)

### Settings
![Settings page](assets/screenshots/settings.png)

### Backup & Restore
![Backup and restore](assets/screenshots/settings-backup.png)

### Login
![Login page](assets/screenshots/login.png)

## Installation

### Docker (recommended)

```bash
docker run -d \
  --name switchdeck \
  -p 8080:8080 \
  -v switchdeck-data:/data \
  --restart unless-stopped \
  techblog/switchdeck:latest
```

Or with Docker Compose:

```yaml
services:
  switchdeck:
    image: techblog/switchdeck:latest
    ports:
      - "8080:8080"
    volumes:
      - switchdeck-data:/data
    restart: unless-stopped

volumes:
  switchdeck-data:
```

Then open `http://localhost:8080` in your browser.

### From pre-built binaries

Download the latest release from the [Releases page](https://github.com/t0mer/SwitchDeck/releases), then:

```bash
# Linux amd64 example
chmod +x switchdeck-linux-amd64
./switchdeck-linux-amd64
```

### Build from source

Requirements: **Go 1.25+**

```bash
git clone https://github.com/t0mer/SwitchDeck.git
cd SwitchDeck
go build -o switchdeck ./cmd/switchdeck
./switchdeck
```

## Running

### CLI flags

| Flag | Default | Description |
|---|---|---|
| `--port` | `8080` | HTTP listening port |
| `--data` | `/data/switchdeck` | Data directory (SQLite database) |
| `--log-level` | `info` | Log level: `debug`, `info`, `warning`, `error` |
| `--version` | — | Print version and exit |
| `--reset-password` | — | Interactively reset admin credentials and exit |

### Environment variables

Environment variables take precedence over built-in defaults; flags take precedence over environment variables.

| Variable | Equivalent flag | Description |
|---|---|---|
| `PORT` | `--port` | Listening port |
| `DATA_DIR` | `--data` | Data directory |
| `LOG_LEVEL` | `--log-level` | Log level |

### Examples

```bash
# Custom port and data directory
./switchdeck --port 9090 --data /var/lib/switchdeck

# Reset admin password from the CLI (no server needed)
./switchdeck --reset-password --data /var/lib/switchdeck
```

### As a system service

```bash
./switchdeck --service install
./switchdeck --service start
```

Supported actions: `install`, `uninstall`, `start`, `stop`, `restart`.

## Configuration

All configuration is managed through the **Settings** page in the UI. No configuration files are needed.

### Authentication

By default SwitchDeck is open (no login required). To protect the application:

1. Open **Settings → Web Access** and enter a username and password, then click **Save Credentials**.
2. Toggle **Require login to access SwitchDeck** on.

All web UI pages and API endpoints will now require authentication.

To reset credentials without access to the UI:

```bash
./switchdeck --reset-password --data /var/lib/switchdeck
```

### API tokens

For external integrations (Home Assistant, scripts, monitoring tools):

1. Open **Settings → API Tokens** and click **+ Add Token**.
2. Give the token a name and an optional expiry.
3. Copy the token from the confirmation dialog — it is shown **only once**.

Use the token in requests:

```bash
curl -H "Authorization: Bearer <token>" http://localhost:8080/api/v1/switches
```

### Notifications

Open the **Notifications** page and click **+ Add Channel**. Supported providers:

| Provider | Use case |
|---|---|
| **Shoutrrr** | Slack, Discord, Telegram, SMTP, ntfy, Gotify, and [many more](https://containrrr.dev/shoutrrr/) via a single URL scheme |
| **GreenAPI** | WhatsApp messages via the GreenAPI cloud service |
| **WhatsApp Web** | WhatsApp messages via a self-hosted `go-whatsapp-web-multidevice` instance |

Each channel can be configured to notify on **switch offline**, **switch back online**, or both. Click **Send Test** to verify the configuration before saving.

### Adding a switch

Click **+ Add Switch** on the dashboard and fill in:

| Field | Description |
|---|---|
| **Name** | Display name (e.g. `Core`, `Floor 2`) |
| **IP / Hostname** | Switch management address (e.g. `192.168.0.10`) |
| **Username / Password** | Switch admin credentials |
| **Stats Poll** | How often to refresh counters in seconds (default: 60) |
| **Config Poll** | How often to refresh port/VLAN config in seconds (default: 300) |
| **Allow self-signed TLS** | Enable when the switch uses HTTPS with a self-signed certificate |

SwitchDeck starts collecting data immediately after the switch is saved.

### Backup & Restore

Open **Settings → Backup & Restore**.

- **Download Backup** — exports a `switchdeck-backup-<timestamp>.json` file containing all switches (including passwords), auth credentials, API token hashes, and notification channel credentials. **Store the file securely — it contains passwords in plaintext.**
- **Restore** — upload a backup file and confirm. All existing switches, API tokens, and notification channels are replaced with the contents of the file. Credentials are automatically re-encrypted with the target server's encryption key, so backups are fully portable across machines.

Restore can also be triggered via API:

```bash
curl -X POST http://localhost:8080/api/v1/backup/restore \
  -H "Authorization: Bearer <token>" \
  -F "file=@switchdeck-backup-2026-05-31T120000Z.json"
```

### Prometheus metrics

SwitchDeck exposes a Prometheus-compatible `/metrics` endpoint. No credentials are required. Add it to your Prometheus `scrape_configs`:

```yaml
scrape_configs:
  - job_name: switchdeck
    static_configs:
      - targets: ["localhost:8080"]
```

Metrics are read from the in-memory worker cache — scraping never triggers a new login or collection cycle on the switches.

**Available metrics** (all prefixed `switchdeck_`):

| Metric | Type | Description |
|---|---|---|
| `switch_info` | Gauge | Metadata labels: ip, model, firmware, hardware |
| `switch_up` | Gauge | 1 if online |
| `switch_ports_total/up/down` | Gauge | Port link summary |
| `switch_last_collected_timestamp_seconds` | Gauge | Unix timestamp of last collection |
| `port_up` / `port_enabled` / `port_speed_mbps` | Gauge | Per-port link state |
| `port_rx/tx_bytes/packets/errors/dropped_total` | Counter | Per-port traffic counters |
| `poe_budget_watts` / `poe_consumed_watts` | Gauge | PoE budget |
| `poe_port_enabled` / `poe_port_watts` | Gauge | Per-port PoE consumption |
| `stp_enabled` / `stp_port_state` | Gauge | Spanning Tree state |
| `mac_table_entries_total` / `lldp_neighbors_total` | Gauge | L2 table sizes |
| `igmp_enabled` / `igmp_groups_total` | Gauge | IGMP snooping |
| `qos_port_priority` | Gauge | Per-port QoS priority |
| `bandwidth_ingress/egress_kbps` | Gauge | Rate limits |
| `storm_broadcast/multicast/unknown_unicast_kbps` | Gauge | Storm thresholds |
| `loop_prevention_enabled` / `vlan_count` / `lag_count` | Gauge | Misc switch config |

## API

The REST API is available under `/api/v1`. When authentication is enabled, include either a session cookie (obtained via `POST /api/v1/auth/login`) or an `Authorization: Bearer <token>` header.

### Switches

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/switches` | List all switches |
| `POST` | `/api/v1/switches` | Add a switch |
| `GET` | `/api/v1/switches/{id}` | Get a single switch |
| `PUT` | `/api/v1/switches/{id}` | Update switch configuration |
| `DELETE` | `/api/v1/switches/{id}` | Remove a switch |
| `POST` | `/api/v1/switches/{id}/collect` | Trigger an immediate data collection |
| `GET` | `/api/v1/switches/{id}/ports` | List ports |
| `PUT` | `/api/v1/switches/{id}/ports/{port}` | Update port settings (enable/disable, speed, etc.) |
| `GET` | `/api/v1/switches/{id}/stats` | Port statistics |
| `GET` | `/api/v1/switches/{id}/vlans` | VLAN table |
| `POST` | `/api/v1/switches/{id}/reboot` | Reboot the switch |

### Settings

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/settings` | Get current settings |
| `PUT` | `/api/v1/settings` | Update settings |
| `GET` | `/api/v1/settings/tokens` | List API tokens |
| `POST` | `/api/v1/settings/tokens` | Create an API token (returns plaintext once) |
| `DELETE` | `/api/v1/settings/tokens/{id}` | Revoke an API token |

### Notifications

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/notifications` | List notification channels |
| `POST` | `/api/v1/notifications` | Add a channel |
| `PUT` | `/api/v1/notifications/{id}` | Update a channel |
| `DELETE` | `/api/v1/notifications/{id}` | Remove a channel |
| `POST` | `/api/v1/notifications/test` | Send a test message |

### Auth

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/auth/login` | Create a session (returns session cookie) |
| `POST` | `/api/v1/auth/logout` | Destroy the current session |
| `GET` | `/api/v1/auth/session` | Check auth status |

### Backup

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/backup` | Required | Download a full config backup as JSON |
| `POST` | `/api/v1/backup/restore` | Required | Restore from a backup file (`multipart/form-data` field `file` or raw JSON body) |

### Metrics

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/metrics` | None | Prometheus metrics (reads worker cache — no switch logins triggered) |

## License

[MIT](LICENSE)
