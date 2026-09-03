# ============================================================================
# Makefile for airports
# ----------------------------------------------------------------------------
# All Go work runs inside Docker (casjaysdev/go:latest). The host stays clean.
# CGO_ENABLED=0 is enforced everywhere - pure Go static binaries only.
# Targets (exactly six core + clean, per AI.md PART 25 - never add more):
#   make dev      - Quick development build to ${TMPDIR}/${PROJECT_ORG}/...
#   make local    - Production build to binaries/ (host platform, with version)
#   make build    - Full release: 8 platforms in binaries/
#   make release  - Manual local release: strip/archive/gh release in releases/
#   make test     - Unit tests (in Docker)
#   make docker   - Multi-arch Docker build & push to ghcr.io
#   make clean    - Remove build artifacts (prerequisite of build/local)
# ============================================================================

PROJECT_NAME    := $(shell git remote get-url origin 2>/dev/null | sed -E 's|.*/([^/]+)(\.git)?$$|\1|' || basename "$$(pwd)")
PROJECT_ORG     := $(shell git remote get-url origin 2>/dev/null | sed -E 's|.*/([^/]+)/[^/]+(\.git)?$$|\1|' || basename "$$(dirname "$$(pwd)")")
# Frozen forever per AI.md PART 2/3 - read from IDEA.md, never re-derived from PROJECT_NAME
INTERNAL_NAME   := $(shell grep -E '^internal_name:[[:space:]]*.+$$' IDEA.md 2>/dev/null | sed -E 's/^internal_name:[[:space:]]*//' | tr -d '[:space:]')
ifeq ($(INTERNAL_NAME),)
INTERNAL_NAME   := $(PROJECT_NAME)
endif
CLIENT_NAME     := airports-cli

# Version precedence: release.txt (wins if present) > VERSION env var > "devel" fallback
# Command-line override still works: make build VERSION=1.2.3
VERSION         := $(shell cat release.txt 2>/dev/null || echo "$${VERSION:-devel}")
COMMIT_ID       := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
# Build info - BUILD_EPOCH is the single captured time source
# Unix build timestamp (seconds, UTC) - used by the updater's daily-channel check
BUILD_EPOCH     := $(shell date -u +%s)
# Derived from BUILD_EPOCH - ISO 8601 UTC; used only for Docker OCI labels, not an ldflag
BUILD_DATE      := $(shell date -u -d @$(BUILD_EPOCH) +"%Y-%m-%dT%H:%M:%SZ")
OFFICIAL_SITE   ?= $(shell cat site.txt 2>/dev/null || echo "")

# BuildDate is NOT embedded - it is derived from BuildEpoch at process start
LDFLAGS         := -s -w -trimpath \
                   -X 'main.Version=$(VERSION)' \
                   -X 'main.CommitID=$(COMMIT_ID)' \
                   -X 'main.BuildEpoch=$(BUILD_EPOCH)' \
                   -X 'main.OfficialSite=$(OFFICIAL_SITE)'

