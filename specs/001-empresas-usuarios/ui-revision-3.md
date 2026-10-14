# Revisión 3 de `ui.md`: realineación con el backend implementado

**Spec**: [`spec.md`](spec.md) · **Diseño afectado**: [`ui.md`](ui.md) y la sección Frontend de
[`tasks.md`](tasks.md) · **Fecha**: 2026-10-12 · **Autor**: `frontend-architect`
**Estado**: Aprobado por el usuario el 2026-10-12 (P-F7: sí; P-F8 y P-F9: cerradas con la revisión 16 del backend, ver `ui.md` §26)
**Contrato**: diseño original contra `contracts/openapi.yaml` **v0.3.1** (2026-09-29); hoy **v0.4.3**
**Backend**: Fases 0 a 9 en `main` (`b945373`); plan §18, tandas 3 a 15; DD-31 a DD-43

Este documento tiene el análisis completo. Los cambios de texto en `ui.md` y en `tasks.md` van
aparte, como pares "texto viejo → texto nuevo", para que los aplique quien tenga permiso de
escritura sobre esos archivos.

---

## 0. Cómo se hizo y límites

- **Fuentes leídas**: `contracts/openapi.yaml` v0.4.3 completo (incluido su "Historial", que
  documenta cada versión desde 0.2.0); `plan.md` §6 (DD-9, DD-19 a DD-43), §9.1, §10.7 y §18
  (tandas 1 a 15); la sección Frontend y la tabla "Coordinación con la sección Frontend" de
  `tasks.md`; y el código: `internal/app/{api,root,ops,phase3..7}.go`, `internal/tenant/{http,
  service,update,validation}.go`, `internal/identity/{middleware,users}.go`,
  `internal/platform/httpx/{problem,json}.go`, `internal/platform/password/password.go`,
  `internal/platform/ratelimit` y `cmd/crm/{serve,main_test}.go`.
- **Límite**: este agente no tiene shell, así que **no pude correr**
  `git log -p -- specs/001-empresas-usuarios/contracts/openapi.yaml`. La lista de diferencias de
  §2 se reconstruyó con el "Historial" del propio contrato (una entrada por versión), las tandas
  del plan que lo cambiaron (3, 7, corrección de la 8 y 10) y la lectura del YAML actual contra lo
  que `ui.md` asume. Para confirmar que no hay un cambio sin documentar, alcanza con correr
  `git log -p --follow -- specs/001-empresas-usuarios/contracts/openapi.yaml` y verificar que los
  *diffs* posteriores al 2026-09-29 tocan solo lo de §2. Si aparece algo más, esta revisión se
  completa con eso.
- **Sistema visual**: no se tocó (§19 de `ui.md` sin cambios). Ninguna diferencia obliga a una
  decisión visual.

---

## 1. Resumen

**Cuánto cambió**: poco y en los bordes. Entre v0.3.1 y v0.4.3 **ninguna operación, ningún esquema
de request ni de response cambió de forma**: hay un valor nuevo en `ErrorCode`
(`method_not_allowed`), una cabecera opcional nueva (`Retry-After` en los `5XX`), un `413`
declarado en todo body JSON y dos cambios de texto con semántica (cupo del registro y `PATCH
/tenant`). Lo que más mueve a la UI no está en el contrato sino en el **código**: el servidor usa
`invalid_value` donde `ui.md` esperaba `required` o `too_long` para los campos de texto, y los cupos
de tasa compartidos entre pantallas (DD-9) cambian lo que conviene decirle al usuario cuando espera.

| Tipo de cambio | Cantidad | Dónde pega en la UI |
|---|---|---|
| Contrato que obliga a cambiar código del cliente | 1 (`method_not_allowed`) | Mapa de mensajes (no compila sin él, INV-F05) |
| Contrato que mejora el comportamiento | 3 (`Retry-After` en `503`, semántica de `PATCH`, `413` JSON) | §11, §12, §13.1, §13.11 |
| Decisiones del backend sin cambio de contrato | 3 con impacto (DD-9 cupos, DD-42, DD-43); 5 sin impacto (DD-32 a DD-35, DD-39, DD-40, DD-41) | §11.1, §13.3, §13.11, §14.4 |
| Comportamiento verificado en el código que el contrato no dice | 5 (códigos de campo, formato del CUIT, campos desconocidos, *stub* de la SPA, cupos reales) | §12.3, §13.11, §16, §21, T-F007/T-F008 |
| Decisiones nuevas del frontend | DD-F24 a DD-F27, INV-F15 | §24, §9.2 |
| Hallazgos | H-12 sigue abierto; H-13 a H-15 nuevos; los dos puntos menores de §27.2, cerrados | §27 |
| Preguntas para el usuario | P-F7 (con default); P-F8 y P-F9 condicionales a la revisión 16 | §26 |
| Fases F afectadas | Todas, con cambios chicos de casos de test; **ninguna tarea nueva ni reordenada** | `tasks.md` §Frontend |

