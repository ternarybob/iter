# iter-service Docker image
# Multi-stage build for minimal production image

FROM golang:1.24-bookworm AS builder

WORKDIR /build

# Copy go mod files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X main.version=${VERSION} -s -w" \
    -o iter-service ./cmd/iter-service

# Production image
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create non-root user
RUN useradd -r -u 1000 -m iter

# Create data directory
RUN mkdir -p /data && chown iter:iter /data

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/iter-service /app/iter-service
COPY --from=builder /build/web /app/web

# Set ownership
RUN chown -R iter:iter /app

USER iter

# Default port
EXPOSE 8420

# Data volume
VOLUME /data

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/app/iter-service", "health"] || exit 1

# Default command
ENTRYPOINT ["/app/iter-service"]
CMD ["serve", "--data-dir", "/data"]
