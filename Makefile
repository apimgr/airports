# ============================================================================
# Makefile for airports
# ----------------------------------------------------------------------------
# All Go work runs inside Docker (casjaysdev/go:latest). The host stays clean.
# CGO_ENABLED=0 is enforced everywhere - pure Go static binaries only.
# Targets (exactly seven core + clean and release-check, per AI.md PART 25):
#   make dev      - Quick development build to ${TMPDIR}/${PROJECT_ORG}/...
#   make local    - Production build to binaries/ (host platform, with version)
#   make build    - Full release: 8 platforms in binaries/
#   make release  - Manual local release: strip/archive/gh release in releases/
#   make beta     - Tag and push the current main commit for CI beta publishing
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
BINDIR          ?= binaries
RELDIR          ?= releases

# Local version precedence: VERSION override > non-empty release.txt > short commit id.
VERSION         ?= $(shell v=; [ -r release.txt ] && v=$$(tr -d '[:space:]' < release.txt); if [ -n "$$v" ]; then echo "$$v"; else git rev-parse --short=7 HEAD 2>/dev/null || echo devel; fi)
# Release tags use v only for semantic versions; text and timestamp tags stay plain.
RELEASE_TAG     := $(shell tag='$(VERSION)'; if printf '%s' "$$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+'; then printf '%s' "$$tag"; elif printf '%s' "$$tag" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+'; then printf 'v%s' "$$tag"; elif [ "$${tag#v}" != "$$tag" ]; then printf '%s' "$${tag#v}"; else printf '%s' "$$tag"; fi)
COMMIT_ID       := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
# Build info - BUILD_EPOCH is the single captured time source
# Unix build timestamp (seconds, UTC) - used by the updater's daily-channel check
BUILD_EPOCH     := $(shell date -u +%s)
# Derived from BUILD_EPOCH - ISO 8601 UTC; used only for Docker OCI labels, not an ldflag
BUILD_DATE      := $(shell date -u -d @$(BUILD_EPOCH) +"%Y-%m-%dT%H:%M:%SZ")
OFFICIAL_SITE   := $(shell if [ -f site.txt ]; then cat site.txt; else printf '%s' "$${OFFICIAL_SITE:-}"; fi)
REGISTRY        ?= ghcr.io/$(PROJECT_ORG)/$(PROJECT_NAME)

# BuildDate is NOT embedded - it is derived from BuildEpoch at process start
LDFLAGS         := -s -w \
                   -X 'github.com/apimgr/airports/src/app.Version=$(VERSION)' \
                   -X 'github.com/apimgr/airports/src/app.CommitID=$(COMMIT_ID)' \
                   -X 'github.com/apimgr/airports/src/app.BuildEpoch=$(BUILD_EPOCH)' \
                   -X 'github.com/apimgr/airports/src/app.OfficialSite=$(OFFICIAL_SITE)'
CLI_LDFLAGS     := -s -w \
                   -X 'main.Version=$(VERSION)' \
                   -X 'main.CommitID=$(COMMIT_ID)' \
                   -X 'main.BuildEpoch=$(BUILD_EPOCH)' \
                   -X 'main.OfficialSite=$(OFFICIAL_SITE)'

# Host detection (used by make local)
HOSTOS          := $(shell uname -s | tr '[:upper:]' '[:lower:]' | sed 's/mingw.*/windows/;s/msys.*/windows/')
HOSTARCH        := $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')

# Build temp dir (for `make dev`)
TMPDIR          ?= /tmp
DEV_DIR_BASE    := $(TMPDIR)/$(PROJECT_ORG)

# Go module/build cache (host-persisted across Docker runs)
GO_CACHE        ?= $(HOME)/go/pkg/mod
GO_BUILD        ?= $(HOME)/.cache/go-build/$(PROJECT_NAME)

# Docker image to build inside
GO_IMAGE        := casjaysdev/go:latest


DOCKER_MEM       ?= 4g
DOCKER_CPUS      ?= 2
DOCKER_CPU_SHARES ?= 256
DOCKER_TIMEOUT   ?= 3600

DOCKER_RUN       := timeout -k 30 $(DOCKER_TIMEOUT) docker run --rm --no-healthcheck \
	--name $(PROJECT_NAME)-$$(tr -dc 'a-z0-9' </dev/urandom | head -c8) \
	--memory=$(DOCKER_MEM) --cpus=$(DOCKER_CPUS) --cpu-shares=$(DOCKER_CPU_SHARES)

GO_DOCKER       := $(DOCKER_RUN) \
	-v $(PWD):/app -w /app \
	-v $(GO_CACHE):/usr/local/share/go/pkg/mod \
	-v $(GO_BUILD):/usr/local/share/go/cache \
	-e CGO_ENABLED=0 \
	-e GOFLAGS=-buildvcs=false \
	$(GO_IMAGE)

# Whether the optional client lives at src/client/
HAS_CLIENT      := $(shell test -d src/client && echo yes || echo no)

# ----------------------------------------------------------------------------
.PHONY: build local release release-check beta docker test dev clean

# ----------------------------------------------------------------------------
# make dev: quick build to a fresh temp dir for active development
# ----------------------------------------------------------------------------
dev:
	@mkdir -p $(DEV_DIR_BASE) $(GO_CACHE) $(GO_BUILD)
	@set -e; DEV_DIR=$$(mktemp -d $(DEV_DIR_BASE)/$(INTERNAL_NAME)-XXXXXX); \
	echo "Building $(PROJECT_NAME) (dev) -> $$DEV_DIR"; \
	$(DOCKER_RUN) \
		-v $(PWD):/app -w /app \
		-v $$DEV_DIR:/out \
		-v $(GO_CACHE):/usr/local/share/go/pkg/mod \
		-v $(GO_BUILD):/usr/local/share/go/cache \
		-e CGO_ENABLED=0 \
		-e GOFLAGS=-buildvcs=false \
		$(GO_IMAGE) sh -c '\
			set -e; \
			go mod tidy; \
			go mod download; \
			GOOS=linux GOARCH=amd64 go run ./src/swagger/cmd/openapi-gen; \
			go build -buildvcs=false -trimpath -ldflags "$(LDFLAGS)" -o /out/$(PROJECT_NAME) ./src && \
			if [ -d src/client ]; then go build -buildvcs=false -trimpath -ldflags "$(CLI_LDFLAGS)" -o /out/$(CLIENT_NAME) ./src/client; fi \
		'; \
	echo "Build dir: $$DEV_DIR"

# ----------------------------------------------------------------------------
# make local: production-style build for host platform only
# ----------------------------------------------------------------------------
local: clean
	@mkdir -p $(BINDIR) $(GO_CACHE) $(GO_BUILD)
	@echo "Building $(PROJECT_NAME) $(VERSION) for host ($(HOSTOS)/$(HOSTARCH))..."
	@$(DOCKER_RUN) \
		-v $(PWD):/app -w /app \
		-v $(abspath $(BINDIR)):/out \
		-v $(GO_CACHE):/usr/local/share/go/pkg/mod \
		-v $(GO_BUILD):/usr/local/share/go/cache \
		-e GOOS=$(HOSTOS) -e GOARCH=$(HOSTARCH) -e CGO_ENABLED=0 \
		-e GOFLAGS=-buildvcs=false \
		$(GO_IMAGE) sh -c '\
			set -e; \
			go mod tidy; \
			go mod download; \
			GOOS=linux GOARCH=amd64 go run ./src/swagger/cmd/openapi-gen; \
			go build -buildvcs=false -trimpath -ldflags "$(LDFLAGS)" -o /out/$(PROJECT_NAME) ./src; \
			if [ -d src/client ]; then go build -buildvcs=false -trimpath -ldflags "$(CLI_LDFLAGS)" -o /out/$(CLIENT_NAME) ./src/client; fi \
		'
	@echo "Built: $(BINDIR)/$(PROJECT_NAME)"

# ----------------------------------------------------------------------------
# make build: host binaries plus all 8 release platforms and supported GUI clients
# ----------------------------------------------------------------------------
build: clean
	@mkdir -p $(BINDIR) $(GO_CACHE) $(GO_BUILD)
	@echo "Building $(PROJECT_NAME) $(VERSION) for all platforms..."
	@$(DOCKER_RUN) \
		-v $(PWD):/app -w /app \
		-v $(abspath $(BINDIR)):/out \
		-v $(GO_CACHE):/usr/local/share/go/pkg/mod \
		-v $(GO_BUILD):/usr/local/share/go/cache \
		-e CGO_ENABLED=0 \
		-e GOFLAGS=-buildvcs=false \
		$(GO_IMAGE) sh -c '\
			set -e; \
			go mod tidy; \
			go mod download; \
			GOOS=linux GOARCH=amd64 go run ./src/swagger/cmd/openapi-gen; \
			hostext=""; [ "$(HOSTOS)" = "windows" ] && hostext=".exe"; \
			GOOS=$(HOSTOS) GOARCH=$(HOSTARCH) go build -buildvcs=false -trimpath -ldflags "$(LDFLAGS)" -o /out/$(PROJECT_NAME)$$hostext ./src; \
			if [ -d src/client ]; then GOOS=$(HOSTOS) GOARCH=$(HOSTARCH) go build -buildvcs=false -trimpath -ldflags "$(CLI_LDFLAGS)" -o /out/$(CLIENT_NAME)$$hostext ./src/client; fi; \
			for tgt in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64 freebsd/arm64; do \
				os=$${tgt%/*}; arch=$${tgt#*/}; ext=""; \
				[ "$$os" = "windows" ] && ext=".exe"; \
				echo "  -> $$os/$$arch"; \
				GOOS=$$os GOARCH=$$arch go build -buildvcs=false -trimpath -ldflags "$(LDFLAGS)" -o /out/$(PROJECT_NAME)-$$os-$$arch$$ext ./src; \
				if [ -d src/client ]; then \
					GOOS=$$os GOARCH=$$arch go build -buildvcs=false -trimpath -ldflags "$(CLI_LDFLAGS)" -o /out/$(CLIENT_NAME)-$$os-$$arch$$ext ./src/client; \
				fi; \
			done; \
			if [ -d src/client/gui ]; then \
				for tgt in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do \
					os=$${tgt%/*}; arch=$${tgt#*/}; ext=""; \
					[ "$$os" = "windows" ] && ext=".exe"; \
					echo "  -> GUI $$os/$$arch (dynamically linked)"; \
					GOOS=$$os GOARCH=$$arch go build -buildvcs=false -trimpath -tags gui -ldflags "$(CLI_LDFLAGS)" -o /out/$(CLIENT_NAME)-gui-$$os-$$arch$$ext ./src/client; \
				done; \
			fi \
		'
	@ls -1 $(BINDIR)/

# ----------------------------------------------------------------------------
# make release: manual local release (build -> strip -> archive -> gh release)
# NOTE: manual/local only - CI/CD (release.yml) handles automated releases.
# ----------------------------------------------------------------------------
release: release-check build
	@mkdir -p $(RELDIR)
	@echo "Preparing release $(VERSION)..."
	@echo "$(VERSION)" > $(RELDIR)/version.txt
	@for f in $(BINDIR)/$(PROJECT_NAME)-* $(BINDIR)/$(CLIENT_NAME)-*; do \
		[ -f "$$f" ] || continue; \
		strip "$$f" 2>/dev/null || true; \
		cp "$$f" $(RELDIR)/; \
	done
	@tar --exclude='.git' --exclude='.github' --exclude='.gitea' --exclude='.forgejo' \
		--exclude='$(BINDIR)' --exclude='$(RELDIR)' --exclude='*.tar.gz' \
		-czf $(RELDIR)/$(PROJECT_NAME)-$(VERSION)-source.tar.gz .
	@cd $(RELDIR) && FILES="$$(ls)" && sha256sum $$FILES > sha256.txt && sha512sum $$FILES > sha512.txt
	@gh release delete $(RELEASE_TAG) --yes 2>/dev/null || true
	@git tag -d $(RELEASE_TAG) 2>/dev/null || true
	@git push origin :refs/tags/$(RELEASE_TAG) 2>/dev/null || true
	@gh release create $(RELEASE_TAG) $(RELDIR)/* \
		--title "$(PROJECT_NAME) $(VERSION)" \
		--notes "Release $(VERSION)" \
		--latest
	@echo "Release complete: $(RELEASE_TAG)"

release-check:
	@printf '%s\n' "$(VERSION)" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$$' || { echo "ERROR: make release needs a stable SemVer X.Y.Z in release.txt (got '$(VERSION)')" >&2; exit 2; }

beta:
	@current_branch=$$(git branch --show-current); \
	if [ "$$current_branch" != main ]; then echo "ERROR: make beta requires the current branch to be main" >&2; exit 2; fi; \
	if [ -n "$$(git status --porcelain)" ]; then echo "ERROR: make beta requires a clean working tree" >&2; exit 2; fi; \
	tag=$$(date -u +%Y%m%d%H%M%S)-beta; \
	git tag "$$tag" && git push origin "$$tag" && echo "Pushed beta tag $$tag"

# ----------------------------------------------------------------------------
# make test: unit tests in Docker
# ----------------------------------------------------------------------------
test:
	@mkdir -p $(DEV_DIR_BASE) $(GO_CACHE) $(GO_BUILD)
	@echo "Running tests..."
	@$(GO_DOCKER) sh -c '\
		temp_parent="$${TMPDIR:-/tmp}/$(PROJECT_ORG)" && \
		mkdir -p "$$temp_parent" && \
		COVDIR=$$(mktemp -d "$$temp_parent/$(INTERNAL_NAME)-XXXXXX") && \
		trap "rm -rf $$COVDIR" EXIT && \
		export TMPDIR="$$COVDIR" && \
		go mod download && \
		GOOS=linux GOARCH=amd64 go run ./src/swagger/cmd/openapi-gen && \
		mkdir -p "$$COVDIR/data" && \
		go test -p 1 -timeout 5m -cover ./... -args -test.gocoverdir="$$COVDIR/data" && \
		go tool covdata textfmt -i="$$COVDIR/data" -o="$$COVDIR/coverage.out" && \
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
		-t $(REGISTRY):latest \
		-t $(REGISTRY):$(VERSION) \
		--push \
		.

# ----------------------------------------------------------------------------
# make clean
# ----------------------------------------------------------------------------
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BINDIR)/ $(RELDIR)/
	@echo "Clean complete"