---

## 2. Diferencias del contrato, v0.3.1 → v0.4.3

| Versión | Cambio | Origen | Lo que `ui.md` asumía | Impacto en la UI |
|---|---|---|---|---|
| 0.4.0 | `method_not_allowed` agregado a `ErrorCode` (`405` con cabecera `Allow`; regla global para toda ruta de `/api/v1`) | Plan §18, tercera tanda | `ErrorCode` sin ese valor | **Obligatorio**: el mapa `Record<ErrorCode, …>` de `api/errorMessages.ts` no compila hasta tener su texto (a propósito, INV-F05). Es un bug del cliente (método equivocado): mismo tratamiento que `malformed_request`, con `requestId`. §12.1, §12.2, T-F004, T-F005, T-F101 |
| 0.4.0 | `Retry-After` opcional en `ServerError` (`5XX`); el registro lo manda con `2` cuando la espera por el lock de `GRANT crm_tenant` vence (DD-33 R-a) | Plan §18, tercera tanda | `Retry-After` solo en `429` | `toApiError` ya lee `Retry-After` de cualquier respuesta (§11.3): el campo ahora también aparece en `503`. El `503` del registro dice "Probá de nuevo en unos segundos" y deja claro que no se creó nada; **sin reintento automático** (DD-F25). §12.2, §13.1, T-F005, T-F101, T-F202 |
| 0.4.1 | `413 payload_too_large` declarado en toda operación con body JSON (límite de 65 536 bytes que el servidor ya aplicaba) | Plan §18, séptima tanda | `413` solo en el logo | Ninguno en comportamiento: §12.2 ya trataba `payload_too_large` fuera del logo "como `malformed_request`", y ningún formulario de 001 se acerca a 64 KiB. Los tipos generados suman la respuesta `413` (T-F004). §12.2 lo aclara |
| 0.4.2 | Texto del cupo de `POST /auth/signup`: ráfaga de 5 y una reposición cada 12 min (no "5 por hora") | Corrección de la octava tanda | "5 intentos por hora" no se usaba en la UI | Ninguno en código. §11.1 suma una tabla de cupos reales (incluye los compartidos de DD-9) |
| 0.4.3 | Semántica de `PATCH /tenant` (DD-43): texto vacío en un opcional = `null`; `null` en `name` o `timezone` → `400`; `Local` no es una zona válida (`422 invalid_timezone`); un `PATCH` sin cambios responde `200` sin modificar `updated_at` | Plan §18, décima tanda | `PATCH` con `dirtyFields`, `null` para vaciar; nada sobre `null` en `name`/`timezone` ni sobre el no-op | La UI ya mandaba lo correcto (el plan lo dice en DD-43). Se vuelve explícito como **INV-F15** (nunca `null` en `name`/`timezone`, nunca claves fuera de `TenantUpdate`) y se aprovecha el no-op: **reintentar un `PATCH` es seguro** y no cambia la URL del logo. §11.1, §13.11, §16, T-F004, T-F602 |
| — | Sin cambios | — | — | Operaciones, rutas, `SessionInfo`, `Tenant`, `User`, `InvitationPreview`, `FieldError.code`, `SuggestedAction`, cabeceras del logo (`ETag`, `If-None-Match`, `304`, `private, no-cache`, `v`), cookie `__Host-crm_session` y CSRF (`403 forbidden`, DD-30): iguales a v0.3.1 |

---

## 3. Decisiones del backend posteriores al 2026-09-29 que tocan (o no) la UI

