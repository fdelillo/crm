# Development and CI commands (specs/001-empresas-usuarios/tasks.md, "Comandos de validación").
# `make check` is the checkpoint of every phase. Frontend targets are added by T-F009.

GO   ?= go
# Dev tools (sqlc, golangci-lint) are pinned in tools.mod, apart from go.mod so their many
# dependencies never mix with the application's.
TOOL  = $(GO) tool -modfile=tools.mod

.PHONY: generate lint test test-int check db-reset dev-certs

# Regenerate the sqlc code from the migrations and the queries of each module.
generate:
	$(TOOL) sqlc generate

# gofmt, go vet, golangci-lint (with depguard) and generated code up to date.
lint:
	@unformatted="$$(find . -name '*.go' -not -path './.git/*' -not -path '*/node_modules/*' -print0 | xargs -0 gofmt -l)"; \
	if [ -n "$$unformatted" ]; then echo "gofmt needed on:"; echo "$$unformatted"; exit 1; fi
	$(GO) vet -tags=integration ./...
	$(TOOL) golangci-lint run ./...
	$(TOOL) sqlc diff

# Unit tests. No Docker needed.
test:
	$(GO) test -race ./...

# Unit and integration tests against a real PostgreSQL 18 (needs Docker).
test-int:
	$(GO) test -race -tags=integration ./...

check: lint test test-int

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
