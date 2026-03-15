.PHONY: build test vet lint e2e clean

BINARY_NAME := slack-cli
BUILD_DIR := .

VERSION ?= dev

build:
	go build -ldflags "-X github.com/natikgadzhi/slack-cli/internal/commands.Version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/slack-cli

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