| Decisión | Qué dice | Impacto en la UI |
|---|---|---|
| **DD-9** (texto corregido el 2026-10-05 y 2026-10-06) | Cupos por IP (/64 en IPv6) con ráfaga + reposición. **Compartidos**: pedido y confirmación de reset; `confirm` y `resend` de verificación; `preview` y `accept` de invitación. Por email (reset): 3 y 1 cada 20 min, **sin `429`** (responde `202` igual y no envía) | **Sí**. (1) Tabla de cupos en §11.1. (2) Las pantallas que comparten cupo lo dicen en su matriz (S-03/S-04, S-05/C-01, S-06). (3) El `429` del login por IP tiene `Retry-After` de segundos: "Esperá 1 minutos" era incorrecto; ahora "unos segundos" (DD-F25). (4) "¿No llegó? Pedilo de nuevo" puede no enviar nada después de 3 pedidos del mismo email: P-F7 propone una línea de texto. No hay `429` en `POST /users/invitations` (puede cambiar en la revisión 16) |
| **DD-32** | IP del cliente detrás de proxies; clientes detrás del mismo NAT comparten cupo | Indirecto: refuerza que el `429` en S-06 puede aparecer en uso legítimo de una oficina (DD-9 lo acepta). Ningún cambio de UI |
| **DD-33** (R-a a R-f) | `503` + `Retry-After: 2` si el registro espera más de 2 s por el lock; no crea nada | **Sí**: §13.1, DD-F25 (texto en segundos, sin reintento automático, lo cargado se conserva) |
| **DD-34** | Primer `SET ROLE` a una empresa recién creada: lectura de catálogo y un reintento en `InTenantTx`; el camino de la sesión no se reintenta | No. Si alguna vez falla, el primer `GET /tenant` del panel daría `500`: la matriz de S-07 ya muestra "No pudimos cargar los datos de tu empresa." + "Reintentar". T-B910 midió `set_role_retry_total` = 0 |
| **DD-35 a DD-38** | Outbox, SMTP, auditoría, `user_agent` | No (no son observables por HTTP) |
| **DD-39** | `POST /auth/password-reset` es uniforme en status, cabeceras y cuerpo (`202` vacío), **no** en tiempo | No cambia nada que la UI muestre: el texto de S-03 ya es neutro y no depende del tiempo. Regla explícita: la UI no mide ni muestra el tiempo de esa respuesta (nada en el cliente debe agregar una señal). §13.3 lo dice |
| **DD-40** | Locks `FOR NO KEY UPDATE`; administración de usuarios serializada por empresa | No. Como mucho, dos administradores que actúan a la vez esperan milisegundos |
| **DD-41** | La sesión de un login concurrente con la desactivación puede quedar sin revocar, pero no autentica | No: el usuario desactivado recibe `401` en el request siguiente, que es lo que la UI ya maneja (§12.4) |
| **DD-42** | Logo: `COMMIT` de resultado incierto → relectura con lock; `200` si quedó guardado; si la relectura falla, el error original aunque el logo pueda haber quedado. `GET /tenant/logo`: `404` con el objeto ausente, `503` con el almacenamiento caído. `DELETE` sin logo: `204` sin escribir | **Sí**: ante `5xx` o error de red en `PUT`/`DELETE /tenant/logo`, la UI vuelve a pedir `GET /tenant` y decide por lo que lee (DD-F26). El `404`/`503` del `<img>` ya caía a las iniciales; se documenta. §10.3, §11.1, §13.11, §14.4, T-F603 |
| **DD-43** | Semántica de `PATCH /tenant` (ver §2, 0.4.3) | **Sí**: INV-F15; reintento seguro; §13.11 y T-F602 |

---

## 4. Lo que el contrato no dice y el código sí (verificado el 2026-10-12)

### 4.1 Códigos de error de campo que el servidor emite de verdad

`ui.md` §12.3 asumía `required` para un campo vacío y `too_long` para uno largo. El servidor usa
otros códigos según el endpoint:

| Endpoint | Campo | Condición | `FieldError.code` real | Fuente |
|---|---|---|---|---|
| `POST /auth/signup` | `name`, `company_name` | vacío, más de 120, o con `NUL` | `invalid_value` | `tenant/service.go` `Register` |
| | `email` | inválido o más de 254 | `invalid_format` | idem |
| | `password` | < 10 / > 128 / igual al email | `too_short` / `too_long` / `same_as_email` | `password.Validate` |
| | `base_currency` | falta o no es `ARS`/`USD` | `invalid_value` | idem |
| | `industry_template_code` | falta o desconocido | `unknown_template` | idem |
| `POST /auth/login` | `email` | inválido | `invalid_format` | `app/phase4.go` |
| | `password` | vacía o más de 128 | `invalid_value` | idem |
| `POST /auth/password-reset` | `email` | inválido | `invalid_format` | `app/phase5.go` |
| `POST /auth/password-reset/confirm` | `password` | como en el registro | `too_short` / `too_long` / `same_as_email` | idem |
| `POST /auth/invitations/accept` | `name` | vacío, más de 120 o con `NUL` | `invalid_value` | `identity/users.go` `AcceptInvitation` |
| | `password` | como en el registro | `too_short` / `too_long` / `same_as_email` | idem |
| `POST /users/invitations` y `PUT …/role` | `email` / `role` | inválido / desconocido | `invalid_format` / `invalid_value` | `identity/users.go` `Invite` |
| `PATCH /tenant` | `name` | vacío tras el recorte / más de 120 o con `NUL` | `required` / `invalid_value` | `tenant/update.go`, `validation.go` |
| | `legal_name`, `address`, `phone` | más de 200 / 300 / 50 o con `NUL` | `invalid_value` | idem |
| | `tax_id` | forma distinta de 11 dígitos o `NN-NNNNNNNN-N`, o más de 13 | `invalid_format`; dígito verificador: `invalid_tax_id` | idem |
| | `email` | inválido o más de 254 | `invalid_format` | idem |
| | `timezone` | desconocida, vacía, `Local` o más de 64 | `invalid_timezone` | idem |
| `PUT /tenant/logo` | `file` | falta o vacío / ilegible o > 2000 px | `required` / `invalid_value` | `tenant/http.go` |

