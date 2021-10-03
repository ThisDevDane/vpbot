################
## ODIN Build
#FROM ubuntu:latest as ODINBUILDER
#
#RUN apt-get update && \
#  apt-get install -y --no-install-recommends \
#  llvm-11 \
#  llvm-11-dev \
#  git \
#  make \
#  clang  \
#  apt-transport-https \
#  ca-certificates \
#  && rm -rf /var/lib/apt/lists/*
#RUN update-ca-certificates
#
#RUN git clone --depth=1 https://github.com/odin-lang/Odin.git
#RUN cd Odin && make release

###############
# VPBOT Build
FROM golang:latest as builder

ARG GIT_COMMIT=DEV
ARG BUILD_TIME=DEV
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go build -ldflags "-X main.versionStr=$GIT_COMMIT -X main.buildTimeStr=$BUILD_TIME" -o vpbot .

###############
# VPBOT Runner
FROM ubuntu:latest
WORKDIR /app

LABEL maintainer="Mikkel Hjortshoej <hoej@northwolfprod.com>"

RUN apt-get update && \
  apt-get install -y --no-install-recommends \
  llvm-10 \
  clang-10 \
  ca-certificates \
  && rm -rf /var/lib/apt/lists/*
RUN update-ca-certificates

#COPY --from=ODINBUILDER /Odin/core/ core/
#COPY --from=ODINBUILDER /Odin/odin .
COPY --from=builder /app/vpbot .

ENV PATH="/app:${PATH}"

# Command to run the executable
ENTRYPOINT ["./vpbot"] 
