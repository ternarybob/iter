#!/bin/bash
# iter-service macOS installation script
# Installs iter-service as a launchd user agent

set -e

INSTALL_DIR="${HOME}/.local/bin"
PLIST_DIR="${HOME}/Library/LaunchAgents"
DATA_DIR="${HOME}/Library/Application Support/iter-service"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Installing iter-service...${NC}"

# Create directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$PLIST_DIR"
mkdir -p "$DATA_DIR"

# Build the binary
echo "Building iter-service..."
cd "$(dirname "$0")/.."
go build -ldflags "-X main.version=$(cat .version 2>/dev/null || echo dev)" -o iter-service ./cmd/iter-service

# Install binary
echo "Installing binary to $INSTALL_DIR..."
cp iter-service "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/iter-service"

# Check if $INSTALL_DIR is in PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo -e "${YELLOW}Warning: $INSTALL_DIR is not in your PATH${NC}"
    echo "Add to your ~/.zshrc or ~/.bash_profile:"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
fi

# Create launchd plist
echo "Creating launchd agent..."
cat > "$PLIST_DIR/com.iter.service.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.iter.service</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/iter-service</string>
        <string>serve</string>
    </array>
    <key>RunAtLoad</key>
    <false/>
    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>
    <key>StandardOutPath</key>
    <string>${DATA_DIR}/logs/stdout.log</string>
    <key>StandardErrorPath</key>
    <string>${DATA_DIR}/logs/stderr.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin</string>
    </dict>
</dict>
</plist>
EOF

echo -e "${GREEN}Installation complete!${NC}"
echo ""
echo "To start the service:"
echo "  launchctl load ~/Library/LaunchAgents/com.iter.service.plist"
echo ""
echo "To stop the service:"
echo "  launchctl unload ~/Library/LaunchAgents/com.iter.service.plist"
echo ""
echo "To enable on login, edit the plist and set RunAtLoad to true"
echo ""
echo "Or run manually:"
echo "  iter-service"
echo ""
echo "Configuration: $DATA_DIR/config.yaml"
echo "Logs: $DATA_DIR/logs/"
