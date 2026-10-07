# CRM para pymes que trabajan por proyecto

Sistema web **sencillo** para gestionar clientes, proyectos, presupuestos y flujo de
caja (clientes, proveedores, empleados y socios). El primer usuario es una carpintería
de aluminio; el diseño es genérico y se adapta a otros rubros mediante configuración.

> Estado: **especificación** (Spec Driven Development) con el esqueleto del backend (spec 001, Fase 0).
> Todavía no hay funcionalidades: ver [`specs/001-empresas-usuarios/tasks.md`](specs/001-empresas-usuarios/tasks.md).

## Documentación

| Documento | Contenido |
|-----------|-----------|
| [Constitución](.specify/memory/constitution.md) | Principios no negociables, stack y convenciones |
| [Visión](docs/vision.md) | Problema, usuarios, alcance del MVP y orden de construcción |
| [Modelo de dominio](docs/modelo-dominio.md) | Entidades, efectos de cada operación, multimoneda |
| [Glosario](docs/glosario.md) | Términos del negocio y su nombre en el código |

## Specs del MVP

| # | Módulo | Spec |
|---|--------|------|
| 001 | Empresas, usuarios y roles | [spec](specs/001-empresas-usuarios/spec.md) |
| 002 | Configuración por empresa | [spec](specs/002-configuracion/spec.md) |
| 003 | Terceros | [spec](specs/003-terceros/spec.md) |
| 004 | Proyectos | [spec](specs/004-proyectos/spec.md) |
| 005 | Presupuestos | [spec](specs/005-presupuestos/spec.md) |
| 006 | Cuentas de dinero | [spec](specs/006-cuentas-dinero/spec.md) |
| 007 | Cuentas corrientes | [spec](specs/007-cuentas-corrientes/spec.md) |
| 008 | Importación de extractos | [spec](specs/008-importacion-extractos/spec.md) |
| 009 | Reportes | [spec](specs/009-reportes/spec.md) |
| 010 | Cheques | [spec](specs/010-cheques/spec.md) |

## Stack

