.PHONY: build test vet lint e2e clean

BINARY_NAME := slack-cli
BUILD_DIR := .

VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/natikgadzhi/slack-cli/internal/commands.Version=$(VERSION) \
           -X github.com/natikgadzhi/slack-cli/internal/commands.Commit=$(COMMIT) \
           -X github.com/natikgadzhi/slack-cli/internal/commands.Date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/slack-cli

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

e2e:
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	go test -tags e2e -v -timeout 120s ./tests/

clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME)
	rm -rf dist/
