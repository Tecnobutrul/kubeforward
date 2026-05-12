# Dockerfile References: https://docs.docker.com/engine/reference/builder/

# Start from the latest golang base image
FROM golang:latest as build

# Add Maintainer Info
LABEL maintainer="Fran Dieguez <fran.dieguez@mabishu.com>"

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY kubeforward.go .

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o build/kubeforward . && \
    rm -rf $GOCACHE $GOPATH/pkg/mod

ARG KUBELOGIN_VERSION=v0.2.17
ARG TARGETARCH

RUN apt-get update && \
    apt-get install -y --no-install-recommends wget unzip && \
    case ${TARGETARCH} in \
        arm64|aarch64) \
            KUBELOGIN_URL="https://github.com/Azure/kubelogin/releases/download/${KUBELOGIN_VERSION}/kubelogin-linux-arm64.zip" ;; \
        *) \
            KUBELOGIN_URL="https://github.com/Azure/kubelogin/releases/download/${KUBELOGIN_VERSION}/kubelogin-linux-amd64.zip" ;; \
    esac && \
    wget -O kubelogin.zip "$KUBELOGIN_URL" && \
    unzip -j kubelogin.zip "*/kubelogin" -d /usr/local/bin && \
    rm -rf kubelogin.zip /var/lib/apt/lists/*

FROM bitnami/kubectl:latest

COPY --from=build /app/build/kubeforward /usr/local/bin/

COPY --from=build /usr/local/bin/kubelogin /usr/local/bin/

USER root

# Command to run the executable
ENTRYPOINT ["/usr/local/bin/kubeforward","--file=port-forward.yml"]
