# k13s Makefile
# Supports cross-compilation for air-gapped/offline environments

APP_NAME := k13s
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOCLEAN := $(GOCMD) clean
GOMOD := $(GOCMD) mod

# Build directories
BUILD_DIR := build
DIST_DIR := dist

# Supported platforms for air-gapped environments
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	linux/arm \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

.PHONY: all build clean test lint deps help
.PHONY: build-all build-linux build-darwin build-windows
.PHONY: package package-all docker

# Default target
all: clean deps test build

# Build for current platform
build:
	@echo "Building $(APP_NAME) for current platform..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./cmd/kube-ai-dashboard-cli/main.go
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)"

# Build for all platforms
build-all: clean
	@echo "Building $(APP_NAME) for all platforms..."
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		output="$(DIST_DIR)/$(APP_NAME)-$$os-$$arch"; \
		if [ "$$os" = "windows" ]; then output="$$output.exe"; fi; \
		echo "Building $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $$output ./cmd/kube-ai-dashboard-cli/main.go || exit 1; \
	done
	@echo "All builds complete in $(DIST_DIR)/"

# Build for Linux only (common for Kubernetes environments)
build-linux:
	@echo "Building $(APP_NAME) for Linux..."
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-linux-amd64 ./cmd/kube-ai-dashboard-cli/main.go
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-linux-arm64 ./cmd/kube-ai-dashboard-cli/main.go
	GOOS=linux GOARCH=arm CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-linux-arm ./cmd/kube-ai-dashboard-cli/main.go
	@echo "Linux builds complete"

# Build for macOS
build-darwin:
	@echo "Building $(APP_NAME) for macOS..."
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-darwin-amd64 ./cmd/kube-ai-dashboard-cli/main.go
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-darwin-arm64 ./cmd/kube-ai-dashboard-cli/main.go
	@echo "macOS builds complete"

# Build for Windows
build-windows:
	@echo "Building $(APP_NAME) for Windows..."
	@mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(APP_NAME)-windows-amd64.exe ./cmd/kube-ai-dashboard-cli/main.go
	@echo "Windows build complete"

# Create distribution packages with checksums
package: build-all
	@echo "Creating distribution packages..."
	@cd $(DIST_DIR) && \
	for f in $(APP_NAME)-*; do \
		if [ -f "$$f" ]; then \
			tar -czvf "$$f.tar.gz" "$$f" 2>/dev/null || zip "$$f.zip" "$$f"; \
		fi \
	done
	@cd $(DIST_DIR) && sha256sum $(APP_NAME)-* > checksums.txt 2>/dev/null || shasum -a 256 $(APP_NAME)-* > checksums.txt
	@echo "Packages created in $(DIST_DIR)/"

# Create offline bundle (includes dependencies)
bundle-offline: deps
	@echo "Creating offline bundle..."
	@mkdir -p $(DIST_DIR)/offline-bundle
	@$(GOMOD) vendor
	@cp -r vendor $(DIST_DIR)/offline-bundle/
	@cp go.mod go.sum $(DIST_DIR)/offline-bundle/
	@cp -r cmd pkg $(DIST_DIR)/offline-bundle/
	@cp Makefile $(DIST_DIR)/offline-bundle/
	@echo "# Offline Build Instructions" > $(DIST_DIR)/offline-bundle/BUILD.md
	@echo "" >> $(DIST_DIR)/offline-bundle/BUILD.md
	@echo "1. Copy this directory to your air-gapped environment" >> $(DIST_DIR)/offline-bundle/BUILD.md
	@echo "2. Run: go build -mod=vendor -o k13s ./cmd/kube-ai-dashboard-cli/main.go" >> $(DIST_DIR)/offline-bundle/BUILD.md
	@echo "" >> $(DIST_DIR)/offline-bundle/BUILD.md
	@echo "Or use Makefile:" >> $(DIST_DIR)/offline-bundle/BUILD.md
	@echo "  make build-offline" >> $(DIST_DIR)/offline-bundle/BUILD.md
	@tar -czvf $(DIST_DIR)/$(APP_NAME)-offline-bundle-$(VERSION).tar.gz -C $(DIST_DIR) offline-bundle
	@rm -rf $(DIST_DIR)/offline-bundle vendor
	@echo "Offline bundle created: $(DIST_DIR)/$(APP_NAME)-offline-bundle-$(VERSION).tar.gz"

# Build from offline bundle (for air-gapped environments)
build-offline:
	@echo "Building from vendored dependencies..."
	$(GOBUILD) -mod=vendor $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./cmd/kube-ai-dashboard-cli/main.go

# Docker build
docker:
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .

# Docker multi-arch build
docker-multiarch:
	@echo "Building multi-arch Docker image..."
	docker buildx build --platform linux/amd64,linux/arm64 -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@mkdir -p $(BUILD_DIR)
	$(GOTEST) -v -race -coverprofile=$(BUILD_DIR)/coverage.out ./...
	$(GOCMD) tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html
	@echo "Coverage report: $(BUILD_DIR)/coverage.html"

# Lint
lint:
	@echo "Running linters..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	golangci-lint run ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR) $(DIST_DIR) vendor

# Install locally
install: build
	@echo "Installing $(APP_NAME)..."
	cp $(BUILD_DIR)/$(APP_NAME) /usr/local/bin/$(APP_NAME)
	@echo "Installed to /usr/local/bin/$(APP_NAME)"

# Uninstall
uninstall:
	@echo "Uninstalling $(APP_NAME)..."
	rm -f /usr/local/bin/$(APP_NAME)

# Show version info
version:
	@echo "Version: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

# Help
help:
	@echo "k13s Makefile - AI-Powered Kubernetes Dashboard"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build targets:"
	@echo "  build          Build for current platform"
	@echo "  build-all      Build for all supported platforms"
	@echo "  build-linux    Build for Linux (amd64, arm64, arm)"
	@echo "  build-darwin   Build for macOS (amd64, arm64)"
	@echo "  build-windows  Build for Windows (amd64)"
	@echo "  build-offline  Build using vendored dependencies"
	@echo ""
	@echo "Distribution targets:"
	@echo "  package        Create release packages with checksums"
	@echo "  bundle-offline Create offline bundle with vendored deps"
	@echo "  docker         Build Docker image"
	@echo "  docker-multiarch Build multi-arch Docker image"
	@echo ""
	@echo "Development targets:"
	@echo "  test           Run tests"
	@echo "  test-coverage  Run tests with coverage report"
	@echo "  lint           Run linters"
	@echo "  deps           Download dependencies"
	@echo "  clean          Clean build artifacts"
	@echo ""
	@echo "Installation targets:"
	@echo "  install        Install to /usr/local/bin"
	@echo "  uninstall      Remove from /usr/local/bin"
	@echo ""
	@echo "Other targets:"
	@echo "  version        Show version information"
	@echo "  help           Show this help message"
	@echo ""
	@echo "Supported platforms: $(PLATFORMS)"
