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
COPY . .

# Build the Go app
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o kubeforward .

FROM alpine:latest

WORKDIR /root

COPY --from=build /app/kubeforward .

# Command to run the executable
CMD ["./kubeforward"]

