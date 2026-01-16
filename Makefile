.PHONY: build run test lint clean

BINARY_NAME=ccmanager
BUILD_DIR=./bin

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/ccmanager

run: build
	$(BUILD_DIR)/$(BINARY_NAME)

test:
	go test -v ./...

lint:
	golangci-lint run

clean:
	rm -rf $(BUILD_DIR)
	go clean

deps:
	go mod tidy

dev: build
	$(BUILD_DIR)/$(BINARY_NAME)

run-debug: build
	CCMANAGER_DEBUG=1 $(BUILD_DIR)/$(BINARY_NAME)
