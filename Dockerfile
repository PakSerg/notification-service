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

# The pgx and kafka-go drivers are pure Go, so a fully static binary needs no
# CGO. Both the API (notihub) and the worker (notihub-worker) are built into
# the same image; compose.yaml picks which one a given container runs via
# its entrypoint.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build \
        -trimpath \
        -ldflags "-s -w -X main.version=${VERSION}" \
        -o /out/ ./cmd/...

FROM alpine:3.21

# ca-certificates for outgoing HTTPS (webhooks + TLS to Postgres/Kafka),
# tzdata so timestamps in logs are not stuck in UTC when TZ is set, wget for
# the healthcheck below.
RUN apk add --no-cache ca-certificates tzdata \
    && adduser --disabled-password --no-create-home --uid 10001 notihub

COPY --from=build /out/notihub /out/notihub-worker /usr/local/bin/

USER notihub

# Point at the `db` and `kafka` services from compose.yaml by default;
# override NOTIHUB_DATABASE_URL/NOTIHUB_KAFKA_BROKERS to run against
# different instances.
ENV NOTIHUB_HTTP_ADDR=:8080 \
    NOTIHUB_DATABASE_URL="postgres://notihub:notihub@db:5432/notihub?sslmode=disable" \
    NOTIHUB_KAFKA_BROKERS="kafka:19092"

EXPOSE 8080

# Only meaningful for the notihub (API) service; compose.yaml disables it for
# notihub-worker, which exposes no HTTP port.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --spider http://127.0.0.1:8080/health || exit 1

ENTRYPOINT ["notihub"]
