# Webox — canonical task interface
# Every dev workflow goes through this file. CI mirrors targets 1:1 — what
# passes locally MUST pass in CI and vice versa. No ad-hoc scripts.
#
# Conventions:
# - phony targets only; no real file outputs (those live in /dist/, /bin/).
# - any target that depends on Go tools shells out through `go run` for
#   reproducibility; no global tool versions sneaking in.
# - tabs (Make requires them) — gofmt/golangci-lint handle Go style.
# - `make help` prints every target with its one-line summary.

# Tool versions are pinned in tools/go.mod (Go 1.24 `tool` directive in
# an isolated modfile so they don't constrain the main module's go
# version). All `make` targets invoke them via `go tool -modfile=...`.
# To bump, run `cd tools && go get -tool <pkg>@<version>` then
# `make tools-tidy`.

SHELL          := /bin/bash
.SHELLFLAGS    := -eu -o pipefail -c
.DEFAULT_GOAL  := help

MODULE         := github.com/webox/webox
GO             ?= go
GOFLAGS        ?= -trimpath
LDFLAGS_BASE   := -s -w
VERSION        ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.0.0-dev")
COMMIT         := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE           := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS        := $(LDFLAGS_BASE) -X $(MODULE)/internal/version.Version=$(VERSION) \
                                  -X $(MODULE)/internal/version.Commit=$(COMMIT) \
                                  -X $(MODULE)/internal/version.Date=$(DATE)

BIN_DIR        := bin
DIST_DIR       := dist
COVER_FILE     := coverage.out
COVER_HTML     := coverage.html
COVER_MIN      := 70
TOOLS_MODFILE  := tools/go.mod
# Some dev tools (golangci-lint v2.12+) require a newer Go than the main
# module's `go 1.24`. Auto-toolchain doesn't kick in when -modfile=
# points at a different module, so we extract the tools' `go` directive
# and pin GOTOOLCHAIN explicitly. Result: every contributor — and CI —
# uses the exact same Go version for linting/formatting (reproducible),
# auto-fetched on first run if not already cached.
TOOLS_GO       := $(shell awk '$$1=="go"{print "go"$$2; exit}' $(TOOLS_MODFILE) 2>/dev/null)
TOOL           := GOTOOLCHAIN=$(TOOLS_GO) $(GO) tool -modfile=$(TOOLS_MODFILE)

