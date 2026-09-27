# ADR-003: Acceso a datos con sqlc + pgx v5, sin ORM

**Status**: Accepted (decisión del usuario)
**Fecha**: 2026-09-27
**Origen**: spec 001

## Contexto

El sistema maneja dinero y datos de varias empresas en la misma base. Dos propiedades tienen que
poder **auditarse leyendo el código**: que cada query filtre por `tenant_id` (principio III) y
que cada operación financiera sea una transacción (principio IV). Además, el aislamiento usa
`SET LOCAL ROLE` por transacción (ADR-005), lo que exige control total sobre la conexión y la
transacción.

## Decisión

- Queries escritas en SQL en archivos `internal/<módulo>/store/*.sql`, compiladas a Go con
  **sqlc** (`sql_package: pgx/v5`), usando las migraciones de goose como esquema.
- Driver y pool: **`github.com/jackc/pgx/v5`** con `pgxpool`.
- Sin ORM. Las transacciones las abre solo `platform/db.TxRunner` (ADR-005); el código sqlc
  recibe la transacción a través de la interfaz `DBTX`.
- Overrides de tipos: `uuid → github.com/google/uuid.UUID`, `inet → net/netip.Addr`.
- Los structs generados no salen del módulo: el servicio los convierte a tipos de dominio.
- `sqlc diff` en `make lint` asegura que el código generado está al día.

## Fundamento

- SQL explícito: el filtro por `tenant_id` se ve en el archivo `.sql` y un test puede
  verificarlo (INV-04).
- sqlc valida las queries contra el esquema en tiempo de generación: una columna mal escrita no
  llega a runtime.
- pgx es el driver nativo más completo de PostgreSQL en Go: tipos `jsonb`, `inet`, `uuid`,
  errores con SQLSTATE y nombre de constraint (necesario para mapear `23505` a `email_taken`
  y `42501` a violación de RLS).
- Control total de conexión y transacción: indispensable para `SET LOCAL ROLE`.

## Alternativas consideradas

- **GORM / ent / bun**: el SQL real es implícito (difícil auditar el filtro por empresa),
  *hooks* y carga perezosa generan queries ocultas, y el manejo de conexiones del ORM complica
  `SET LOCAL ROLE` por transacción.
- **sqlx**: SQL explícito pero sin verificación en compilación; los errores de columnas
  aparecen en runtime.
- **pgx puro**: control total, pero mapeo manual de filas repetido en cada query.

## Consecuencias

- Ganás: queries auditables, tipos seguros, errores de PostgreSQL precisos.
- Aceptás: las queries con filtros opcionales (búsquedas, reportes en 009) son incómodas en
  sqlc; se resuelven con `sqlc.narg` y `COALESCE`, o como excepción justificada con SQL armado
  en `platform/db` (nunca con concatenación de valores). Hay que correr `make generate` al
  cambiar una query.
- sqlc parsea todo el esquema, incluidas políticas y `GRANT`; que lo haga sin errores se valida
  en la Fase 0 (spike T-B008).
