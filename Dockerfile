# Dockerfile References: https://docs.docker.com/engine/reference/builder/
FROM ubuntu:latest as ODINBUILDER

# From https://github.com/docker-library/golang
RUN apt-get update && \
  apt-get install -y --no-install-recommends \
  llvm \
  git \
  make \
  clang  \
  apt-transport-https \
  ca-certificates \
  && rm -rf /var/lib/apt/lists/*
RUN update-ca-certificates

RUN git clone https://github.com/odin-lang/Odin.git
RUN cd Odin && make release

# Start from the latest golang base image
FROM golang:latest

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
RUN apt-get update && \
  apt-get install -y --no-install-recommends \
  llvm \
  && rm -rf /var/lib/apt/lists/*

# Copy the source from the current directory to the Working Directory inside the container
COPY . .
COPY --from=ODINBUILDER /Odin/core/ core/
COPY --from=ODINBUILDER /Odin/odin .
ENV PATH="/app:${PATH}"

# Build the Go app
RUN go build -ldflags "-X main.versionStr=$GIT_COMMIT -X main.buildTimeStr=$BUILD_TIME" -o vpbot .

# Command to run the executable
ENTRYPOINT ["./vpbot"] 