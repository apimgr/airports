# =============================================================================
# Build Stage - Compile Go binary (cross-compile via BuildKit args)
# =============================================================================
FROM --platform=$BUILDPLATFORM golang:alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev
ARG BUILD_DATE
ARG VCS_REF

WORKDIR /build

# Copy go.mod first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build server + client
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
        -ldflags "-s -w -X 'main.Version=${VERSION}' -X 'main.CommitID=${VCS_REF}' -X 'main.BuildDate=${BUILD_DATE}'" \
        -o /out/airports ./src && \
    if [ -d src/client ]; then \
        CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
        go build \
            -ldflags "-s -w -X 'main.Version=${VERSION}' -X 'main.CommitID=${VCS_REF}' -X 'main.BuildDate=${BUILD_DATE}'" \
            -o /out/airports-cli ./src/client; \
    fi

# =============================================================================
# Runtime Stage - Minimal Alpine image
# =============================================================================
FROM alpine:latest

ARG VERSION=dev
ARG BUILD_DATE
ARG VCS_REF
ARG LICENSE=MIT

LABEL maintainer="apimgr <apimgr@casjay.pro>" \
      org.opencontainers.image.vendor="apimgr" \
      org.opencontainers.image.authors="apimgr" \
      org.opencontainers.image.title="airports" \
      org.opencontainers.image.base.name="airports" \
      org.opencontainers.image.description="Airports API - global airport information service" \
      org.opencontainers.image.url="https://github.com/apimgr/airports" \
      org.opencontainers.image.source="https://github.com/apimgr/airports" \
      org.opencontainers.image.documentation="https://github.com/apimgr/airports" \
      org.opencontainers.image.vcs-type="Git" \
      com.github.containers.toolbox="false"

LABEL org.opencontainers.image.licenses="${LICENSE}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.schema-version="${VERSION}" \
      org.opencontainers.image.revision="${VCS_REF}"

# Required runtime packages per spec
RUN apk add --no-cache \
        git \
        curl \
        bash \
        tini \
        tor

# Runtime data layout (filled by entrypoint.sh / app at first run)
RUN mkdir -p /config /data/db /data/logs /data/tor /data/geoip /data/backup

# Copy server (always present) and client (optional)
COPY --from=builder /out/airports     /usr/local/bin/airports
COPY --from=builder /out/             /tmp/builder-out/
RUN if [ -f /tmp/builder-out/airports-cli ]; then \
        cp /tmp/builder-out/airports-cli /usr/local/bin/airports-cli; \
    fi && rm -rf /tmp/builder-out

# Container filesystem overlay (entrypoint.sh, service configs, etc.)
COPY docker/rootfs/ /

RUN chmod 755 /usr/local/bin/*

ENV MODE=production \
    TZ=America/New_York \
    ENABLE_TOR=true \
    PORT=80 \
    ADDRESS=0.0.0.0

EXPOSE 80

STOPSIGNAL SIGRTMIN+3

HEALTHCHECK --start-period=10m --interval=5m --timeout=15s --retries=3 \
    CMD /usr/local/bin/airports --status || exit 1

# tini propagates SIGTERM and reaps zombies; entrypoint handles startup
ENTRYPOINT [ "tini", "-p", "SIGTERM", "--", "/usr/local/bin/entrypoint.sh" ]
