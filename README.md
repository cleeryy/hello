# 🌙 hello

[![Build and Push Docker Image to GHCR](https://github.com/cleeryy/hello/actions/workflows/docker-build.yml/badge.svg)](https://github.com/cleeryy/hello/actions/workflows/docker-build.yml)
[![Go Version](https://img.shields.io/badge/Go-1.21-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A simple, fast, and containerized **wake on lan** built with Go and Gin framework. Send magic packets to wake up your devices over the network with ease.

## ✨ Features

- 🚀 **Lightning Fast** - Built with Go and Gin for high performance
- 🐳 **Docker Ready** - Multi-stage build for minimal image size (~20MB)
- 🔄 **CI/CD Integrated** - Automatic builds and pushes to GitHub Container Registry
- 📦 **Single Binary** - No runtime dependencies needed
- 🌐 **Simple REST API** - Easy-to-use endpoints
- 🔧 **Configurable** - Environment variable support for default MAC address

## 📋 Prerequisites

- **Docker** (for containerized deployment)
- **Go 1.21+** (for local development)
- Network access to devices you want to wake

## 🚀 Quick Start

### Using Docker (Recommended)

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
echo "DEFAULT_MAC=00:11:22:33:44:55" > .env

# Run the API
go run main.go config.go wol.go
```

The API will be available at `http://localhost:8080`

## 📚 API Endpoints

### 1. Health Check
```
GET /
```

**Response:**
```
{
  "status": 200,
  "message": "welcome to the hello api!",
  "defaultMac": "00:11:22:33:44:55"
}
```

### 2. Wake Default Device
```
GET /wake
```

Sends a magic packet to the `DEFAULT_MAC` configured in environment variables.

**Response:**
```
{
  "status": 200,
  "message": "magic packet sent to 00:11:22:33:44:55 !"
}
```

### 3. Wake Specific Device
```
GET /wake/:macAddress
```

Sends a magic packet to the specified MAC address.

**Example:**
```
curl http://localhost:8080/wake/AA:BB:CC:DD:EE:FF
```

**Response:**
```
{
  "status": 200,
  "message": "magic packet sent to AA:BB:CC:DD:EE:FF !"
}
```

## 🛠️ Configuration

Set the default MAC address using environment variables:

```
export DEFAULT_MAC=00:11:22:33:44:55
```

Or create a `.env` file in the project root:

```
DEFAULT_MAC=00:11:22:33:44:55
```

## 🐳 Docker Build & Push

### Build Locally

```
docker build -t hello .
docker run -p 8080:8080 -e DEFAULT_MAC=00:11:22:33:44:55 hello
```

### Automatic CI/CD

Push to the `main` branch to automatically:
1. Build the Docker image
2. Push to GHCR: `ghcr.io/cleeryy/hello:latest`
3. Tag with commit SHA for version tracking

## 📁 Project Structure

```
hello/
├── main.go              # API routes and server setup
├── config.go            # Configuration and .env loading
├── wol.go               # Wake-on-LAN packet sender
├── Dockerfile           # Multi-stage Docker build
├── go.mod               # Go module dependencies
├── go.sum               # Dependency checksums
├── .env                 # Environment variables (local)
├── .github/
│   └── workflows/
│       └── docker-build.yml  # GitHub Actions CI/CD
└── README.md            # This file
```

## 🛠️ Built With

- **[Go 1.21](https://golang.org/)** - Programming language
- **[Gin](https://gin-gonic.com/)** - Web framework
- **[gowol](https://github.com/linde12/gowol)** - Wake-on-LAN implementation
- **[godotenv](https://github.com/joho/godotenv)** - .env file loader
- **[Docker](https://www.docker.com/)** - Containerization
- **[GitHub Actions](https://github.com/features/actions)** - CI/CD

## 📝 Example Usage

### Send to default device
```
curl http://localhost:8080/wake
```

### Send to specific device
```
curl http://localhost:8080/wake/00:1A:2B:3C:4D:5E
```

### Check API status
```
curl http://localhost:8080/
```

## 🤝 Development

### Run tests locally

```
go test ./...
```

### Format code

```
go fmt ./...
```

### Lint code

```
golangci-lint run
```

## 🔒 Security Notes

- This API doesn't include authentication - consider adding it for production use
- Keep your `.env` file private and never commit it to version control
- Consider running this service only on your local network

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 👤 Author

**Cléry Arque-Ferradou**
- GitHub: [@cleeryy](https://github.com/cleeryy)
- Repository: [cleeryy/hello](https://github.com/cleeryy/hello)

## 🙏 Acknowledgments

- Inspired by the need for a lightweight, Dockerized Wake-on-LAN service
- Built with ❤️ using Go and Gin

---

**Happy waking! 🌙**
