#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
CMD_DIR="$ROOT_DIR/cmd/dockstep"
BIN_DIR="$ROOT_DIR/bin"

GOOS_TARGET=${GOOS_TARGET:-}
GOARCH_TARGET=${GOARCH_TARGET:-}
LDFLAGS=${LDFLAGS:-}

mkdir -p "$BIN_DIR"

# Ensure UI is built so embed picks up ui/dist
if [ ! -d "$ROOT_DIR/ui/dist" ]; then
	echo "UI assets not found; building UI first..."
	"$ROOT_DIR/scripts/build-ui.sh"
fi

# Build flags
BUILD_ARGS=("-trimpath")
if [ -n "$LDFLAGS" ]; then
	BUILD_ARGS+=("-ldflags" "$LDFLAGS")
fi

# Output name
OUT_NAME="dockstep"

if [ -n "$GOOS_TARGET" ] && [ -n "$GOARCH_TARGET" ]; then
	echo "Building for $GOOS_TARGET/$GOARCH_TARGET"
	GOOS="$GOOS_TARGET" GOARCH="$GOARCH_TARGET" CGO_ENABLED=0 \
		go build "${BUILD_ARGS[@]}" -o "$BIN_DIR/${OUT_NAME}-${GOOS_TARGET}-${GOARCH_TARGET}" "$CMD_DIR"
else
	echo "Building for host platform"
	CGO_ENABLED=0 go build "${BUILD_ARGS[@]}" -o "$BIN_DIR/$OUT_NAME" "$CMD_DIR"
fi

echo "CLI built successfully into $BIN_DIR"
