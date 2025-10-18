#!/usr/bin/env bash
set -euo pipefail

# Dockstep update script
# This script updates dockstep to the latest version

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if dockstep is installed
if ! command -v dockstep >/dev/null 2>&1; then
    print_error "dockstep is not installed or not in PATH"
    print_status "Please install dockstep first using the install script:"
    print_status "curl -sSL https://raw.githubusercontent.com/leonardmq/dockstep/main/scripts/install.sh | bash"
    exit 1
fi

# Get current version
current_version=$(dockstep --version 2>/dev/null | head -n1 | grep -o '[0-9]\+\.[0-9]\+\.[0-9]\+' || echo "unknown")
print_status "Current version: $current_version"

# Get latest version
print_status "Checking for updates..."
latest_version=$(curl -s "https://api.github.com/repos/leonardmq/dockstep/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$latest_version" ]; then
    print_error "Failed to get latest version"
    exit 1
fi

print_status "Latest version: $latest_version"

# Compare versions
if [ "$current_version" = "$latest_version" ]; then
    print_success "You are already running the latest version!"
    exit 0
fi

# Ask for confirmation
read -p "Do you want to update from $current_version to $latest_version? [y/N]: " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    print_status "Update cancelled"
    exit 0
fi

# Run the install script to update
print_status "Updating dockstep..."
curl -sSL https://raw.githubusercontent.com/leonardmq/dockstep/main/scripts/install.sh | bash

print_success "Update completed!"
print_status "New version: $(dockstep --version | head -n1)"
