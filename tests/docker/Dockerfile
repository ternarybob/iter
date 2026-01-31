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
    && rm -rf /var/lib/apt/lists/*

# Set Chrome path for chromedp
ENV CHROME_BIN=/usr/bin/chromium
ENV CHROMEDP_NO_SANDBOX=true

# Set working directory
WORKDIR /app

# Copy go mod files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN go build -o /usr/local/bin/iter-service ./cmd/iter-service

# Create directories with proper permissions
# Results dir is mounted from host, but we need the parent
RUN mkdir -p /app/tests/results \
    && chmod 777 /app/tests/results

# Set test-specific environment
ENV ITER_TEST_MODE=1
ENV PATH="/usr/local/bin:${PATH}"

# Default command runs all tests sequentially
CMD ["go", "test", "-p", "1", "-v", "-timeout", "300s", "./tests/..."]
