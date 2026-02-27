# VMDocker Container

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.19+-blue.svg)](https://golang.org/)
[![Docker](https://img.shields.io/badge/docker-supported-blue.svg)](https://www.docker.com/)

VMDocker Container is a Docker-based runtime environment designed to execute computational tasks for `HyMatrix`, working seamlessly with `Vmdocker` for distributed computing scenarios.

More about HyMatrix & Vmdocker:
> - [Vmdocker](https://github.com/cryptowizard0/vmdocker)
> - [HyMatrix Website](https://hymatrix.com/)
> - [HyMatrix Documentation](https://docs.hymatrix.com/)

## 🚀 Features

- **Multi-Runtime Support**: Supports multiple execution environments
- **Docker Integration**: Containerized deployment for consistency and scalability
- **RESTful API**: Clean and intuitive API endpoints
- **Production Ready**: Built with Go for high performance and reliability

## 📋 Supported Runtimes

| Runtime | Description |
|---------|---------|
| [AOS](https://github.com/cryptowizard0/aos) | AOS v2.0.1 env|
| Ollama  | Large Language Model serving runtime |
| Test    | Lightweight in-memory runtime for message-path verification (`RUNTIME_TYPE=test`) |

## 🐳 Quick Start with Docker

### Prerequisites

- Docker installed and running
- Go 1.19+ (for local development)

### Build Docker Image

```bash
# Build GoLua runtime image
./docker_build.sh golua <VERSION>

# Build Ollama runtime image
./docker_build.sh ollama <VERSION>
```

**Parameters:**
- `golua|ollama`: Runtime type to build
- `<VERSION>`: Image version tag (e.g., v1.0.0, latest, dev)

**Examples:**
```bash
# Build with specific version
./docker_build.sh golua v1.0.0
./docker_build.sh ollama v2.1.0

# Build with latest tag
./docker_build.sh golua latest
```

### Run Container

```bash
./docker_run.sh
```

The container will start and expose the API on the configured port.

## 🛠️ Local Development

### Running Locally

```bash
# Run directly with Go
go run -tags=lua53 main.go

# Or build and run binary
go build -tags=lua53 -o vmdocker-container
./vmdocker-container
```

### Testing

```bash
# Run all tests
go test -tags=lua53 -v ./...

# Run tests with coverage
go test -tags=lua53 -v -cover ./...
```


## 🏗️ Project Structure

```
.
├── ao/                 # AO runtime files
├── common/             # Shared utilities
├── runtime/            # Runtime implementations
│   ├── runtime_ollama/ # Ollama runtime
│   └── runtime_vmgolua/# GoLua runtime
├── server/             # HTTP server implementation
├── utils/              # Helper utilities
├── Dockerfile.*        # Docker build files
├── docker_build.sh     # Build script
├── docker_run.sh       # Run script
└── main.go            # Application entry point
```

## 🤝 Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🔗 Related Projects

- [Hymx](https://github.com/cryptowizard0/hymx) - The main computation framework
- [Vmdocker](https://github.com/cryptowizard0/vmdocker) - Container orchestration system
- [AOS](https://github.com/cryptowizard0/aos) - Actor Oriented System