Consecuencias:

- El cliente valida antes de enviar (longitudes, requeridos), así que estos `422` solo aparecen si
  el cliente y el servidor se desalinean. Pero cuando aparecen, el texto por defecto de
  `invalid_value` ("Elegí una opción de la lista.") es **incorrecto** para un campo de texto.
  **DD-F24**: variantes por campo para `invalid_value` y `too_long`, y un texto por defecto neutro.
- `required` y `too_long` siguen en uso: los emite el **cliente** (Zod usa los mismos códigos,
  T-F201) y el servidor en `PATCH name` y en `password`.
- El contrato no documenta qué código usa cada campo: **H-13** (no bloquea).

### 4.2 Otros comportamientos

| Tema | Lo que hace el código | Impacto |
|---|---|---|
| CUIT | El servidor acepta **exactamente** `^[0-9]{11}$` o `^[0-9]{2}-[0-9]{8}-[0-9]$` (largo medido antes del recorte: máx. 13); otra posición de guiones → `invalid_format` | `isValidCuit` tiene que aceptar las mismas dos formas y rechazar el resto. La ayuda "11 números, con o sin guiones" sigue siendo correcta. T-F601 suma el caso |
| Campos desconocidos | `DecodeJSON` usa `DisallowUnknownFields`: cualquier clave de más → `400 malformed_request` (también `base_currency` en `PATCH /tenant`) | INV-F15. Los tipos generados lo previenen en compilación casi siempre; el test de T-F602 lo cierra |
| Cupos reales | `internal/app/api.go`: registro 5 / 12 min; login 20 / 3 s; reset (pedido y confirmación) 5 / 12 min; verificación (`confirm` y `resend`) 20 / 3 min; invitación (`preview` y `accept`) 20 / 3 min. Sin cupo: `GET /industry-templates`, `/me`, `/tenant*`, `/users*`, `POST /users/invitations`, `logout` | §11.1 (tabla). El contrato declara `429` en `GET /industry-templates` y el código no limita: **H-14** (documental, no bloquea; la UI maneja el `429` igual) |
| `Retry-After` | El *middleware* de rate limit siempre lo manda en el `429` (redondeado hacia arriba a segundos); `login_locked` con lo que falta de los 15 min; `503` del registro con `2` | DD-F25 (textos por rango de segundos) |
| *Stub* de la SPA | `cmd/crm/serve.go` usa `app.SPAUnavailableHandler()` (`503` `text/plain`, mismo texto que el `web` sin `index.html`). `cmd/crm/main_test.go` exige `503` en `GET /login` | T-F008 reemplaza el *stub*. Con la SPA compilada y embebida, ese test pasaría a recibir `200` y fallaría según el orden en que se corran `make web-build` y `go test`: **H-15**. Además, T-F008 tiene que mantener en verde el guard de `crm serve` (T-B801, reglas (1) a (6)): el literal `RootDeps` sigue siendo argumento directo de `NewRootHandler`, y `cmd/crm` no importa `net/http` (importar `web` está permitido) |
| `405` del handler de la SPA | `getOnly` del mux raíz responde `405` con `Allow: GET, HEAD` en `/healthz` y `/readyz`; T-F007 pedía `405` para `POST /login` sin cabecera | T-F007 pide también `Allow: GET, HEAD` (RFC 9110; misma regla que el resto del binario) |
| `GET /tenant/logo` en `304` | Repite `Cache-Control`, `ETag` y `Content-Type` | Ninguno: el navegador lo maneja |

---

## 5. Impacto en `ui.md`, sección por sección

