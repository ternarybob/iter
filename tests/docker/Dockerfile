# Dockerfile for iter-service integration testing
# Provides an isolated Linux environment for running tests

FROM golang:1.21-bookworm

# Install system dependencies
RUN apt-get update && apt-get install -y \
    git \
    curl \
    jq \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy go mod files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN go build -o /usr/local/bin/iter-service ./cmd/iter-service

# Create test user and directories
RUN useradd -m -s /bin/bash testuser \
    && mkdir -p /home/testuser/.iter-service \
    && mkdir -p /app/tests/results \
    && chown -R testuser:testuser /home/testuser \
    && chown -R testuser:testuser /app/tests/results

# Switch to test user
USER testuser
ENV HOME=/home/testuser
ENV PATH="/usr/local/bin:${PATH}"

# Set test-specific environment
ENV ITER_TEST_MODE=1
ENV ITER_DATA_DIR=/home/testuser/.iter-service

# Default command runs all tests
CMD ["go", "test", "-v", "-timeout", "300s", "./tests/..."]
