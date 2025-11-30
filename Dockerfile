# ============================================
# Build stage
# ============================================
FROM golang:alpine AS builder

# Build arguments
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# Install build dependencies
RUN apk add --no-cache git make ca-certificates tzdata

# Set working directory
WORKDIR /build

# Copy go mod files first (for caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY src/ ./src/

# Build static binary with all assets embedded
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE} -w -s" \
    -a -installsuffix cgo \
    -o airports \
    ./src

# ============================================
# Runtime stage - Alpine with tini
# ============================================
FROM alpine:latest

# Build arguments for labels
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# Install runtime dependencies (tini as PID 1)
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    tini \
    curl \
    && rm -rf /var/cache/apk/*

# Copy binary to /usr/local/bin
COPY --from=builder /build/airports /usr/local/bin/airports

# Make binary executable
RUN chmod +x /usr/local/bin/airports

# Environment variables - uses /etc/apimgr/airports/ path structure
ENV PORT=80 \
    CONFIG_DIR=/config \
    DATA_DIR=/data \
    LOGS_DIR=/logs \
    ADDRESS=0.0.0.0

# Create directories with apimgr organization structure
RUN mkdir -p /config /data /logs && \
    chown -R 65534:65534 /config /data /logs

# Metadata labels (OCI standard)
LABEL org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.authors="apimgr" \
      org.opencontainers.image.url="https://github.com/apimgr/airports" \
      org.opencontainers.image.source="https://github.com/apimgr/airports" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.vendor="apimgr" \
      org.opencontainers.image.licenses="MIT" \
      org.opencontainers.image.title="Airports API Server" \
      org.opencontainers.image.description="Global airport location information API with GeoIP integration - Single static binary" \
      org.opencontainers.image.documentation="https://github.com/apimgr/airports/blob/main/docs/README.md" \
      org.opencontainers.image.base.name="alpine:latest"

# Expose default port
EXPOSE 80

# Create mount points for volumes
VOLUME ["/config", "/data", "/logs"]

# Run as non-root user (nobody)
USER 65534:65534

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/airports", "--status"]

# Use tini as init system (proper PID 1 signal handling)
ENTRYPOINT ["/sbin/tini", "--"]

# Run airports server
CMD ["/usr/local/bin/airports", "--port", "80"]
