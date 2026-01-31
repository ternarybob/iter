#!/bin/bash
# iter-service Linux installation script
# Installs iter-service as a systemd user service

set -e

INSTALL_DIR="${HOME}/.local/bin"
SERVICE_DIR="${HOME}/.config/systemd/user"
DATA_DIR="${HOME}/.iter-service"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Installing iter-service...${NC}"

# Create directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$SERVICE_DIR"
mkdir -p "$DATA_DIR"

# Build the binary directly to install location
echo "Building iter-service..."
cd "$(dirname "$0")/.."
go build -ldflags "-X main.version=$(cat .version 2>/dev/null || echo dev)" -o "$INSTALL_DIR/iter-service" ./cmd/iter-service
chmod +x "$INSTALL_DIR/iter-service"

# Check if $INSTALL_DIR is in PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo -e "${YELLOW}Warning: $INSTALL_DIR is not in your PATH${NC}"
    echo "Add to your ~/.bashrc or ~/.zshrc:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

# Create systemd service file
echo "Creating systemd service..."
cat > "$SERVICE_DIR/iter-service.service" << 'EOF'
[Unit]
Description=iter-service - Code indexing and discovery service
After=network.target

[Service]
Type=simple
ExecStart=%h/.local/bin/iter-service serve
Restart=on-failure
RestartSec=5
Environment=GEMINI_API_KEY=

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
echo "  iter-service"
echo ""
echo "Configuration: $DATA_DIR/config.yaml"
echo "Logs: $DATA_DIR/logs/service.log"