| Sección de `ui.md` | Cambio | Motivo |
|---|---|---|
| Cabecera | "Revisión 3" y contrato consumido **v0.4.3** | — |
| §0 | El backend está implementado (Fases 0 a 9) | Estado real |
| §7 BR-F05 | Marca "dependiente de la revisión 16" | Verificación de email (P-F8) |
| §9.2 | **INV-F15** nueva (`PATCH /tenant` sin `null` en `name`/`timezone` ni claves fuera de `TenantUpdate`) | DD-43, `DisallowUnknownFields` |
| §10.3 | `PATCH` sin cambios no mueve `updated_at`; fila nueva: `5xx`/red en el logo → releer `['tenant']` | DD-43 (4), DD-42, DD-F26 |
| §11 | Canónico v0.4.3; tabla de cupos; resumen de cambios; `PATCH /tenant` y `COMMIT` incierto del logo; `404`/`503` del `<img>` | §2, §3 |
| §11.2 | Alias `TokenRequest` y `RoleChangeRequest` (estaban escritos a mano en firmas: INV-F05) | Corrección propia |
| §11.3 | `retryAfterSeconds` también en `503` | v0.4.0 |
| §12.1 | Fila `405`; `503` con `Retry-After` | v0.4.0 |
| §12.2 | Fila `method_not_allowed`; `malformed_request` con los casos de DD-43; `payload_too_large` JSON; `rate_limited` y `service_unavailable` con textos por rango de segundos | v0.4.0, v0.4.1, DD-F25 |
| §12.3 | Variantes de `invalid_value` y `too_long` por campo; texto por defecto neutro; tabla de §4.1 resumida | DD-F24, H-13 |
| §13.1 | `503` con `Retry-After`; cupo; dependiente de la revisión 16 | DD-33, DD-9 |
| §13.3, §13.4 | Cupo compartido; DD-39; texto de P-F7 (pendiente) | DD-9, DD-39 |
| §13.6 | Códigos reales del `422`; cupo compartido de preview/accept | §4.1, DD-9 |
| §13.10 | `422 role`; `429` posible (revisión 16) | §4.1 |
| §13.11 | `PATCH` (INV-F15, reintento seguro, CUIT en dos formas), zona guardada en el selector, `5xx` del logo con relectura | DD-43, DD-42, DD-F26, DD-F27 |
| §14.4 | Rama `5xx` o sin red en el diagrama de la subida | DD-42 |
| §15.4 | `useConfirmEmailVerification` con `TokenRequest` | INV-F05 |
| §16 | Fila "Datos de la empresa" con CUIT en dos formas e INV-F15 | §4.2 |
| §21, §21.2 | Nombre del *stub*, guard de T-B801, H-15; §10.7 verificada de nuevo | §4.2 |
| §24 | DD-F24 a DD-F27 | §6 |
| §25 | S-F11 (alias de zonas horarias en `Intl`) | §6 |
| §26 | P-F7 a P-F9 | §8 |
| §27 | H-12 actualizado; H-13 a H-15; puntos menores cerrados; verificación de la revisión 3 | §7 |
| §28 | RF-14 (revisión 16), RF-15 (zona horaria) | §9 |
| §29 | Fila para cupos de DD-9 y códigos de campo por endpoint | Documentación viva |
| §19 (sistema visual), §5, §6, §8, §17, §18, §20, §22, §23 | Sin cambios | — |

---

## 6. Decisiones nuevas del frontend