COLOR_RESET    := \033[0m
COLOR_BOLD     := \033[1m
COLOR_GREEN    := \033[32m
COLOR_CYAN     := \033[36m
COLOR_YELLOW   := \033[33m
COLOR_RED      := \033[31m

# ── Help ───────────────────────────────────────────────────────────────

.PHONY: help
help: ## Print this help screen.
	@printf "$(COLOR_BOLD)Webox — Makefile$(COLOR_RESET)\n"
	@printf "Usage: $(COLOR_CYAN)make <target>$(COLOR_RESET)\n\n"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / \
		{printf "  $(COLOR_GREEN)%-22s$(COLOR_RESET) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# ── Build ──────────────────────────────────────────────────────────────

.PHONY: build
build: ## Build local webox binary into bin/webox.
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/webox ./cmd/webox

.PHONY: install
install: ## Install webox into $GOPATH/bin via `go install`.
	$(GO) install $(GOFLAGS) -ldflags "$(LDFLAGS)" ./cmd/webox

.PHONY: snapshot
snapshot: ## GoReleaser snapshot build (every OS/arch matrix) into dist/.
	$(TOOL) goreleaser release --snapshot --clean

.PHONY: clean
clean: ## Remove build artifacts and coverage output.
	rm -rf $(BIN_DIR) $(DIST_DIR) $(COVER_FILE) $(COVER_HTML)

# ── Test ───────────────────────────────────────────────────────────────

.PHONY: test
test: ## Run all tests with race detector and coverage.
	$(GO) test -race -timeout 120s -coverprofile=$(COVER_FILE) -covermode=atomic ./...

.PHONY: test-short
test-short: ## Run only short unit tests (skip integration + e2e).
	$(GO) test -race -short -timeout 60s ./...

.PHONY: test-tui
test-tui: ## Regenerate teatest golden files (-update). Review diff before commit.
	$(GO) test -update ./tui/...

.PHONY: test-integration
test-integration: ## Run integration tests (needs sshmock + cassettes).
	$(GO) test -race -tags=integration -timeout 300s ./...

.PHONY: test-integration-live
test-integration-live: ## Run maintainer-only live tests against real small.pl sandbox.
	@test -n "$${WEBOX_TEST_HOST:-}" || { echo "WEBOX_TEST_HOST required"; exit 1; }
	@test -n "$${WEBOX_TEST_USER:-}" || { echo "WEBOX_TEST_USER required"; exit 1; }
	@test -n "$${WEBOX_TEST_KEY:-}" || { echo "WEBOX_TEST_KEY required"; exit 1; }
	$(GO) test -race -tags='integration live' -timeout 600s ./...

.PHONY: cover
cover: test ## Show coverage report in a browser.
	$(GO) tool cover -html=$(COVER_FILE) -o $(COVER_HTML)
	@printf "$(COLOR_CYAN)Open $(COVER_HTML) in your browser.$(COLOR_RESET)\n"

.PHONY: cover-check
cover-check: test ## Fail if total coverage is below COVER_MIN (default 70).
	@total=$$($(GO) tool cover -func=$(COVER_FILE) | awk '/total:/ {print $$3}' | tr -d '%'); \
	awk -v t=$$total -v min=$(COVER_MIN) 'BEGIN { exit !(t+0 >= min+0) }' \
		&& printf "$(COLOR_GREEN)✓ coverage %s%% ≥ %s%%$(COLOR_RESET)\n" $$total $(COVER_MIN) \
		|| { printf "$(COLOR_RED)✗ coverage %s%% < %s%%$(COLOR_RESET)\n" $$total $(COVER_MIN); exit 1; }

# ── Lint / Static analysis ─────────────────────────────────────────────

.PHONY: lint
lint: ## Run golangci-lint v2 (covers gofmt, vet, staticcheck, gosec, …).
	$(TOOL) golangci-lint run ./...

.PHONY: fmt
fmt: ## Run gofumpt + goimports on all packages.
	@$(TOOL) gofumpt -l -w .
	@$(TOOL) goimports -l -w .

.PHONY: vet
vet: ## Run go vet.
	$(GO) vet ./...

.PHONY: vulncheck
vulncheck: ## Run govulncheck against current dependencies.
	$(TOOL) govulncheck ./...

# ── Doctor / Smoke ─────────────────────────────────────────────────────

.PHONY: doctor
doctor: build ## Run `webox doctor` against current environment.
	$(BIN_DIR)/webox doctor

.PHONY: doctor-security
doctor-security: build ## Run `webox doctor security` (post-MVP).
	$(BIN_DIR)/webox doctor security

# ── Modules / Tools ────────────────────────────────────────────────────

.PHONY: deps
deps: ## Download Go modules.
	$(GO) mod download

.PHONY: tidy
tidy: ## go mod tidy + verify.
	$(GO) mod tidy
	$(GO) mod verify

.PHONY: tools-tidy
tools-tidy: ## Sync development tool versions (tools/go.mod) + main go.mod.
	$(GO) mod tidy
	cd tools && $(GO) mod tidy

.PHONY: tools-install
tools-install: ## Install dev tools (from tools/go.mod) into $$(go env GOBIN) for direct CLI use.
	@printf "$(COLOR_CYAN)Installing dev tools to $$($(GO) env GOBIN)…$(COLOR_RESET)\n"
	@cd tools && for t in github.com/golangci/golangci-lint/v2/cmd/golangci-lint \
	                       golang.org/x/vuln/cmd/govulncheck \
	                       mvdan.cc/gofumpt \
	                       golang.org/x/tools/cmd/goimports \
	                       github.com/goreleaser/goreleaser/v2; do \
	  printf "  $(COLOR_BOLD)%s$(COLOR_RESET)\n" "$$t"; \
	  $(GO) install "$$t" || exit 1; \
	done
	@printf "$(COLOR_GREEN)✓ tools installed$(COLOR_RESET)\n"

# ── Documentation / Audit ──────────────────────────────────────────────

.PHONY: docs-links
docs-links: ## Check all relative links in docs/ resolve.
	@$(GO) run github.com/raviqqe/muffet/v2@latest \
		--exclude '^http' \
		--rate-limit 10 \
		file://$$(pwd)/docs/

.PHONY: audit-status
audit-status: ## Print outstanding AUDIT.md items.
	@printf "$(COLOR_BOLD)AUDIT.md outstanding items:$(COLOR_RESET)\n"
	@grep -E "^### [A-D][0-9]+\." docs/AUDIT.md | head -20 || true
	@printf "\n$(COLOR_BOLD)Folded IMP-* findings in AUDIT.md §8:$(COLOR_RESET)\n"
	@grep -E "^\| IMP-[0-9]+ \|" docs/AUDIT.md | head -20 || true

# ── Internationalization ───────────────────────────────────────────────

.PHONY: i18n-check
i18n-check: ## Verify translations/ have identical key sets.
	@$(GO) run ./tools/i18ncheck

# ── Release (maintainer only) ──────────────────────────────────────────

.PHONY: release-dry-run
release-dry-run: ## GoReleaser dry-run for a release candidate.
	$(TOOL) goreleaser release --snapshot --skip=publish --clean

# ── Dev workflow / automation ──────────────────────────────────────────

PKG ?= ./...

.PHONY: dev
dev: ## TDD watch loop: re-runs `go test` on every change. Override with PKG=./config/...
	@scripts/dev-watch.sh "$(PKG)"

.PHONY: bootstrap
bootstrap: ## One-shot local environment setup: tools, deps, git hooks.
	@scripts/bootstrap.sh

.PHONY: setup-hooks
setup-hooks: ## Install versioned git hooks (.githooks/) into this clone.
	@scripts/install-git-hooks.sh

.PHONY: sprint-status
sprint-status: ## Show current sprint progress (tasks done / open).
	@scripts/sprint-status.sh

.PHONY: next-task
next-task: ## Print the next open task id (parseable) from current sprint.
	@scripts/next-task.sh

.PHONY: next-task-verbose
next-task-verbose: ## Print the next open task with full block.
	@scripts/next-task.sh --verbose

.PHONY: sprint-start
sprint-start: ## Create branch + open current sprint plan. Pass TASK=TASK-01.3 to pick explicitly.
	@scripts/start-sprint.sh $(TASK) $(SLUG)

.PHONY: new-task
new-task: ## Append a task to current sprint. Required: NAME="..."  Optional: EST=M
	@test -n "$(NAME)" || { echo "usage: make new-task NAME=\"...\" [EST=S|M|L|XL]"; exit 1; }
	@scripts/new-task.sh "$(NAME)" "$(EST)"

.PHONY: retro
retro: ## Generate retrospective skeleton for current sprint (or pass SPRINT=01).
	@scripts/retro-new.sh $(SPRINT) $(DATE)

.PHONY: pr
pr: ## Create draft PR via gh, body pre-filled from sprint/task context.
	@scripts/pr-create.sh $(TITLE)

.PHONY: commit-suggest
commit-suggest: ## Print Conventional Commit suggestion from staged changes.
	@scripts/commit-msg-suggest.sh

.PHONY: changelog
changelog: ## Append entry to CHANGELOG.md Unreleased. Required: KIND=added|fixed|... MSG="..."
	@test -n "$(KIND)" -a -n "$(MSG)" || { echo "usage: make changelog KIND=added MSG=\"...\""; exit 1; }
	@scripts/changelog-add.sh "$(KIND)" "$(MSG)"

# ── CI bundle ──────────────────────────────────────────────────────────

.PHONY: ci
ci: tidy lint vet vulncheck test cover-check build ## Run the exact CI bundle locally.
	@printf "$(COLOR_GREEN)✓ CI bundle passed.$(COLOR_RESET)\n"

.PHONY: ci-fast
ci-fast: lint vet test-short ## Lightweight CI subset for local pre-push.
	@printf "$(COLOR_GREEN)✓ ci-fast passed.$(COLOR_RESET)\n"
