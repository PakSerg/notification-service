# syntax=docker/dockerfile:1

ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /src

# Dependencies change far less often than the code, so download them first
# and let Docker reuse the layer on every subsequent build.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

ARG VERSION=dev

# The SQLite driver is pure Go, so a fully static binary needs no CGO.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build \
        -trimpath \
        -ldflags "-s -w -X main.version=${VERSION}" \
        -o /out/notihub ./cmd/notihub

FROM alpine:3.21

# ca-certificates for outgoing HTTPS (webhooks), tzdata so timestamps in logs
# are not stuck in UTC when TZ is set, wget for the healthcheck below.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser --disabled-password --no-create-home --uid 10001 notihub \
    && mkdir -p /data \
    && chown notihub:notihub /data

COPY --from=build /out/notihub /usr/local/bin/notihub

USER notihub

# /data is the volume mount point, so the database survives container replacement.
WORKDIR /data
VOLUME ["/data"]

ENV NOTIHUB_HTTP_ADDR=:8080 \
    NOTIHUB_DB_PATH=/data/notihub.db

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --spider http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["notihub"]
