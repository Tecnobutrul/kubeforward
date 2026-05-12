# Kubeforward

![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)
![Kubernetes](https://img.shields.io/badge/kubernetes-%23326ce5.svg?style=for-the-badge&logo=kubernetes&logoColor=white)
![GitHub Actions](https://img.shields.io/badge/github%20actions-%232671E5.svg?style=for-the-badge&logo=githubactions&logoColor=white)
![GitHub Release](https://img.shields.io/github/v/release/Tecnobutrul/kubeforward?style=for-the-badge)
![GPLv3](https://img.shields.io/badge/license-GPLv3-blue.svg?style=for-the-badge)

kubeforward is a command line utility built to port forward some or all pods within a Kubernetes namespace. kubeforward uses the same port exposed by the service and forwards it from a loopback IP address on your local workstation. It loads all the configurations from a yml file so the configuration is quite easy.

When developing on our local workstation, you often build applications that need to access services through ports within a Kubernetes namespace. kubefwd allows us to develop locally with services available as we would be in the cluster.

## Features

- Forward multiple deployments concurrently via goroutines
- Auto-retry on pod restart (waits for the new pod and reconnects)
- `--context` flag for multi-cluster support
- Graceful shutdown on Ctrl+C (SIGINT/SIGTERM)
- YAML configuration file or CLI arguments (`name:host:pod`)
- `--quiet` and `--verbose` execution modes
- Multi-architecture Docker image (linux/amd64, linux/arm64)

## Installation

```shell
go install github.com/Tecnobutrul/kubeforward@latest
```

Or build from source:

```shell
git clone https://github.com/Tecnobutrul/kubeforward.git
cd kubeforward
go build -o build/kubeforward .
```

## Usage

```shell
kubeforward [--context <context>] [--quiet] [--verbose] [--file=<path>] <deploy>:<host_port>:<pod_port> [...]
```

Examples:

```shell
# Using a config file
kubeforward --file=deploy.yaml

# Using a different kubectl context
kubeforward --context my-cluster --file=deploy.yaml

# From CLI arguments
kubeforward myapp:8080:80 auth:3000:3000

# Combined
kubeforward --context prod --file=config.yml api:8080:80
```

Config file format (`deploy.yaml`):

```yml
deploy:
  - name: myapp
    hostport: 8080
    podport: 80

  - name: auth
    hostport: 3000
    podport: 3000
```

## Build

The binary outputs to `build/`:

```shell
go build -o build/kubeforward .
```

## Docker image (multi-arch)

Build and push to Docker Hub with podman:

```bash
podman login docker.io
podman manifest create kubeforward:latest
podman build --build-arg KUBELOGIN_VERSION=v0.2.17 --platform linux/amd64 --manifest kubeforward:latest .
podman build --build-arg KUBELOGIN_VERSION=v0.2.17 --platform linux/arm64 --manifest kubeforward:latest .
podman manifest push kubeforward:latest docker.io/frandieguez/kubeforward:latest
```

## Maintainers

- [Fran Dieguez](https://github.com/frandieguez)

## LICENSE

[GPLv3](https://www.gnu.org/licenses/gpl-3.0.html)
