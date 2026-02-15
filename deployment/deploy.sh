#!/bin/bash
# EBS SSO Gateway Deployment Script

set -e

# Configuration
APP_NAME="ebssso"
INSTALL_DIR="/opt/ebssso"
BIN_DIR="/usr/local/bin"
SERVICE_USER="ebssso"
SERVICE_FILE="/etc/systemd/system/ebssso.service"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}EBS SSO Gateway Deployment Script${NC}"
echo "=================================="
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo -e "${RED}Please run as root or with sudo${NC}"
    exit 1
fi

# Check if binary exists
if [ ! -f "./$APP_NAME" ]; then
    echo -e "${RED}Error: $APP_NAME binary not found. Please build it first with 'make build'${NC}"
    exit 1
fi

# Check if config template exists
if [ ! -f "./config.yaml.template" ]; then
    echo -e "${RED}Error: config.yaml.template not found. This file is required to initialize the configuration.${NC}"
    exit 1
fi

# Create service user if it doesn't exist
if ! id "$SERVICE_USER" &>/dev/null; then
    echo -e "${YELLOW}Creating service user: $SERVICE_USER${NC}"
    useradd --system --shell /bin/false --home-dir $INSTALL_DIR $SERVICE_USER
fi

# Create installation directory
echo -e "${YELLOW}Creating installation directory: $INSTALL_DIR${NC}"
mkdir -p $INSTALL_DIR/logs
mkdir -p $INSTALL_DIR/templates

# Copy binary
echo -e "${YELLOW}Installing binary to $BIN_DIR${NC}"
cp ./$APP_NAME $BIN_DIR/
chmod +x $BIN_DIR/$APP_NAME

# Copy templates
echo -e "${YELLOW}Copying templates${NC}"
cp -r templates/* $INSTALL_DIR/templates/

# Copy config template if config doesn't exist
if [ ! -f "$INSTALL_DIR/config.yaml" ]; then
    echo -e "${YELLOW}Copying configuration template${NC}"
    cp config.yaml.template $INSTALL_DIR/config.yaml
    echo -e "${RED}IMPORTANT: Please edit $INSTALL_DIR/config.yaml with your settings${NC}"
else
    echo -e "${GREEN}Configuration file already exists, skipping${NC}"
fi

# Set ownership
echo -e "${YELLOW}Setting ownership${NC}"
chown -R $SERVICE_USER:$SERVICE_USER $INSTALL_DIR

# Install systemd service
echo -e "${YELLOW}Installing systemd service${NC}"
cp deployment/ebssso.service $SERVICE_FILE
systemctl daemon-reload

# Enable service
echo -e "${YELLOW}Enabling service${NC}"
systemctl enable ebssso

echo ""
echo -e "${GREEN}Installation complete!${NC}"
echo ""
echo "Next steps:"
echo "1. Edit the configuration: sudo nano $INSTALL_DIR/config.yaml"
echo "2. Start the service: sudo systemctl start ebssso"
echo "3. Check status: sudo systemctl status ebssso"
echo "4. View logs: sudo journalctl -u ebssso -f"
echo ""
