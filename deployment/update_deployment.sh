#!/bin/bash
set -e

# Configuration
APP_NAME="ebssso"
INSTALL_DIR="/opt/ebssso"
BIN_DIR="/usr/local/bin"
SERVICE_NAME="ebssso"

echo "Starting update deployment..."

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root or with sudo"
    exit 1
fi

# Stop service
echo "Stopping $SERVICE_NAME service..."
systemctl stop $SERVICE_NAME || true

# Backup existing config
if [ -f "$INSTALL_DIR/config.yaml" ]; then
    cp "$INSTALL_DIR/config.yaml" "$INSTALL_DIR/config.yaml.bak_$(date +%F_%T)"
fi

# Update binary
if [ -f "./$APP_NAME" ]; then
    echo "Updating binary..."
    cp "./$APP_NAME" "$BIN_DIR/"
    chmod +x "$BIN_DIR/$APP_NAME"
else
    echo "Error: Binary ./$APP_NAME not found."
    exit 1
fi

# Update config
if [ -f "./config.yaml" ]; then
    echo "Updating configuration..."
    cp "./config.yaml" "$INSTALL_DIR/config.yaml"
    chown ebssso:ebssso "$INSTALL_DIR/config.yaml"
else
    echo "Warning: No new config.yaml found. Keeping existing configuration."
fi

# Update specific templates if needed
if [ -d "./templates" ]; then
   echo "Updating templates..."
   cp -r ./templates/* "$INSTALL_DIR/templates/"
   chown -R ebssso:ebssso "$INSTALL_DIR/templates"
fi

# Reload and Start service
echo "Starting $SERVICE_NAME service..."
systemctl daemon-reload
systemctl start $SERVICE_NAME
systemctl status $SERVICE_NAME --no-pager

echo "Deployment update complete."
