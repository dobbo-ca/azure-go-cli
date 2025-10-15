.PHONY: build all clean test help

# Binary name
BINARY_NAME=az
CMD_PATH=./cmd/az
OUTPUT_DIR=bin/az

# Get current OS and architecture
GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)

# Version information
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(BUILD_DATE)"

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build for current OS/architecture
	@echo "Building $(BINARY_NAME) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(OUTPUT_DIR)
	@go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME) $(CMD_PATH)/main.go
	@echo "Binary created: $(OUTPUT_DIR)/$(BINARY_NAME)"

all: clean ## Build for all supported platforms
	@echo "Building $(BINARY_NAME) for all platforms..."
	@mkdir -p $(OUTPUT_DIR)
	@echo "Building for linux/amd64..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_PATH)/main.go
	@echo "Building for linux/arm64..."
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_PATH)/main.go
	@echo "Building for darwin/amd64..."
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH)/main.go
	@echo "Building for darwin/arm64..."
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH)/main.go
	@echo "Building for windows/amd64..."
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_PATH)/main.go
	@echo "Building for windows/arm64..."
	@GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME)-windows-arm64.exe $(CMD_PATH)/main.go
	@echo "All binaries created in $(OUTPUT_DIR)/"
	@ls -lh $(OUTPUT_DIR)/

clean: ## Remove built binaries
	@echo "Cleaning build artifacts..."
	@rm -rf $(OUTPUT_DIR)/$(BINARY_NAME)*
	@echo "Clean complete"

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

install: build ## Install binary to system
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp $(OUTPUT_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "Install complete. Run '$(BINARY_NAME)' to verify."

# Default target
.DEFAULT_GOAL := build
