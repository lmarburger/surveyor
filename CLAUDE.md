# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Surveyor is a Prometheus metrics exporter that collects signal statistics from Surfboard cable modems. It exposes cable modem signal quality metrics (SNR, power levels, error counts) as Prometheus metrics for monitoring and visualization.

## Architecture

The codebase follows a clean Go architecture:
- `main.go`: Entry point that sets up the HTTP server and Prometheus metrics endpoint
- `surveyor/`: Core package containing:
  - `hnap.go`: HNAP (Home Network Administration Protocol) client for modem authentication
  - `channelinfo.go`: Parser for channel signal data from the modem
  - `report.go`: Prometheus collector implementation that exposes metrics

The application uses HNAP protocol with HMAC-MD5 authentication to securely communicate with cable modems and parse their signal data.

## Development Commands

### Build and Run
```bash
# Run locally
go run main.go

# Build binary
go build -o surveyor

# Run with custom address
go run main.go -addr :8080
```

### Testing
```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -cover ./...

# Run tests for specific package
go test -v ./surveyor/...
```

### Code Quality
```bash
# Format code
go fmt ./...

# Vet code for common mistakes
go vet ./...

# Run staticcheck
staticcheck ./...
```

### Docker Development
```bash
# Build and run with docker-compose
docker compose up --build

# Run in detached mode
docker compose up -d

# Build Docker image directly
docker build -t surveyor .
```

## Important Notes

1. **Metrics Endpoint**: Prometheus metrics are exposed at `/metrics` on the configured port.

2. **Target Device**: Default target is a Surfboard SB6141 modem at `https://192.168.100.1/HNAP1/`.

3. **Dependencies**: Uses Go 1.22 with minimal external dependencies (Prometheus client, goquery for HTML parsing, testify for testing).

4. **Testing**: Tests use testify assertions. Always run tests before committing changes to ensure the HNAP client and channel info parser work correctly.