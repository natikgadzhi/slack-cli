.PHONY: build test vet lint e2e clean

BINARY_NAME := slack-cli
BUILD_DIR := .

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/slack-cli

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

e2e:
	go test -tags e2e -v ./tests/

clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME)
	rm -rf dist/
