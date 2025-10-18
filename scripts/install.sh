#!/usr/bin/env bash
set -euo pipefail

# Dockstep installer script
# This script downloads and installs the latest version of dockstep

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO="leonardmq/dockstep"
BINARY_NAME="dockstep"
INSTALL_DIR="$HOME/.local/bin"
TEMP_DIR=$(mktemp -d)

# Print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" >&2
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1" >&2
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

# Verify temp directory was created
if [ ! -d "$TEMP_DIR" ]; then
    print_error "Failed to create temporary directory"
    exit 1
fi

# Cleanup function
cleanup() {
    # rm -rf "$TEMP_DIR"  # Disabled - let OS clean up temp files
    print_status "Temp directory preserved: $TEMP_DIR"
}
trap cleanup EXIT

# Detect OS and architecture
detect_platform() {
    local os arch
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m | tr '[:upper:]' '[:lower:]')
    
    case $arch in
        x86_64|amd64)
            arch="amd64"
            ;;
        arm64|aarch64)
            arch="arm64"
            ;;
        armv7l)
            arch="armv7"
            ;;
        *)
            print_error "Unsupported architecture: $arch"
            exit 1
            ;;
    esac
    
    case $os in
        darwin)
            os="darwin"
            ;;
        linux)
            os="linux"
            ;;
        msys*|cygwin*|mingw*|nt|windows*)
            os="windows"
            ;;
        *)
            print_error "Unsupported operating system: $os"
            exit 1
            ;;
    esac
    
    echo "${os}_${arch}"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Get latest release version
get_latest_version() {
    local version
    if command_exists curl; then
        version=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command_exists wget; then
        version=$(wget -qO- "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        print_error "Neither curl nor wget is available. Please install one of them."
        exit 1
    fi
    
    if [ -z "$version" ]; then
        print_error "Failed to get latest version"
        exit 1
    fi
    
    echo "$version"
}

# Download binary
download_binary() {
    local version=$1
    local platform=$2
    local download_url
    local binary_path
    local full_path
    
    # Construct download URL
    if [ "$platform" = "windows_amd64" ] || [ "$platform" = "windows_arm64" ]; then
        binary_path="$BINARY_NAME-${platform//_/-}.exe"
    else
        binary_path="$BINARY_NAME-${platform//_/-}"
    fi
    
    download_url="https://github.com/$REPO/releases/download/$version/$binary_path"
    full_path="$TEMP_DIR/$binary_path"
    
    print_status "Downloading $BINARY_NAME $version for $platform..."
    print_status "URL: $download_url"
    print_status "Target: $full_path"
    
    if command_exists curl; then
        if ! curl -L -o "$full_path" "$download_url"; then
            print_error "Failed to download binary from $download_url"
            exit 1
        fi
    elif command_exists wget; then
        if ! wget -O "$full_path" "$download_url"; then
            print_error "Failed to download binary from $download_url"
            exit 1
        fi
    fi
    
    # Verify the file was actually created
    if [ ! -f "$full_path" ]; then
        print_error "Download completed but file not found at $full_path"
        exit 1
    fi
    
    # Check file size (should be > 0)
    if [ ! -s "$full_path" ]; then
        print_error "Downloaded file is empty"
        exit 1
    fi
    
    print_success "Download completed successfully"
    echo "$full_path"
}

# Verify binary
verify_binary() {
    local binary_path=$1
    
    print_status "Verifying downloaded binary at: $binary_path"
    
    if [ ! -f "$binary_path" ]; then
        print_error "Binary not found at $binary_path"
        exit 1
    fi
    
    if [ ! -x "$binary_path" ]; then
        print_status "Making binary executable..."
        chmod +x "$binary_path"
    fi
    
    # Test if binary works
    print_status "Testing binary..."
    local test_output
    local test_exit_code
    test_output=$("$binary_path" --help 2>&1)
    test_exit_code=$?
    
    if [ $test_exit_code -ne 0 ]; then
        print_error "Downloaded binary appears to be corrupted"
        print_status "Binary path: $binary_path"
        print_status "File size: $(ls -la "$binary_path" 2>/dev/null || echo 'File not found')"
        print_status "File type: $(file "$binary_path" 2>/dev/null || echo 'Cannot determine file type')"
        print_status "Exit code: $test_exit_code"
        print_status "Error output: $test_output"
        exit 1
    fi
    
    print_success "Binary verification successful"
}

# Install binary
install_binary() {
    local binary_path=$1
    local target_path="$INSTALL_DIR/$BINARY_NAME"
    
    print_status "Installing $BINARY_NAME to $target_path..."
    
    # Create install directory if it doesn't exist
    if [ ! -d "$INSTALL_DIR" ]; then
        print_status "Creating directory $INSTALL_DIR"
        mkdir -p "$INSTALL_DIR"
    fi
    
    # Copy binary
    if ! cp "$binary_path" "$target_path"; then
        print_error "Failed to install binary"
        exit 1
    fi
    
    # Make executable
    chmod +x "$target_path"
    
    # Remove macOS quarantine attribute (fixes "unidentified developer" warning)
    if [[ "$OSTYPE" == "darwin"* ]]; then
        print_status "Removing macOS quarantine attribute..."
        xattr -d com.apple.quarantine "$target_path" 2>/dev/null || true
    fi
    
    print_success "$BINARY_NAME installed successfully!"
}

# Check if already installed
check_existing() {
    if command_exists "$BINARY_NAME"; then
        print_warning "$BINARY_NAME is already installed"
        
        read -p "Do you want to update it? [y/N]: " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_status "Installation cancelled"
            exit 0
        fi
    fi
}

# Main installation function
main() {
    print_status "Dockstep Installer"
    print_status "=================="
    
    # Check for existing installation
    check_existing
    
    # Detect platform
    local platform
    platform=$(detect_platform)
    print_status "Detected platform: $platform"
    
    # Get latest version
    local version
    version=$(get_latest_version)
    print_status "Latest version: $version"
    
    # Download binary
    print_status "Starting download process..."
    local binary_path
    binary_path=$(download_binary "$version" "$platform")
    print_status "Download returned path: $binary_path"
    
    # Verify binary
    print_status "Starting verification process..."
    verify_binary "$binary_path"
    
    # Install binary
    print_status "Starting installation process..."
    install_binary "$binary_path"
    
    # Show success message
    print_success "Installation completed!"
    print_status "Installed to: $INSTALL_DIR/$BINARY_NAME"
    print_status "You can now run: $BINARY_NAME --help"
    
    # Check if binary is in PATH
    if ! command_exists "$BINARY_NAME"; then
        print_warning "The binary was installed but may not be in your PATH."
        print_warning "Make sure $INSTALL_DIR is in your PATH environment variable."
        print_status "You may need to restart your terminal or run: source ~/.bashrc (or ~/.zshrc)"
    fi
}

# Run main function
main "$@"