| ID | Decisión | Por qué | Alternativas descartadas | Trade-off |
|---|---|---|---|---|
| **DD-F24** | `fieldErrorMessage(field, code)` tiene variantes por campo para `invalid_value` (textos: `name`/`company_name` → "Tiene que tener entre 1 y {n} caracteres."; `legal_name`/`address`/`phone` → "Es demasiado largo (máximo {n} caracteres)."; `base_currency` → "Elegí la moneda base."; `role` → "Elegí un rol."; `password` → "Ingresá tu contraseña (hasta 128 caracteres)."; `file` → el de §13.11.1) y para `too_long` en `password`; el texto por defecto de `invalid_value` pasa a ser "Revisá este dato." | El servidor usa `invalid_value` para vacíos y largos en campos de texto (§4.1); el texto anterior hablaba de "opciones de una lista" | (a) Pedir al backend que use `required`/`too_long`: cambia comportamiento ya probado de tres fases para un caso que el cliente evita; se deja como alternativa en H-13. (b) Ignorar el código del servidor y mostrar siempre "Revisá este dato": pierde precisión cuando sí se puede decir qué pasa | Más filas en el mapa de mensajes; los textos de `name` cubren a la vez "vacío" y "largo" porque el servidor no los distingue |
| **DD-F25** | Esperas cortas: con `Retry-After` < 60 s, el texto es "unos segundos" (`429` del login por IP, `503` del registro); con 60 s, "1 minuto"; con más, `ceil(s/60)` minutos; sin cabecera, "unos minutos". El `503` del registro **no** se reintenta solo: "Reintentar" a mano, lo cargado se conserva y el texto aclara que no se creó nada | DD-33 R-a manda `Retry-After: 2`; el login por IP repone cada 3 s; "Esperá 1 minutos" era un error de concordancia y exageraba la espera. Mantener las mutaciones sin reintento automático sigue la regla de §10.2 (los `POST` no son idempotentes en general, y esta excepción no vale la rama de código) | Reintento automático del registro tras `Retry-After`: seguro según el contrato ("no crea nada"), pero agrega un camino que solo se ve con registros simultáneos (raro a la escala S-3) y que el usuario no controla. Deshabilitar "Reintentar" durante `Retry-After`: más estado para 2 segundos | Un toque más si el registro coincide con otros |
| **DD-F26** | Ante `5xx` o error de red en `PUT` o `DELETE /tenant/logo`, la UI vuelve a pedir `GET /tenant` (y `['session']`). Si `has_logo` y `updated_at` muestran que la operación quedó hecha (subida: `has_logo = true` y `updated_at` distinto del previo; baja: `has_logo = false`), se trata como éxito (toast normal); si no, se muestra el error con "Reintentar" | DD-42: el servidor puede devolver el error original aunque el `COMMIT` haya confirmado (relectura fallida). Sin releer, el encabezado mostraría el logo viejo y el usuario reintentaría una subida que ya estaba | (a) Solo invalidar `['tenant']` y mostrar el error igual: mensaje contradictorio con el logo nuevo en pantalla. (b) No hacer nada: el logo correcto aparece recién en el próximo refetch (5 min de `staleTime`) | Un `GET /tenant` más en el camino de error; si otro Administrador cambió el logo en ese instante, se tomaría como éxito propio (despreciable) |
| **DD-F27** | El selector de zona horaria de S-11 incluye **siempre** la zona guardada de la empresa (seleccionada), aunque `Intl.supportedValuesOf('timeZone')` no la devuelva; el grupo "Argentina" se arma con las zonas `America/Argentina/*` **y** sus alias de `Intl` (p. ej. `America/Buenos_Aires`) | Hay al menos un reporte de un navegador que devuelve el nombre viejo `America/Buenos_Aires` en lugar del IANA `America/Argentina/Buenos_Aires` (S-F11, no verificado en los navegadores de NFR-F02). Sin esto, la zona por defecto de las empresas podría no aparecer en el selector | Normalizar alias en el cliente con una tabla propia: más código y otra lista para mantener | La lista puede mostrar dos nombres para la misma zona en algún navegador |
| **INV-F15** | El body de `PATCH /tenant` solo lleva claves de `TenantUpdate` que cambiaron; `name` y `timezone` nunca van como `null`; `base_currency` nunca va. Test: T-F602 | DD-43 (1) y `DisallowUnknownFields`: cualquiera de esos casos es un `400` que el usuario no puede resolver | — | — |

---

## 7. Hallazgos

### 7.1 Cerrados o actualizados

| ID | Estado al 2026-10-12 |
|---|---|
| **H-12** (prueba en un celular real) | **Sigue abierto, no bloquea**. No hay *staging* y plan P-1 (hosting) sigue abierta. La recomendación (a) *staging* y el respaldo (b) reenvío de puertos en Android no cambian |
| Punto menor 1 de §27.2 (índice de ADR muestra ADR-019 *Proposed*) | **Cerrado**: `docs/adr/README.md` ya dice "Accepted (… detalles aceptados el 2026-09-29)" |
| Punto menor 2 de §27.2 (encabezado de `tasks.md` decía que la sección Frontend no se tocó) | **Cerrado**: el encabezado ya dice "La sección Frontend se actualizó en la misma ronda" |
| Coordinación de `tasks.md` (Backend): `method_not_allowed` en T-F004/T-F101 | **Resuelto en esta revisión** |
| Coordinación de `tasks.md` (Backend): `503` del registro con `Retry-After`, "decisión del `frontend-architect`" | **Decidido**: DD-F25 |

### 7.2 Nuevos (para el `backend-architect`)

