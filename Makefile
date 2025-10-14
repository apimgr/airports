# ============================================
# Variables
# ============================================
PROJECTNAME = airports
PROJECTORG = casapps

# VERSION can be overridden: make build VERSION=1.2.3
# Otherwise, read from release.txt or default to 0.0.1
VERSION ?= $(shell cat release.txt 2>/dev/null || echo "0.0.1")

COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILD_DATE) -w -s"

# Detect host OS and architecture
HOSTOS := $(shell go env GOOS)
HOSTARCH := $(shell go env GOARCH)

# ============================================
# Main Targets
# ============================================
.PHONY: build release test docker clean
.DEFAULT_GOAL := build

# Build all binaries for all platforms
build:
	@echo "Building $(PROJECTNAME) v$(VERSION) for all platforms..."
	@mkdir -p binaries release
	@docker run --rm -v $$(pwd):/workspace -w /workspace golang:1.23-alpine sh -c ' \
		apk add --no-cache git binutils > /dev/null 2>&1 && \
		echo "  → Linux AMD64" && \
		GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-linux-amd64 ./src && \
		strip binaries/$(PROJECTNAME)-linux-amd64 2>/dev/null || true && \
		echo "  → Linux ARM64" && \
		GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-linux-arm64 ./src && \
		echo "  → Windows AMD64" && \
		GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-windows-amd64.exe ./src && \
		echo "  → Windows ARM64" && \
		GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-windows-arm64.exe ./src && \
		echo "  → macOS AMD64" && \
		GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-macos-amd64 ./src && \
		echo "  → macOS ARM64" && \
		GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-macos-arm64 ./src && \
		echo "  → FreeBSD AMD64" && \
		GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-bsd-amd64 ./src && \
		strip binaries/$(PROJECTNAME)-bsd-amd64 2>/dev/null || true && \
		echo "  → FreeBSD ARM64" && \
		GOOS=freebsd GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-bsd-arm64 ./src && \
		echo "  → Host ($(HOSTOS)/$(HOSTARCH))" && \
		GOOS=$(HOSTOS) GOARCH=$(HOSTARCH) CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME) ./src && \
		(ldd binaries/$(PROJECTNAME) 2>&1 | grep -q musl && strip binaries/$(PROJECTNAME) 2>/dev/null || true) \
	'
	@cp binaries/$(PROJECTNAME)-* release/ 2>/dev/null || true
	@echo ""
	@echo "✓ Built $(PROJECTNAME) v$(VERSION)"
	@echo "  Binaries: $$(ls -1 binaries/ | wc -l) files"
	@echo "  Host binary: binaries/$(PROJECTNAME)"
ifeq ($(VERSION),$(shell cat release.txt 2>/dev/null))
	@echo "$(VERSION)" | awk -F. '{printf "%d.%d.%d\n", $$1, $$2, $$3+1}' > release.txt
	@echo "  Next version: $$(cat release.txt)"
else
	@echo "$(VERSION)" > release.txt
	@echo "  Version saved: $(VERSION)"
endif

# Create GitHub release
release: build
	@echo "Creating GitHub release v$(VERSION)..."
	@gh release delete v$(VERSION) -y 2>/dev/null || true
	@git tag -d v$(VERSION) 2>/dev/null || true
	@gh release create v$(VERSION) ./release/* \
		--title "$(PROJECTNAME) v$(VERSION)" \
		--notes "Release v$(VERSION)\n\nCommit: $(COMMIT)\nBuilt: $(BUILD_DATE)"
	@echo "✓ Release v$(VERSION) created"

# Run all tests
test:
	@echo "Running tests..."
	@go test -v -race -timeout 5m ./...
	@echo "✓ All tests passed"

# Build and push multi-platform Docker images
docker:
	@echo "Building multi-platform Docker images..."
	@docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-t ghcr.io/$(PROJECTORG)/$(PROJECTNAME):latest \
		-t ghcr.io/$(PROJECTORG)/$(PROJECTNAME):$(VERSION) \
		--push \
		.
	@echo "✓ Docker images pushed to ghcr.io/$(PROJECTORG)/$(PROJECTNAME):$(VERSION)"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf binaries/ release/ coverage.out
	@go clean
	@echo "✓ Clean complete"
