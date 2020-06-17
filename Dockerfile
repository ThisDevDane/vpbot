# Dockerfile References: https://docs.docker.com/engine/reference/builder/

# Start from the latest golang base image
FROM golang:latest as builder

# Add Maintainer Info
LABEL maintainer="Mikkel Hjortshoej <hoej@northwolfprod.com>"

ARG GIT_COMMIT=DEV
ARG BUILD_TIME=DEV

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source from the current directory to the Working Directory inside the container
COPY . .

# Build the Go app
RUN go build -ldflags "-X main.versionStr=$GIT_COMMIT -X main.buildTimeStr=$BUILD_TIME" -o vpbot .

# From https://github.com/docker-library/golang
RUN apt-get update && \
  apt-get install -y --no-install-recommends \
  odin \
  && rm -rf /var/lib/apt/lists/*

# Command to run the executable
ENTRYPOINT ["./vpbot"] 