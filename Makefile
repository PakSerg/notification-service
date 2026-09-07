BINARY := notihub
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all build run test test-race test-integration cover lint tidy docker-build docker-up docker-down clean

all: lint test build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/notihub

run:
	go run ./cmd/notihub

test:
	go test ./...

# The race detector needs cgo, so this one requires a C toolchain.
test-race:
	CGO_ENABLED=1 go test -race ./...

# Spins up a throwaway PostgreSQL container so the repository package's
# integration tests (skipped otherwise) actually run.
test-integration:
	docker compose up -d db
	NOTIHUB_TEST_DATABASE_URL="postgres://notihub:notihub@localhost:5432/notihub?sslmode=disable" go test ./... || (docker compose stop db && exit 1)
	docker compose stop db

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

lint:
	go vet ./...
	gofmt -l .

tidy:
	go mod tidy

docker-build:
	docker build --build-arg VERSION=$(VERSION) -t $(BINARY):$(VERSION) .

docker-up:
	VERSION=$(VERSION) docker compose up --build -d

docker-down:
	docker compose down

clean:
	rm -rf bin coverage.out
