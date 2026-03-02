# VMDocker Container

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://golang.org/)
[![Docker](https://img.shields.io/badge/docker-supported-blue.svg)](https://www.docker.com/)

VMDocker Container is a Docker-based runtime environment designed to execute computational tasks for `HyMatrix`, working seamlessly with `Vmdocker` for distributed computing scenarios.

More about HyMatrix & Vmdocker:
> - [Vmdocker](https://github.com/cryptowizard0/vmdocker)
> - [HyMatrix Website](https://hymatrix.com/)
> - [HyMatrix Documentation](https://docs.hymatrix.com/)

## 🚀 Features

- **Runtime Modes**: Supports `openclaw` and in-memory `test` runtimes via `RUNTIME_TYPE`
- **Docker Integration**: Containerized deployment for consistency
- **RESTful API**: `/vmm/health`, `/vmm/spawn`, `/vmm/apply`

## 🐳 Quick Start with Docker

### Prerequisites

- Docker installed and running
- Go 1.24+ (for local development)

### Build Docker Image

```bash
# Build image
./docker_build.sh <VERSION>
```

**Parameters:**
- `<VERSION>`: Image version tag (e.g., v1.0.0, latest, dev)

**Examples:**
```bash
# Build with specific version
./docker_build.sh v1.0.0

# Build with latest tag
./docker_build.sh latest
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
go run main.go

# Or build and run binary
go build -o vmdocker-container
./vmdocker-container
```

### Testing

```bash
# Run all tests
go test -v ./...

# Run tests with coverage
go test -v -cover ./...
```


## 🏗️ Project Structure

```
.
├── common/             # Shared utilities
├── runtime/            # Runtime implementations
│   ├── openclaw/        # Openclaw runtime
│   └── testrt/          # In-memory test runtime
├── server/             # HTTP server implementation
├── utils/              # Helper utilities
├── Dockerfile          # Docker build file
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
