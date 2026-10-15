# Development and CI commands (specs/001-empresas-usuarios/tasks.md, "Comandos de validación").
# `make check` validates the backend; `make check-all` validates the backend and frontend.

GO   ?= go
NPM  ?= npm

# One exact Go version (ADR-026 I-2 and its 2026-10-14 note). setup-go only installs the latest
# 1.27.x and, with GOTOOLCHAIN=auto, the `toolchain` line is a minimum, so a newer patch would be
# used silently. Every target runs with exactly the `toolchain` of go.mod (go downloads it if
# needed). The guard runs while make parses the file, before any target: it fails if go.mod has no
# `toolchain` line or if tools.mod has a different one.
GO_TOOLCHAIN       := $(shell awk '$$1=="toolchain"{print $$2}' go.mod)
TOOLS_GO_TOOLCHAIN := $(shell awk '$$1=="toolchain"{print $$2}' tools.mod)
ifeq ($(GO_TOOLCHAIN),)
$(error go.mod has no `toolchain` line (ADR-026 I-2))
endif
ifneq ($(GO_TOOLCHAIN),$(TOOLS_GO_TOOLCHAIN))
$(error `toolchain` differs: go.mod=$(GO_TOOLCHAIN) tools.mod=$(TOOLS_GO_TOOLCHAIN) (ADR-026 I-2))
endif
export GOTOOLCHAIN := $(GO_TOOLCHAIN)
# Dev tools (sqlc, golangci-lint, govulncheck) are pinned in tools.mod, apart from go.mod so their many
# dependencies never mix with the application's.
TOOL  = $(GO) tool -modfile=tools.mod

.PHONY: generate codex-agents lint test test-int check vuln db-reset dev-certs web-build web-check build check-all

# Regenerate the sqlc code from the migrations and the queries of each module.
generate:
	$(TOOL) sqlc generate

# Regenerate the Codex custom agents (.codex/agents/*.toml) from .claude/agents/*.md (see AGENTS.md).
codex-agents:
	sh .codex/sync-agents.sh

# gofmt, go vet, golangci-lint (with depguard) and generated code (sqlc, Codex agents) up to date.
lint:
	@unformatted="$$(find . -name '*.go' -not -path './.git/*' -not -path '*/node_modules/*' -print0 | xargs -0 gofmt -l)"; \
	if [ -n "$$unformatted" ]; then echo "gofmt needed on:"; echo "$$unformatted"; exit 1; fi
	$(GO) vet -tags=integration ./...
	$(TOOL) golangci-lint run ./...
	$(TOOL) sqlc diff
	sh .codex/sync-agents.sh --check

# Unit tests. No Docker needed.
test:
	$(GO) test -race ./...

# Unit and integration tests against a real PostgreSQL 18 (needs Docker).
test-int:
	$(GO) test -race -tags=integration ./...

check: lint test test-int

# Known vulnerabilities (ADR-026): govulncheck in source mode, symbol level, no tests, against the
# live vuln.go.dev database. Needs network and its result changes with the date, so it is NOT part
# of `check`. Text format on purpose: with json, sarif or openvex govulncheck exits 0 even when it
# finds something. Exit status: 3 = reachable finding, 1 = tool error. VULN_DIR is for the T-B1101
# controls only (another module); `=` and not `?=` so an exported
# VULN_DIR in the environment cannot make it scan another module (it can only be set on the command line).
VULN_DIR = .
vuln:
	$(TOOL) govulncheck -C $(VULN_DIR) -show traces,version ./...

# Development only: recreate the local PostgreSQL (its roles are cluster-wide, so the whole
# volume goes), run the bootstrap through the container's init script, then the migrations.
# Reads DATABASE_MIGRATION_URL and friends from .env (copy .env.example first).
db-reset:
	@test -f .env || { echo "Falta .env: copiá .env.example a .env"; exit 1; }
	docker compose rm -sfv postgres
	docker volume rm -f crm_pgdata
	docker compose up -d --wait postgres
	set -a && . ./.env && set +a && $(GO) run ./cmd/crm migrate up

# Development and E2E only: certificate for localhost and 127.0.0.1 signed by the local mkcert CA.
# Idempotent: it does nothing if the files exist. `make check` does not need it.
dev-certs:
	@command -v mkcert >/dev/null 2>&1 || { \
		echo "No se encontró mkcert. Instalalo: https://github.com/FiloSottile/mkcert#installation"; \
		echo "En Linux instalá también certutil (paquete libnss3-tools o equivalente) y corré 'mkcert -install' una vez."; \
		exit 1; }
	@mkdir -p .certs
	@if [ -f .certs/localhost.pem ] && [ -f .certs/localhost-key.pem ]; then \
		echo ".certs/localhost.pem ya existe; para regenerarlo borrá el directorio .certs"; \
	else \
		mkcert -cert-file .certs/localhost.pem -key-file .certs/localhost-key.pem localhost 127.0.0.1 && \
		echo "Certificado generado en .certs/ (no se versiona)"; \
	fi

# Frontend checks and builds use the committed lockfile and need no certificates.
web-build:
	cd web && $(NPM) ci && $(NPM) run build

web-check:
	cd web && $(NPM) ci && $(NPM) run check

# Build the SPA before Go reads its embedded files.
build: web-build
	mkdir -p bin
	$(GO) build -o bin/crm ./cmd/crm

# Keep Go's embed reads separate from Vite's replacement of dist, also with make -j.
check-all:
	$(MAKE) check
	$(MAKE) web-check
