#!/bin/bash
# iter-service Docker installation script
# Installs iter-service as a Docker container with systemd integration
#
# Requirements:
#   - Docker (docker.io or docker-ce)
#   - Docker Compose v2
#
# Usage:
#   ./scripts/install-docker.sh
#   ./scripts/install-docker.sh --uninstall

set -e

# Configuration
INSTALL_DIR="${HOME}/.iter-service"
SERVICE_DIR="${HOME}/.config/systemd/user"
IMAGE_NAME="iter-service"
CONTAINER_NAME="iter-service"
VERSION="${VERSION:-latest}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check for uninstall flag
if [[ "$1" == "--uninstall" ]]; then
    log_info "Uninstalling iter-service..."

    # Stop and remove container
    docker stop "$CONTAINER_NAME" 2>/dev/null || true
    docker rm "$CONTAINER_NAME" 2>/dev/null || true

    # Stop systemd service
    systemctl --user stop iter-service 2>/dev/null || true
    systemctl --user disable iter-service 2>/dev/null || true
    rm -f "$SERVICE_DIR/iter-service.service"
    systemctl --user daemon-reload

    log_info "iter-service uninstalled"
    log_warn "Data directory preserved at: $INSTALL_DIR"
    log_warn "To remove data: rm -rf $INSTALL_DIR"
    exit 0
fi

# Check requirements
check_requirements() {
    log_info "Checking requirements..."

    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        echo "Install Docker: https://docs.docker.com/engine/install/"
        exit 1
    fi

    # Check Docker daemon is running
    if ! docker info &> /dev/null; then
        log_error "Docker daemon is not running"
        echo "Start Docker: sudo systemctl start docker"
        exit 1
    fi

    # Check user is in docker group
    if ! groups | grep -q docker; then
        log_warn "User is not in docker group"
        echo "Add user to docker group: sudo usermod -aG docker \$USER"
        echo "Then log out and back in"
    fi

    log_info "Requirements OK"
}

# Create directories
setup_directories() {
    log_info "Setting up directories..."

    mkdir -p "$INSTALL_DIR"
    mkdir -p "$INSTALL_DIR/data"
    mkdir -p "$INSTALL_DIR/logs"
    mkdir -p "$SERVICE_DIR"
}

# Create minimal config file
create_config() {
    local config_file="$INSTALL_DIR/config.toml"

    if [[ -f "$config_file" ]]; then
        log_info "Config file exists, preserving: $config_file"
        return
    fi

    log_info "Creating config file: $config_file"

    cat > "$config_file" << 'EOF'
# iter-service configuration
# Only add settings you want to override from defaults
# See: https://github.com/ternarybob/iter for full options

[service]
# Uncomment to change the port
# port = 8420

[llm]
# Set your API key (or use GEMINI_API_KEY env var)
# api_key = "your-api-key-here"

[logging]
# Log level: debug, info, warn, error
level = "info"
EOF
}

# Build Docker image
build_image() {
    log_info "Building Docker image..."

    # Get script directory and project root
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

    cd "$PROJECT_DIR"

    # Get version from .version file
    if [[ -f ".version" ]]; then
        VERSION=$(cat .version | tr -d '[:space:]')
    fi

    docker build \
        --build-arg VERSION="$VERSION" \
        -f deployments/docker/Dockerfile \
        -t "$IMAGE_NAME:$VERSION" \
        -t "$IMAGE_NAME:latest" \
        .

    log_info "Image built: $IMAGE_NAME:$VERSION"
}

# Create systemd service
create_systemd_service() {
    log_info "Creating systemd service..."

    cat > "$SERVICE_DIR/iter-service.service" << EOF
[Unit]
Description=iter-service - Code indexing and discovery service
After=network.target

[Service]
Type=simple
Restart=on-failure
RestartSec=10

ExecStartPre=-/usr/bin/docker stop $CONTAINER_NAME
ExecStartPre=-/usr/bin/docker rm $CONTAINER_NAME

ExecStart=/usr/bin/docker run --rm \\
    --name $CONTAINER_NAME \\
    -p 8420:8420 \\
    -v $INSTALL_DIR/data:/data \\
    -v $INSTALL_DIR/config.toml:/data/config.toml:ro \\
    -e GEMINI_API_KEY \\
    $IMAGE_NAME:latest serve --data-dir /data

ExecStop=/usr/bin/docker stop $CONTAINER_NAME

[Install]
WantedBy=default.target
EOF

    systemctl --user daemon-reload
}

# Start service
start_service() {
    log_info "Starting iter-service..."

    systemctl --user enable iter-service
    systemctl --user start iter-service

    # Wait for service to start
    sleep 2

    if systemctl --user is-active --quiet iter-service; then
        log_info "Service started successfully"
    else
        log_error "Service failed to start"
        log_info "Check logs: journalctl --user -u iter-service -f"
        exit 1
    fi
}

# Print completion message
print_completion() {
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}iter-service installed successfully!${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    echo "Service Status:"
    echo "  systemctl --user status iter-service"
    echo ""
    echo "View Logs:"
    echo "  journalctl --user -u iter-service -f"
    echo ""
    echo "Stop Service:"
    echo "  systemctl --user stop iter-service"
    echo ""
    echo "Web UI:"
    echo "  http://localhost:8420"
    echo ""
    echo "Configuration:"
    echo "  $INSTALL_DIR/config.toml"
    echo ""
    echo "Data Directory:"
    echo "  $INSTALL_DIR/data"
    echo ""

    # Check if service is healthy
    if curl -s http://localhost:8420/health | grep -q "ok"; then
        echo -e "${GREEN}Health Check: OK${NC}"
    else
        echo -e "${YELLOW}Health Check: Service is starting...${NC}"
        echo "  Wait a few seconds and check: curl http://localhost:8420/health"
    fi
}

# Main installation
main() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}iter-service Docker Installation${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""

    check_requirements
    setup_directories
    create_config
    build_image
    create_systemd_service
    start_service
    print_completion
}

main "$@"
