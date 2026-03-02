# VMDocker Container

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.19+-blue.svg)](https://golang.org/)
[![Docker](https://img.shields.io/badge/docker-supported-blue.svg)](https://www.docker.com/)

VMDocker Container is a Docker-based runtime environment designed to execute computational tasks for **HyMatrix**, working seamlessly with **Vmdocker** for distributed computing scenarios.

## 🔗 Related Projects

| Project | Description | Link |
|---------|-------------|------|
| **Vmdocker** | Container orchestration system | [GitHub](https://github.com/cryptowizard0/vmdocker) |
| **HyMatrix** | Distributed computation framework | [Website](https://hymatrix.com/) |
| **AOS** | Actor Oriented System | [GitHub](https://github.com/cryptowizard0/aos) |

---

## 📋 Table of Contents

- [Features](#-features)
- [Quick Start](#-quick-start)
- [API Reference](#-api-reference)
- [Development](#-development)
- [Configuration](#-configuration)
- [Contributing](#-contributing)
- [License](#-license)

---

## ✨ Features

- **🧪 Test Runtime**: In-memory test runtime for protocol and message-path verification
- **🐳 Docker Integration**: Containerized deployment for consistency across environments
- **🔌 RESTful API**: Simple HTTP API for lifecycle management
- **⚡ Lightweight**: Minimal dependencies, fast startup
- **🔒 Type-Safe**: Protocol buffer schemas for message passing

---

## 🚀 Quick Start

### Prerequisites

- [Docker](https://www.docker.com/) installed and running
- [Go 1.19+](https://golang.org/) (for local development)

### Option 1: Using Docker (Recommended)

```bash
# 1. Clone the repository
git clone https://github.com/cryptowizard0/vmdocker_agent.git
cd vmdocker_agent

# 2. Build Docker image
./docker_build.sh latest

# 3. Run container
./docker_run.sh
```

The API will be available at `http://localhost:8080`.

### Option 2: Local Development

```bash
# 1. Clone and enter directory
git clone https://github.com/cryptowizard0/vmdocker_agent.git
cd vmdocker_agent

# 2. Install dependencies
go mod download

# 3. Run directly
go run main.go

# Or build binary
go build -o vmdocker-container
./vmdocker-container
```

---

## 📡 API Reference

### Base URL
```
http://localhost:8080/vmm
```

### Endpoints

#### 1. Health Check
Check if the service is running.

```http
POST /vmm/health
```

**Response:**
```json
{
  "status": "ok"
}
```

#### 2. Spawn Runtime
Initialize a new runtime instance.

```http
POST /vmm/spawn
Content-Type: application/json
```

**Request Body:**
```json
{
  "pid": "process-001",
  "owner": "user-123",
  "cuAddr": "cu-456",
  "data": {},
  "tags": ["test", "demo"],
  "evn": {
    "runtime": "test"
  }
}
```

**Response:**
```json
{
  "status": "ok"
}
```

**Error Response:**
```json
{
  "status": "error",
  "msg": "runtime is not nil"
}
```

#### 3. Apply Action
Execute an action on the spawned runtime.

```http
POST /vmm/apply
Content-Type: application/json
```

**Request Body:**
```json
{
  "from": "target-1",
  "meta": {
    "action": "Ping",
    "sequence": 7
  },
  "params": {
    "Action": "Ping",
    "Reference": "7"
  }
}
```

**Response:**
```json
{
  "status": "ok",
  "result": "{\"data\": \"Pong\", \"messages\": [...]}"
}
```

### Example Usage with cURL

```bash
# Health check
curl -X POST http://localhost:8080/vmm/health

# Spawn runtime
curl -X POST http://localhost:8080/vmm/spawn \
  -H "Content-Type: application/json" \
  -d '{
    "pid": "test-pid",
    "owner": "test-owner",
    "cuAddr": "test-cu"
  }'

# Apply action
curl -X POST http://localhost:8080/vmm/apply \
  -H "Content-Type: application/json" \
  -d '{
    "from": "test-target",
    "meta": {"action": "Ping", "sequence": 1},
    "params": {"Action": "Ping"}
  }'
```

---

## 🛠️ Development

### Project Structure

```
.
├── ao/                      # AO runtime files (v2.0.1)
│   ├── test/               # Test suites
│   ├── handlers.md         # Handler documentation
│   └── README.md           # AO specific docs
├── common/                  # Shared utilities and logging
├── runtime/                 # Runtime implementations
│   ├── runtime.go          # Runtime interface
│   ├── runtime_testrt/     # Test runtime implementation
│   └── schema/             # Protocol schemas
├── server/                  # HTTP server implementation
│   ├── server.go           # Server setup and lifecycle
│   ├── api.go              # API handlers
│   └── api_test.go         # API tests
├── utils/                   # Helper utilities
├── Dockerfile.testrt       # Docker build configuration
├── docker_build.sh         # Docker build script
├── docker_run.sh           # Docker run script
├── main.go                 # Application entry point
├── go.mod                  # Go module definition
└── readme.md               # This file
```

### Running Tests

```bash
# Run all tests
go test -v ./...

# Run tests with coverage
go test -v -cover ./...

# Run specific package tests
go test -v ./server/...
```

### Building

```bash
# Build for current platform
go build -o vmdocker-container

# Build for Linux (Docker)
GOOS=linux GOARCH=amd64 go build -o vmdocker-container-linux
```

---

## ⚙️ Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `AO_PATH` | Path to AO runtime files | `./ao/2.0.1` |
| `RUNTIME_TYPE` | Runtime type (test, etc.) | `test` |
| `PORT` | HTTP server port | `8080` |

### Example with Custom Config

```bash
# Set environment variables
export AO_PATH=/custom/ao/path
export PORT=9090

# Run
./vmdocker-container
```

---

## 🤝 Contributing

We welcome contributions! Here's how to get started:

1. **Fork** the repository
2. **Create** your feature branch:
   ```bash
   git checkout -b feature/amazing-feature
   ```
3. **Make** your changes
4. **Test** your changes:
   ```bash
   go test -v ./...
   ```
5. **Commit** with clear messages:
   ```bash
   git commit -m "Add: amazing feature description"
   ```
6. **Push** to your branch:
   ```bash
   git push origin feature/amazing-feature
   ```
7. **Open** a Pull Request

### Commit Message Convention

- `Add:` - New features
- `Fix:` - Bug fixes
- `Docs:` - Documentation changes
- `Refactor:` - Code refactoring
- `Test:` - Test additions/changes

---

## 📄 License

This project is licensed under the **MIT License** - see the [LICENSE](LICENSE) file for details.

---

## 🆘 Support

If you encounter any issues or have questions:

1. Check the [HyMatrix Documentation](https://docs.hymatrix.com/)
2. Open an issue on [GitHub](https://github.com/cryptowizard0/vmdocker_agent/issues)
3. Join the community discussions

---

<p align="center">
  Built with ❤️ for the HyMatrix ecosystem
</p>