| ID | Hallazgo | Propuesta | Impacto en el frontend |
|---|---|---|---|
| **H-13** (no bloquea) | El contrato declara el enum de `FieldError.code` pero no qué código usa cada campo, y el código usa `invalid_value` para campos de texto vacíos o largos (registro, aceptación, `PATCH` de opcionales) y `password` en el login, donde un lector del contrato esperaría `required`/`too_long` (tabla de §4.1) | (a) **Recomendada**: documentar en la descripción de cada operación (contrato *patch*) los códigos por campo, tal como están. (b) Alinear el servidor a `required`/`too_long`: cambia comportamiento probado en tres fases, sin beneficio para el usuario | Ninguno bloqueante: DD-F24 cubre los dos casos. Si se elige (b), T-F101 suma filas, nada más |
| **H-14** (documental) | `GET /industry-templates` declara `429 RateLimited` en el contrato, pero `industrytemplate.RegisterRoutes` no aplica ningún limitador | Quitar el `429` del contrato o agregar el cupo (decisión del backend; posiblemente en la revisión 16) | Ninguno: la UI trata el `429` igual en cualquier query (sin reintento, "Reintentar" a mano) |
| **H-15** (coordinación con T-F008) | `cmd/crm/main_test.go` exige `503` en `GET /login` ("without a built SPA"). Cuando T-F008 embeba `web/dist`, ese test depende de si la SPA se compiló antes de `go test` (`make check-all` corre `web-check`, que compila; la corrida siguiente de `make check` embebe el `dist` compilado) | T-F008 ajusta ese test: (a) **recomendada**: aceptar `503` con el texto de "interfaz no compilada" **o** `200` `text/html`, según exista `index.html` en `web.DistFS()`; (b) inyectar un `fs.FS` vacío en ese test | T-F008 (lo implementa `backend-developer`) |

---

## 8. Preguntas para el usuario

| ID | Pregunta | Default propuesto | Bloquea |
|---|---|---|---|
| **P-F7** | El backend acepta hasta 3 pedidos de restablecimiento por email y después uno cada 20 minutos, pero sigue respondiendo "listo" sin enviar nada (para no revelar si el email existe, DD-9, DD-39). ¿Agregamos una línea en "Revisá tu email": "Si ya lo pediste varias veces, esperá unos 20 minutos antes de pedirlo de nuevo"? | **Sí**, con ese texto. Si la revisión 16 cambia el cupo por email, el número del texto cambia con él | No (F4) |
| **P-F8** | *Solo si la revisión 16 exige el email verificado para invitar.* ¿Cómo lo muestra la UI? | **Deshabilitar** "Invitar", "Invitá a tu equipo" (panel), "Reenviar invitación" y "Reactivar" de un invitado, con el motivo visible ("Confirmá tu email para invitar a tu equipo") y el botón "Reenviar email de confirmación" al lado, como BR-F02 hace con el último administrador. El servidor sigue decidiendo: su rechazo (código nuevo) tiene mensaje propio | Solo F5, y solo si pasa |
| **P-F9** | *Solo si la revisión 16 propone una prueba anti-bots en el registro o el login.* `ui.md` §18 se comprometió a "sin CAPTCHA ni pruebas cognitivas" y la CSP solo admite scripts propios | **No** agregarla en 001: alcanzan los cupos por IP, la verificación de email y la métrica de enumeración (DD-19). Si se decide igual, es un ADR (cambia ADR-019 por la CSP y §18 por la accesibilidad) | Solo F2/F3, y solo si pasa |

Ninguna diferencia obliga a una decisión visual.

---

## 9. Dependiente de la revisión 16 (auditoría de seguridad del backend, en curso)

Estas pantallas y flujos pueden cambiar según lo que decida el `backend-architect`. El resto del
diseño no depende de eso y se puede implementar ya.

| Pantalla o flujo | Qué podría cambiar | Qué tocaría en la UI | Fase y tareas |
|---|---|---|---|
| S-01 Registro (+ §14.2) | Cupo de `signup`; postura de enumeración (DD-19: si el registro deja de confirmar que el email existe, el `409 email_already_registered` y su "Recuperar contraseña" desaparecen); prueba anti-bots (P-F9) | §13.1, §14.2, BR-F06, DD-F18, mensajes de §12.2 | F2: T-F202 (y E2E de T-F702) |
| S-05 Confirmar email y C-01 Aviso | Cupo de `confirm`/`resend`; si la verificación pasa a ser requisito de alguna acción, BR-F05 ("no bloquea nada") deja de ser cierta | §7 BR-F05, §13.5, §13.13 | F4: T-F404, T-F405 |
| S-10 Invitar y S-09 (reenviar invitación, reactivar a un invitado) | Un `429` en `POST /users/invitations`; exigir el email verificado antes de invitar (código de error nuevo y quizás `suggested_action` nueva: el mapa exhaustivo no compila hasta tener su texto) | §12.2, §13.9, §13.10, BR-F08, P-F8 | F5: T-F503, T-F504 |
| S-03 Olvidé mi contraseña y S-04 Restablecer | Cupos por IP y por email; condiciones de revisión de DD-39 (si DD-19 cambia, DD-39 pasa a la alternativa B, "piso de tiempo": invisible para la UI) | §13.3 (texto de P-F7), §13.4 | F4: T-F402, T-F403 |
| S-06 Aceptar invitación | Cupo compartido `preview`/`accept` | §13.6 | F5: T-F505 |
| S-02 Ingresar (posible, no anunciado) | Cupos o bloqueo | §13.2 | F3: T-F301 |
| §11.1 tabla de cupos, §12.2 mapa de errores | Cualquier cupo o código nuevo | Se actualizan en el mismo cambio (matriz §29) | F1: T-F101 |

