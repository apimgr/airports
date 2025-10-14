# Variables
PROJECTNAME = airports
PROJECTORG = casapps

# Allow VERSION to be overridden via command line: make build VERSION=1.2.3
# Otherwise read from release.txt or default to 0.0.1
ifndef VERSION
VERSION = $(shell cat release.txt 2>/dev/null || echo "0.0.1")
endif

COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILD_DATE) -w -s"

# Detect host OS and ARCH
HOSTOS := $(shell go env GOOS)
HOSTARCH := $(shell go env GOARCH)

# Incus instance name
INCUS_INSTANCE = $(PROJECTNAME)-test

# Targets
.PHONY: build release test docker clean deps incus-test incus-shell incus-run incus-logs incus-clean

# Install dependencies
deps:
	@echo "Installing Go dependencies..."
	@go mod download
	@go mod tidy

# Build for all platforms using Docker Alpine
build: deps
	@echo "Building $(PROJECTNAME) v$(VERSION) for all platforms using Docker Alpine..."
	@mkdir -p binaries release
	@docker run --rm -v $$(pwd):/workspace -w /workspace golang:1.23-alpine sh -c ' \
		apk add --no-cache git make > /dev/null 2>&1 && \
		echo "  → Linux AMD64" && \
		GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-linux-amd64 ./src && \
		echo "  → Linux ARM64" && \
		GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-linux-arm64 ./src && \
		echo "  → Windows AMD64" && \
		GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-windows-amd64.exe ./src && \
		echo "  → Windows ARM64" && \
		GOOS=windows GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-windows-arm64.exe ./src && \
		echo "  → macOS AMD64" && \
		GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-macos-amd64 ./src && \
		echo "  → macOS ARM64 (Apple Silicon)" && \
		GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-macos-arm64 ./src && \
		echo "  → FreeBSD AMD64" && \
		GOOS=freebsd GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-bsd-amd64 ./src && \
		echo "  → FreeBSD ARM64" && \
		GOOS=freebsd GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME)-bsd-arm64 ./src && \
		echo "  → Host (linux/amd64)" && \
		CGO_ENABLED=0 go build $(LDFLAGS) -o binaries/$(PROJECTNAME) ./src \
	'
	@cp binaries/$(PROJECTNAME)-* release/ 2>/dev/null || true
ifndef VERSION
	@echo "$(VERSION)" | awk -F. '{printf "%d.%d.%d\n", $$1, $$2, $$3+1}' > release.txt
	@echo "✓ Built version $(VERSION) → $$(cat release.txt)"
else
	@echo "✓ Built version $(VERSION) (manually set)"
endif

# Create GitHub release
release: build
	@echo "Creating GitHub release v$(VERSION)..."
	@gh release delete v$(VERSION) -y 2>/dev/null || true
	@git tag -d v$(VERSION) 2>/dev/null || true
	@gh release create v$(VERSION) ./release/* \
		--title "$(PROJECTNAME) v$(VERSION)" \
		--notes "Airport location API server with GeoIP integration\n\nCommit: $(COMMIT)\nBuilt: $(BUILD_DATE)"
	@echo "✓ Release v$(VERSION) created"

# Run tests
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@echo ""
	@echo "Running benchmarks..."
	@go test -v -bench=. -benchmem ./...
	@echo "✓ Tests complete"

# Build and push Docker image
docker:
	@echo "Building Docker image..."
	@docker build -t $(PROJECTNAME):dev \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) .
	@docker tag $(PROJECTNAME):dev ghcr.io/$(PROJECTORG)/$(PROJECTNAME):latest
	@docker tag $(PROJECTNAME):dev ghcr.io/$(PROJECTORG)/$(PROJECTNAME):$(VERSION)
	@echo "Pushing to ghcr.io..."
	@docker push ghcr.io/$(PROJECTORG)/$(PROJECTNAME):latest
	@docker push ghcr.io/$(PROJECTORG)/$(PROJECTNAME):$(VERSION)
	@echo "✓ Docker images pushed"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf binaries/ release/
	@rm -f coverage.out
	@go clean
	@echo "✓ Clean complete"

# Incus Alpine - Testing/Debugging Only
# Use this for interactive debugging and testing in a clean Alpine environment
incus-test:
	@echo "Setting up Incus Alpine environment for testing/debugging..."
	@incus launch images:alpine/edge $(INCUS_INSTANCE) 2>/dev/null || incus start $(INCUS_INSTANCE) 2>/dev/null || true
	@echo "Waiting for instance to start..."
	@sleep 3
	@echo "Installing dependencies..."
	@incus exec $(INCUS_INSTANCE) -- apk add --no-cache bash curl vim nano git htop
	@echo "Copying project files..."
	@incus exec $(INCUS_INSTANCE) -- mkdir -p /workspace
	@incus file push -r . $(INCUS_INSTANCE)/workspace/ 2>/dev/null || true
	@echo ""
	@echo "✓ Incus Alpine test environment ready!"
	@echo ""
	@echo "Instance: $(INCUS_INSTANCE)"
	@echo "Workspace: /workspace"
	@echo ""
	@echo "Usage:"
	@echo "  make incus-shell  - Open interactive shell"
	@echo "  make incus-run    - Run the application"
	@echo "  make incus-logs   - View application logs"
	@echo "  make incus-clean  - Remove instance"

incus-shell:
	@echo "Opening shell in $(INCUS_INSTANCE)..."
	@echo "Working directory: /workspace"
	@incus exec $(INCUS_INSTANCE) -- /bin/sh -c "cd /workspace && exec /bin/bash || exec /bin/sh"

incus-run:
	@echo "Running $(PROJECTNAME) in Incus..."
	@incus exec $(INCUS_INSTANCE) -- /bin/sh -c "cd /workspace && ./binaries/$(PROJECTNAME) || echo 'Binary not found. Build first with: docker run --rm -v \$$(pwd):/workspace -w /workspace golang:1.21-alpine sh -c \"apk add git make && make build\"'"

incus-logs:
	@echo "Viewing logs in $(INCUS_INSTANCE)..."
	@incus exec $(INCUS_INSTANCE) -- /bin/sh -c "cd /workspace && tail -f *.log 2>/dev/null || echo 'No log files found'"

incus-clean:
	@echo "Removing Incus test instance..."
	@incus stop $(INCUS_INSTANCE) 2>/dev/null || true
	@incus delete $(INCUS_INSTANCE) 2>/dev/null || true
	@echo "✓ Incus instance removed"

.DEFAULT_GOAL := build
