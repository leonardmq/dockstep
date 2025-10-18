.PHONY: all ui build clean dist

ROOT_DIR := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
BIN_DIR := $(ROOT_DIR)/bin

all: build

ui:
	bash $(ROOT_DIR)/scripts/build-ui.sh

build: ui
	bash $(ROOT_DIR)/scripts/build-cli.sh

clean:
	rm -rf $(BIN_DIR)
	rm -rf $(ROOT_DIR)/ui/node_modules $(ROOT_DIR)/ui/dist

# Build release artifacts for multiple platforms
# Usage: make dist
# Artifacts will be in bin/

dist: ui
	GOOS_TARGET=darwin GOARCH_TARGET=arm64 bash $(ROOT_DIR)/scripts/build-cli.sh
	GOOS_TARGET=darwin GOARCH_TARGET=amd64 bash $(ROOT_DIR)/scripts/build-cli.sh
	GOOS_TARGET=linux GOARCH_TARGET=amd64 bash $(ROOT_DIR)/scripts/build-cli.sh
	GOOS_TARGET=linux GOARCH_TARGET=arm64 bash $(ROOT_DIR)/scripts/build-cli.sh
	GOOS_TARGET=windows GOARCH_TARGET=amd64 bash $(ROOT_DIR)/scripts/build-cli.sh

	# Create archives
	cd $(BIN_DIR) && \
		zip -q dockstep-darwin-arm64.zip dockstep-darwin-arm64 && \
		zip -q dockstep-darwin-amd64.zip dockstep-darwin-amd64 && \
		tar -czf dockstep-linux-amd64.tar.gz dockstep-linux-amd64 && \
		tar -czf dockstep-linux-arm64.tar.gz dockstep-linux-arm64 && \
		zip -q dockstep-windows-amd64.zip dockstep-windows-amd64.exe || true
