# ADR-008: Identificadores UUIDv7

**Status**: Accepted (aprobado por el usuario con el plan de 001, 2026-09-29)
**Fecha**: 2026-09-27
**Origen**: spec 001

## Contexto

Los identificadores aparecen en URLs (`/users/{userId}`) y, en specs posteriores, en enlaces
compartidos (presupuestos). No deben permitir enumerar recursos ni revelar el volumen de una
empresa, y deben indexarse bien en PostgreSQL.

## Decisión

- Todas las claves primarias son `uuid` **versión 7** (RFC 9562).
- Por defecto las genera la base con `DEFAULT uuidv7()` (función nativa de PostgreSQL 18).
- Se generan en Go con `github.com/google/uuid` (`uuid.NewV7()`) solo cuando el id hace falta
  antes del `INSERT`: en 001, `tenants.id`, porque el nombre del rol de la empresa se deriva de él
  (ADR-005).
- En Go, el tipo es `uuid.UUID` (override de sqlc). En el contrato, `type: string, format: uuid`.

## Fundamento

- No secuenciales hacia afuera: un id no permite adivinar otros.
- El prefijo temporal hace que las inserciones caigan al final del índice B-tree (índices más
  compactos y con menos páginas modificadas que con UUIDv4).
- Tipo nativo `uuid` de 16 bytes en PostgreSQL y generación nativa en la versión 18.

## Alternativas consideradas

- **UUIDv4**: totalmente aleatorio; inserciones dispersas en el índice.
- **`bigserial`**: compacto, pero enumerable en URLs y revela cuántos registros hay.
- **ULID**: ordenado, pero no es el tipo `uuid` nativo; exige conversiones.
- **Generar siempre en Go**: posible, pero agrega una dependencia en cada `INSERT`; se reserva
  para los casos en que el id hace falta antes.

## Consecuencias

- Ganás: ids no enumerables con buen rendimiento de índice.
- Aceptás: 16 bytes por id y que el id revele el instante aproximado de creación (irrelevante para
  este dominio). Requiere PostgreSQL ≥ 18 para `uuidv7()`.