# Host detection (used by make local)
HOSTOS          := $(shell go env GOOS 2>/dev/null || uname -s | tr '[:upper:]' '[:lower:]')
HOSTARCH        := $(shell go env GOARCH 2>/dev/null || uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')

# Build temp dir (for `make dev`)
TMPDIR          ?= /tmp
DEV_DIR_BASE    := $(TMPDIR)/$(PROJECT_ORG)

# Go module/build cache (host-persisted across Docker runs)
GO_CACHE        ?= $(HOME)/go/pkg/mod
GO_BUILD        ?= $(HOME)/.cache/go-build/$(PROJECT_NAME)

# Docker image to build inside
GO_IMAGE        := casjaysdev/go:latest

GO_DOCKER       := docker run --rm \
	--name $(PROJECT_NAME)-$$(tr -dc 'a-z0-9' </dev/urandom | head -c8) \
	-v $$(pwd):/workspace -w /workspace \
	-v $(GO_CACHE):/usr/local/share/go/pkg/mod \
	-v $(GO_BUILD):/usr/local/share/go/cache \
	-e CGO_ENABLED=0 \
	-e GOFLAGS=-buildvcs=false \
	$(GO_IMAGE)

# Whether the optional client lives at src/client/
HAS_CLIENT      := $(shell test -d src/client && echo yes || echo no)

# ----------------------------------------------------------------------------
.PHONY: build local release docker test dev clean

# ----------------------------------------------------------------------------
# make dev: quick build to a fresh temp dir for active development
# ----------------------------------------------------------------------------
dev:
	@mkdir -p $(DEV_DIR_BASE) $(GO_CACHE) $(GO_BUILD)
	@DEV_DIR=$$(mktemp -d $(DEV_DIR_BASE)/$(INTERNAL_NAME)-XXXXXX); \
	echo "Building $(PROJECT_NAME) (dev) -> $$DEV_DIR"; \
	docker run --rm \
		--name $(PROJECT_NAME)-$$(tr -dc 'a-z0-9' </dev/urandom | head -c8) \
		-v $$(pwd):/workspace -w /workspace \
		-v $$DEV_DIR:/out \
		-v $(GO_CACHE):/usr/local/share/go/pkg/mod \
		-v $(GO_BUILD):/usr/local/share/go/cache \
		-e CGO_ENABLED=0 \
		-e GOFLAGS=-buildvcs=false \
		$(GO_IMAGE) sh -c '\
			go build -buildvcs=false -ldflags "$(LDFLAGS)" -o /out/$(PROJECT_NAME) ./src && \
			if [ -d src/client ]; then go build -buildvcs=false -ldflags "$(LDFLAGS)" -o /out/$(CLIENT_NAME) ./src/client; fi \
		'; \
	echo "Build dir: $$DEV_DIR"

# ----------------------------------------------------------------------------
# make local: production-style build for host platform only
# ----------------------------------------------------------------------------
local: clean
	@mkdir -p binaries $(GO_CACHE) $(GO_BUILD)
	@echo "Building $(PROJECT_NAME) $(VERSION) for host ($(HOSTOS)/$(HOSTARCH))..."
	@docker run --rm \
		--name $(PROJECT_NAME)-$$(tr -dc 'a-z0-9' </dev/urandom | head -c8) \
		-v $$(pwd):/workspace -w /workspace \
		-v $(GO_CACHE):/usr/local/share/go/pkg/mod \
		-v $(GO_BUILD):/usr/local/share/go/cache \
		-e GOOS=$(HOSTOS) -e GOARCH=$(HOSTARCH) -e CGO_ENABLED=0 \
		-e GOFLAGS=-buildvcs=false \
		$(GO_IMAGE) sh -c '\
			go build -buildvcs=false -ldflags "$(LDFLAGS)" -o binaries/$(PROJECT_NAME) ./src && \
			if [ -d src/client ]; then go build -buildvcs=false -ldflags "$(LDFLAGS)" -o binaries/$(CLIENT_NAME) ./src/client; fi \
		'
	@echo "Built: binaries/$(PROJECT_NAME)"

# ----------------------------------------------------------------------------
# make build: full 8-platform release
# ----------------------------------------------------------------------------
build: clean
	@mkdir -p binaries $(GO_CACHE) $(GO_BUILD)
	@echo "Building $(PROJECT_NAME) $(VERSION) for all platforms..."
	@docker run --rm \
		--name $(PROJECT_NAME)-$$(tr -dc 'a-z0-9' </dev/urandom | head -c8) \
		-v $$(pwd):/workspace -w /workspace \
		-v $(GO_CACHE):/usr/local/share/go/pkg/mod \
		-v $(GO_BUILD):/usr/local/share/go/cache \
		-e CGO_ENABLED=0 \
		-e GOFLAGS=-buildvcs=false \
		$(GO_IMAGE) sh -c '\
			set -e; \
			for tgt in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64 freebsd/arm64; do \
				os=$${tgt%/*}; arch=$${tgt#*/}; ext=""; \
				[ "$$os" = "windows" ] && ext=".exe"; \
				echo "  -> $$os/$$arch"; \
				GOOS=$$os GOARCH=$$arch go build -buildvcs=false -ldflags "$(LDFLAGS)" -o binaries/$(PROJECT_NAME)-$$os-$$arch$$ext ./src; \
				if [ -d src/client ]; then \
					GOOS=$$os GOARCH=$$arch go build -buildvcs=false -ldflags "$(LDFLAGS)" -o binaries/$(CLIENT_NAME)-$$os-$$arch$$ext ./src/client; \
				fi; \
			done \
		'
	@ls -1 binaries/

# ----------------------------------------------------------------------------
# make release: manual local release (build -> strip -> archive -> gh release)
# NOTE: manual/local only - CI/CD (release.yml) handles automated releases.
# ----------------------------------------------------------------------------
release: build
	@mkdir -p releases
	@echo "Preparing release $(VERSION)..."
	@echo "$(VERSION)" > releases/version.txt
	@for f in binaries/$(PROJECT_NAME)-* binaries/$(CLIENT_NAME)-*; do \
		[ -f "$$f" ] || continue; \
		strip "$$f" 2>/dev/null || true; \
		cp "$$f" releases/; \
	done
	@tar --exclude='.git' --exclude='.github' --exclude='.gitea' \
		--exclude='binaries' --exclude='releases' --exclude='*.tar.gz' \
		-czf releases/$(PROJECT_NAME)-$(VERSION)-source.tar.gz .
	@gh release delete $(VERSION) --yes 2>/dev/null || true
	@git tag -d $(VERSION) 2>/dev/null || true
	@git push origin :refs/tags/$(VERSION) 2>/dev/null || true
	@gh release create $(VERSION) releases/* \
		--title "$(PROJECT_NAME) $(VERSION)" \
		--notes "Release $(VERSION)" \
		--latest
	@echo "Release complete: $(VERSION)"

# ----------------------------------------------------------------------------
# make test: unit tests in Docker
# ----------------------------------------------------------------------------
test:
	@mkdir -p $(DEV_DIR_BASE) $(GO_CACHE) $(GO_BUILD)
	@echo "Running tests..."
	@$(GO_DOCKER) sh -c '\
		mkdir -p "/tmp/$(PROJECT_ORG)" && \
		COVDIR=$$(mktemp -d "/tmp/$(PROJECT_ORG)/$(INTERNAL_NAME)-XXXXXX") && \
		go test -timeout 5m -covermode=atomic -coverprofile=$$COVDIR/coverage.out ./... && \
		total=$$(go tool cover -func=$$COVDIR/coverage.out | awk "/^total:/ {gsub(/%/, \"\", \$$3); print \$$3}") && \
		echo "Total coverage: $${total}%" && \
		awk "BEGIN { exit !($${total} >= 60) }" || { echo "Coverage $${total}% is below the 60% floor"; exit 1; } \
	'
	@echo "All tests passed"

# ----------------------------------------------------------------------------
# make docker: multi-arch image build & push (uses docker/Dockerfile)
# ----------------------------------------------------------------------------
docker:
	@echo "Building & pushing multi-arch image..."
	@docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT_ID=$(COMMIT_ID) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		--build-arg BUILD_EPOCH=$(BUILD_EPOCH) \
		--build-arg OFFICIAL_SITE=$(OFFICIAL_SITE) \
		-f docker/Dockerfile \
		-t ghcr.io/$(PROJECT_ORG)/$(PROJECT_NAME):latest \
		-t ghcr.io/$(PROJECT_ORG)/$(PROJECT_NAME):$(VERSION) \
		--push \
		.

# ----------------------------------------------------------------------------
# make clean
# ----------------------------------------------------------------------------
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf binaries/ releases/ coverage.out
	@find $(DEV_DIR_BASE) -maxdepth 1 -type d -name '$(INTERNAL_NAME)-*' -exec rm -rf {} + 2>/dev/null || true
	@echo "Clean complete"
