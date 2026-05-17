# ============================================================================
# Makefile for airports
# ----------------------------------------------------------------------------
# All Go work runs inside Docker (golang:alpine). The host stays clean.
# CGO_ENABLED=0 is enforced everywhere - pure Go static binaries only.
# Targets:
#   make dev      - Quick development build to ${TMPDIR}/${PROJECT_ORG}/...
#   make local    - Production build to binaries/ (host platform, with version)
#   make build    - Full release: 8 platforms in binaries/
#   make test     - Unit tests (in Docker)
#   make docker   - Multi-arch Docker build & push to ghcr.io
#   make clean    - Remove build artifacts
# ============================================================================

PROJECT_NAME    := airports
PROJECT_ORG     := apimgr
CLIENT_NAME     := airports-cli

# VERSION can be overridden: make build VERSION=1.2.3
VERSION         ?= $(shell cat release.txt 2>/dev/null || echo "0.0.1")
COMMIT          := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE      := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
OFFICIAL_SITE   ?= $(shell cat site.txt 2>/dev/null || echo "")

LDFLAGS         := -s -w \
                   -X 'main.Version=$(VERSION)' \
                   -X 'main.CommitID=$(COMMIT)' \
                   -X 'main.BuildDate=$(BUILD_DATE)' \
                   -X 'main.OfficialSite=$(OFFICIAL_SITE)'

