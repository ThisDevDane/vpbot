FROM golang:1.20 as builder

ARG GIT_COMMIT=DEV
ARG BUILD_TIME=DEV
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN go build -ldflags "-X main.versionStr=$GIT_COMMIT -X main.buildTimeStr=$BUILD_TIME" -o vpbot .

FROM alpine:latest
WORKDIR /app

COPY --from=builder /app/vpbot .

ENTRYPOINT ["./vpbot"] 
