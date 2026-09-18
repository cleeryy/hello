# hello

[![Docker Build](https://github.com/cleeryy/hello/actions/workflows/docker-build.yml/badge.svg)](https://github.com/cleeryy/hello/actions/workflows/docker-build.yml)
[![Go Version](https://img.shields.io/badge/Go-1.25-blue.svg)](https://golang.org)
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
- **Go 1.25+** (for local development)
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

### 5. Realtime status

```
GET /ws
```

WebSocket broadcasting `{ "type": "status", "device": { ... } }` every time
the monitor observes a status change.

**Example:**

```
curl http://localhost:8080/wake/AA:BB:CC:DD:EE:FF
curl -X POST http://localhost:8080/devices/pc-salon/wake
```

A full walkthrough lives in `test-api.sh` (requires `jq` and a running server).

### 6. Machine-readable docs

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

Or create a `.env` file in the project root (see `.env.example`).

## Project Structure

```
hello/
├── cmd/api/             # Binary entrypoint (graceful shutdown)
├── internal/
│   ├── config/          # Env-based configuration
│   ├── handlers/        # HTTP routes (wake + devices CRUD)
│   ├── models/          # Device type + validation
│   ├── monitor/         # Background ping monitor
│   ├── ping/            # ICMP/TCP ping helpers
│   ├── storage/         # File-backed device registry
│   ├── websocket/       # Live-status hub (/ws)
│   └── wol/             # Wake-on-LAN packet sender
├── devices.example.json # Sample registry (copy to devices.json)
├── Dockerfile           # Multi-stage Docker build
├── docker-compose.yml   # Local deployment
├── test-api.sh          # Manual API walkthrough
├── .github/workflows/   # CI, Docker build, security scans
└── README.md            # This file
```

## Built With

- **[Go 1.25](https://golang.org/)** - Programming language
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

- This API doesn't include authentication - consider adding it for production use
- Keep your `.env` file private and never commit it to version control
- Consider running this service only on your local network

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