# Host detection (used by make local)
HOSTOS          := $(shell go env GOOS 2>/dev/null || uname -s | tr '[:upper:]' '[:lower:]')
HOSTARCH        := $(shell go env GOARCH 2>/dev/null || uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')

# Build temp dir (for `make dev`)
TMPDIR          ?= /tmp
DEV_DIR_BASE    := $(TMPDIR)/$(PROJECT_ORG)

# Docker image to build inside
GO_IMAGE        := golang:alpine

# Whether the optional client lives at src/client/
HAS_CLIENT      := $(shell test -d src/client && echo yes || echo no)

# ----------------------------------------------------------------------------
.PHONY: help dev local build test lint docker docker-dev docker-test clean release
.DEFAULT_GOAL := help

help:
	@echo "airports Makefile targets:"
	@echo "  make dev         - Quick dev build to $(DEV_DIR_BASE)/$(PROJECT_NAME)-XXXXXX/"
	@echo "  make local       - Production build for host ($(HOSTOS)/$(HOSTARCH)) -> binaries/"
	@echo "  make build       - Full release: 8 platforms -> binaries/"
	@echo "  make test        - Unit tests (in Docker)"
	@echo "  make lint        - Run go vet and staticcheck (in Docker)"
	@echo "  make docker      - Multi-arch Docker build & push to ghcr.io"
	@echo "  make docker-dev  - Local dev Docker image (not pushed)"
	@echo "  make docker-test - Run docker/docker-compose.test.yml in temp dir"
	@echo "  make clean       - Remove build artifacts"

# ----------------------------------------------------------------------------
# make dev: quick build to a fresh temp dir for active development
# ----------------------------------------------------------------------------
dev:
	@mkdir -p $(DEV_DIR_BASE)
	@DEV_DIR=$$(mktemp -d $(DEV_DIR_BASE)/$(PROJECT_NAME)-XXXXXX); \
	echo "Building $(PROJECT_NAME) (dev) -> $$DEV_DIR"; \
	docker run --rm \
		-v $$(pwd):/workspace -w /workspace \
		-v $$DEV_DIR:/out \
		-e CGO_ENABLED=0 \
		$(GO_IMAGE) sh -c '\
			go build -ldflags "$(LDFLAGS)" -o /out/$(PROJECT_NAME) ./src && \
			if [ -d src/client ]; then go build -ldflags "$(LDFLAGS)" -o /out/$(CLIENT_NAME) ./src/client; fi \
		'; \
	echo "Build dir: $$DEV_DIR"

# ----------------------------------------------------------------------------
# make local: production-style build for host platform only
# ----------------------------------------------------------------------------
local:
	@mkdir -p binaries
	@echo "Building $(PROJECT_NAME) $(VERSION) for host ($(HOSTOS)/$(HOSTARCH))..."
	@docker run --rm \
		-v $$(pwd):/workspace -w /workspace \
		-e GOOS=$(HOSTOS) -e GOARCH=$(HOSTARCH) -e CGO_ENABLED=0 \
		$(GO_IMAGE) sh -c '\
			go build -ldflags "$(LDFLAGS)" -o binaries/$(PROJECT_NAME) ./src && \
			if [ -d src/client ]; then go build -ldflags "$(LDFLAGS)" -o binaries/$(CLIENT_NAME) ./src/client; fi \
		'
	@echo "Built: binaries/$(PROJECT_NAME)"

# ----------------------------------------------------------------------------
# make build: full 8-platform release
# ----------------------------------------------------------------------------
build:
	@mkdir -p binaries
	@echo "Building $(PROJECT_NAME) $(VERSION) for all platforms..."
	@docker run --rm \
		-v $$(pwd):/workspace -w /workspace \
		-e CGO_ENABLED=0 \
		$(GO_IMAGE) sh -c '\
			set -e; \
			for tgt in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64 freebsd/arm64; do \
				os=$${tgt%/*}; arch=$${tgt#*/}; ext=""; \
				[ "$$os" = "windows" ] && ext=".exe"; \
				echo "  -> $$os/$$arch"; \
				GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o binaries/$(PROJECT_NAME)-$$os-$$arch$$ext ./src; \
				if [ -d src/client ]; then \
					GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" -o binaries/$(CLIENT_NAME)-$$os-$$arch$$ext ./src/client; \
				fi; \
			done \
		'
	@ls -1 binaries/

# ----------------------------------------------------------------------------
# make test: unit tests in Docker
# ----------------------------------------------------------------------------
test:
	@echo "Running tests..."
	@docker run --rm \
		-v $$(pwd):/workspace -w /workspace \
		-e CGO_ENABLED=0 \
		$(GO_IMAGE) sh -c '\
			go test -timeout 5m ./... \
		'
	@echo "All tests passed"

# ----------------------------------------------------------------------------
# make lint: run go vet and staticcheck in Docker
# ----------------------------------------------------------------------------
lint:
	@echo "Running linters..."
	@docker run --rm \
		-v $$(pwd):/workspace -w /workspace \
		-e CGO_ENABLED=0 \
		$(GO_IMAGE) sh -c '\
			go vet ./... && \
			if command -v staticcheck >/dev/null 2>&1; then staticcheck ./...; else echo "staticcheck not installed, skipping"; fi \
		'
	@echo "Lint complete"

# ----------------------------------------------------------------------------
# make docker-test: run test compose in temp dir (AI/automated only)
# ----------------------------------------------------------------------------
docker-test:
	@echo "Running docker-compose test workflow..."
	@mkdir -p $(DEV_DIR_BASE)
	@TEMP_DIR=$$(mktemp -d $(DEV_DIR_BASE)/$(PROJECT_NAME)-XXXXXX); \
	mkdir -p "$$TEMP_DIR/volumes/config" "$$TEMP_DIR/volumes/data"; \
	cp docker/docker-compose.test.yml "$$TEMP_DIR/docker-compose.yml"; \
	echo "Test dir: $$TEMP_DIR"; \
	(cd "$$TEMP_DIR" && docker compose up --abort-on-container-exit); \
	EXIT=$$?; \
	(cd "$$TEMP_DIR" && docker compose down --volumes --remove-orphans 2>/dev/null || true); \
	rm -rf "$$TEMP_DIR"; \
	exit $$EXIT

# ----------------------------------------------------------------------------
# make docker: multi-arch image build & push (uses docker/Dockerfile)
# ----------------------------------------------------------------------------
docker:
	@echo "Building & pushing multi-arch image..."
	@docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg VCS_REF=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-f docker/Dockerfile \
		-t ghcr.io/$(PROJECT_ORG)/$(PROJECT_NAME):latest \
		-t ghcr.io/$(PROJECT_ORG)/$(PROJECT_NAME):$(VERSION) \
		--push \
		.

docker-dev:
	@echo "Building local dev image..."
	@docker build \
		--build-arg VERSION=$(VERSION)-dev \
		--build-arg VCS_REF=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		-f docker/Dockerfile \
		-t $(PROJECT_NAME):dev \
		.

# ----------------------------------------------------------------------------
# make clean
# ----------------------------------------------------------------------------
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf binaries/ releases/ coverage.out
	@find $(DEV_DIR_BASE) -maxdepth 1 -type d -name '$(PROJECT_NAME)-*' -exec rm -rf {} + 2>/dev/null || true
	@echo "Clean complete"
