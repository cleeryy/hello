# hello

[![Docker Build](https://github.com/cleeryy/hello/actions/workflows/docker-build.yml/badge.svg)](https://github.com/cleeryy/hello/actions/workflows/docker-build.yml)
[![Go Version](https://img.shields.io/badge/Go-1.26-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A simple, fast, and containerized **wake on lan** built with Go and Gin framework. Send magic packets to wake up your devices over the network with ease.

## Features

- **Lightning Fast** - Built with Go and Gin for high performance
- **Device registry** - CRUD over `/devices`, persisted to a JSON file
- **Live status** - Background ping monitor with realtime updates over `/ws`
- **Docker Ready** - Multi-stage build for minimal image size
- **CI/CD Integrated** - Tests, lint, CodeQL and GHCR pushes via GitHub Actions
- **Single Binary** - No runtime dependencies needed
- **Configurable** - Environment variable support (see `.env.example`)

## Prerequisites

- **Docker** (for containerized deployment)
- **Go 1.26+** (for local development)
- Network access to devices you want to wake

## Quick Start

### Using Docker Compose (Recommended)

```
cp .env.example .env
cp devices.example.json devices.json
docker compose up --build
```

> Magic packets use subnet broadcast, which does not cross Docker's bridge
> network. If wake-ups never arrive, use `network_mode: host` (Linux only).

### Using Docker

```
docker pull ghcr.io/cleeryy/hello:latest

docker run -p 8080:8080 \
  -e DEFAULT_MAC=00:11:22:33:44:55 \
  ghcr.io/cleeryy/hello:latest
```

### Local Development

```
# Clone the repository
git clone https://github.com/cleeryy/hello.git
cd hello

# Install dependencies
go mod download

# Create .env file
cp .env.example .env

# Run the API
go run ./cmd/api
```

The API will be available at `http://localhost:8080`

## API Endpoints

### 1. Health Check

```
GET /
GET /health
```

### 2. Wake Default Device

```
GET /wake
```

Sends a magic packet to the `DEFAULT_MAC` configured in environment variables.

### 3. Wake Specific Device

```
GET /wake/:macAddress
```

### 4. Devices

```
GET    /devices
POST   /devices
GET    /devices/:id
PUT    /devices/:id
DELETE /devices/:id
POST   /devices/:id/wake
```

Device JSON shape (see `devices.example.json`):

```
{
  "id": "pc-salon",
  "name": "PC Salon",
  "mac": "00:11:22:33:44:55",
  "ip": "192.168.1.50",
  "status": "unknown",
  "ping_enabled": true,
  "last_seen": 0
}
```

### 5. Wake history

```
GET /history
```

Every sent magic packet — manual or scheduled, success or failure — newest
first. Filter by registry id, cap the page:

```
curl "http://localhost:8080/history?device_id=pc-salon&limit=20"
```

### 6. Realtime status

```
GET /ws
```

WebSocket broadcasting `{ "type": "device.status", "device": { ... } }` every time
the monitor observes a status change.

**Example:**

```
curl http://localhost:8080/wake/AA:BB:CC:DD:EE:FF
curl -X POST http://localhost:8080/devices/pc-salon/wake
```

A full walkthrough lives in `test-api.sh` (requires `jq`, a running server, and `API_TOKEN`).

### 7. Schedules

```
GET    /schedules
POST   /schedules
PUT    /schedules/:id
DELETE /schedules/:id
```

Cron wakes for registered devices. Standard 5-field cron
(`minute hour day month weekday`) or descriptors like `@daily`:

```bash
curl -X POST http://localhost:8080/schedules \
  -H 'Content-Type: application/json' \
  -d '{"id":"morning","device_id":"pc-salon","cron":"0 7 * * 1-5"}'
```

Creation always enables the schedule; `PUT` the full object with
`"enabled": false` to pause it, `DELETE` removes it. Fires are logged to
`GET /history` with `"trigger": "schedule"`.

### 8. LAN discovery

```
POST /discover
POST /discover/adopt
```

Shows which boxes are actually out there — TCP-probes the local /24,
then maps live IPs to MACs via the OS ARP table:

```bash
curl -X POST http://localhost:8080/discover
# {"hosts":[{"ip":"192.168.1.10","mac":"11:22:33:44:55:66",
#             "hostname":"nas.local","known":false}, ...]}
```

Adopt the keepers as devices (all-or-nothing: one duplicate MAC 409s the
whole batch, so a retry never creates half a fleet):

```bash
curl -X POST http://localhost:8080/discover/adopt \
  -H 'Content-Type: application/json' \
  -d '{"hosts":[{"mac":"11:22:33:44:55:66","ip":"192.168.1.10"}]}'
# → 201 {"devices":[...]} — name defaults to hostname, else host-<ip>
```

One scan at a time with a 30s cooldown; concurrent or early rescans get
`429` with a `Retry-After` hint. No root required, stdlib only.

### 9. Machine-readable docs

```
GET /openapi.yaml
GET /docs
```

`/openapi.yaml` serves the OpenAPI 3.1 document (`internal/spec/openapi.yaml`,
embedded in the binary); `/docs` serves Swagger UI for it. Error responses
follow RFC 9457 (`application/problem+json`):

```
{
  "type": "about:blank",
  "title": "not found",
  "status": 404,
  "detail": "storage: device not found"
}
```

Conventions: `POST /wake` and `POST /wake/:macAddress` are canonical (the
`GET` variants remain as deprecated legacy), `PUT /devices/:id` is a full
replacement, `DELETE` returns `204 No Content`.

## Configuration

| Variable              | Required | Default           | Description                        |
|-----------------------|----------|-------------------|------------------------------------|
| `DEFAULT_MAC`         | yes      | -                 | MAC woken by `GET /wake`           |
| `PORT`                | no       | `8080`            | Listen port                        |
| `BROADCAST_IP`        | no       | `255.255.255.255` | Subnet broadcast for magic packets |
| `DEVICES_FILE`        | no       | `devices.json`    | Device registry file               |
| `MONITOR_INTERVAL_SEC`| no       | `30`              | Status poll interval in seconds    |
| `API_TOKEN`           | yes      | -                 | Bearer token (min 16 chars, never `change-me`) |
| `HISTORY_FILE`        | no       | `wake-history.json` | Wake log file                    |
| `SCHEDULES_FILE`      | no       | `schedules.json` | Wake schedules file               |
| `TRUSTED_PROXIES`     | no       | `127.0.0.1,::1` | Reverse-proxy IPs/CIDRs for `X-Forwarded-For` |
| `CORS_ORIGINS`        | no       | *(same-origin)* | Extra browser origins, comma-separated |

Or create a `.env` file in the project root (see `.env.example`).

## Authentication

`API_TOKEN` is required (min 16 chars, `change-me` rejected). Every route except
`GET /health`, `/`, `/docs`, and `/openapi.yaml` requires `Authorization: Bearer <token>`.
Browsers calling `/ws` should prefer the header; `?token=` still works for compat
but leaks in proxy logs, so avoid it.

```bash
curl -H "Authorization: Bearer $API_TOKEN" http://localhost:8080/devices
```

## Project Structure

```
hello/
├── cmd/api/             # Binary entrypoint (graceful shutdown)
├── internal/
│   ├── config/          # Env-based configuration + validation
│   ├── handlers/        # HTTP routes (wake, devices, history, schedules, discover, ws + CORS/throttle)
│   ├── history/         # Wake log (ring buffer + persistence)
│   ├── scheduler/       # Wake schedules (cron runner + file store)
│   ├── models/          # Device/Schedule/WakeEvent + validation
│   ├── monitor/         # Background ping monitor
│   ├── ping/            # ICMP/TCP ping helpers
│   ├── storage/         # File-backed device registry
│   ├── discover/        # LAN scan + adopt
│   ├── websocket/       # Live-status hub (/ws, strict origin)
│   ├── spec/            # Embedded OpenAPI + Swagger UI
│   └── wol/             # Wake-on-LAN packet sender
├── devices.example.json # Sample registry (copy to devices.json)
├── Dockerfile           # Multi-stage Docker build
├── docker-compose.yml   # Local deployment
├── test-api.sh          # Manual API walkthrough
├── .github/workflows/   # CI, Docker build, security scans
└── README.md            # This file
```

## Built With

- **[Go 1.26](https://golang.org/)** - Programming language
- **[Gin](https://gin-gonic.com/)** - Web framework
- **[gowol](https://github.com/linde12/gowol)** - Wake-on-LAN implementation
- **[godotenv](https://github.com/joho/godotenv)** - .env file loader
- **[gorilla/websocket](https://github.com/gorilla/websocket)** - Realtime status
- **[Docker](https://www.docker.com/)** - Containerization
- **[GitHub Actions](https://github.com/features/actions)** - CI/CD

## Development

### Run tests locally

```
go test -race -shuffle=on -count=1 ./...
```

### Format code

```
gofmt -l .
```

### Lint code

```
golangci-lint run ./...
```

## Security Notes

- `API_TOKEN` is mandatory (min 16 chars); the server refuses to boot without it
- Keep your `.env` file private and never commit it; rotate the token by restart
- `?token=` on `/ws` is deprecated (leaks in proxy logs), prefer the `Authorization` header
- CORS defaults to same-origin; set `CORS_ORIGINS` only for explicit browser origins
- WebSocket origin is locked to same-origin + `CORS_ORIGINS`
- Wake (`10/min/IP`) and adopt (`5/min/IP`) are throttled with `429` + `Retry-After`; discovery keeps its 30s global cooldown
- JSON files are written `0600` mono-instance; mount all three in Compose

## Reverse proxy (Caddy + generic)

TLS terminates at the proxy. Trust only its IPs via `TRUSTED_PROXIES`, forward
`X-Forwarded-For/Proto/Host`, and strip incoming forwarding headers from clients.

```caddy
# Caddyfile (same host)
lan.example.com {
  reverse_proxy 127.0.0.1:8080
}
# env: TRUSTED_PROXIES=127.0.0.1/32,::1/128 CORS_ORIGINS=https://lan.example.com
```

Generic Nginx/Traefik: same env pattern, allowlist `10.0.0.0/8,172.16.0.0/12,192.168.0.0/16`
via `TRUSTED_PROXIES` when the proxy is on LAN.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Author

**Cléry Arque-Ferradou**
- GitHub: [@cleeryy](https://github.com/cleeryy)
- Repository: [cleeryy/hello](https://github.com/cleeryy/hello)

## Acknowledgments

- Inspired by the need for a lightweight, Dockerized Wake-on-LAN service
- Built with love using Go and Gin

---

**Happy waking!**
