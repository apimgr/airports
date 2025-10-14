# Build stage
FROM golang:1.23-alpine AS builder

# Build arguments
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy go mod files first (for caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binary with optimizations
# Note: GeoIP databases are downloaded on first run, not embedded
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE} -w -s" \
    -o airports \
    ./src

# Runtime stage - scratch for minimal size
FROM scratch

ENV PORT=80 \
    CONFIG_DIR=/config \
    DATA_DIR=/data \
    LOGS_DIR=/logs \
    ADDRESS=0.0.0.0 \
    DATABASE_URL=sqlite:/data/airports.db


# Copy CA certificates for HTTPS
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary to /usr/local/bin
COPY --from=builder /build/airports /usr/local/bin/airports

# Pass build arguments to runtime stage
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# Metadata labels (OCI standard)
LABEL org.opencontainers.image.created="${BUILD_DATE}"
LABEL org.opencontainers.image.authors="casapps"
LABEL org.opencontainers.image.url="https://github.com/apimgr/airports"
LABEL org.opencontainers.image.source="https://github.com/apimgr/airports"
LABEL org.opencontainers.image.version="${VERSION}"
LABEL org.opencontainers.image.revision="${COMMIT}"
LABEL org.opencontainers.image.vendor="casapps"
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.title="Airports API Server"
LABEL org.opencontainers.image.description="Global airport location information API with GeoIP integration"
LABEL org.opencontainers.image.documentation="https://github.com/apimgr/airports/blob/main/docs/README.md"
LABEL org.opencontainers.image.base.name="scratch"

# Expose default port
EXPOSE 80

# Run as non-root (numeric UID for scratch)
USER 65534:65534

# Create mount points for volumes
VOLUME ["/config", "/data", "/logs"]

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 CMD ["/usr/local/bin/airports", "--status"]

# Run
ENTRYPOINT ["/usr/local/bin/airports"]
