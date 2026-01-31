# Dockerfile for iter-service integration testing
# Provides an isolated Linux environment for running tests
# Always built fresh to ensure clean state

FROM golang:1.24-bookworm

# Install system dependencies including Chromium for UI tests
RUN apt-get update && apt-get install -y \
    git \
    curl \
    jq \
    chromium \
    chromium-driver \
    sudo \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user for tests (required for Claude CLI --dangerously-skip-permissions)
RUN useradd -m -s /bin/bash testuser \
    && echo "testuser ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers

# Set Chrome path for chromedp
ENV CHROME_BIN=/usr/bin/chromium
ENV CHROMEDP_NO_SANDBOX=true

# Disable Claude auto-updates and telemetry for CI
ENV DISABLE_AUTOUPDATER=1
ENV DISABLE_TELEMETRY=1
ENV DISABLE_ERROR_REPORTING=1
ENV CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1

# Set working directory
WORKDIR /app

# Copy go mod files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN go build -o /usr/local/bin/iter-service ./cmd/iter-service

# Create directories with proper permissions for testuser
RUN mkdir -p /app/tests/results \
    && mkdir -p /home/testuser/.claude \
    && mkdir -p /home/testuser/.local/bin \
    && chown -R testuser:testuser /app \
    && chown -R testuser:testuser /home/testuser

# Install Claude CLI as testuser
USER testuser
RUN curl -fsSL https://claude.ai/install.sh | bash
ENV PATH="/home/testuser/.local/bin:/usr/local/bin:${PATH}"

# Set test-specific environment
ENV ITER_TEST_MODE=1
ENV HOME=/home/testuser

# Default command runs all tests sequentially
CMD ["go", "test", "-p", "1", "-v", "-timeout", "300s", "./tests/..."]
