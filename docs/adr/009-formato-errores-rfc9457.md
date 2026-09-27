# ADR-009: Formato de errores HTTP RFC 9457 con código estable

**Status**: Proposed
**Fecha**: 2026-09-27
**Origen**: spec 001

## Contexto

El frontend necesita distinguir errores que comparten status HTTP (un `409` puede ser
`email_taken`, `last_admin` o `invalid_state`) y mostrar mensajes en español que decide la UI. El
backend no debe filtrar detalles internos (SQL, stack) ni datos de otras empresas en los errores.

## Decisión

- Todas las respuestas de error usan `application/problem+json` (RFC 9457) con:
  `type` (`/problems/{code}`, URI relativa), `title` (genérico en español), `status`, `detail`
  (opcional, en español, sin datos internos), `instance` (id del request) y el miembro de
  extensión **`code`**: un identificador estable en inglés (`email_taken`, `last_admin`,
  `token_invalid`…).
- Los errores de validación agregan `errors: [{field, code}]`.
- La lista cerrada de `code` vive en el contrato (`components/schemas/ErrorCode` del contrato de
  001) y se amplía con cada spec.
- Mapeo único: los servicios devuelven errores de dominio (centinelas con `errors.Is` o tipos con
  `errors.As`); solo la capa HTTP de cada módulo los traduce a `code` + status, con una tabla.
- Un error no mapeado es `500 internal`; una violación de RLS es siempre `500` (es un bug), nunca
  `403`/`404`.
- Se distingue **recuperable** (`429`, `503`: reintentar tiene sentido) de **rechazo definitivo**
  (`4xx` restantes).

## Fundamento

- Estándar reconocido por herramientas y clientes HTTP; evita inventar un envoltorio propio.
- `code` desacopla el texto visible (responsabilidad de la UI, en español) de la semántica del
  error; el frontend no parsea `title` ni `detail`.
- Una sola tabla de mapeo por módulo hace auditable qué error se convierte en qué respuesta.

## Alternativas consideradas

- **Envoltorio propio** (`{"error": {"code": …}}`): mismo resultado sin el estándar.
- **Solo status HTTP**: no distingue errores con el mismo status.
- **Mensajes finales en español generados por el backend**: acopla textos de UI al servidor y
  complica ajustar la redacción.

## Consecuencias

- Ganás: errores predecibles para el frontend, tipados en el contrato y validados por los tests
  de contrato.
- Aceptás: mantener la lista de `code` sincronizada entre contrato, mapeo y UI (está en la matriz
  de mantenimiento de cada plan).
