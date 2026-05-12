# Changelog

## [Unreleased]

### Added

- `--context` flag to select kubectl context
- Graceful shutdown on SIGINT/SIGTERM with context cancellation
- Multi-arch Docker image support (linux/amd64, linux/arm64)
- `KUBELOGIN_VERSION` build arg in Dockerfile

### Changed

- Separated `quiet` and `verbose` flags into independent variables
- Refactored `getConfFile` to return error instead of calling `log.Fatalf`

### Added

- Comprehensive test suite with 49 tests covering validations, config parsing, pod lookup, and graceful shutdown
- GitHub Actions CI workflow (`go test` on push/PR)
- `.github/workflows/test.yml`

### Removed

- Removed `io/ioutil` import (deprecated since Go 1.16)

## [0.9.0] - 2026-05-12

### Added

- `.tool-versions` file for mise/go version management

### Changed

- Updated Go version from 1.15 to 1.26
- Optimized Dockerfile for multi-architecture (arm64/amd64) with `ARG TARGETARCH`
- Cleaned up Go build cache (`$GOCACHE`, `$GOPATH/pkg/mod`) and apt cache in Dockerfile
- Updated kubelogin from v0.0.29 to v0.2.17 with `ARG KUBELOGIN_VERSION`
- Consolidated apt-get, wget, and unzip into single RUN layer to reduce image layers

## [0.4.0] - 2024-05-17

### Changed

- Added `--address 0.0.0.0` flag to kubectl port-forward for external accessibility

## [0.3.0] - 2023-06-01

### Added

- Installed kubelogin in Docker image for Azure AKS authentication support

### Fixed

- Removed duplicated ENTRYPOINT in Dockerfile

## [0.2.0] - 2020-10-30

### Added

- Go modules support (`go.mod`, `go.sum`)
- Multi-stage Dockerfile for optimized image size

## [0.1.0] - 2019-12-01

### Added

- `--verbose` and `--quiet` execution modes

### Fixed

- Removed hardcoded namespace when fetching pods, uses default kubeconfig namespace

## [0.0.1] - 2019-10-09

### Added

- Initial implementation with `kubectl port-forward` orchestration
- YAML configuration file support (`deploy.yaml`)
- Port forwarding for multiple deployments concurrently via goroutines
- Auto-retry on pod failure (waits for pod restart)
- CLI arguments support: `<deploy_name>:<host_port>:<pod_port>`
- Help message and usage information
- Validación de formato de argumentos con regex
- README with installation and usage instructions
