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
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o kubeforward .

RUN apt-get update

RUN apt install -y wget

RUN apt install -y unzip

RUN wget https://github.com/Azure/kubelogin/releases/download/v0.0.29/kubelogin-linux-amd64.zip

RUN unzip -j kubelogin-linux-amd64.zip bin/linux_amd64/kubelogin -d /usr/local/bin

FROM bitnami/kubectl:latest

COPY --from=build /app/kubeforward /usr/local/bin/

COPY --from=build /usr/local/bin/kubelogin /usr/local/bin/

USER root

# Command to run the executable
ENTRYPOINT ["/usr/local/bin/kubeforward","--file=port-forward.yml"]
