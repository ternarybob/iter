#!/bin/bash
# iter-service Linux installation script
# Installs iter-service as a systemd user service
#
# Usage:
#   ./install-linux.sh              # Full install with systemd
#   ./install-linux.sh --no-systemd # Install without systemd (for containers/testing)
#   ./install-linux.sh --run        # Install and run immediately (no systemd)

set -e

# Parse arguments
NO_SYSTEMD=false
RUN_AFTER=false
for arg in "$@"; do
    case $arg in
        --no-systemd)
            NO_SYSTEMD=true
            ;;
        --run)
            NO_SYSTEMD=true
            RUN_AFTER=true
            ;;
    esac
done

INSTALL_DIR="${HOME}/.local/bin"
SERVICE_DIR="${HOME}/.config/systemd/user"
DATA_DIR="${HOME}/.iter-service"
CONFIG_DIR="${HOME}/.config/iter-service"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Installing iter-service...${NC}"

# Create directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$DATA_DIR"
mkdir -p "$CONFIG_DIR"

# Change to project root
cd "$(dirname "$0")/.."
PROJECT_ROOT=$(pwd)

# Build the binary directly to install location
echo "Building iter-service..."
go build -ldflags "-X main.version=$(cat .version 2>/dev/null || echo dev)" -o "$INSTALL_DIR/iter-service" ./cmd/iter-service
chmod +x "$INSTALL_DIR/iter-service"

# Copy default config if it doesn't exist
if [ ! -f "$CONFIG_DIR/config.toml" ]; then
    echo "Installing default configuration..."
    cp "$PROJECT_ROOT/configs/config.example.toml" "$CONFIG_DIR/config.toml"
    echo -e "${YELLOW}Edit $CONFIG_DIR/config.toml to configure your settings${NC}"
fi

# Check if $INSTALL_DIR is in PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo -e "${YELLOW}Warning: $INSTALL_DIR is not in your PATH${NC}"
    echo "Add to your ~/.bashrc or ~/.zshrc:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

# Install systemd service unless --no-systemd flag is set
if [ "$NO_SYSTEMD" = false ]; then
    mkdir -p "$SERVICE_DIR"

    echo "Creating systemd service..."
    cat > "$SERVICE_DIR/iter-service.service" << EOF
[Unit]
Description=iter-service - Code indexing and discovery service
After=network.target

[Service]
Type=simple
ExecStart=%h/.local/bin/iter-service serve --config %h/.config/iter-service/config.toml
Restart=on-failure
RestartSec=5
Environment=GOOGLE_GEMINI_API_KEY=

[Install]
WantedBy=default.target
EOF

    # Reload systemd
    systemctl --user daemon-reload

    echo -e "${GREEN}Installation complete!${NC}"
    echo ""
    echo "To start the service:"
    echo "  systemctl --user start iter-service"
    echo ""
    echo "To enable on login:"
    echo "  systemctl --user enable iter-service"
    echo ""
    echo "To check status:"
    echo "  systemctl --user status iter-service"
    echo ""
    echo "Or run manually:"
    echo "  iter-service serve --config $CONFIG_DIR/config.toml"
else
    echo -e "${GREEN}Installation complete (no systemd)!${NC}"
    echo ""
    echo "To run the service:"
    echo "  $INSTALL_DIR/iter-service serve --config $CONFIG_DIR/config.toml"
fi

echo ""
echo "Configuration: $CONFIG_DIR/config.toml"
echo "Data: $DATA_DIR/"
echo "Logs: $DATA_DIR/logs/service.log"

# Run immediately if requested
if [ "$RUN_AFTER" = true ]; then
    echo ""
    echo -e "${GREEN}Starting iter-service...${NC}"
    exec "$INSTALL_DIR/iter-service" serve --config "$CONFIG_DIR/config.toml"
fi
