SHELL := /bin/bash
BINARY_NAME := tidy
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
SYSTEMD_USER_DIR ?= $(HOME)/.config/systemd/user

.PHONY: all build test clean install uninstall status

all: build

## build: Compile single static binary with stripped debug symbols
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(BINARY_NAME) main.go
	@echo "✓ Binary built at bin/$(BINARY_NAME)"

## test: Run complete test suite across all modules
test:
	@echo "Running tests..."
	go test -v ./...

## install: Install binary and enable background systemd service
install: build
	@echo "Installing $(BINARY_NAME) to $(BINDIR)..."
	@mkdir -p $(BINDIR)
	@cp bin/$(BINARY_NAME) $(BINDIR)/$(BINARY_NAME)
	@chmod +x $(BINDIR)/$(BINARY_NAME)
	@echo "✓ Installed binary to $(BINDIR)/$(BINARY_NAME)"
	@echo "Initializing default configuration if missing..."
	@$(BINDIR)/$(BINARY_NAME) init
	@echo "Installing systemd user service..."
	@mkdir -p $(SYSTEMD_USER_DIR)
	@cp tidy.service $(SYSTEMD_USER_DIR)/tidy.service
	@systemctl --user daemon-reload
	@systemctl --user enable --now tidy.service
	@echo ""
	@echo "=================================================="
	@echo "✓ tidy installed and started as a background service!"
	@echo "  Check status with: systemctl --user status tidy"
	@echo "  Stop service with:  systemctl --user stop tidy"
	@echo "  Start service with: systemctl --user start tidy"
	@echo "=================================================="

## uninstall: Stop service and remove binary and systemd files
uninstall:
	@echo "Stopping and disabling tidy systemd service..."
	@-systemctl --user stop tidy.service 2>/dev/null || true
	@-systemctl --user disable tidy.service 2>/dev/null || true
	@rm -f $(SYSTEMD_USER_DIR)/tidy.service
	@systemctl --user daemon-reload
	@echo "Removing $(BINDIR)/$(BINARY_NAME)..."
	@rm -f $(BINDIR)/$(BINARY_NAME)
	@echo "✓ tidy has been successfully uninstalled."

## status: Check systemd service status
status:
	@systemctl --user status tidy.service

## clean: Remove build artifacts
clean:
	@rm -rf bin/