**Recomendación de orden**: F0, F1, F3 y F6 no dependen de la revisión 16 (F1 solo recibe códigos
nuevos si los hay, y el compilador lo exige). F2, F4 y F5 conviene empezarlas después de que la
revisión 16 esté aprobada, o empezarlas igual sabiendo que las filas marcadas pueden cambiar.

---

## 10. Fases F afectadas (sección Frontend de `tasks.md`)

| Fase | Tareas | Cambio | ¿Bloquea? |
|---|---|---|---|
| F0 | T-F004, T-F005, T-F007, T-F008 | Contrato v0.4.3 en los tests de tipos (`method_not_allowed`, `TenantUpdate`); `Retry-After` en `503` y `405` en el cliente; `Allow` en el `405` de la SPA; T-F008 con el *stub* real, el guard de T-B801 y H-15 | No |
| F1 | T-F101 | Mensaje de `method_not_allowed`; textos por rango de segundos; variantes de campo (DD-F24) | No |
| F2 | T-F202 | `422` con el código real; `503` con `Retry-After` | **Dependiente de la revisión 16** |
| F3 | T-F301 | `429` del login en segundos | No |
| F4 | T-F402 | Cupo compartido; línea de P-F7 | **Dependiente de la revisión 16** y de P-F7 |
| F5 | T-F504, T-F505 | `422 role`; `422` de la aceptación con códigos reales; `429` de la vista previa | **Dependiente de la revisión 16** |
| F6 | T-F601, T-F602, T-F603 | CUIT en dos formas; INV-F15; reintento seguro; zona guardada; relectura tras `5xx` del logo; `404`/`503` del `<img>` | No |
| F7 | Tabla de dependencias | Backend completo: ya no espera fases | No |

Ninguna tarea nueva, ninguna renumerada, ningún cambio de orden.

---

## 11. Invariantes

- **Se agrega** INV-F15 (`PATCH /tenant`).
- **Se preservan** INV-F01 a INV-F14. INV-F05 es la que detecta el cambio obligatorio
  (`method_not_allowed`). INV-F14 sigue siendo cierta con DD-43: un `PATCH` sin cambios no mueve
  `updated_at`, así que la URL del logo solo cambia cuando cambia algo.
- Ninguna invariante se altera.

---

## 12. Mantenimiento

Esta revisión agrega una fila a la matriz de `ui.md` §29: "Un cupo de DD-9, o el código de error de
campo que usa un endpoint" → §11.1 (tabla de cupos) + §12.3 + `api/errorMessages.ts` + el test de la
pantalla. Cuando la revisión 16 se apruebe, se actualizan las filas de §9 de este documento que se
hayan concretado y se cierra esta lista en `ui.md` §26.

---

## 13. Fuentes

- WebKit Bugzilla 218542 (Safari devolvía `America/Buenos_Aires` en lugar de
  `America/Argentina/Buenos_Aires`): <https://auto-bugs.webkit.org/show_bug.cgi?id=218542>. Es
  la base de S-F11; no verifiqué el comportamiento actual de Chrome, Firefox ni Safari 16.4+.

---

## 14. Nota posterior: Revisión 4 (2026-10-13)

La lista de §9 ("Dependiente de la revisión 16") y la condición de P-F8 y P-F9 quedaron resueltas
con la decimosexta revisión del backend (aprobada el 2026-10-12, contrato **v0.5.0**). Lo que
decidió el frontend está en dos lugares:
- la **Revisión 4** de [`ui.md`](ui.md): cabecera, BR-F05, BR-F14, INV-F16, §11.1, §12.2, §13.1,
  §13.5, §13.7, §13.9, §13.10, §13.13, DD-F28 a DD-F30, P-F10, P-F11 y H-16;
- la cuarta revisión de la sección Frontend de [`tasks.md`](tasks.md).

Este documento no se reescribe: §9 y §10 describen el estado al 2026-10-12.
