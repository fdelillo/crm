# ADR-009: Formato de errores HTTP RFC 9457 con código estable

**Status**: Proposed
**Fecha**: 2026-09-27
**Origen**: spec 001
**Revisión 2026-09-27**: se agrega el miembro de extensión `suggested_action` (respuesta P-4 de 001).

## Contexto

El frontend necesita distinguir errores que comparten status HTTP (un `409` puede ser
`email_already_registered`, `email_taken`, `last_admin` o `invalid_state`) y mostrar mensajes en
español que decide la UI. En algunos errores la UI además debe ofrecer una salida (p. ej. "¿Querés
recuperar la contraseña?" cuando alguien se registra con un email que ya existe). El backend no
debe filtrar detalles internos (SQL, stack) ni datos de otras empresas en los errores.

## Decisión

- Todas las respuestas de error usan `application/problem+json` (RFC 9457) con:
  `type` (`/problems/{code}`, URI relativa), `title` (genérico en español), `status`, `detail`
  (opcional, en español, sin datos internos), `instance` (id del request) y el miembro de
  extensión **`code`**: un identificador estable en inglés (`email_already_registered`,
  `last_admin`, `token_invalid`…).
- Segundo miembro de extensión, opcional: **`suggested_action`**, un enum estable que indica qué
  salida puede ofrecer la UI (en 001, solo `password_reset`). Es un nombre de acción, no una URL:
  la UI decide la ruta y el texto.
- Los errores de validación agregan `errors: [{field, code}]`.
- Las listas cerradas de `code` y `suggested_action` viven en el contrato
  (`components/schemas/ErrorCode` y `SuggestedAction` del contrato de 001) y se amplían con cada
  spec.
- Mapeo único: los servicios devuelven errores de dominio (centinelas con `errors.Is` o tipos con
  `errors.As`); solo la capa HTTP de cada módulo los traduce a `code` + status (+
  `suggested_action`), con una tabla. Un mismo error de dominio puede mapearse a códigos distintos
  según el endpoint cuando la UI necesita tratarlos distinto (p. ej. `ErrEmailTaken` →
  `email_already_registered` en el registro, `email_taken` en la invitación).
- Un error no mapeado es `500 internal`; una violación de RLS es siempre `500` (es un bug), nunca
  `403`/`404`.
- Se distingue **recuperable** (`429`, `503`: reintentar tiene sentido) de **rechazo definitivo**
  (`4xx` restantes).

## Fundamento

- Estándar reconocido por herramientas y clientes HTTP; evita inventar un envoltorio propio. RFC
  9457 admite miembros de extensión, que es lo que son `code` y `suggested_action`.
- `code` desacopla el texto visible (responsabilidad de la UI, en español) de la semántica del
  error; el frontend no parsea `title` ni `detail`.
- `suggested_action` como enum evita que el backend conozca rutas de la SPA y que la UI tenga que
  deducir la salida a partir del `code`.
- Una sola tabla de mapeo por módulo hace auditable qué error se convierte en qué respuesta.

## Alternativas consideradas

- **Envoltorio propio** (`{"error": {"code": …}}`): mismo resultado sin el estándar.
- **Solo status HTTP**: no distingue errores con el mismo status.
- **Mensajes finales en español generados por el backend como única fuente**: acopla textos de UI
  al servidor y complica ajustar la redacción (el backend igual manda `title`/`detail` en español
  como respaldo).
- **Link absoluto a la pantalla de recuperación dentro del error**: acopla el backend a las rutas
  de la SPA.

## Consecuencias

- Ganás: errores predecibles para el frontend, tipados en el contrato y validados por los tests
  de contrato; salidas sugeridas sin acoplar rutas.
- Aceptás: mantener las listas de `code` y `suggested_action` sincronizadas entre contrato, mapeo
  y UI (está en la matriz de mantenimiento de cada plan).