Go (API REST) · PostgreSQL · React + TypeScript (PWA) · contrato OpenAPI · monolito modular.
Detalle y motivos en la [constitución](.specify/memory/constitution.md#stack-tecnológico).

## Flujo de trabajo (GitHub Spec Kit)

Cada funcionalidad sigue este ciclo en su rama `NNN-nombre`:

1. `/speckit.specify`: `spec.md` (qué y por qué). **Hecho para 001–010.**
2. `/speckit.clarify`: resolver ambigüedades de la spec. **Primera ronda hecha**
   (ver sección "Clarificaciones" de cada spec).
3. `/speckit.plan`: `plan.md`, `data-model.md` y `contracts/` (cómo), verificando la constitución.
4. `/speckit.tasks`: `tasks.md`.
5. `/speckit.implement`: implementación con tests.

Los agentes de `.claude/agents/` cubren el plan, la implementación y la revisión de cada
spec; ver [`CLAUDE.md`](CLAUDE.md#agentes-y-skills-del-proyecto-claude).

Para instalar los comandos de Spec Kit en este repo:
`uvx --from git+https://github.com/github/spec-kit.git specify init --here`
(si pregunta, **conservar** la constitución existente).

## Desarrollo

Requisitos: Go 1.27 (con `GOTOOLCHAIN=auto` el comando `go` descarga la versión fijada en `go.mod`),
Docker, `make`, y [`mkcert`](https://github.com/FiloSottile/mkcert#installation) para HTTPS local.
`sqlc` y `golangci-lint` no se instalan: `make` los corre desde `tools.mod` (`go tool -modfile=tools.mod`).

### Comandos

| Comando | Qué hace |
|---------|----------|
| `make check` | `lint` + `test` + `test-int`. Es el checkpoint de cada fase; no necesita mkcert |
| `make lint` | `gofmt`, `go vet`, `golangci-lint` (con `depguard`) y `sqlc diff` |
| `make test` | Tests unitarios (`go test -race ./...`); no necesitan Docker |
| `make test-int` | Además, tests de integración con PostgreSQL 18 real (necesita Docker) |
| `make generate` | Regenera el código de `sqlc` |
| `make db-reset` | Solo desarrollo: recrea el PostgreSQL local, corre el bootstrap y las migraciones |
| `make dev-certs` | Solo desarrollo y E2E: genera `.certs/localhost.pem` y `.certs/localhost-key.pem` con mkcert |

### Primera vez

1. Instalar `mkcert`. En Linux instalar también `certutil` (paquete `libnss3-tools` o equivalente).
2. `mkcert -install` (una vez por equipo: crea una CA local y la agrega a los navegadores).
3. `make dev-certs` (genera el certificado de `localhost` y `127.0.0.1`; `.certs/` no se versiona).
4. `cp .env.example .env` y reemplazar `AUTH_HMAC_KEY` (`openssl rand -base64 32`).

> **No compartas ni copies al repositorio `rootCA-key.pem`** (está en `mkcert -CAROOT`): con esa
> clave se puede interceptar el HTTPS de tu equipo. La CA es de cada persona.

### Levantar todo

```sh
docker compose up -d --wait        # PostgreSQL 18 (con los roles y la base), Mailpit y MinIO
set -a && . ./.env && set +a       # carga la configuración en el shell
go run ./cmd/crm migrate up        # migraciones, con el rol crm_owner
go run ./cmd/crm serve             # loguea listen=https_local
curl https://localhost:8443/healthz   # sin -k: {"status":"ok"}
```

`--wait` deja también creado el bucket de desarrollo (`S3_BUCKET`, por defecto `crm-dev`): el
healthcheck de MinIO lo crea de forma idempotente, incluso después de `docker compose down -v`.

Hasta que exista el frontend embebido, `https://localhost:8443/` responde `503` ("La interfaz no
está compilada"): es esperado.

### Probar el registro (Fase 3)

Con los servicios y el backend levantados como arriba:

```sh
curl -c /tmp/crm-cookies.txt -H 'Content-Type: application/json' \
  --data '{"name":"Ana","email":"ana@example.com","password":"una-clave-segura-123","company_name":"Mi Empresa","base_currency":"ARS","industry_template_code":"generic"}' \
  https://localhost:8443/api/v1/auth/signup
curl -b /tmp/crm-cookies.txt https://localhost:8443/api/v1/me
```

El registro responde `201`, `/me` devuelve el usuario `admin` con sus 15 permisos y el correo de
verificación aparece en [Mailpit](http://localhost:8025). El adaptador SMTP usa por defecto
`localhost:1025` y `no-reply@crm.local` en modo local; `SMTP_HOST`, `SMTP_PORT` y `SMTP_FROM`
permiten cambiarlos.

### Modos de desarrollo (plan 001, §10.5.1)

La sesión es una cookie `__Host-` `Secure`, así que el navegador solo la guarda por HTTPS. En
desarrollo `APP_BASE_URL` es siempre `https://`; `TLS_CERT_FILE` y `TLS_KEY_FILE` solo se aceptan
cuando el host es `localhost` o `127.0.0.1`.

| Modo | `crm serve` | El navegador abre | `APP_BASE_URL` |
|------|-------------|-------------------|----------------|
| Binario completo (y E2E) | `HTTP_ADDR=:8443` con `TLS_CERT_FILE` y `TLS_KEY_FILE` | `https://localhost:8443` | `https://localhost:8443` |
| Vite + API | igual que arriba | `https://localhost:5173` (Vite con el mismo certificado; *proxy* de `/api` a `https://localhost:8443`) | `https://localhost:5173` |
| Solo backend (`curl`, sin navegador) | sin `TLS_*` (HTTP plano en `:8080`) | — | `https://localhost:8080` |

Node no usa el almacén de certificados del sistema: para que el *proxy* de Vite y los scripts de
Node confíen en el certificado, exportar
`NODE_EXTRA_CA_CERTS="$(mkcert -CAROOT)/rootCA.pem"`.

### Base de datos: bootstrap y migraciones (ADR-004)

Son dos mecanismos:

- **Bootstrap** (`db/bootstrap/`): crea los roles del clúster y la base `crm`. Lo corre un DBA una
  vez por clúster; en desarrollo, lo ejecuta el contenedor de PostgreSQL al inicializarse. Es
  idempotente y necesita `psql`. Las contraseñas de `crm_owner` y `crm_app` **no** están en el
  script: se toman de `CRM_OWNER_PASSWORD` y `CRM_APP_PASSWORD` si están definidas.

  ```sh
  psql -v ON_ERROR_STOP=1 -U postgres -d postgres -f db/bootstrap/001_roles_and_database.sql
  ```

- **Migraciones** (`db/migrations/`, goose embebido en el binario): `crm migrate up|down|status`,
  siempre con `DATABASE_MIGRATION_URL` (rol `crm_owner`), como paso de despliegue.

Los roles son globales al clúster: **un clúster de PostgreSQL por entorno**, y los respaldos
necesitan también los roles (`pg_dumpall --roles-only`) o un backup físico del clúster.
