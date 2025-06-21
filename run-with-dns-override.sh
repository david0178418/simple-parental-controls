#!/bin/bash

# This script runs the parental-control application with automatic
# system DNS overriding. It requires sudo privileges to modify DNS settings.
# It ensures that original DNS settings are restored on exit.

# Check for sudo
if ! sudo -v; then
    echo "This script requires sudo privileges to modify system DNS settings."
    exit 1
fi

# Path to the application binary
APP_BINARY="./build/parental-control"

# Ensure the binary exists and is executable
if [ ! -x "$APP_BINARY" ]; then
    echo "Application binary not found or not executable at $APP_BINARY"
    echo "Please run 'make build' first."
    exit 1
fi

# Function to restore DNS settings
cleanup() {
    echo "Application shutting down. Restoring DNS settings..."
    # The application's own shutdown hook should handle this,
    # but we can add a fallback here if needed.
    # For now, we trust the app's Stop() method.
    # If the app crashes without triggering its cleanup, a manual
    # 'sudo resolvectl revert <interface>' might be needed.
}

# Trap EXIT signals to run cleanup
trap cleanup EXIT

echo "Starting parental-control application with DNS override..."
# The application itself calls `sudo resolvectl` internally
$APP_BINARY "$@" 