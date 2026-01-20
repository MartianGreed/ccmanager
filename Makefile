.PHONY: build run test lint clean install uninstall

BINARY_NAME=ccmanager
BUILD_DIR=./bin
INSTALL_DIR=$(HOME)/.local/bin

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

install: build
	@mkdir -p $(INSTALL_DIR)
	@ln -sf $(CURDIR)/$(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Linked $(INSTALL_DIR)/$(BINARY_NAME) -> $(CURDIR)/$(BUILD_DIR)/$(BINARY_NAME)"

uninstall:
	@rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Removed $(INSTALL_DIR)/$(BINARY_NAME)"
