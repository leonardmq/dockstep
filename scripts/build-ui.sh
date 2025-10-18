#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
UI_DIR="$ROOT_DIR/ui"

if ! command -v npm >/dev/null 2>&1; then
	echo "Error: npm is required to build the UI." >&2
	exit 1
fi

cd "$UI_DIR"

# Install deps (CI-friendly, reproducible)
if [ -f package-lock.json ]; then
	npm ci
else
	npm install
fi

# Build Vite app
npm run build

# Verify dist exists
if [ ! -d "$UI_DIR/dist" ]; then
	echo "Error: UI dist not found at $UI_DIR/dist" >&2
	exit 1
fi

echo "UI built successfully at $UI_DIR/dist"
