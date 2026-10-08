# Tasks: Empresas, usuarios y roles

**Spec**: [`spec.md`](spec.md) · **Plan**: [`plan.md`](plan.md) · **Modelo**: [`data-model.md`](data-model.md)
· **Contrato**: [`contracts/openapi.yaml`](contracts/openapi.yaml)
**Revisión 2026-09-27**: incorporadas P-2 a P-5 (tareas afectadas: T-B002, T-B201, T-B303, T-B305,
T-B307, T-B402, T-B501, T-B506, T-B604, T-B606, T-B903).
**Revisión 2026-09-29**: incorporados los hallazgos H-1 a H-9 del frontend (plan §18) en la
sección Backend (tareas afectadas: T-B002, T-B004, T-B005, T-B011, T-B203, T-B204, T-B213,
T-B303, T-B305, T-B601, T-B603, T-B605, T-B606, T-B702, T-B703, T-B705, T-B801, T-B901, T-B903).
La sección Frontend se revisó aparte (ver su nota de revisión).
**Segunda revisión 2026-09-29**: hallazgos H-10 (HTTPS local con mkcert) y H-11 (límite exacto del
logo) en la sección Backend (plan §18; tareas afectadas: T-B002, T-B004, T-B005, T-B013, T-B014
nueva, T-B203, T-B213, T-B305, T-B404, T-B703, T-B705, T-B706). La sección Frontend se actualizó en la misma ronda (ver su nota de revisión).
**Tercera revisión 2026-09-30**: decisiones previas a la Fase 2 aprobadas por el usuario (plan §18,
tercera tanda): código `method_not_allowed` (contrato v0.4.0), IP del cliente detrás de proxies
(DD-32, INV-25), cancelaciones (`db.ErrCanceled`), serialización de registros por el lock de
`GRANT crm_tenant` (DD-33, INV-26), rutas exactas de las queries de sistema y FK de
`sessions`/`user_tokens` alineadas en `data-model.md`. Tareas afectadas de la sección Backend:
T-B002, T-B004, T-B005, T-B011, T-B105, T-B106, T-B112, T-B203, T-B204, T-B211, T-B217, T-B218,
T-B219 y T-B220 (nuevas), T-B303, T-B304, T-B305, T-B901, T-B902, T-B903, T-B905, T-B906, T-B907,
T-B908. La sección Frontend no se tocó en esta revisión (lo que le pide el backend está en
"Coordinación con la sección Frontend").
**Cuarta revisión 2026-09-30**: primer `SET ROLE` a una empresa recién aprovisionada desde otra
conexión (`42501` en PostgreSQL 18.6): lectura de catálogo previa y un único reintento en
`InTenantTx` (plan DD-34, INV-27, R-17; research R-28). Tareas afectadas: T-B103 y T-B104 (y
menciones en T-B903 y T-B905). La sección Frontend no cambia: el contrato no cambia.
**Quinta revisión 2026-10-01** (**Accepted** por el usuario): decisiones de la
revisión del PR fdelillo/crm#8 (plan §18, quinta tanda): clasificación de fallos de entrega,
`last_error` saneado, presupuesto del envío y aislamiento de fallos por mensaje (ADR-024, DD-35,
INV-28 a INV-31); imagen de MinIO (DD-36). Tareas afectadas: T-B112, T-B211, T-B212, T-B213,
T-B214, T-B215, T-B216 (Fase 2, en el PR abierto) y T-B903, T-B908, T-B909 (Fase 9). La sección
Frontend no cambia: el contrato no cambia.
**Sexta revisión 2026-10-01** (*Accepted*): tercera revisión del PR fdelillo/crm#8 (plan §18, sexta
tanda): credenciales en `audit_log.data` (DD-37, INV-32), `user_agent` acotado (DD-38, INV-33),
`last_error` según ADR-025 (INV-30), `RequestID` antes de `ClientIP` (DD-22, DD-32), texto de DD-35.
Tareas afectadas: T-B203, T-B204, T-B209, T-B210, T-B211, T-B212, T-B213, T-B214, T-B219 (Fase 2,
en el PR abierto) y T-B303, T-B304 (Fase 3). La sección Frontend no cambia: el contrato no cambia.
**Séptima revisión 2026-10-02** (*Accepted*, aprobada por el usuario el 2026-10-02): pendientes
antes de la Fase 3 (plan §18, séptima tanda): credencial incrustada en un texto de
`audit_log.data` y `data` exacto del catálogo (DD-37 (4) y (5), INV-32), texto de DD-38 sobre
`U+FFFD` alineado con el código (sin cambio de comportamiento), validador del contrato para los
tests HTTP (T-B311 y T-B312 nuevas; cierra el hueco diferido de T-B201/T-B004; nota 2026-10-02 en
ADR-014), el ajuste de DD-36 que quedó sin aplicar en la Fase 2 (obligatorio al comienzo de la
Fase 3, decisión del usuario) y el contrato **v0.4.1** (`413 payload_too_large` declarado en toda
operación con body JSON, decisión del usuario). Tareas afectadas: T-B004, T-B201, T-B209, T-B210,
T-B215, T-B216 (ajustes al comienzo de la Fase 3) y T-B301, T-B302, T-B303, T-B304, T-B305,
T-B309, T-B311, T-B312 (Fase 3). La sección Frontend no cambia: el contrato 0.4.1 no agrega valores
a ningún enum (ver "Coordinación con la sección Frontend").
**Octava revisión 2026-10-04** (*Accepted*, aprobada por el usuario el 2026-10-04): revisión posterior a la Fase 5 (plan §18, octava
tanda): tiempo de respuesta del pedido de reset aceptado como riesgo residual (DD-39, INV-13
precisado, R-7) y código muerto en `issueToken`. Tareas afectadas: T-B501, T-B503, T-B506 (Fase 5,
ajustes antes de la Fase 6). La sección Frontend no cambia: el contrato no cambia.
**Corrección 2026-10-05** (revisión del PR #12; plan §18, al final de la octava tanda): DD-9, DD-39 y R-7 corregidos en su descripción (cada cupo es ráfaga + reposición, no un máximo por hora; el pedido de reset tiene tres caminos, no dos). La decisión de DD-39 no cambia; T-B501 precisa el caso del 4.º pedido. El cupo por IP compartido entre pedido y confirmación (reset) y entre `confirm` y `resend` (verificación) queda registrado como deliberado. Contrato **v0.4.2**: solo el texto del rate limit de signup (*patch*; el frontend no cambia). Sin cambio de código.
**Novena revisión 2026-10-05** (*Accepted*, aprobada por el usuario el 2026-10-05): locks de la empresa y del usuario en la Fase 6 (plan §18, novena tanda). La query de INV-10 y el lock del usuario pasan de `FOR UPDATE` a `FOR NO KEY UPDATE`, porque `FOR UPDATE` choca con el `FOR KEY SHARE` con que PostgreSQL verifica las FK y producía deadlocks con el reset, el login y el cierre de sesión (el de la empresa, confirmado en CI; DD-40, INV-10). La sesión de un login concurrente con la desactivación queda como riesgo aceptado (DD-41, INV-11). También: regresión que reemplaza al test diagnóstico (T-B604), fila de reinvitaciones concurrentes de T-B601 corregida, orden de escritura de la fila de `tenants` en la Fase 7 (T-B704) y caso de limpieza de T-B606 diferido a T-B901. Tareas afectadas: T-B503 (Fase 5, ajuste de `GetTokenFlowUser` dentro del PR de la Fase 6), T-B601, T-B604, T-B605, T-B606 (Fase 6, en curso), T-B704 (Fase 7), T-B901 (Fase 9). La sección Frontend no cambia: el contrato no cambia.
**Décima revisión 2026-10-06** (*Accepted*, aprobada por el usuario el 2026-10-06): revisión del PR fdelillo/crm#15 (Fase 7; plan §18, décima tanda): `COMMIT` de resultado incierto al subir el logo y lectura con el objeto ausente (DD-42, INV-17 precisada), `LockTenant` en `PATCH /tenant` y en el logo con sus tests de concurrencia (DD-40 corregida, INV-34 nueva), semántica de `PATCH /tenant` y `Local` en el registro (DD-43, DD-27), defensas sin test de la subida y bucket de desarrollo. Tareas afectadas: T-B303 (Fase 3), T-B604 (Fase 6, regla R0), T-B701 a T-B706 y T-B707 nueva (Fase 7). Contrato **v0.4.3** (*patch*: solo descripciones). La sección Frontend no cambia.
**Undécima revisión 2026-10-07** (*Accepted*): revisión del PR fdelillo/crm#16 (Fase 8; plan §18, undécima tanda): precisa lo que T-B801 a T-B803 verifican (cabeceras del `404` de un id ajeno, lecturas del operador, centinelas en cuerpo y cabeceras, sesiones sin revocar de los desactivados, aplazamiento como `crm_worker`, router de `crm serve`, conteo real). Tareas afectadas: fixture común, T-B801, T-B802, T-B803 y checkpoint de la Fase 8. La sección Frontend no cambia: el contrato no cambia. (Línea agregada en la duodécima revisión.)
**Duodécima revisión 2026-10-08** (*Accepted*, aprobada por el usuario el 2026-10-08): pendientes de la revisión de cierre del PR fdelillo/crm#16, aplicados en el paso 0 de la Fase 9 (plan §18, duodécima tanda): guard del router de `crm serve` con `go/types` que cubre también el servidor de métricas, armado por `internal/app` (`app.NewMetricsServer`); centinelas cortos y conteo por tipo en T-B801. Cierres para la Fase 9: `/readyz` lee la versión de la base con `SELECT (version_id)` concedido solo a `crm_auth` (migración `00009`), las métricas que leen la base se muestrean cada 60 s y el inventario de roles suma `tenant_roles_missing` y `tenants_total`. Tareas afectadas: T-B801 (Fase 8) y T-B903, T-B904, T-B908 (Fase 9). La sección Frontend no cambia: el contrato no cambia.
**Decimotercera revisión 2026-10-08** (*Accepted*, aprobada por el usuario el 2026-10-08): detención en T-B905 y pendientes de la revisión de código del PR fdelillo/crm#17 (plan §18, decimotercera tanda; [`revision-13-t-b905.md`](revision-13-t-b905.md)). Target de §13 redefinido (retención sin contención y ráfaga), DD-33 R-e, re-medición T-B910 con reglas de decisión, reglas (4) y (5) del guard de `crm serve` y tests de `runServe` (T-B911). Tareas afectadas: T-B801 (Fase 8, en la rama de la Fase 9), T-B905, T-B910 y T-B911 (nuevas), y la prueba independiente y el checkpoint de la Fase 9. La sección Frontend no cambia: el contrato no cambia.

---

## Backend

Autor: `backend-architect`. Implementa: `backend-developer`, **una fase por invocación**.

### Estado de la implementación (2026-10-05)

- **Fase 0**: implementada y mergeada (PR fdelillo/crm#6).
- **Fase 1**: implementada y mergeada.
- La tercera y la cuarta revisión tocan tareas ya implementadas. Se aplican como ajustes en la rama
  de la Fase 1 (si sigue abierta) o como primer cambio de la Fase 2, **antes** de T-B201, con
  `make check` en verde:

  | Tarea ya implementada | Fase | Ajuste |
  |---|---|---|
  | T-B002 / T-B003 | 0 | Variable `TRUSTED_PROXIES` (DD-32): filas nuevas de la tabla; `Config.TrustedProxies []netip.Prefix`; `.env.example` la documenta vacía, con un comentario que remite a plan §10.5 |
  | T-B004 / T-B005 | 0 | `405` de la API con `code: method_not_allowed` (antes `malformed_request`). El cálculo de `Allow` con `Match` ya está en `internal/app/root.go` y no cambia; log de arranque con `trusted_proxies` |
  | T-B011 | 0 | Regla nueva: las cabeceras de IP solo se leen en `internal/platform/httpx` (INV-25) |
  | T-B103 / T-B104 | 1 | (Cuarta revisión, DD-34) La lectura de catálogo antes del `SET LOCAL ROLE` ya está aplicada (commit e3d990b, con el test de reproducción); falta el reintento único en `InTenantTx` (lo está implementando el `backend-developer`) y los casos nuevos de T-B103 |
  | T-B105 / T-B106 | 1 | `db.ErrCanceled` y `55P03` → `db.ErrUnavailable`, con el orden de clasificación de plan §9.2 |
  | T-B112 | 1 | Lista exacta de las cuatro rutas de plan §4.4: `queryrules.DefaultExceptions` suma `internal/identity/store/cleanup.sql` y `internal/tenant/store/provisioning.sql` a las dos que ya tiene |

- **Fase 2**: implementada en `feat/001-backend-phase-2` (PR fdelillo/crm#8) y **mergeada** en
  `main` (2ff34be, 2026-10-02). La quinta y la sexta revisión se aplicaron en esa rama, **salvo** el
  ajuste de T-B215/T-B216 (DD-36): ver la tabla de la Fase 3. Las dos tablas siguientes quedan como
  historia de lo que se pidió en esa rama:

  | Tarea | Ajuste |
  |---|---|
  | T-B112 (Fase 1) | `queryrules.DefaultExceptions` lista `DeferMessage` en `internal/platform/outbox/store/worker.sql`, con el motivo "crm_worker: aplazar un mensaje pendiente cuya fase de empresa falló (ADR-024 §6)" |
  | T-B211 / T-B212 | Casos nuevos de T-B211 (marcados "Quinta revisión"); `DeliveryError` reemplaza a `PermanentError`; aislamiento por mensaje; contextos y presupuesto; `Enqueue` con el reloj inyectable |
  | T-B213 / T-B214 | Casos nuevos de T-B213 (servidor SMTP falso); adaptador con `WithTimeout(5 s)`, `WithoutRset()` e `IsDelivered()`; el handler de plantillas devuelve `DeliveryError{bug, compose}` |
  | T-B215 / T-B216 | Imagen de MinIO de DD-36 y paquete `internal/testsupport/containers` con su test contra `compose.yaml` |
  | T-B201 (pendiente) / T-B004 | La validación de las respuestas contra `Problem`/`ValidationProblem` del contrato con `libopenapi-validator` (ya elegido, ADR-012:38, ADR-014:22) no se implementó: el hueco viene de T-B004 (Fase 0). Se difiere al inicio de la Fase 3, donde T-B305 ya la exige ("contrato validado en cada respuesta"). Mientras tanto, `TestProblemCodesMatchContract` (`internal/platform/httpx`) es un control provisorio y más barato: compara el conjunto de claves de `problems` contra el enum `ErrorCode` del contrato, como `authz_test.go` ya hace con `Permission` |
  | T-B903 / T-B908 / T-B909 (Fase 9) | Métricas `outbox_delivery_errors_total{cause}` y `outbox_deferred_total`; envío que termina bien durante el apagado queda `sent`; timeout de apagado ≥ 25 s |

  **Sexta revisión (2026-10-01, *Accepted*)**, tercera revisión del PR (HEAD 31fb773). Se aplica en
  la misma rama una vez aprobada; los casos marcados "Sexta revisión" se escriben primero y deben
  fallar por la razón indicada en cada tarea:

  | Tarea | Ajuste |
  |---|---|
  | T-B209 / T-B210 | Política de credenciales de DD-37 (centinelas `ErrSecretInData`, `ErrUnsupportedData`, `ErrTxRequired`); `user_agent` con `httpx.NormalizeUserAgent` (DD-38) |
  | T-B203 / T-B204 / T-B219 | `RequestID` antes de `ClientIP`; el aviso `bad_forwarded_for` lleva `request_id` (DD-32) |
  | T-B211 / T-B212 | `LastError()`: sin texto del proveedor en la fase `data`; redacción de `://` y de secuencias de 20 o más caracteres (ADR-025 §1–§2) |
  | T-B213 / T-B214 | Código extendido leído del texto en todas las fases, con control de clase (ADR-025 §3); caso de *deadline* de DD-35; comentario de `Mailer.Send` |
  | T-B303 / T-B304 (Fase 3) | `CreateSession` normaliza `user_agent` (DD-38): se hace al implementar la Fase 3, no en este PR |

- **Fase 4** (2026-10-03): implementación de T-B401 a T-B405 en `feat/001-backend-phase-4`, desde el merge del PR #9. Login con bloqueo por HMAC del email, auditoría, rehash y sesión; logout idempotente; pruebas unitarias, de integración y HTTP con contrato. `make lint` y `make test` locales en verde; `make check` en CI en verde (Docker no está disponible en el equipo local). **Mergeada** en `main` con el PR fdelillo/crm#10 (f379160).

- **Fase 3** (2026-10-03): implementada en `feat/001-backend-phase-3`, creada desde `main`
  (2ff34be). **Séptima revisión (*Accepted*, aprobada por el usuario el 2026-10-02)**: antes de
  T-B301, en este orden y con `make check` en verde después de cada paso, se aplican estos ajustes.
  **Los cuatro son obligatorios.** Los casos marcados "Séptima revisión" se escriben primero; cada
  tarea dice si deben fallar (Red) o si caracterizan el comportamiento actual. El contrato ya está
  en **v0.4.1** (lo cambió esta revisión; no hay paso de código para eso): sus casos de `413` están
  en T-B201, T-B305 y T-B311.

  | Orden | Tarea | Ajuste |
  |---|---|---|
  | 1 | T-B209 / T-B210 (Fase 2) | DD-37 (4): una clave de credencial seguida de `=` o `:`, o una secuencia de 43 caracteres base64url, **dentro** de un texto (casos "Séptima revisión", fallan hoy); texto de DD-38 sobre `U+FFFD` alineado con el código (los casos nuevos de `NormalizeUserAgent` pasan sin cambios) |
  | 2 | T-B311 / T-B312 (nuevas, Fase 3) | Validador del contrato `internal/testsupport/contract` con `libopenapi-validator`: cierra el hueco diferido de T-B201/T-B004 (fila de la tabla de la Fase 2) |
  | 3 | T-B201 / T-B004 (Fases 2 y 0) | Sus casos de problem+json validan con `contract` (detalle en T-B312), incluido el `413` de un body JSON de más de 65 536 bytes. `TestProblemCodesMatchContract` se mantiene |
  | 4 | T-B215 / T-B216 (Fase 2; ajuste de la quinta revisión que no se aplicó; **obligatorio**, decisión del usuario del 2026-10-02) | Hoy en `main`: `compose.yaml` usa `quay.io/minio/minio:latest` (no se descarga sin login desde el 2026-09-24/25, DD-36), `internal/platform/objectstore/objectstore_integration_test.go` usa `bitnamilegacy/minio@sha256:…` (que T-B215 prohíbe) y no existe `internal/testsupport/containers`. Primero se escriben los casos de T-B215 marcados "Quinta revisión" (fallan hoy). Después: **(a) `compose.yaml`**: el servicio `minio` usa `ghcr.io/coollabsio/minio:RELEASE.2025-10-15T17-29-55Z@sha256:<digest>`, con el digest del **índice multiplataforma** de ese tag (`docker buildx imagetools inspect`); mantiene `command: server /data --console-address ":9001"` y pierde el comentario sobre `quay.io`. **(b) Test de S3** (T-B215): usa `containers.MinIO` en lugar de `bitnamilegacy/…`; ningún archivo del repo fuera de la documentación nombra `quay.io/minio`, `minio/minio:` ni `bitnamilegacy/`. **(c) `internal/testsupport/containers`**: constantes `Postgres` (`postgres:18`), `Mailpit` (`axllent/mailpit:v1.27`) y `MinIO` (la referencia de (a)); las usan `pgtest`, los dos tests que levantan Mailpit (`internal/identity/emails/emails_integration_test.go` e `internal/app/phase2_independent_integration_test.go`) y el de S3; su test unitario verifica que los `image:` de `postgres`, `mailpit` y `minio` en `compose.yaml` son exactamente esas constantes. Si el tag no se descarga sin login, no se elige otra imagen por cuenta propia: respaldo de DD-36 (`cgr.dev/chainguard/minio` por digest) y aviso al arquitecto. **Verificación**: `docker compose up -d` con **todos** los servicios, sin login a ningún registro, y `make check` en verde |

  La implementación de T-B301 a T-B312 pasó `make check`. El checkpoint independiente usó el
  binario real con PostgreSQL, Mailpit y MinIO levantados: `curl` devolvió `201`, el correo de
  verificación apareció en Mailpit y el registro repetido devolvió `409`. PostgreSQL se publicó
  temporalmente en `127.0.0.1:15432` porque el puerto 5432 del host ya estaba ocupado; no se
  modificó `compose.yaml` por este conflicto local.

- **Fase 5** (2026-10-04): T-B501 a T-B507 implementadas en `feat/001-backend-phase-5` y
  **mergeadas** en `main` con el PR fdelillo/crm#11 (fa22314). `make check` en CI en verde. Revisada
  con Codex: el orden de locks entre el login y la confirmación de reset (`throttle → user → token`)
  se corrigió en el mismo PR (`TestConcurrentLoginAndPasswordResetUseSameLockOrder`). Las Fases 3 y 4
  también están en `main`. **Checkpoint verificado el 2026-10-04** en `fix/001-phase-5-followups`
  (ver "Ajustes posteriores a la Fase 5" y el resultado más abajo): `make check` en verde y la
  prueba independiente (a) a (h) ejecutada con el binario real contra `docker compose`.

  **Ajustes posteriores a la Fase 5 (octava revisión, 2026-10-04, *Accepted*)**. Se aplican en
  `fix/001-phase-5-followups` (creada desde `main` en fa22314), antes de la Fase 6, en este orden y
  con `make check` en verde después de cada paso:

  | Orden | Tarea | Ajuste |
  |---|---|---|
  | 1 | T-B503 (refactor; sin Red) | En `issueToken` (`internal/identity/token_flows.go`) se borran la variable `template` y el `if purpose == "invitation"`, y `outbox.Message.Template` recibe `purpose`. **No hay test que deba fallar**: no cambia el comportamiento. La red de seguridad ya existe; correrla **antes** de tocar el código para confirmar que está en verde. En `internal/identity/token_flows_integration_test.go`, `deliveredToken` busca el mensaje por plantilla en `TestPasswordResetLifecycle` (`password_reset`), `TestPasswordResetInvitedAndUnknown` (`invitation`, reemisión de DD-20) y `TestEmailVerificationLifecycle` (`email_verification`). Si alguno deja de encontrar su mensaje después del cambio, el refactor rompió algo. **Refactor**: nada más en esa función |
  | 2 | T-B501 / T-B503 / T-B506 (DD-39; sin cambio de comportamiento) | El tiempo de respuesta **no** se iguala (DD-39): no se agrega piso de tiempo, ni procesamiento fuera del request, ni un test de tiempos (sería frágil y fijaría algo que la decisión no promete). Único cambio: el comentario de `RequestPasswordReset` dice qué es uniforme y qué no. Uniforme: el resultado `nil`, y el `202` sin cuerpo del handler en todos los estados (INV-13). No uniforme: el trabajo y el tiempo, que dependen del estado de la cuenta (riesgo aceptado, DD-39). **Tests**: `TestPasswordResetInvitedAndUnknown` y `TestPasswordResetEmailRateLimit` (T-B501) y `TestRecoveryHTTPContractAndUniformResponses` (T-B506) no cambian y siguen en verde. DD-39 aprobada por el usuario el 2026-10-04 |
  | 3 | Checkpoint de la Fase 5 (**verificado 2026-10-04**, resultado abajo) | `make check` en verde y la **prueba independiente** con el binario real y `docker compose up -d` (PostgreSQL, Mailpit, MinIO): (a) registrar una empresa; (b) `POST /api/v1/auth/password-reset` con su email → `202` sin cuerpo; (c) el email con el enlace aparece en Mailpit; (d) `POST /api/v1/auth/password-reset/confirm` con el token del fragmento `#token=` y una contraseña nueva → `204`; (e) la cookie de la sesión del registro da `401` en `GET /api/v1/me`; (f) login con la contraseña nueva → `200`, con la anterior → `401`; (g) registrar otra empresa y llevar a su administrador a `invited` con `psql` en el contenedor (solo para esta prueba, respetando `users_active_complete_chk`; hasta la Fase 6 no hay endpoint de invitación) y pedir reset → llega a Mailpit un email de **invitación** nuevo y ningún enlace de reset; (h) email inexistente → el mismo `202` vacío y nada nuevo en Mailpit. El resultado se reporta en esta sección, como el checkpoint de la Fase 3, incluidos los desvíos locales (p. ej. el puerto de PostgreSQL) |

  **Resultado del checkpoint de la Fase 5 (2026-10-04)**: pasos 1 y 2 aplicados (el paso 1 con
  `TestPasswordResetLifecycle`, `TestPasswordResetInvitedAndUnknown` y `TestEmailVerificationLifecycle`
  en verde antes de tocar el código) y `make check` en verde (lint, `go test -race` y
  `go test -race -tags=integration` de todos los paquetes). Prueba independiente con el binario
  real (`crm migrate up` y `crm serve`) contra `docker compose up -d --wait` (PostgreSQL, Mailpit,
  MinIO): (a) alta de empresa → `201` con cookie de sesión; (b) `POST /auth/password-reset` →
  `202`, `Content-Length: 0`; (c) en Mailpit, el email "Recuperá tu contraseña" con
  `/reset-password#token=...`; (d) confirm con ese token y contraseña nueva → `204`; (e) la
  cookie previa en `GET /me` → `401` (antes del reset daba `200`); (f) login con la nueva → `200`,
  con la anterior → `401`; (g) segunda empresa, su administrador llevado a `invited` con
  `UPDATE app.users SET status='invited', password_hash=NULL` como `postgres` (cumple
  `users_active_complete_chk`; sin choque con CHECK ni RLS) y reset → `202` vacío y un único
  email nuevo, "Invitación a tu empresa" con `/accept-invitation#token=...`, ningún enlace de
  reset; (h) email inexistente → el mismo `202` vacío, nada nuevo en Mailpit. Desvíos locales:
  el puerto 5432 del host estaba ocupado por otro contenedor, así que PostgreSQL se publicó en
  `127.0.0.1:15432` con un archivo de override fuera del repo (`!override` de `ports`), sin tocar
  `compose.yaml`; `APP_BASE_URL=https://localhost:8080` sin TLS (modo "backend only" de
  `.env.example`: HTTP plano, sin certificados); no se usó `.env`. Al terminar se bajaron los contenedores y volúmenes con `down -v`.

- **Fase 6** (2026-10-06): implementación completada en `feat/001-backend-phase-6` (PR fdelillo/crm#13), retomada con la **novena revisión** (*Accepted*, DD-40 y DD-41). `main` se incorporó mediante merge; en el único conflicto de esta sección ganó la entrada de `main`. T-B601 a T-B607 completas, con la fila de limpieza a 40 días de T-B606 diferida a T-B901 según la decisión aprobada.

  | Orden | Tarea | Resultado |
  |---|---|---|
  | 1 | T-B604 (Red) | R0 detectó las tres queries con `FOR UPDATE`; R1/R2/R4 detectaron inmediatamente backend bloqueado por la empresa con `pg_blocking_pids`; R3/L1/L2 recibieron `40P01`. R5 pasó desde el principio. Se borró `TestProposedTenantLockConflictsWithPasswordReset`. Mecanismos E/U/S/V, contexto de 20 s y sondeo cada 10 ms, sin `sleep` en el camino exitoso |
  | 2 | T-B503 | `GetTokenFlowUser` pasó a `FOR NO KEY UPDATE`, seguido de `make generate`. L2 y los tests existentes de la Fase 5, incluido `TestConcurrentLoginAndPasswordResetUseSameLockOrder`, pasaron sin cambiar esos tests |
  | 3 | T-B605 | `LockUsersTenant` y `LockManagedUser` usan `FOR NO KEY UPDATE`; mutaciones, protección del último admin y auditoría atómica completas. R0–R5/L1/L2 en Green. Las operaciones de administración toman empresa → usuario → tokens; la aceptación toma usuario → token, permitido por DD-40 |
  | 4 | T-B601 | 20 repeticiones de reinvitaciones de un operador como operator/admin; cadena exacta de roles, rol final, dos reemisiones, cambios reales y pares de auditorías con el mismo `xmin`; una invitación abierta de la última operación. Los otros cuatro casos de concurrencia también pasaron 20 repeticiones |
  | 5 | T-B606/T-B607 | Tabla HTTP completa con OpenAPI, estados, invitación vencida hace tres días, 403 previos a buscar recursos, 404 idénticos, conflictos, cookies y validaciones. Handlers conectados en `serve.go`; preview/accept comparten ráfaga 20 y reposición de 1 cada 3 min por IP. Limpieza a 40 días diferida a T-B901 |
  | 6 | Checkpoint | `make check` local completo en verde (lint, tests con `-race`, integración PostgreSQL 18, queryrules y reporules). CI: aprobado en [GitHub Actions](https://github.com/fdelillo/crm/actions/runs/37463948391) para `0a41d91` (código final). Prueba independiente: 8/8 pasos en verde |

  Durante Red, `make check` de los pasos 1 y 2 permaneció rojo por las queries pendientes de R0: no se ocultó la regresión para obtener un checkpoint verde artificial. La verificación completa de los pasos 3 y 4 pasó en una corrida conjunta después de adaptar el test previo de reinvitaciones al orden aprobado (ya no usa `occurred_at`) y aislar las filas que crea el fixture. Los pasos 5 y 6 también tuvieron `make check` completo en verde. La validación HTTP adicional tuvo su propio Red (código fuera del enum) y Green.

  **Prueba independiente**: binario real, `crm migrate up` y `crm serve`, contra `docker compose` con PostgreSQL, Mailpit y MinIO. (1) registro de A → 201 con cookie; (2) invitación operator → 201 y correo recibido; (3) reinvitación admin → 200, correo nuevo y token anterior → 400 `token_invalid`; (4) preview → 200 con admin, accept → 201 con cookie segura y `/me` → admin verificado; (5) segundo operador acepta y `/users` → 403; (6) desactivación → 200 y siguiente `/me` del operador → 401; (7) reactivación active → 200 y login con la contraseña original → 200; (8) bajar rol y desactivar al único admin → 409 `last_admin` en ambos. Para preparar (8), B se bajó a operator después de aceptar como admin. Desvíos locales: puerto PostgreSQL 15432 con override fuera del repo porque 5432 estaba ocupado; modo backend HTTP plano con `APP_BASE_URL=https://localhost:8080` y envío explícito de cookies en el cliente de prueba. Binario detenido y contenedores/volúmenes retirados con `down -v`.

  **Alcance sobre fases anteriores**: el único ajuste de concurrencia fue `GetTokenFlowUser`. Como pide T-B605, también se adaptaron mínimamente `issueToken`, `reissueInvitation` y `InsertUserToken` para el actor/creador de la invitación; sus llamadas anteriores conservan actor/creador nulos. No se modificaron login/logout/reset/verificación ni sus tests. Sin dependencias nuevas y sin otros cambios al diseño. El riesgo de R5 es el aceptado DD-41; no quedan huecos de diseño pendientes.

  **Correcciones tras la revisión del PR #13** (2026-10-06, base `6befd1d`): puntos 1 a 6 y los cuatro nits implementados. El email de 255 caracteres tuvo Red medido (`500`) y Green (`422`, `email: invalid_format`); `ValidEmail` se comparte entre Invite/login/reset, conservando la normalización y las respuestas de login/reset. T-B601 agrega la prueba determinista de dos invitaciones nuevas, creación + reemisión, un usuario, dos mensajes pendientes, auditorías exactas y una invitación abierta de la reemisión; el caso anterior se llama `reinvite_distinct_roles`. T-B604 afirma `revoked_at` de cada token pendiente y conserva los usados, ya revocados y del otro usuario. T-B606 verifica 10 preview + 10 accept intercalados con token inválido → 400 y el 21.º preview → 429 con `Retry-After`. El caso reactivate/deactivate afirma el estado final y el error de Deactivate. `withInvitation` consulta la tabla de transiciones; el actor de los helpers es explícito (`*uuid.UUID`), R2 selecciona el mensaje por la transacción de la invitación sin filtrar el rol, el email propio exige `ErrEmailTaken` y el token sustituido por reset se rechaza antes de aceptar el nuevo.

  **Aclaración aprobada por el usuario en esta sesión**: para la fila nueva de T-B601, el test cuenta bloqueos directos **o transitivos** mediante `pg_blocking_pids`, con contexto de 20 s y sondeo de 10 ms. PostgreSQL 18 encola los locks de tupla y mostró `T_admin → Invite 1 → Invite 2`; contar solo los bloqueos directos no llegaba a dos. Se conservan los locks y su orden. **Mutaciones comprobadas y restauradas**: quitar `LockUsersTenant` de Invite falla porque un pedido devuelve antes del ROLLBACK; moverlo después de buscar el email falla por `ErrEmailTaken`; quitar `RevokeDisabledUserTokens` falla por la invitación, el reset y la verificación pendientes con `revoked_at` nulo. Checkpoint local: `make check` completo en verde, incluido `-race`, PostgreSQL 18, queryrules y reporules. CI de las correcciones: `make check` aprobado para `85e6381` en [GitHub Actions](https://github.com/fdelillo/crm/actions/runs/37508150534); el último commit solo registra este resultado.

- **Fase 7** (2026-10-06): T-B701 a T-B707 completas en `feat/001-backend-phase-7`, creada desde `main` actualizado en `71f08a2`; PR [fdelillo/crm#15](https://github.com/fdelillo/crm/pull/15), abierto como borrador al primer push. Validaciones de CUIT con tabla y fuente oficial de ARCA (incluidos resultados 10/11 del módulo 11), nombre y timezone; PATCH parcial y nullable, rechazo de campos desconocidos y auditoría solo de nombres; logo PNG/JPEG por firma y DecodeConfig, límites exactos, EXIF intacto, UUIDv7/ETag, compensación y borrado después del COMMIT. Handlers y S3 conectados al binario; permisos antes del payload, MultipartReader acotado y políticas de caché del contrato. `Local` se rechaza en PATCH por no identificar una zona IANA; el registro aplica el default ante `Local` con `signup_timezone_defaulted` (DD-27/DD-43, décima revisión).

  **Verificación inicial**: Red de las operaciones faltantes y Green de las tablas; 20 mutaciones efectivas comprobadas y restauradas, incluidas las cinco obligatorias. Quitar tamaño o dimensiones, permitir CUIT/zonas/nombres inválidos, omitir null o permitir campos desconocidos, consultar S3 con ETag coincidente, omitir compensación, borrar antes del COMMIT (SetLogo y RemoveLogo), auditar antes del UPDATE, volver el lock nuevo a FOR UPDATE, quitar MaxBytesReader/permisos/control de partes, alterar caché, SSL, bytes del logo ajeno o el Content-Type de JPEG causaron fallos en sus tests. El fake observa Put/Get/Delete y una conexión separada observa el estado confirmado al borrar; un wrapper del Tx real verifica UPDATE tenants como primera escritura y auditoría después. R0 admite `LockUsersTenant` y `LockTenant` por nombre y ruta exacta, ambas con FOR NO KEY UPDATE. `make check` completo local en verde (lint, sqlc diff, -race, PostgreSQL 18, queryrules y reporules), repetido tras el ajuste de timezone. CI del código `576e30e` en verde en [GitHub Actions](https://github.com/fdelillo/crm/actions/runs/37530001293).

  **Prueba independiente**: binario real, PostgreSQL 18.6 y MinIO/Mailpit de compose, con curl en los 14 pasos: signup 201; PNG 200; CUIT 200 normalizado; invitación 201 y correo recibido; aceptación del operador 201; datos y logo 200; PATCH del operador 403; ETag vigente 304 sin cuerpo; reemplazo 200; ETag viejo 200 con bytes nuevos y otro ETag; archivo de 2 097 153 bytes 413; DELETE 204; GET con ETag viejo tras DELETE 404, nunca 304. Caché y nosniff verificados. Binario detenido y `down -v` completado, incluidos los dos volúmenes. Desvíos locales: proyecto compose `crm-phase7`, PostgreSQL 15432 por conflicto con 5432 (override fuera del repo); HTTP backend 18080 con APP_BASE_URL HTTPS y cookie explícita según DD-24; Ryuk deshabilitado solo local por fallos de puertos de Docker 29.7.2; cachés de Go/lint bajo /tmp por el sandbox. CI usa make check sin estos overrides.

  **Alcance**: se agregó la opción de storage al constructor de tenant, las variables S3 en config y el cableado de serve; el comentario de archivos de .env.example se actualizó. El control estático de locks de la Fase 6 se extendió con una excepción precisa para la query nueva. La décima revisión ajusta también la validación del timezone del registro de la Fase 3 y refuerza los tests de R0 de la Fase 6. Login/logout/reset/verificación y gestión de usuarios conservan sus cuerpos; sus regresiones siguen en verde. Sin dependencias nuevas, migraciones ni cambios a plan.md o al contrato; esta es la única entrada modificada en tasks.md.

  **Correcciones de la décima revisión** (tras `git pull` de `102fb27`, en los siete pasos aprobados): R0 recorre recursivamente `internal` y evalúa cada query nombrada de los `store/*.sql` (incluidas subqueries y locks de JOIN); su excepción precisa ya explicaba el Green inicial. Registro y PATCH comparten `validTimezone`. DD-43: null no nullable → 400, opcionales recortados vacíos → NULL, no-op sin UPDATE/auditoría/timestamp, fields en orden del modelo. DD-42: rollback seguro separado de COMMIT incierto; relectura con LockTenant, WithoutCancel y 5 s; committed/rolled_back/unknown, logs logo_commit_uncertain/logo_delete_failed/logo_object_missing; GetLogo relee una vez y usa ErrLogoNotFound. Tests E deterministas de dos PATCH, tres subidas, subida seguida de baja y COMMIT en vuelo, con bloqueos directos/transitivos y sin sleep. Limpieza con contexto cancelado e idempotencia sin escritura comprobadas. HTTP: epílogo acotado antes de Put, S3 caído → 503, corte de descarga → INFO client_canceled/499, otros fallos → WARN logo_stream_failed. Healthcheck exec de MinIO crea el bucket con mc mb --ignore-existing; mc verificado en la imagen fijada. README y .env.example alineados.

  **Checkpoint de las correcciones**: make check completo en verde después de cada uno de los siete pasos (lint, generación, -race, PostgreSQL 18, queryrules, reporules); 15 mutaciones adicionales detectadas y restauradas, incluidos cada LockTenant por separado, compensar siempre, contexto del request en Delete y omitir el epílogo. La selección de fields reales en un PATCH mixto ya pasaba en `102fb27`, comprobado contra ese código; se protege con una mutación que audita también el CUIT sin cambios. Prueba independiente repetida desde down -v/up --wait, sin crear el bucket a mano: los 14 pasos anteriores pasan; PATCH sin cambios → 200, Tenant/updated_at iguales, auditoría 7 → 7; tax_id vacío → 200, CUIT NULL también al releer como operador. Binario detenido y down -v completado. El primer CI de las correcciones detectó agotamiento del pool de cuatro conexiones en el test de tres subidas: T_admin y las tres transacciones ocupaban todo el pool y la observación no podía consultar pg_blocking_pids. Se reprodujo con cuatro conexiones y se corrigió el helper para observar desde una conexión crm_app independiente, sin cambiar timeouts ni agregar sleep; la misma configuración quedó verde y las tres mutaciones de locks se repitieron. Se conservan los desvíos locales documentados arriba; no hay dependencias, migraciones ni cambios de diseño.

  **Ajustes finales de la revisión corta** (base `7007a65`): M-B restaura los códigos por campo de `c4650c9`: timezone largo/NUL/UTF-8 inválido → invalid_timezone; email y tax_id largo/NUL/UTF-8 inválido → invalid_format. La comparación completa detectó además name de más de 120 espacios → required; legal_name, address, phone y los otros rechazos de name conservan invalid_value. Se mantienen los cambios de DD-43 (recorte, opcionales vacíos a NULL, límites sobre el texto recibido, null no nullable → 400). Red medido: 10 filas de servicio y 7 HTTP fallaron con invalid_value; Green: las 27 filas y las regresiones de DD-43 en verde con -race. M-A verifica al entrar que el pool alcance para T_admin más las llamadas en espera; T-B601 observa pg_blocking_pids con una conexión crm_app dedicada y el mismo aviso. Los helpers se mantienen paralelos, sin introducir una abstracción para sus resultados diferentes. El test de COMMIT en vuelo espera el PID con select contra ctx.Done y el resultado prematuro del servicio, con liberación del lock preparada antes de la espera. Los siete TestConcurrent* de tenant e identity pasaron -count=20 -race tanto con el pool por defecto (8) como con pool_max_conns=4 temporal solo en los pools de pgtest; override restaurado. Una cuarta llamada temporal con pool 4 falló al entrar: "app pool has 4 connections: insufficient for T_admin plus 4 waiting calls (need 5)"; el test completo tardó 2,57 s incluido el arranque de PostgreSQL, sin esperar los 20 s; mutación restaurada. Checkpoint make check completo en verde; sin cambios de diseño ni dependencias. No se repite la prueba independiente por indicación del usuario.

- **Fase 8** (2026-10-07): T-B801 a T-B805 implementadas en `feat/001-backend-phase-8`, desde `main` actualizado en `607b5cb`; PR [fdelillo/crm#16](https://github.com/fdelillo/crm/pull/16), abierto como borrador inmediatamente después del primer push. Modo completo, sin pendientes de implementación ni dependencias nuevas.

  | Tarea | Resultado |
  |---|---|
  | Fixture común | `internal/testsupport/fixture/e2e`: A/B con admin, operador, invitado, desactivado con contraseña y sin contraseña; cookies reales de todos, datos completos, logos PNG distintos y tokens vigentes de reset/verificación/invitación. Usa Register, Invite, AcceptInvitation, Deactivate, Update, SetLogo, CreateSession y los servicios de tokens; ningún INSERT manual. El subpaquete evita el ciclo que produciría importar los servicios desde el fixture de BD ya usado por tests internos de identity/outbox. Las cookies de los desactivados se crean con CreateSession después de Deactivate y se comprueba como su empresa que la sesión esté sin revocar y el usuario disabled (DD-41, INV-11). El storage es un falso en memoria del puerto ObjectStorage; las claves, escrituras de BD y streaming usan el servicio real |
  | T-B801 | Tabla explícita de 21 operaciones → casos; `chi.Walk` sobre `app.BuildAPIRouter`, el mismo router que usa `crm serve`. Comparación bidireccional con métodos/paths de OpenAPI v0.4.3: 21 operaciones de API y 21 del contrato, más 2 ops del mux raíz excluidas. Una ruta sin fila, una fila sin ruta, una operación sin implementar o una ruta fuera del contrato fallan nombrando método/ruta. Centinelas de la empresa ajena definidos por fila (B para A; A para los flujos públicos de B), buscados en cuerpo y nombres/valores de todas las cabeceras, incluso 202/204/304 y 404: todos los ids/emails/nombres, datos completos, timestamps, clave/ETag/uuid desnudo del logo, tokens crudos, cookies y sufijo expuesto del fixture. Solo se exceptúa el código de plantilla en el catálogo. El helper tiene seis casos propios y prueba la excepción acotada. Lecturas como admin y cuatro GET del operador derivados de T-B802; 404 con cuerpo y todas las cabeceras idénticos en los cinco estados; escrituras solo en A; tokens públicos de B con cookie A; ETag de B contra logo de A devuelve 200 con bytes/ETag de A para ambos roles. Snapshots completos de las seis tablas y de los objetos de storage, leídos como la empresa correspondiente con InTenantTx, incluida toda la auditoría |
  | T-B802 | Segunda tabla explícita: 21 endpoints × anónimo/operador/admin = 63 casos, estados literales. 403 con ids ajenos/inexistentes y payload malformado, antes de buscar/decodificar; recorrido ordenado; todas las rutas protegidas rechazan cookies de invitados y ambos desactivados de A y B, antes de ejecutar una operación que pueda reactivarlos |
  | T-B803 | Login de B persiste tenant_id B y su cookie devuelve B en /me; reset cambia solo el usuario de B y conserva todas las sesiones de A; reset del invitado reemite/revoca invitación y audita en B; ResolveSession devuelve el Principal de B. El worker usa el runner/Dispatcher reales: inmediatamente antes de cada UPDATE, la misma conexión consulta current_user, current_setting('role') y app.current_tenant_id(); compara rol, binding, parámetro y dueño persistido del mensaje. Lista cerrada por el nombre de query de sqlc: MarkSent, MarkRecoverable, MarkFailed y DeferMessage; regexp compiladas fuera de Exec. Se observan once marcados y un aplazamiento de B, una escritura por mensaje. El primer AsTenant(B) falla solo en el wrapper del test; DeferMessage exige current_user y role crm_worker, current_tenant_id nulo y transacción sin empresa. Como B se comprueba pending, attempts/last_error intactos y next_attempt_at adelantado, sin marcado en esa corrida. Reloj del dispatcher congelado después de crear el fixture para que ningún aplazado vuelva a vencer durante la prueba. El login comprueba EXISTS de su sesión sin revocar, usuario y rol/setting/tenant real de B |
  | T-B804 | Query de test con la misma proyección/JOIN/orden de ListManagedUsers pero sin WHERE tenant_id: solo los cinco usuarios de A. Se comprueba que la query no derive con respecto al SQL real. Query real con parámetro A: cinco; parámetro B bajo InTenantTx(A): cero |
  | T-B805 | No se encontró una falla de aislamiento del backend. Las fallas iniciales fueron del harness: email de fixture sin normalizar, multipart sin parte file y detección de UPDATE que no contemplaba el comentario inicial de sqlc; corregidas antes de Green. No se modificó lógica de negocio de fases anteriores |

  **Router compartido**: `internal/app/api.go`, `BuildAPIRouter(users, companies, clock, logger)`, contiene las mismas siete llamadas de registro, en el mismo orden, y los mismos cinco limitadores con sus ráfagas, reposiciones y tiempos de limpieza. `serve.go` solo reemplaza ese bloque por la llamada al composition root; mux raíz, cadena común, servicios y worker conservan su cableado. `TestServeUsesSharedAPIRouter` recorre todos los .go no-test de cmd/crm, resuelve las declaraciones locales con objetos de go/ast y los imports con ImportSpec: builder una vez hacia RootDeps.API, ausencia de chi/registradores net/http, root con único uso hacia NewServer sin wrappers ni asignaciones a Handler. Verificado con la suite existente y con los 14 pasos independientes de Fase 7 mediante el binario real: registro/cookie; PNG; CUIT normalizado; invitación/correo en Mailpit; aceptación del operador; datos; logo; PATCH del operador 403; ETag vigente 304 vacío; reemplazo; ETag viejo con bytes/ETag nuevos; 2 097 153 bytes 413; DELETE 204; GET después de borrar con ETag viejo 404. Caché/nosniff comprobados y bucket creado por el healthcheck de compose, sin intervención manual.

  **Red mediante mutaciones mínimas** (todas restauradas):

  | Mutación | Falla observada |
  |---|---|
  | Ruta GET /api/v1/isolation-probe registrada sin fila | TestIsolationRouteCoverage: missing isolation case y implemented route outside contract, ambos con GET y ruta |
  | Quitar WHERE tenant_id de ListManagedUsers (SQL real y generado) | La fila HTTP GET /users **sigue verde**: RLS mantiene solo A. TestIsolationDefenseInDepth falla: parámetro B bajo rol A devuelve 5, quería 0; TestRepositoryQueriesFilterByCompany también falla por el filtro ausente |
  | 404 distinto para los ids ajenos | TestIsolationHTTPMatrix/PUT /users/{userId}/role detecta diferencia byte a byte en los cinco estados. Para distinguir los controles durante la mutación se usaron los ids reales UUIDv7 frente al id inexistente UUIDv4 |
  | Quitar RequirePermission del grupo de usuarios | TestIsolationPermissionMatrix/GET /users/operator: 200 en lugar de 403 |
  | GetLogo devuelve 304 ante cualquier If-None-Match no vacío | TestIsolationHTTPMatrix/GET /tenant/logo: ETag de B devuelve 304 en lugar de 200 con bytes de A |
  | Verificación pública usa empresa de cookie A en vez del token B | TestIsolationHTTPMatrix/POST /auth/email-verification/confirm: 400 token_invalid en lugar de 204; la RLS impide leer el token B bajo A |
  | Worker vuelve a crm_worker antes de marcar | TestIsolationWorkerSessionRoles observa current_user/role=crm_worker y current_tenant_id nulo, en lugar del rol de la empresa; falla aunque el binding Go conserve la empresa |

  **Checkpoint**: `make check` local completo en verde: lint (0 issues), sqlc diff, tests con -race e integración PostgreSQL 18, queryrules y reporules. Suite Isolation con -race y pool de 4 en verde: `Isolation: covered routes=21/21; cross-company accesses=0; verifications=18240`. Prueba independiente `go test -tags=integration -run Isolation ./...` en verde con el pool por defecto; corrida con -v para el reporte visible. La prueba independiente de Fase 7 pasó 14/14. El primer CI del PR, `7f47693`, aprobó make check en [GitHub Actions](https://github.com/fdelillo/crm/actions/runs/37561133231). Los resultados del SHA final quedan en los [checks del PR](https://github.com/fdelillo/crm/pull/16/checks); el borrador se retira solo después de verificar ese CI.

  **Conexiones y desvíos locales**: fixture, requests y snapshots son secuenciales y usan como máximo una conexión del pool a la vez. El observador del worker usa su misma transacción: necesita 1 de las 4 conexiones del runner CI, sin conexión de observación adicional. Toda la suite `Isolation` se corrió con `-race` y `pool_max_conns=4` temporal solo en los DSN de pgxpool de pgtest; el override se restauró. El primer intento aplicó el parámetro también al DSN de migración database/sql y falló antes de los tests con parámetro desconocido; se corrigió el ensayo, sin cambiar pgtest en el PR. Prueba independiente de Fase 7: proyecto compose crm-phase8; puertos fuera del repo PostgreSQL 15432, SMTP 11025, Mailpit 18025, S3 19000/consola 19001 y API 18080; modo backend HTTP con APP_BASE_URL HTTPS y cookies explícitas según DD-24. down -v antes/después, sin tocar el contenedor PostgreSQL ajeno. Ryuk deshabilitado solo local por el entorno Docker Desktop; TestMain retira los contenedores de test. Cachés Go/lint bajo /tmp. Los 404 se comparan con un único contexto real de RequestID para conservar también instance sin normalizar cuerpos; el resto usa el harness HTTPS apitest y la cadena común completa. `go test` sin -v oculta los logs de tests exitosos: se ejecutó también con -v para dejar visible 21/21 y 0 accesos cruzados.

  **Alcance sobre fases anteriores**: extracción mecánica del registro de API de serve a app; extensión del validador existente de contrato para exponer sus operaciones; nuevo subpaquete de fixture y tests en el soporte de aislamiento existente. Login, logout, reset, verificación, invitaciones, permisos, queries de negocio, RLS, migraciones y almacenamiento S3 no cambiaron. Ningún cambio a plan.md, data-model.md ni OpenAPI; en tasks.md solo se agrega esta entrada de Fase 8 del estado de implementación.

  **Correcciones del PR #16 — undécima revisión** (2026-10-08, sobre `1b961ce`, documentación aprobada el 2026-10-07): aplicadas en los siete pasos y con `make check` completo en verde después de cada uno. Esta revisión solo cambia tests, soporte del fixture y esta entrada; producción, plan.md, data-model.md, contrato y dependencias permanecen intactos.

  | Paso | Cambio y verificación |
  |---|---|
  | 1 | Sesiones de ambos desactivados posteriores a Deactivate, precondiciones persistidas verificadas y comentario de New corregido; T-B802 cubre inactivos de ambas empresas y ordena las claves |
  | 2 | Centinelas ampliados en todas las respuestas/cabeceras; test independiente del helper; comparación completa de cabeceras 404. Comentario de duplicate_signup documenta sus cinco solicitudes frente a la ráfaga de cinco |
  | 3 | Lecturas del operador derivadas de los GET exitosos de T-B802, conjunto no vacío; logo con ETag ajeno devuelve bytes y ETag propios |
  | 4 | Red observado: el observador viejo rechazó DeferMessage por falta de predicado de empresa. Green: once marcados bajo sus roles y un aplazamiento como crm_worker, sin hooks nuevos. EXISTS de la sesión bajo rol B reemplaza la comparación tautológica |
  | 5 | Las tres reglas del router sobre todos los archivos de producción de cmd/crm. Las cinco mutaciones pasaban con el guard viejo y fallan con el nuevo, cada una con archivo y línea; renombrar api/root y el alias del import de app sigue verde |
  | 6 | Contadores reales de detecciones y verificaciones de centinelas, snapshots, 404, sesiones y logos; t.Cleanup imprime siempre, incluso después de Fatal. Fallan tanto una detección como cero verificaciones. El caso de ids de usuarios exige {userId}: otros recursos necesitan su caso propio (H3) |
  | 7 | Checkpoint completo e independiente Isolation: 21/21 rutas, 21 operaciones del contrato, 0 accesos cruzados y 18 240 verificaciones ejecutadas. Repetido con -race y pool_max_conns=4 temporal; DSN restaurado |

  | Mutación de la revisión (restaurada) | Falla observada |
  |---|---|
  | ResolveSession: status != active → status == invited | GET /me devuelve 200 en vez de 401 para ambos desactivados de A y B; también falla TestTenantLockRegressionR5 |
  | Cabecera X-Foreign-ID solo para UUIDv7 | PUT /users/{userId}/role: las cinco cabeceras 404 difieren; el contador imprime 5 detecciones |
  | Ningún GET exitoso para el operador en T-B802 | Falla por conjunto derivado vacío |
  | GetLogo acepta cualquier If-None-Match, fila del operador | 304 en vez de 200; imprime 1 detección aun al terminar con Fatal |
  | Alias r := api y r.Get | serve.go: uso adicional del objeto declarado, fuera de RootDeps.API |
  | Función local register(api) | serve.go: uso adicional del objeto del router |
  | API: wrap(app.BuildAPIRouter(...)) | serve.go: el resultado no llega directamente al campo API |
  | chi.NewRouter en otro archivo | phase8_router_probe.go: import chi prohibido, con línea del import |
  | app.NewServer(cfg, wrap(root)) | serve.go: el root no llega directamente al argumento de NewServer |
  | Una detección forzada, sin Error en la mutación | Reporte 1/21, 1 acceso cruzado, 414 verificaciones; falla exclusivamente el guard final que exige cero |
  | Omitir todos los casos de admin y operador | Reporte 0/21, 0 accesos y 0 verificaciones; falla por no ejecutar verificaciones |

  **Desvíos y hallazgos de esta revisión**: ninguno requiere cambiar producción y no se encontró una falla de aislamiento. El primer paso pidió un switch por lint y corrigió el orden del harness, que verificaba la cookie de un usuario después de reactivarlo. SA1019 se suprime solo en la declaración del mapa de objetos go/ast, mecanismo expresamente permitido por la spec; no se agregó dependencia. Tests de Fase 8, fixture y observación siguen usando como máximo una conexión simultánea; la observación no adquiere otra. Se mantienen las cachés de /tmp y Ryuk deshabilitado únicamente en local. El comando independiente sin -v pasa; se repite con -v para que Go muestre los conteos de los tests exitosos. Push y CI final verificables en los checks del PR #16.

- **Décima revisión (2026-10-06, *Accepted*: aprobada por el usuario)**: revisión del PR fdelillo/crm#15 (Fase 7, HEAD `c4650c9`; plan §18, décima tanda). Se aplica en `feat/001-backend-phase-7`, en este orden y con `make check` en verde después de cada paso. Los casos marcados "Décima revisión" se escriben primero. Cada uno dice si es Red (falla hoy) o si caracteriza código que ya está bien y se protege con una **mutación**: se aplica, el caso tiene que fallar, y se restaura (el resultado se reporta como en la Fase 6).

  | Orden | Tarea | Ajuste |
  |---|---|---|
  | 1 | T-B604 (Fase 6, regla R0) | R0 (a) admite `LockTenant` por nombre, además de la query de INV-10, las dos en `FOR NO KEY UPDATE`. Si R0 (a) pasa hoy con `LockTenant` en `internal/tenant/store`, primero averiguar por qué (debería recorrer `internal/*/store/*.sql`) |
  | 2 | T-B303 (Fase 3) | `timezone: "Local"` en el registro → default y `signup_timezone_defaulted` (Red: hoy se guarda `Local`). Usa la misma función de validación que el `PATCH` (DD-43) |
  | 3 | T-B701, T-B702, T-B705 (`PATCH`) | DD-43: `Local` y `""` (T-B701); texto vacío → `NULL`, `PATCH` sin cambios y `fields` solo con cambios reales (Red); `null` en `name`/`timezone` → `400` (Red); dos `PATCH` concurrentes (mutación: quitar `LockTenant` de `Update`) |
  | 4 | T-B703, T-B704 (logo, servicio) | DD-42: `COMMIT` confirmado con error, `COMMIT` que no ocurrió, relectura que falla y `COMMIT` en vuelo (Red); `GetLogo` con el objeto ausente, con un reemplazo entre la lectura y el `Get`, y `ErrLogoNotFound` en lugar de `db.ErrNotFound` (Red); subidas concurrentes y subida con baja (mutación: quitar `LockTenant` de `SetLogo` y de `RemoveLogo`); limpieza con el contexto cancelado (mutación: el contexto del request en el `Delete`); quitar el logo cuando no hay, sin auditoría (Red si hoy audita) |
  | 5 | T-B705, T-B706 (HTTP) | Epílogo de más de `LogoMaxBodyBytes` (mutación: quitar la lectura del resto del cuerpo); `GET /tenant/logo` con el storage caído → `503` (caracteriza); corte del cliente durante la descarga → `INFO` y `499` (Red: hoy `WARN`) |
  | 6 | T-B707 (nueva) | Bucket de desarrollo con el *healthcheck* de `minio` (plan §10.5.1) |
  | 7 | Checkpoint de la Fase 7 | `make check` en verde y la prueba independiente con `docker compose down -v && docker compose up -d --wait`, sin crear el bucket a mano |

- **Undécima revisión (2026-10-07, *Accepted*)**: revisión del PR fdelillo/crm#16 (Fase 8, HEAD `ccaf265`; plan §18, undécima tanda). Sin bloqueantes; cierra huecos de lo que T-B801 a T-B803 exigen verificar. Se aplica en `feat/001-backend-phase-8`, en este orden y con `make check` en verde después de cada paso. Los casos marcados "Revisión del PR #16" se escriben primero; cada uno dice si es Red o si caracteriza código correcto y se protege con una **mutación** (se aplica, el caso falla, se restaura).

  | Orden | Tarea | Ajuste |
  |---|---|---|
  | 1 | Fixture común, T-B802 | Cookies de los desactivados creadas después de `Deactivate`, con la precondición `revoked_at` nulo verificada en el fixture; fila nueva de T-B802 (caracteriza; mutación `row.Status == "invited"`). Se corrige el comentario de `e2e.New`, que hoy afirma una fila de sesión válida que para los desactivados no existe |
  | 2 | T-B801 | Centinelas ampliados en cuerpo y cabeceras de toda respuesta, con el test del helper; cabeceras en la comparación de los `404` (mutación: una cabecera solo cuando el id del path es UUIDv7, como en la mutación del `404` de esta fase) |
  | 3 | T-B801 | Fila de lecturas del operador derivada de T-B802 (caracteriza; el test falla si el conjunto derivado sale vacío) |
  | 4 | T-B803 | Observador con lista cerrada de `UPDATE` y `DeferMessage` como `crm_worker`, con el aplazamiento inyectado en el *wrapper* (Red: hoy el observador lo marca como error) |
  | 5 | T-B801 (`TestServeUsesSharedAPIRouter`) | Las tres reglas sobre `cmd/crm` (Red: hoy un alias del router, o la variable con otro nombre que `api`, pasan) |
  | 6 | T-B801, checkpoint | Contador real de accesos cruzados y de verificaciones (mutación: forzar una detección → el reporte imprime 1 y el test falla) |
  | 7 | Checkpoint de la Fase 8 | `make check` en verde y la suite `Isolation` con el reporte de los conteos |

- **Duodécima revisión (2026-10-08, *Accepted*: aprobada por el usuario el 2026-10-08)**: pendientes de la revisión de cierre del PR fdelillo/crm#16 (Fase 8, mergeado en `main` en `a06b8b8`; plan §18, duodécima tanda), llevados por el usuario al **paso 0 de la Fase 9**: se aplican en la rama de la Fase 9 antes de T-B901, en este orden y con `make check` en verde después de cada paso. Un hueco de diseño (el guard del router de `crm serve` se esquivaba de dos formas, y la Fase 9 agrega un segundo servidor HTTP) y dos precisiones de T-B801. El paso 3 adelanta de T-B903/T-B904 solo el armado del servidor de métricas, para que el guard nuevo cubra los dos servidores desde el principio. Los cierres de la misma revisión para la Fase 9 (versión de esquema de `/readyz` con la migración `00009`, métricas que leen la base muestreadas cada 60 s, inventario de roles) **no son pasos del paso 0**: son filas Red de T-B903 y trabajo de T-B904, en el orden normal de la fase, después de T-B901/T-B902 (las tareas de muestreo usan el registro de `PeriodicTask` de T-B902). Los casos marcados "Duodécima revisión" se escriben primero; cada uno dice si es Red o si caracteriza código correcto y se protege con una **mutación** (se aplica, el caso falla, se restaura). La entrada de estado de esta revisión corrige además lo que la de la Fase 8 dice del `nolint`: la spec no "permitía expresamente" `go/ast`, pedía identificadores "resueltos por su declaración"; con `go/types` el `nolint` desaparece.

  | Orden | Tarea | Ajuste | Red / mutación |
  |---|---|---|---|
  | 1 | T-B801 (helper de centinelas) | Centinelas de menos de 8 bytes buscados como palabra completa (delimitadores fuera de `[A-Za-z0-9_-]`); sufijo del fixture de 8 bytes o más, comprobado por el test del helper | Red: el test del helper con `ARS` dentro de un token base64url de `Set-Cookie` hoy lo detecta (falso acceso cruzado). Mutaciones: volver a `bytes.Contains` para los cortos → falla esa fila; quitar el delimitador derecho → falla la fila de `ARS` pegado a letras |
  | 2 | T-B801 (conteo) | Reporte con `sentinel_checks` y `state_checks` separados; el test falla si alguno es 0 | Red: hoy el reporte imprime un solo total. Mutaciones: contar toda verificación como de centinela → `state_checks=0`, falla; no contar las búsquedas de centinelas → `sentinel_checks=0`, falla |
  | 3 | T-B903/T-B904 (solo el servidor de métricas) | `app.NewMetricsServer(cfg)`; `app.NewServer` con `root` nil → error; `serve.go` escucha `HTTP_ADDR` y `METRICS_ADDR` antes de arrancar el worker y llama a `app.Serve` una vez por servidor | Red: `NewMetricsServer` no existe; `NewServer(cfg, nil)` hoy devuelve un servidor. Mutaciones: **el handler de métricas monta un router de la API en `/api/`** (o registra `/healthz`) → falla la fila de rutas de T-B903; `Handler: nil` → falla la sonda de `http.DefaultServeMux`; `NewRootHandler` registra `/debug/vars` → falla la fila del mux raíz |
  | 4 | T-B801 (`TestServeUsesSharedAPIRouter`) | Reescrito con `go/types` y las reglas (1) a (4); sin `ast.Object` ni `nolint`; si el chequeo de tipos falla, el test falla | Red: los dos esquives de la revisión de cierre pasan hoy y tienen que fallar nombrando archivo, línea y regla: (a) `deps := app.RootDeps{API: app.BuildAPIRouter(...), …}` y `deps.API.(interface{ Get(string, http.HandlerFunc) }).Get("/api/v1/backdoor", h)` antes de `app.NewRootHandler(deps, …)`; (b) `_ = srv` y `app.Serve(ctx, &http.Server{Handler: otro}, ln, logger)`. Mutaciones: (a') `deps.Liveness = deps.API`, sin importar `net/http` → regla (1); (c) listeners intercambiados entre las dos llamadas a `app.Serve` → regla (4); (d) `metricsSrv.Handler = …` → reglas (3) y (4); las cinco mutaciones de la undécima revisión siguen fallando. Siguen en verde: renombrar variables, cambiar el alias del import de `app` y mover el cableado a otro archivo de `cmd/crm` |
  | 5 | Checkpoint del paso 0 | `make check` en verde; suite `Isolation` con `covered routes=21/21`, `cross-company accesses=0` y `sentinel_checks` y `state_checks` mayores que 0; `crm serve` real: `GET /debug/vars` en `METRICS_ADDR` → `200` JSON, `GET /api/v1/me` en `METRICS_ADDR` → `404`, `GET /debug/vars` en `HTTP_ADDR` → respuesta sin `memstats` | — |

- **Fase 9** (2026-10-08, detenida por incumplimiento de T-B905; checkpoint no aprobado): trabajo sobre `feat/001-backend-phase-9` desde `3365e98`, sin rebase ni force-push. Paso 0.1 completo: centinelas menores de 8 bytes con ambos delimitadores fuera de `[A-Za-z0-9_-]`; sufijo real del fixture comprobado ≥ 8 bytes. Red: cinco falsos positivos (`ARS` en cookie, pegado a letras, `_` o `-`). Green y mutaciones restauradas: volver a `bytes.Contains` falla en esos cinco casos; quitar el delimitador derecho falla en `ARSx` y `ARS_`. `make check` completo en verde (127,115 s; lint 0 issues, generación, `-race`, integración PostgreSQL 18, queryrules y reporules).

  Paso 0.2 completo: reporte separado `sentinel_checks`/`state_checks`, ambos exigidos mayores que cero. Red: no existía el reporte tipado. Mutaciones restauradas: contar todo como centinela → `state_checks=0`; omitir centinelas → `sentinel_checks=0`; ambas hacen fallar el caso del reporte. `make check` completo en verde (89,830 s).

  Paso 0.3 completo: `NewMetricsServer` con mux privado (solo `GET /debug/vars`), timeouts y sin TLS; `NewServer` rechaza nil. Dos listeners/servidores, ambos ligados al contexto común antes de iniciar worker; una falla cancela al otro. Red: factory ausente y `METRICS_ADDR` ocupado aceptado. Mutaciones API en métricas, `/healthz`, handler nil y vars en mux raíz fallaron; restauradas. Tests del comando en verde; `make check` completo en verde (98,758 s).

  Paso 0.4 completo: guard con `go/types` (`Defs`, `Uses`, `Selections`, `Implicits`), sin `ast.Object` ni `nolint`, sobre todos los archivos de producción. Comprueba las cuatro reglas, variables de un solo uso, `RootDeps` (incluidos resultados sin nombre), fábricas de servidores y listeners por campo de `config.Config`. Un error de tipos detiene el análisis. Los dos esquives pasaron con el guard anterior; ambos fallan con el nuevo. Mutaciones ejecutadas sobre copias de las fuentes, con el mismo parser/chequeo de tipos/guard: alias de router y función local → regla 1; API envuelta → 1/2; chi en otro archivo → 2; root envuelto → 2/3; deps retenido con HTTP → 1/2, sin HTTP → 1; servidor ajeno → 2/3/4; listeners intercambiados → 4; campo Handler de métricas → 3/4. Cada fallo nombra archivo/línea/regla. En verde: renombrado, alias de import y cableado en otro archivo; error de tipos rechazado.

  **Supuesto del importer**: `source` resuelve correctamente bajo `-race`. Dentro de `make check` (232,044 s, en verde): guard 30,679 s unitario / 33,137 s integración; suite de mutaciones 46,57 s / 79,24 s. Se usa el respaldo autorizado `gc` con lookup de archivos de export de `go list -export -deps -json`, solo stdlib, por ese costo. Las reglas no cambian. Con `gc`: guard 154 ms, suite de mutaciones 1,15 s, `make check` completo 16,913 s (cachés reutilizadas), en verde.

  **Checkpoint del paso 0 completo**: `make check` en verde después de restaurar el pool temporal (12,123 s). `Isolation` con `-race`, `pool_max_conns=4` solo local y sin aplicarlo al DSN de migración: `covered routes=21/21; cross-company accesses=0; sentinel_checks=17841; state_checks=399`. Binario real y proyecto Compose `crm-phase9`: métricas `/debug/vars` → 200 JSON; métricas `/api/v1/me` → 404; HTTP `/debug/vars` → SPA 503, sin memstats/cmdline; SIGTERM → código 0. Puertos PostgreSQL 15439, SMTP 11029/Mailpit 18029, S3 19009/consola 19019, HTTP 18089 y métricas 19099; modo backend HTTP con base URL HTTPS (DD-24).

  **T-B901/T-B902 completas**: limpieza horaria por dueño de tabla como `crm_worker`, con las cuatro queries por nombre en queryrules, transacción y políticas RLS que protegen filas activas e invitaciones abiertas. Dos empresas, fechas de 40 días, revocación/reinvitación/aceptación/desactivación, throttles de 24 horas y estado HTTP de invitados verificados. Tareas con reloj inyectado, envío entre períodos, repetición después de error y rollback ante cancelación (INFO). Mutaciones restauradas: contexto de empresa → falla la observación de rol y permisos; política que permite borrar invitaciones abiertas → fallan la sonda RLS y la fecha DD-25; query de tokens sin filtro adicional → sigue verde, RLS conserva la invitación; ejecución anticipada → falla el conteo por período; cancelación en ERROR → falla el log. `make check` completo en verde; se corrigió la lista esperada de excepciones de queryrules, sin cambiar sus reglas.

  **T-B903/T-B904 completas**: `00009` concede solo USAGE de public y SELECT de version_id a crm_auth; SchemaVersion usa max como goose y ExpectedVersion valida los nombres embebidos. Readiness con plazo total de 1 s (pool incluido), no-store, estados JSON sin versiones, reasons unavailable/timeout/schema_mismatch/schema_unknown/privilege; INV-19 y cancelación durante la transacción → INFO/499. Snapshot T-B109 ampliado a public; no nueva función SECURITY DEFINER. PostgreSQL 18: public pertenece a pg_database_owner; Up/Down/Up reales como crm_owner pasan y Down revoca ambos permisos. Se corrigió el orden de cleanup del test (cerraba su conexión antes de restaurar).

  Métricas completas de §12.1 publicadas por sus productores. Stats e inventario cada 60 s como crm_worker, snapshots atómicos, cinco gauges -1 sin muestra o tras fallo; edad calculada al scrape con reloj; ninguna consulta durante scrape. Rol eliminado y restablecido modifica missing/total, tenants_total cuenta filas; queries por nombre con las excepciones previstas. Contadores de entrega/deferral solo después de commit; rollback no cuenta. Mutaciones restauradas: `<` → falla base más nueva; sin deadline → falla lock y espera del pool (4 s con límite externo del test); 500 ante privilege → falla REVOKE; query durante scrape → falla sonda de llamadas y gauges iniciales; omitir publicación → fallan cinco causas; contar rollback → falla contador. `make check` completo verde (154,976 s). El primer arranque local del contenedor del snapshot falló por puerto desaparecido; repetición en verde.

  **T-B906/T-B907 completas**: comando real con reporte JSON y error si hay empresas fallidas; listado como worker, provision como signup, una transacción confirmada por empresa. Lock de sesión no bloqueante, clave constante en tenant y conexión dedicada en db; nueva interfaz `SessionLocker.WithSessionLock` agregada únicamente a plan §11.1 (autorización de T-B907), sin cambiar TxRunner. Libera ante error, cancelación y panic. Falla intermedia de tres empresas: reporte 3/1, las otras dos confirmadas; idempotencia; segunda corrida rechazada; registro exitoso mientras se recorren 50 empresas. Pruebas en paquete aislado, sondas dedicadas, pico runtime ≤2 conexiones. Requests de A sin rol 500 y log con rol; B 200; acceso restaurado. Cambio en código anterior: el switch de rol inexistente (22023) ahora registra el rol y SQLSTATE sin datos de la query. Mutaciones restauradas: transacción única → empresa anterior invisible y 1 transacción en lugar de 3; sin advisory lock → segunda corrida termina sin error. Comando y liberación en verde; eliminado el test del placeholder. `make check` completo verde (151,487 s).

  **T-B908/T-B909 completas**: presupuesto compartido `app.ShutdownTimeout = outbox.SendBudget + 5 s` (25 s), para HTTP y espera del worker; cierre de conexiones si Shutdown agota el plazo. Red: presupuesto previo 15 s; luego request cancelado internamente generaba ERROR al devolver 503. Ahora cancelación interna conserva 503 con log INFO; worker cancelado no registra ERROR por cancelación. Tests sin sleep, con barreras de request/envío/Shutdown, métricas cerrada antes de liberar un envío que todavía puede terminar con éxito. Mensaje canceled conserva status/attempts/next_attempt_at/last_error/payload; nil del envío después de la señal confirma sent. Mutaciones restauradas: quitar los 5 s → falla presupuesto; log interno ERROR → falla ausencia de ERROR; marcar intento al cancelar → falla snapshot pending. Regresiones previas del worker verdes. Se reemplazó la espera temporal del test de servidor anterior por RegisterOnShutdown. No cambia el arranque ni los pares Serve/listener: las cuatro reglas del guard siguen vigentes y sus mutaciones pasan. `make check` completo verde (116,313 s).

  **Perfil local de cuatro conexiones**: Fase 9 completa y Isolation con `-race` y pool_max_conns=4 en todos los pools del harness, sin ese parámetro en ownerURL de migración; paquetes serializados localmente (`-p 1`), CI sin cambios. En verde (233,431 s): 21/21, 0, sentinel_checks=17841 y state_checks=399. Perfil restaurado; `make check` verde (17,330 s, cachés). Las sondas/bloqueos usan pgx.Connect dedicadas: readyz runtime 1 + blocker dedicado; pool exhaustivo 4 + sin blocker activo; reprovisión runtime máximo 2 + lock/supervisor dedicados; apagado runtime 1 + sonda dedicada. El test de rollback periódico usa una fila terminal de outbox, para no nombrar tablas de identity desde outbox ni siquiera en ese test.

  **T-B905 ejecutado, falló; retorno al arquitecto por ADR-005**: test con build tag bench, fuera de make check. Corrida única sobre el código de `60adc85bfea57ec847fdb2dc734c0eaa75ad2c65`, duración del test 830,79 s (proceso 836,415 s). PostgreSQL exacta 18.6 (Debian 18.6-1.pgdg13+2), imagen postgres:18, digest `postgres@sha256:5a5a84b19854a9ffaa54082c166ff4ec27473a361e496e5ea167f298f2da9722`; Go 1.27.1, darwin/amd64, 8 CPU, Docker Desktop con 7937 MB, pool normal de 8 conexiones. Se verificaron 10.000 empresas antes de medir. Register real y hash precalculado para la carga y las tandas; login y signup HTTP medidos con argon2id real, router y cadena común reales, sin red HTTP. SET ROLE incluye lectura de catálogo en el mismo viaje; conexión crm_app fresca; revalidación 304 sin almacenamiento. La duración de las tandas mide el callback de transacción hasta su commit, sin adquisición/BEGIN ni hash, e incluye SetSignupLockTimeout anterior a provision: es la cota superior del intervalo provision → commit que pide T-B905, no una medida aislada de retención del lock. Pool y targets intactos.

  | Operación | Muestras | p95 (ms) | Target §13 (ms) | Resultado / 503 |
  |---|---:|---:|---:|---|
  | Transacción de registro, 10 concurrentes | 10 | 672,822 | <250 y 0 respuestas 503 | **Falla p95; 0 respuestas 503** |
  | Transacción de registro, 50 concurrentes | 50 | 730,195 | Reportar, sin target numérico | 0 respuestas 503 |
  | SET LOCAL ROLE con catálogo | 190 | 1,087 | Sin target numérico propio | Reportado |
  | Conexión fresca crm_app | 30 | 11,164 | Sin target numérico propio | Reportado |
  | GET /api/v1/me | 200 | 10,241 | <50 | Cumple |
  | GET /api/v1/tenant/logo, 304 | 200 | 8,285 | <50 | Cumple |
  | POST /api/v1/auth/login | 80 | 53,735 | <400 | Cumple |
  | POST /api/v1/auth/signup | 60 | 236,475 | <1000 | Cumple |

  `set_role_retry_total`: recovered=0, failed=0, total=0. La falla del umbral concurrente detuvo el checkpoint; no se alteró el diseño, no se relajaron targets ni se repitió la corrida buscando aprobar. El contenedor del benchmark fue retirado por el harness. Log completo: `/tmp/crm-phase9-evidence/b905-benchmark.log`.

  **Prueba independiente y checkpoint final no realizados**: se ejecutó y aprobó la prueba real de Compose del paso 0, descrita arriba. La prueba final de readyz con down/up, los dos ciclos de muestreo tras borrar/restaurar un rol y SIGTERM con request en curso no se ejecutó: T-B905 impide avanzar al paso 6. Los casos equivalentes de integración y las mutaciones están en verde, pero no sustituyen esa prueba independiente. Por tanto no hay reason de un migrate down observado en un serve real de cierre; el test de REVOKE verifica reason=privilege. El PR #17 se mantiene borrador. `docker compose -p crm-phase9 ... down -v` completado: retirados PostgreSQL, Mailpit y MinIO, sus dos volúmenes y la red de prueba.

  **Verificación al detener**: `make check` completo volvió a pasar (10,911 s, cachés): lint 0 issues, generación sin diferencias, unitarios e integración PostgreSQL 18 con race, queryrules y reporules. El benchmark queda fuera de ese target y conserva su falla; este verde no aprueba T-B905 ni el checkpoint. CI del push anterior (`ac0225b`, run 37846934604) verde, 4 min 7 s; el trabajo de apagado, benchmark y este reporte se publica en el mismo PR borrador.

  **Alcance sobre fases anteriores**: helper y contadores de Isolation (Fase 8); guard de composición y arranque de serve (dos servidores/listeners); snapshot de privilegios T-B109; runner db (log de rol ausente y publicación de retries), httpx (contador de requests y nivel de cancelación interna), Dispatcher (periodicidad, snapshots/contadores y cancelación), cierre HTTP (presupuesto y barrera del test) y excepciones por nombre de queryrules. Sin dependencias nuevas, cambios al contrato ni a data-model.md. En plan.md cambió únicamente §11.1 para SessionLocker. La afirmación incorrecta del estado de Fase 8 se corrige dentro de esta entrada autorizada, sin editar su entrada anterior.

  **Corrección del estado de Fase 8**: la afirmación anterior de que la spec permitía expresamente `go/ast` era incorrecta: pedía identificadores resueltos por su declaración. El paso 0.4 reemplazó `ast.Object` por `go/types` y eliminó el `nolint`.

  **Desvíos locales**: acceso aprobado a caché Go y Docker; Ryuk falló durante el arranque (puerto `8080/tcp` desaparecido). `TESTCONTAINERS_RYUK_DISABLED=true` solo en el verificador local; no se cambia CI. Evidencia de corridas y mutaciones guardada temporalmente en `/tmp/crm-phase9-evidence`.

  **Reanudación desde 30bbb94, decimotercera revisión aprobada**: la corrida original de T-B905 y la detención anterior se conservan como historia; T-B910 decide la salida a producción y su resultado no bloquea el checkpoint de esta fase. Se omite el paso 0 opcional; la prueba independiente final se ejecutará sobre el HEAD final.

  **Paso 1 completo — T-B801 y env.listen**: Red observado antes del arreglo: usos adicionales por valor de función (e) NewMetricsServer y (f) Serve, bien tipados, dieron violations=0; las aserciones nuevas fallaron. Se prohíbe cualquier uso de las cinco funciones que no sea Fun directo de CallExpr, desde Info.Uses. La regla (4) exige los dos listeners desde env.listen y una sola inicialización de ese campo en run, exactamente (&net.ListenConfig{}).Listen o new(net.ListenConfig).Listen. Mutaciones en copias, restauradas automáticamente: (e) → reglas 5/4, (f)/(g) → 5, (h) net.Listen y ListenConfig.Listen directo → 4, (i) reasignación y otro valor en run → 4; todas nombran archivo/línea/regla. Las anteriores (a)–(d), las cinco de la undécima revisión y los positivos renombrado/import alias/mover archivo siguen pasando. Guard real gc 190,96 ms; make check completo verde (23,338 s, lint 0 issues). Único cambio de producción de este paso: costura env.listen autorizada, sin cambiar la orquestación.

- **Decimotercera revisión (2026-10-08, *Accepted*, aprobada por el usuario el 2026-10-08)**: respuesta del arquitecto a la detención en T-B905 y a los pendientes de diseño de la revisión de código del PR fdelillo/crm#17 (plan §18, decimotercera tanda; detalle en [`revision-13-t-b905.md`](revision-13-t-b905.md)). La corrida 1 de T-B905 no se descarta ni se repite: queda como dato. Su target sumaba la espera en el lock a la retención (con P = 8, lo medido es ≈ 8 × la retención), así que no decide si ADR-005 cumple. Orden sobre `feat/001-backend-phase-9`, con `make check` en verde después de cada paso:

  | Orden | Tarea | Qué | Red / mutación |
  |---|---|---|---|
  | 0 | Prueba independiente (opcional, ya) | La de Compose descrita en la entrada de la Fase 9, como señal temprana. No reemplaza la del checkpoint | — |
  | 1 | T-B801 (reglas (4) y (5)) + costura `env.listen` | Regla (5): las cinco funciones del guard solo como llamada directa. Regla (4): listeners desde `e.listen`, con `(&net.ListenConfig{}).Listen` como único valor en `run` | Red: (e) y (f) dan hoy 0 violaciones. Las mutaciones (e) a (i) de T-B801 fallan nombrando archivo, línea y regla; las anteriores siguen fallando |
  | 2 | T-B911 | Tests de `runServe` | Red: hoy sobreviven quitar la espera de `workerDone` y quitar `stopServers()`. Se aplican las mutaciones de T-B911 |
  | 3 | T-B910 | Re-medición con desglose, dos corridas | Autocontroles del tracer; targets T-1 a T-3; reglas D-a a D-d reportadas |
  | 4 | Checkpoint de la Fase 9 | El texto nuevo, debajo de T-B911 | El veredicto de T-B910 no bloquea el cierre |

  El plan documenta en §11.1 la API exportada de la Fase 9, sin cambio de código. Si T-B910 cumple, DD-33 registra los números. Si no, el arquitecto escribe el ADR que reemplaza a ADR-005, y 001 no sale a producción hasta implementarlo.

### Convenciones de esta sección

- `[T]` = tarea de test. Va **antes** de la tarea de código que la hace pasar, y el test debe
  fallar por la razón correcta antes de implementar (Red).
- Cada tarea lleva trazabilidad: historia (`US-n` = Historia n de la spec), `FR-`, `SC-`, `INV-`
  (plan §5), `DD-` (plan §6), `ADR-`, `P-` (respuestas del usuario, plan §14.2), `H-` (hallazgos
  del frontend, plan §18).
- Tests: paquete `testing` nativo, *table-driven*, sin testify (ADR-012). Unitarios junto al
  código (`*_test.go`). Integración con build tag `integration` (`//go:build integration`) y
  PostgreSQL 18 real vía `internal/testsupport/pgtest`.
- **Qué se sustituye y qué no** (ADR-012):

  | Se sustituye (fake en memoria) | Nunca se sustituye |
  |---|---|
  | `mailer.Mailer` en tests de servicio y HTTP | PostgreSQL (RLS, roles, constraints y locks son lo que se prueba) |
  | `objectstore.ObjectStorage` en tests de servicio y HTTP | El adaptador SMTP en su propio test (contra Mailpit) |
  | `clock.Clock` (tiempo controlado) | El adaptador S3 en su propio test (contra MinIO) |
  | `industrytemplate.Seeder` solo en el test de rollback (para forzar un fallo) y como *hook* para observar la transacción de registro (T-B303) | El router, el mux raíz y los middlewares en los tests HTTP (`httptest` sobre el handler real) |
  | El handler de la SPA (`RootDeps.SPA`) en los tests del backend, por un stub que registra lo que recibe | El paquete `web` en T-F007 (se prueba con `fstest.MapFS`) |
  | El resultado del primer `SET LOCAL ROLE` de `InTenantTx`, solo con un *hook* de test no exportado de `platform/db`, para forzar el `42501` de DD-34 de forma determinista (T-B103) | El comportamiento real de PostgreSQL que origina DD-34: lo prueba el test de reproducción contra el contenedor, sin *hooks* |

- **Tests HTTP con cookie** (H-10, nota en ADR-012): los que encadenan requests con la cookie de
  sesión (registro → `/me`, login → logout → `/me`) usan `internal/testsupport/apitest`:
  `httptest.NewTLSServer` sobre el handler raíz real + `srv.Client()` con un `cookiejar`. La cookie
  viaja por HTTPS como en producción y el cliente ya confía en el certificado de prueba: **ningún
  test necesita mkcert**. Los tests de un solo request siguen con `httptest.NewRecorder`. Como el
  `cookiejar` de Go no verifica el prefijo `__Host-`, los atributos de la cookie se afirman
  explícitamente sobre `Set-Cookie` (INV-23).
- **Contrato en los tests HTTP** (séptima revisión, ADR-014 y su nota 2026-10-02): todo test HTTP
  de la API valida contra el contrato con `internal/testsupport/contract` (T-B311/T-B312). Con
  `apitest`, `contract.Default(t).Wrap(t, srv.Client)` valida cada respuesta contra su operación y,
  si es `2xx`, también el request; con `httptest.NewRecorder`, `RequireRecorded`. Las tablas de las
  tareas no repiten "valida contra el contrato" en cada fila: vale para todas.
- **IP del cliente en los tests** (DD-32): `httptest` fija `RemoteAddr` en `192.0.2.1:1234`. Los
  tests que necesitan otra IP la fijan en `req.RemoteAddr` o configuran `TRUSTED_PROXIES` con ese
  rango y mandan `X-Forwarded-For`; siempre con direcciones de documentación (`192.0.2.0/24`,
  `198.51.100.0/24`, `203.0.113.0/24`, `2001:db8::/32`).
- Los tests de integración se conectan **como `crm_app`** (INV-18). Cada test crea sus propias
  empresas con UUID y emails aleatorios: pueden correr en paralelo (`t.Parallel()`) sobre el mismo
  contenedor sin limpiar tablas. Excepción: los tests que retienen el lock de `crm_tenant` (T-B303,
  T-B305, T-B906) no corren en paralelo con otros registros (`t.Parallel()` no, o un paquete
  propio), porque frenarían a los demás a propósito.
- Las queries que comparan vencimientos reciben `now` como parámetro desde `clock.Clock` (no usan
  `now()` de SQL), para poder controlar el tiempo en los tests (DD-18).

### Comandos de validación (los crea la Fase 0)

| Comando | Qué corre |
|---|---|
| `make generate` | `sqlc generate` |
| `make lint` | `gofmt -l` vacío, `go vet ./...`, `golangci-lint run` (incluye `depguard`), `sqlc diff` (código generado al día) |
| `make test` | `go test -race ./...` (unitarios; no necesita Docker) |
| `make test-int` | `go test -race -tags=integration ./...` (necesita Docker) |
| `make check` | `lint` + `test` + `test-int`. **Es el checkpoint de cada fase** |
| `make db-reset` | Solo desarrollo: recrea la base local y corre bootstrap + migraciones |
| `make dev-certs` | Solo desarrollo y job de E2E: genera `.certs/localhost.pem` y `.certs/localhost-key.pem` con `mkcert` (T-B014). **No** lo necesita `make check` |

Los targets del frontend (`web-build`, `web-check`, `build`, `check-all`) los agrega T-F009; el
backend compila y testea sin el frontend gracias al marcador `web/dist/.gitkeep` (ADR-019).

### Coordinación con la sección Frontend

T-F007 (test del handler de la SPA) y T-F008 (paquete `web` y cableado en el mux raíz) son código
Go y los implementa `backend-developer`, pero están numeradas en la sección Frontend y **no se
duplican acá**. Dependen de:

| Tarea del frontend | Necesita del backend | Por qué |
|---|---|---|
| T-F006 (proxy de Vite y spike de la cookie) | T-B005 (TLS local en `serve`) y T-B014 (`make dev-certs`) | Con H-10 resuelto, Vite corre con `server.https` y hace *proxy* a `https://localhost:8443` (plan §10.5.1) |
| T-F007 | T-B005 (mux raíz con `RootDeps.SPA`) y T-B204 (middlewares comunes) | El test del mux completo verifica `/api/…` → problem+json, `/healthz` y las cabeceras comunes sobre la SPA (HSTS presente con configuración no local, DD-24) |
| T-F008 | T-B005, T-B204 y la aprobación de `gzhttp` (DD-29, ya aprobada) | Reemplaza el stub de `RootDeps.SPA` por `web.NewHandler(web.DistFS())` |
| T-F009 | T-B013 | Agrega targets y el job de frontend al Makefile y a la CI del backend |
| T-F701 (Playwright) | T-B014 y la receta de CI de plan §10.5.1 | El E2E corre contra `https://localhost:8443` con la CA de mkcert instalada en el runner (supuesto S-12) |
| T-F604/T-F605 (preparación del logo) | T-B706 (límites de DD-31) | `LOGO_TARGET_MAX_BYTES` puede ser el límite exacto del archivo, 2 097 152 bytes (H-11) |
| T-F004 (tipos del contrato) y T-F101 (mensajes por `code`) | Contrato **v0.4.0** (tercera revisión) | `ErrorCode` suma `method_not_allowed`: el mapa exhaustivo de T-F101 deja de compilar hasta que tenga su mensaje (a propósito). Lo actualiza el `frontend-architect` en su sección y en `ui.md` |
| T-F004 (tipos del contrato) y T-F101 (mensajes por `code`) | Contrato **v0.4.1** (séptima revisión) | Declara `413 payload_too_large` en las operaciones con body JSON. No agrega valores a `ErrorCode` ni cambia esquemas: el mapa de T-F101 compila igual y `ui.md` §12.2 ya cubre `payload_too_large` fuera del logo ("como `malformed_request`"). Cuando exista `web/`, `gen:api` regenera los tipos (incluyen la respuesta `413` nueva). Sin decisión de UI pendiente |
| T-F202 (pantalla de registro) | T-B305 (tercera revisión) | El `503` del registro puede traer `Retry-After: 2` (DD-33); el mensaje de "Reintentar" puede usarlo. Decisión del `frontend-architect` |

Orden sugerido: Fase 0 del backend completa → T-F007/T-F008 (en la misma rama o la siguiente) →
T-F009.

---

### Fase 0 — Esqueleto del monolito (Setup)

**Objetivo**: el binario compila, sirve `/healthz` desde el mux raíz (por HTTP plano o, en modo
local, por HTTPS), corre migraciones embebidas, genera código sqlc y hay un harness que levanta
PostgreSQL 18 real con los roles del proyecto. CI corre todo.

**Prueba independiente**: `make check` en verde en CI; `docker compose up` + `crm migrate up` +
`crm serve` y `curl /healthz` responde `200 {"status":"ok"}`; `curl /api/v1/no-existe` responde
`404` problem+json. Con los certificados de T-B014 y la configuración del modo "Binario completo"
(plan §10.5.1), `curl https://localhost:8443/healthz` **sin** `-k` responde `200`.

**T-B001 — Módulo Go y estructura de directorios** · ADR-001
- `go mod init github.com/fdelillo/crm` con Go 1.27. Árbol de `plan.md` §11 con paquetes vacíos
  (`doc.go` que explique la responsabilidad de cada uno en una línea).
- `cmd/crm` con subcomandos `serve`, `migrate {up|down|status}`, `tenants reprovision-roles`
  (este último vacío hasta la Fase 9), usando `flag` de la librería estándar. `cmd/crm` importa
  `_ "time/tzdata"` (DD-27).

**T-B002 [T] — Configuración** · ADR-001, ADR-006, plan §10.5, DD-10, DD-14, DD-24, DD-32, INV-23, INV-25, P-5, H-3, H-10
- **Red**:

  | Entrada (env) | Resultado esperado |
  |---|---|
  | Todas las variables obligatorias válidas, `APP_BASE_URL=https://crm.example` | `Config` poblado; `TLS == nil`; `IsLocal() == false`; defaults: `HTTP_ADDR=:8080`, `METRICS_ADDR=127.0.0.1:9090`, `SESSION_IDLE=24h`, `SESSION_ABSOLUTE=168h`, `APP_LINK_RESET=/reset-password`, `APP_LINK_VERIFY=/verify-email`, `APP_LINK_INVITATION=/accept-invitation`, `TrustedProxies` vacío |
  | Falta `DATABASE_URL` | error que nombra la variable |
  | `AUTH_HMAC_KEY` de menos de 32 bytes (decodificada) | error |
  | `APP_BASE_URL=https://localhost:8443`, `https://localhost:5173`, `https://127.0.0.1:8443` sin `TLS_*` | válido; `IsLocal() == true`; `TLS == nil` |
  | Esos mismos orígenes con `TLS_CERT_FILE` y `TLS_KEY_FILE` | válido; `TLS` con ambas rutas (la configuración **no** lee los archivos) |
  | `APP_BASE_URL=https://crm.example` con `TLS_CERT_FILE` y `TLS_KEY_FILE` | error: TLS propio solo en modo local |
  | Solo `TLS_CERT_FILE` o solo `TLS_KEY_FILE` (en modo local) | error que nombra la variable que falta |
  | `APP_BASE_URL=http://localhost:8080`, `http://127.0.0.1:8080`, `http://crm.example` | error (H-10: `http://` ya no se acepta en ningún caso) |
  | `APP_BASE_URL=https://localhost.crm.example` o `https://192.168.0.10:8443` | válido pero `IsLocal() == false` (solo `localhost` y `127.0.0.1` exactos) |
  | `COOKIE_SECURE` definida con cualquier valor | se ignora: no existe la variable y la cookie sigue siendo `Secure` (el `Config` no tiene ningún campo para desactivarlo) |
  | `APP_BASE_URL` sin esquema o con otro esquema (`ftp://…`) | error |
  | `APP_LINK_*` sin `/` inicial o con `#`/`?` | error que nombra la variable |
  | `SESSION_IDLE` > `SESSION_ABSOLUTE` | error |
  | Duración mal formada | error que nombra la variable |
  | `TRUSTED_PROXIES` ausente o vacía (tercera revisión) | válido; `TrustedProxies` vacío (no se confía en ningún proxy) |
  | `TRUSTED_PROXIES=10.0.0.0/8, 2001:db8::/32` (con y sin espacios alrededor de la coma) | válido; dos prefijos, uno IPv4 y uno IPv6 |
  | `TRUSTED_PROXIES=10.1.2.3` / `2001:db8::1` (IP sin prefijo) | válido; equivale a `10.1.2.3/32` / `2001:db8::1/128` |
  | `TRUSTED_PROXIES=0.0.0.0/0`, `::/0`, o cualquiera de los dos dentro de una lista válida | error que nombra la variable (confiar en todos hace falsificable la IP, DD-32) |
  | `TRUSTED_PROXIES=10.0.0.0/33`, `foo` o `10.0.0.0/8,,` | error que nombra la variable y la posición del valor inválido |
  | El error nunca incluye el **valor** de una variable secreta | aserción sobre el texto |
- **Green**: `config.Load(getenv func(string) string) (Config, error)` y `Config.IsLocal()` pasan la
  tabla.
- **Refactor**: una sola función de validación por campo; nada de variables globales; la
  detección de modo local en un solo lugar (la usan TLS y HSTS).

**T-B003 — Implementar `platform/config`**.

**T-B004 [T] — Mux raíz, servidor HTTP/HTTPS y harness `apitest`** · ADR-002, ADR-006, ADR-009, ADR-019, DD-12, DD-22, DD-24, DD-32, INV-22, H-1, H-10, research R-26
- **Red** (`httptest` sobre `app.NewRootHandler` con un router chi de prueba y un **stub** en
  `RootDeps.SPA` que responde `200 text/plain "spa"` y registra la ruta recibida):

  | Caso | Esperado |
  |---|---|
  | `GET /healthz` | `200`, `application/json`, `{"status":"ok"}`, `Cache-Control: no-store`; el stub no recibió nada |
  | `POST /healthz` | `405` (el `ServeMux` rechaza el método) |
  | `GET /api/v1/no-existe` | `404` `application/problem+json` con `code=not_found`; el stub **no** recibió nada |
  | `GET /api/v2/cualquier` y `GET /api/` | `404` problem+json (todo `/api/` es de chi) |
  | `DELETE /api/v1/<ruta que solo acepta GET>` (tercera revisión) | `405` `application/problem+json` con `code=method_not_allowed` y `Allow: GET` (exactamente los métodos registrados para esa ruta); `Cache-Control: no-store`; valida contra `Problem` del contrato v0.4.0 |
  | `PUT /api/v1/<ruta que acepta GET y POST>` | `405` `method_not_allowed`; `Allow` contiene `GET` y `POST` y nada más, en el orden fijo de la lista de plan §9.2 |
  | Método desconocido `FOO /api/v1/<ruta que solo acepta GET>` | la misma respuesta que el `DELETE` (chi lo manda al mismo manejador) |
  | `GET /api` (sin barra) | redirección a `/api/` del `ServeMux`; nunca llega al stub |
  | `GET /`, `GET /login`, `GET /settings/users`, `GET /assets/x.js` | los atiende el stub (ruta registrada igual a la pedida) |
  | Un handler de la API que hace *panic* | `500` problem+json `code=internal`, el proceso sigue vivo, log `ERROR` con `request_id` |
  | Un stub de SPA que hace *panic* | `500` (el recover es común), proceso vivo |
  | Toda respuesta (API, ops y SPA) con configuración **no local** | `X-Request-Id`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `Strict-Transport-Security` |
  | Toda respuesta con configuración **local** (`APP_BASE_URL=https://localhost:8443`) | las mismas cabeceras **sin** `Strict-Transport-Security` (DD-24) |
  | Log de un request a la SPA | `route=spa` (no la URL cruda); a ops, `route=ops`; a la API, el patrón chi |
  | `app.NewServer` con `cfg.TLS` apuntando a un certificado y una clave generados en el test (`crypto/x509`, autofirmado para `127.0.0.1`; nada se versiona) | el servidor atiende por HTTPS: un cliente que confía en ese certificado recibe `200` en `/healthz`; log de arranque `listen=https_local` |
  | `app.NewServer` con `cfg.TLS` apuntando a archivos inexistentes o a un par que no coincide | error **antes** de escuchar, que nombra `TLS_CERT_FILE`/`TLS_KEY_FILE` y no incluye el contenido |
  | `app.NewServer` sin `cfg.TLS` | escucha HTTP plano; log `listen=http` |
  | Log de arranque (tercera revisión) | incluye `trusted_proxies` con la lista de CIDR configurada (no es secreta; vacía si no hay), para el runbook de plan §12.3 |
  | `apitest.NewServer(t, root)` con un handler de prueba que fija `__Host-x=1; Secure; HttpOnly; Path=/` y otro que la lee | el `Client` del harness la reenvía en el segundo request (HTTPS + `cookiejar`) |
- **Green**: `app.NewRootHandler(RootDeps, CommonMiddleware) http.Handler` con `http.ServeMux`
  (`/api/` → chi, `GET /healthz`, `GET /readyz`, `/` → SPA) envuelto por los middlewares
  comunes (HSTS según el modo); `app.NewServer(cfg, root)` con los timeouts de §10.3 del plan y
  TLS local cuando `cfg.TLS != nil`; `internal/testsupport/apitest.NewServer`; apagado ordenado
  con `context` al recibir SIGTERM. (Séptima revisión: la validación contra el contrato de los
  `404` y `405` de la API se hace con `contract.CheckResponse`, T-B311/T-B312, como ajuste al
  comienzo de la Fase 3.)

**T-B005 — Implementar el mux raíz, el router chi de la API (con `NotFound`/`MethodNotAllowed`
problem+json; el `405` con `code: method_not_allowed` y `Allow` calculado con `Match`, plan §9.2),
logging `slog` JSON, `app.NewServer` (HTTP plano o HTTPS local según `cfg.TLS`, DD-24), `apitest`
y `serve`**. Hasta T-F008, `serve` usa como `RootDeps.SPA` un handler que responde
`503 text/plain` "La interfaz no está compilada (correr `make web-build`)" (el mismo mensaje que
usará el paquete `web` sin `index.html`, ADR-019). Ajuste de la tercera revisión sobre lo ya
mergeado: `httpx.MethodNotAllowed` pasa de `CodeMalformedRequest` a `CodeMethodNotAllowed`; el
resto de `internal/app/root.go` no cambia.

**T-B006 — Bootstrap de roles y entorno local** · ADR-004, ADR-005, `data-model.md` §3.1
- `db/bootstrap/`: SQL idempotente que crea los roles de §3.1 con sus atributos y membresías
  (sintaxis de PostgreSQL ≥ 16: `GRANT ... WITH INHERIT FALSE, SET TRUE`), la base `crm` con dueño
  `crm_owner`, `REVOKE ALL ON DATABASE crm FROM PUBLIC`, `GRANT CONNECT` a `crm_app`, y los
  parámetros de `crm_app` (`statement_timeout`, `idle_in_transaction_session_timeout`).
- `compose.yaml` de desarrollo: `postgres:18` (último minor), Mailpit, MinIO; el bootstrap se
  monta como script de inicialización.

**T-B007 — goose embebido y migración 00001** · ADR-004
- Paquete `db/migrations` con `//go:embed *.sql`; `crm migrate` usa `goose.NewProvider` con ese
  `embed.FS` y `DATABASE_MIGRATION_URL` (rol `crm_owner`). Tabla de versiones en `public`.
- `00001_schemas.sql`: esquemas `app` y `provisioning` (este con `AUTHORIZATION crm_provisioner`),
  `REVOKE ALL ON SCHEMA public FROM PUBLIC`, privilegios por defecto (`data-model.md` §3.4) y
  `USAGE` de esquemas (§3.5). Con su `Down`.

**T-B008 — sqlc** · ADR-003
- `sqlc.yaml`: motor `postgresql`, `sql_package: pgx/v5`, esquema = `db/migrations`, un bloque
  por módulo (`internal/<módulo>/store`), overrides `uuid → github.com/google/uuid.UUID`,
  `inet → net/netip.Addr`.
- **Spike (supuesto 2 de `research.md`)**: una query de prueba sobre una tabla con `CREATE POLICY`
  y `GRANT` por columna en el esquema; `sqlc generate` sin errores. Reportar el resultado.

**T-B009 [T] — Harness de PostgreSQL real** · ADR-012, INV-18
- **Red** (`internal/testsupport/pgtest`, integración):

  | Caso | Esperado |
  |---|---|
  | `pgtest.Start(t)` | Levanta `postgres:18` una vez por paquete (`TestMain`), corre bootstrap como superusuario del contenedor y migraciones como `crm_owner` |
  | Pool devuelto por `pgtest.AppPool(t)` | `SELECT current_user, session_user` = `crm_app` |
  | Atributos del rol del pool | `rolsuper = false`, `rolbypassrls = false` (INV-18) |
  | `SELECT 1 FROM app.<cualquier tabla>` sin cambiar de rol | error `42501` (INV-02; con tablas desde la Fase 1) |
  | Docker no disponible | el test **falla** con mensaje claro (nunca `t.Skip` silencioso en CI) |
- **Green**: harness con `testcontainers-go` (módulo postgres); expone además `pgtest.OwnerPool`
  (solo para aserciones de catálogo, nunca para probar comportamiento).

**T-B010 — Implementar `internal/testsupport/pgtest`**.

**T-B011 [T] — Reglas de repositorio** · INV-03, INV-04, INV-22, INV-25, ADR-001
- **Red** (test Go que recorre el árbol de fuentes):

  | Regla | Caso que debe fallar |
  |---|---|
  | `SET ROLE`, `SET LOCAL ROLE`, `RESET ROLE` o `set_config('role'` solo en `internal/platform/db` | Archivo de fixture en `testdata/` con `SET ROLE` en otro paquete → detectado |
  | Ningún `fmt.Sprintf`/concatenación que arme SQL fuera de `platform/db` | Fixture con `"SELECT " + x` → detectado |
  | El router chi de la API no registra rutas fuera de `/api/` (no hay `Mount("/")`, `NotFound` hacia la SPA ni `/*`) | Fixture que registra la SPA en chi → detectado |
  | (Tercera revisión, INV-25) Los literales `"X-Forwarded-For"`, `"Forwarded"`, `"X-Real-IP"` (sin distinguir mayúsculas) y el acceso a `.RemoteAddr` solo aparecen en `internal/platform/httpx`; se excluyen los `_test.go` y `internal/testsupport` | Fixture con `r.Header.Get("X-Forwarded-For")` en otro paquete → detectado; otro con `r.RemoteAddr` fuera de `httpx` → detectado |
- `.golangci.yml` con `depguard`: reglas de `plan.md` §4.1 (platform no importa dominio;
  `identity` no importa `tenant`; nadie importa `*/store` de otro módulo; `web` solo importa la
  librería estándar y `gzhttp`; solo `internal/app` y `cmd/crm` importan `web`).
- **Green**: el test pasa sobre el código real y detecta los fixtures. El log de request que ya
  existe toma la IP de `httpx.ClientIPFrom` (T-B220), no de `RemoteAddr`.

**T-B012 [T] — Spike del rol por empresa** · ADR-005, P-1, supuesto 1 de `research.md`
- **Red** (integración, SQL directo como superusuario del contenedor para preparar):

  | Caso | Esperado |
  |---|---|
  | En **una** transacción de un rol con `CREATEROLE`: `CREATE ROLE r`, `GRANT r TO crm_app WITH SET TRUE`, luego (como `crm_app`) `SET LOCAL ROLE r` | Funciona sin `COMMIT` intermedio |
  | La misma transacción con `ROLLBACK` | El rol `r` no existe después |
  | Correr `db/bootstrap/` contra el PostgreSQL **candidato de producción** (fuera de CI, manual) | Sin errores de privilegios; reportar el resultado para cerrar P-1 |
- **Green**: resultado documentado en el reporte de la fase. Si el primer caso falla, **frenar**
  y volver al arquitecto: el registro atómico (INV-14) depende de esto.

**T-B013 — Makefile y CI** · ADR-012
- Targets de la tabla de comandos. `.github/workflows/ci.yml`: Go 1.27, Docker disponible,
  `make check`. Cachés de módulos. `make check` **no** instala mkcert ni genera certificados (los
  tests HTTP usan `httptest.NewTLSServer`). El job de E2E, con los pasos de mkcert de plan
  §10.5.1, lo agregan T-F009/T-F701. (T-F009 agrega después los targets y el job del frontend.)

**T-B014 — HTTPS local de desarrollo** · DD-24, H-10, plan §10.5.1, R-14
- `make dev-certs`: verifica que `mkcert` esté instalado (si no, mensaje con el enlace de
  instalación y el recordatorio de `certutil` en Linux), genera `.certs/localhost.pem` y
  `.certs/localhost-key.pem` para `localhost` y `127.0.0.1`; idempotente.
- `.gitignore`: `.certs/`.
- `.env.example` (sin secretos reales, solo marcadores) con los valores del modo "Binario
  completo" de plan §10.5.1 (`APP_BASE_URL=https://localhost:8443`, `HTTP_ADDR=:8443`,
  `TLS_CERT_FILE`, `TLS_KEY_FILE`) y, desde la tercera revisión, `TRUSTED_PROXIES=` vacía (en
  desarrollo no hay proxy).
- README, sección de desarrollo: instalar mkcert (y `certutil` en Linux), `mkcert -install`,
  `make dev-certs`, los tres modos de plan §10.5.1, `NODE_EXTRA_CA_CERTS`, y la advertencia de no
  compartir `rootCA-key.pem`.
- **Verificación** (manual, en el checkpoint): con esos valores, `crm serve` loguea
  `listen=https_local`; `curl https://localhost:8443/healthz` sin `-k` → `200`; Chrome abre
  `https://localhost:8443/healthz` sin aviso de certificado.

**Checkpoint Fase 0**: `make check` en verde local y en CI; resultados de los spikes T-B008 y
T-B012 reportados; verificación de T-B014 hecha. **No avanzar a la Fase 1 si T-B012 falla.** Con
la Fase 0 cerrada se pueden hacer T-F007/T-F008 (sección Frontend).

---

### Fase 1 — Fundacional A: base de datos y aislamiento

**Objetivo**: esquema completo de 001 con RLS forzada, rol por empresa, `TxRunner`, mapeo de
errores y los tests que vigilan las invariantes de aislamiento para **todas** las tablas
presentes y futuras.

**Prueba independiente**: con dos empresas creadas por SQL de fixture, un rol de empresa no ve,
no modifica y no inserta filas de la otra en ninguna tabla.

**T-B101 [T] — Funciones de mapeo y aprovisionamiento** · ADR-005, `data-model.md` §3.2–3.3
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | `current_tenant_id()` como `crm_t_<hex de U>` | `U` |
  | `current_tenant_id()` como `crm_auth`, `crm_worker`, `crm_owner` | `NULL` |
  | `current_tenant_id()` con un rol `crm_t_xyz` (no hex de 32) | `NULL` |
  | `provision_tenant_role(U)` como `crm_signup` | Devuelve `crm_t_<hex>`; el rol existe `NOLOGIN`, sin `BYPASSRLS`/`CREATEROLE`/`SUPERUSER`; es miembro de `crm_tenant` con `INHERIT`; `crm_app` lo tiene con `SET TRUE, INHERIT FALSE` |
  | Llamarla dos veces con el mismo `U` | Mismo resultado, sin error (idempotente) |
  | Llamarla dentro de una transacción que hace `ROLLBACK` | El rol no existe |
  | Ejecutarla como `crm_app` sin `SET ROLE crm_signup` | `42501` |
  | Ejecutarla como `crm_auth` o `crm_worker` | `42501` |
  | `CREATE ROLE x` como `crm_app` o `crm_signup` | `42501` |
- **Green**: todos los casos pasan con la migración 00002.
- **Refactor**: nombres de rol derivados en un único lugar en Go (`db.TenantRoleName(uuid.UUID)`),
  con su propio test unitario de formato.

**T-B102 — Migración `00002_tenant_functions.sql`**: `app.current_tenant_id()` y
`provisioning.provision_tenant_role(uuid)` (la segunda creada con `SET LOCAL ROLE crm_provisioner`).

**T-B103 [T] — `TxRunner` y `Tx`** · INV-02, INV-03, INV-19, INV-27, ADR-005, DD-34, R-17
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | `InTenantTx(A, fn)` y dentro `SELECT current_user` | `crm_t_<hex A>` |
  | Tras el `COMMIT`, la misma conexión física del pool | `current_user = crm_app` |
  | `fn` devuelve error | `ROLLBACK`; el error se devuelve envuelto (`errors.Is` funciona) |
  | `fn` hace *panic* | `ROLLBACK` y el *panic* se re-lanza |
  | `InSystemTx(RoleAuth)` → `AsTenant(A)` → `AsSystem(RoleAuth)` | Permitido; `current_user` acompaña cada paso |
  | `InTenantTx(A)` → `AsTenant(B)` | `ErrTenantAlreadyBound`; nada se ejecutó como `B` |
  | `InSystemTx(RoleAuth)` → `AsTenant(A)` → `AsTenant(B)` | `ErrTenantAlreadyBound` |
  | `AsTenant(A)` cuando el rol de `A` no existe | error envuelto que nombra el rol (runbook §12.3) |
  | Contexto cancelado durante `fn` | `ROLLBACK`, error de contexto |
  | Una query con el pool directo (sin `TxRunner`) sobre `app.users` | `42501` (INV-02) |
  | (Cuarta revisión, DD-34) **Reproducción** `TestTxRunner_NewCompanyIsUsableOnAnyConnectionRightAfterProvisioning`: registros concurrentes y, justo después de cada `COMMIT`, el primer `InTenantTx` de esa empresa en **cada** conexión del pool (600 primeros usos), **sin** *hooks* | 0 fallas. Corre en `make check`: es el detector de R-17 cuando cambia la imagen de PostgreSQL |
  | Cada cambio de rol (`InTenantTx`, `InSystemTx`, `AsTenant`, `AsSystem`), observado con un `pgx.QueryTracer` de test | la lectura de `pg_catalog.pg_auth_members` y el `SET LOCAL ROLE` salen en **una sola** llamada, en ese orden |
  | `InTenantTx(A)` con el primer `SET LOCAL ROLE` forzado a fallar con `42501` **una** vez (*hook* de test no exportado) | `ROLLBACK`, un reintento, `fn` ejecutada exactamente **una** vez y con `current_user = crm_t_<hex A>`; log `WARN` `event=set_role_retry` con `tenant_id` y `outcome=recovered`; `set_role_retry_total` +1 |
  | El mismo caso forzando `42501` en los **dos** intentos | `errors.Is(err, db.ErrPrivilege)`; el mensaje nombra el paso y el rol (`set role crm_t_…`); `fn` **nunca** se ejecutó; exactamente dos intentos; `outcome=failed` |
  | `fn` devuelve un `42501` (p. ej. un `INSERT` con el `tenant_id` de otra empresa, RLS) | **sin** reintento: `fn` corrió una sola vez; `ErrPrivilege`; `set_role_retry_total` sin cambios |
  | `InSystemTx(RoleAuth)` con el cambio de rol forzado a `42501` | sin reintento; `ErrPrivilege`; `fn` no se ejecutó |
  | `InSystemTx(RoleAuth)` → `AsTenant(A)` con `42501` forzado | sin reintento; `ErrPrivilege`; `ROLLBACK` |
  | `InTenantTx` de una empresa sin rol (`22023`) | sin reintento; el error nombra el rol (fila de arriba) |
  | Contexto cancelado entre el primer intento y el reintento | `ROLLBACK`, error de contexto, sin segundo intento |
- **Green**: `db.NewTxRunner(pool)` pasa la tabla.
- **Refactor**: `SET LOCAL ROLE` con `pgx.Identifier{...}.Sanitize()`; una sola función privada
  que cambia de rol (con la lectura de catálogo) y la usan todos los caminos; la decisión "¿se
  puede reintentar?" en un solo lugar de `InTenantTx`, no repartida.

**T-B104 — Implementar `platform/db` (`TxRunner`, `Tx`, `TenantRoleName`)**. Cuarta revisión
(DD-34, INV-27): la lectura de catálogo en la función privada de cambio de rol ya está aplicada
(commit e3d990b); falta el reintento único en `InTenantTx` (solo ante `42501` del `SET LOCAL
ROLE` inicial y antes de llamar a `fn`), el *hook* de test no exportado, el error envuelto con el
paso y el rol, el log `set_role_retry` y el contador `set_role_retry_total` (se publica en
`expvar` con el resto de las métricas, T-B904). Las firmas de `TxRunner` y `Tx` no cambian.

**T-B105 [T] — Mapeo de errores de PostgreSQL y de contexto** · INV-19, INV-26, plan §9.2, §9.4, DD-33, research R-27
- **Red** (unitario, con `*pgconn.PgError` construidos a mano):

  | Error de entrada | Salida |
  |---|---|
  | `pgx.ErrNoRows` | `errors.Is(err, db.ErrNotFound)` |
  | SQLSTATE `23505`, constraint `users_email_key` | `db.ErrUniqueViolation` y `*db.ConstraintError{Constraint:"users_email_key"}` |
  | SQLSTATE `42501` | `db.ErrPrivilege` (nunca `ErrNotFound`) |
  | Error de conexión, `57014` (statement timeout), `context.DeadlineExceeded` | `db.ErrUnavailable` |
  | SQLSTATE `55P03` (`lock_not_available`, vence `lock_timeout`) (tercera revisión) | `db.ErrUnavailable` |
  | `context.Canceled` solo, o envuelto con `fmt.Errorf("…: %w", …)` | `db.ErrCanceled` (y **no** `ErrUnavailable`) |
  | Un error que envuelve a la vez `context.Canceled` y un `*pgconn.PgError` `57014` (lo que devuelve pgx cuando cancela la consulta en el servidor porque se canceló el contexto) | `db.ErrCanceled`: `Canceled` se clasifica **primero** (plan §9.2) |
  | `57014` sin `context.Canceled` en la cadena | `db.ErrUnavailable` |
  | Otro SQLSTATE | se devuelve envuelto sin clasificar |
  | En todos los casos | `errors.Is(err, <error original>)` sigue funcionando sobre la salida (la causa se conserva) |
  | En la capa HTTP: `db.ErrPrivilege` | `500 internal` + log `ERROR` con `security_event=rls_violation` |
  | En la capa HTTP: `db.ErrUnavailable` | `503 service_unavailable` |
  | En la capa HTTP: `db.ErrCanceled` con el contexto del request cancelado (el cliente se fue) | **no** se escribe nada; log de request `level=INFO`, `event=client_canceled`, `status=499`; `http_client_canceled_total` +1; ningún log `ERROR` |
  | En la capa HTTP: `db.ErrCanceled` con el contexto del request **vivo** (cancelación interna, p. ej. apagado) | `503 service_unavailable` |
- **Green**: `db.MapError(error) error` y el mapeo por defecto de `httpx` pasan la tabla. Las filas
  de la capa HTTP se escriben acá y pasan con T-B202 (Fase 2).

**T-B106 — Implementar `db.MapError`** (el mapeo HTTP por defecto va en T-B202). Tercera revisión:
agrega `db.ErrCanceled` y el caso `55P03`, con el orden de clasificación de plan §9.2; la firma
`MapError(err error) error` **no** cambia (la pregunta "¿el cliente se fue?" la responde la capa
HTTP mirando `r.Context().Err()`, no `platform/db`).

**T-B107 [T] — Invariantes de catálogo** · INV-01, INV-02, INV-06, INV-08, INV-15
- **Red** (integración, consultas a `pg_catalog` con `OwnerPool`):

  | Aserción | Falla si… |
  |---|---|
  | Tablas esperadas en `app`: `tenants, users, sessions, user_tokens, outbox_messages, audit_log, login_throttles` | falta alguna (lista declarada en el test; cada spec la extiende) |
  | Toda tabla de `app`: `relrowsecurity` y `relforcerowsecurity` | alguna no los tiene |
  | Toda tabla con columna `tenant_id`: `NOT NULL` y política `tenant_isolation` con `USING` y `WITH CHECK` que referencian `current_tenant_id` | falta o difiere |
  | Dueño de toda tabla y función de `app` | ≠ `crm_owner` |
  | Roles `crm_app`, `crm_tenant`, `crm_auth`, `crm_worker`, `crm_signup`, `crm_t_*` | alguno con `rolbypassrls`, `rolsuper` o dueño de algo |
  | `crm_app` | tiene algún privilegio de tabla o columna **directo o heredado** (`has_table_privilege` como `crm_app`) |
  | Funciones `SECURITY DEFINER` | alguna fuera del esquema `provisioning`, sin `search_path` fijo, o con `EXECUTE` para `PUBLIC` |
  | Vistas en `app` | alguna sin `security_invoker = true` |
  | `crm_tenant` sobre `audit_log` | tiene `UPDATE`, `DELETE` o `TRUNCATE` |
- **Green**: pasa tras T-B111. **Es el test que protege a todas las specs futuras**: toda tabla
  nueva queda verificada sin escribir un test nuevo.

**T-B108 [T] — FKs compuestas** · INV-07
- **Red**: toda FK desde una tabla de `app` con `tenant_id` hacia otra tabla con `tenant_id`
  incluye `tenant_id` en ambos lados (consulta a `pg_constraint`). Fixture: insertar como
  `crm_owner` con `FORCE` desactivado **en una transacción de test que hace rollback** una sesión
  de `A` apuntando a un usuario de `B` → violación de FK. (Las FK simples `tenant_id → tenants(id)`
  de `sessions` y `user_tokens` apuntan a `tenants`, que no tiene `tenant_id`: no entran en esta
  regla; `data-model.md` §2.3/§2.4 quedó alineado en la tercera revisión.)
- **Green**: pasa tras T-B111.

**T-B109 [T] — Privilegios de los roles de sistema** · INV-05, plan §4.4
- **Red**: *snapshot* de `information_schema.column_privileges` y `table_privileges` para
  `crm_auth`, `crm_worker`, `crm_signup` comparado con la tabla de `data-model.md` §3.4 declarada
  en el test (incluidas las columnas `purpose`, `used_at` y `revoked_at` de `user_tokens` para
  `crm_worker`). Además, como `crm_auth`: `SELECT password_hash FROM app.users` → `42501`; como
  `crm_worker`: `SELECT payload FROM app.outbox_messages` y `SELECT token_hash FROM
  app.user_tokens` → `42501`.
- **Green**: pasa tras T-B111. Cambiar un privilegio de sistema obliga a cambiar este test y
  `data-model.md` (matriz de mantenimiento).

**T-B110 [T] — Aislamiento en base de datos (SC-002, nivel BD)** · FR-006, INV-01, principio III
- **Red** (integración): fixture con empresas `A` y `B` (roles aprovisionados; filas en **todas**
  las tablas de empresa, insertadas por cada rol de empresa). Para cada tabla con `tenant_id`
  (descubierta desde el catálogo), como rol de `A`:

  | Operación | Esperado |
  |---|---|
  | `SELECT *` sin `WHERE` | solo filas de `A` |
  | `SELECT ... WHERE id = <id de B>` | 0 filas |
  | `UPDATE ... WHERE id = <id de B>` (si tiene `UPDATE`) | 0 filas afectadas; la fila de `B` intacta (verificado como `B`) |
  | `DELETE ... WHERE id = <id de B>` (si tiene `DELETE`) | 0 filas afectadas |
  | `INSERT` con `tenant_id = B` | error de RLS (`42501`) |
  | `UPDATE` de una fila propia cambiando `tenant_id` a `B` | error de RLS |
  | `tenants`: `SELECT` | solo la fila de `A` |
  | Como `crm_auth`: `SELECT id, tenant_id, email FROM users` | ve ambas (esperado y documentado, §4.4); cualquier otra columna → `42501` |
- **Green**: pasa tras T-B111.

**T-B111 — Migraciones 00003 a 00008** · `data-model.md` §2–§3
- Tablas, constraints, índices, RLS habilitada y forzada, políticas y `GRANT` exactamente como
  en `data-model.md` (incluida la política `worker_cleanup` de `user_tokens` que conserva la
  invitación abierta, DD-25). Cada una con `Down`.

**T-B112 [T] — Queries con filtro explícito de empresa** · INV-04, plan §4.4, ADR-001
- **Red**: test que parsea cada `internal/*/store/*.sql` y, para toda query que referencie una
  tabla de empresa, exige un predicado `tenant_id = @tenant_id` (o `id = @tenant_id` en
  `tenants`). Excepciones **por ruta exacta** (tercera revisión) **y por nombre de query** (revisión
  del PR fdelillo/crm#7): en estos archivos solo se saltean las queries listadas por nombre, cada una
  con su motivo; el resto del archivo se revisa:

  | Archivo eximido | Motivo |
  |---|---|
  | `internal/identity/store/auth_lookup.sql` | `crm_auth`: login, reset, sesión y tokens antes de conocer la empresa |
  | `internal/identity/store/cleanup.sql` | `crm_worker`: limpieza de `sessions`, `user_tokens`, `login_throttles` vencidas |
  | `internal/platform/outbox/store/worker.sql` | `crm_worker`: tomar mensajes pendientes y limpiar los terminales de `outbox_messages` |
  | `internal/tenant/store/provisioning.sql` | `crm_worker`/`crm_signup`: listar `tenants.id` y aprovisionar roles |

  | Caso | Esperado |
  |---|---|
  | Fixture en `testdata/` sin filtro | detectado |
  | Fixture con el **mismo nombre** que una excepción pero en otro lugar (p. ej. `internal/tenant/store/auth_lookup.sql` o `internal/identity/store/worker.sql`) | detectado (la excepción es la ruta, no el nombre) |
  | Una excepción cuyo archivo todavía no existe (`cleanup.sql` y `provisioning.sql` llegan en fases posteriores) | no es error si no lista queries |
  | Query **no listada** en un archivo eximido (p. ej. `SELECT id, email FROM app.users;` en `auth_lookup.sql`) | detectada |
  | Nombre de query listado que no está en el archivo, o archivo inexistente con queries listadas | detectado |
  | Query eximida sin motivo | error de configuración |
- **Green**: `queryrules.DefaultExceptions` tiene exactamente las cuatro rutas, todavía sin queries
  listadas (cada fase agrega el nombre de la query que escribe); el detector
  funciona sobre los fixtures y se vuelve efectivo a medida que las fases siguientes agregan
  queries. Agregar una ruta es una decisión de diseño (plan §4.4, matriz §16).

**Checkpoint Fase 1**: `make check` en verde. `go test -tags=integration -run
'Catalog|Isolation|TxRunner|Provision' ./internal/platform/db/... ./db/...` en verde.

---

### Fase 2 — Fundacional B: plataforma HTTP y servicios transversales

**Objetivo**: todo lo que las historias usan y no es de ningún dominio: problem+json,
middlewares de seguridad, IP del cliente, tokens, hashing, autorización, auditoría, outbox con
worker, email y almacenamiento de objetos.

**Prueba independiente**: un mensaje encolado en una transacción de empresa llega a Mailpit;
un `POST` de origen cruzado es rechazado con problem+json; la matriz de permisos coincide con
FR-007; con `TRUSTED_PROXIES` apuntando al proxy de prueba, el log de request muestra la IP del
cliente y no la del proxy.

Antes de T-B201: los ajustes de la tercera y la cuarta revisión sobre las Fases 0 y 1 (tabla de
"Estado de la implementación").

**T-B201 [T] — problem+json y decodificación de JSON** · ADR-009, plan §9
- **Red** (unitario):

  | Caso | Esperado |
  |---|---|
  | `WriteProblem(w, r, code)` para cada `code` de §9.1 (incluido `method_not_allowed`) | status correcto, `Content-Type: application/problem+json`, `type=/problems/{code}`, `instance`=request id, `title` en español |
  | `WriteProblem` con la opción `SuggestedPasswordReset` | el cuerpo incluye `"suggested_action": "password_reset"`; sin la opción, el campo no aparece |
  | `WriteProblem(email_already_registered, SuggestedPasswordReset)` | `409`, `title` "Ya existe un usuario con ese email", `detail` "Ya existe un usuario con ese email. ¿Querés recuperar la contraseña?"; valida contra `EmailAlreadyRegisteredProblem` del contrato |
  | `WriteProblem(service_unavailable)` con la opción `RetryAfter(2 * time.Second)` | `503` con cabecera `Retry-After: 2`; sin la opción, sin cabecera |
  | `ValidationError` con dos campos | `422`, `errors` con `{field, code}` en orden estable |
  | `DecodeJSON` con JSON válido | struct poblado |
  | Campo desconocido | `400 malformed_request` (`DisallowUnknownFields`) |
  | Dos objetos JSON concatenados | `400` |
  | Body > 64 KiB (65 537 bytes) | `413 payload_too_large`; con 65 536 bytes se decodifica normalmente (el límite es inclusivo; contrato v0.4.1, séptima revisión) |
  | Sin `Content-Type: application/json` | `415` |
  | `Content-Type: application/json; charset=utf-8` | aceptado |
- **Green**: pasa la tabla; las respuestas validan contra `Problem`/`ValidationProblem` del
  contrato (séptima revisión: con `contract.CheckSchema`, T-B311/T-B312, como ajuste al comienzo
  de la Fase 3).

**T-B202 — Implementar `platform/httpx` (problem+json con `suggested_action` y `Retry-After` opcionales, DecodeJSON, mapeo por defecto de errores, incluido `db.ErrCanceled` de plan §9.2)**.

**T-B219 [T] — IP del cliente (`httpx.ClientIP`)** · DD-32, INV-25, plan §10.3, research R-25
(Numeración agregada en la tercera revisión; se hace **antes** de T-B203, porque todos los
requests de la cadena común pasan por `ClientIP`. Sexta revisión: `ClientIP` va segundo, después
de `RequestID`, DD-32.)
- **Red** (unitario, `httptest`; `trusted` = `198.51.100.0/24` y `2001:db8:ffff::/48` salvo que la
  fila diga otra cosa; el handler interno registra `httpx.ClientIPFrom(r.Context())`):

  | `RemoteAddr` | `X-Forwarded-For` | IP esperada |
  |---|---|---|
  | `203.0.113.5:4711` (no confiable) | `192.0.2.66` | `203.0.113.5` (la cabecera se ignora) |
  | `203.0.113.5:4711`, con `trusted` **vacío** | `192.0.2.66` | `203.0.113.5` |
  | `198.51.100.10:4711` (confiable) | (ausente) | `198.51.100.10` |
  | `198.51.100.10:4711` | `192.0.2.66` | `192.0.2.66` |
  | `198.51.100.10:4711` | `192.0.2.99, 192.0.2.66` (el cliente inventó la primera) | `192.0.2.66` (la de más a la derecha no confiable) |
  | `198.51.100.10:4711` | `192.0.2.66, 198.51.100.20` (dos proxies de confianza en cadena) | `192.0.2.66` |
  | `198.51.100.10:4711` | `198.51.100.30, 198.51.100.20` (todas de confianza) | `198.51.100.30` (la de más a la izquierda válida) |
  | `198.51.100.10:4711` | dos cabeceras: `192.0.2.99` y luego `192.0.2.66` | `192.0.2.66` (se concatenan en orden) |
  | `198.51.100.10:4711` | `192.0.2.66, basura` | `198.51.100.10` + log `WARN` `event=bad_forwarded_for` (sin el valor de la cabecera); (sexta revisión) con `httpx.RequestID` delante, como en la cadena común, el aviso lleva `request_id` igual a `httpx.RequestIDFrom` del contexto del handler |
  | `198.51.100.10:4711` | `basura, 192.0.2.66` | `192.0.2.66` (la entrada mala está a la izquierda del cliente: no se llega a leer) |
  | `198.51.100.10:4711` | `192.0.2.66:5555` | `192.0.2.66` (sin puerto) |
  | `198.51.100.10:4711` | `[2001:db8::7]:5555` | `2001:db8::7` |
  | `[::ffff:198.51.100.10]:4711` | `::ffff:192.0.2.66` | `192.0.2.66` (el `RemoteAddr` mapeado se reconoce como confiable y el resultado sale con `Unmap()`) |
  | `[2001:db8:ffff::1]:4711` (confiable IPv6) | `2001:db8:1::5` | `2001:db8:1::5` |
  | `203.0.113.5:4711` | con `Forwarded: for=192.0.2.66` y `X-Real-IP: 192.0.2.66` | `203.0.113.5` (solo se lee `X-Forwarded-For`) |
  | Cualquiera, `ClientIPFrom` dentro del middleware | `IsValid() == true` |
  | `ClientIPFrom` sobre un contexto sin el middleware | `netip.Addr{}` (`IsValid() == false`); el test documenta el comportamiento |
- **Green**: pasa la tabla.
- **Refactor**: el algoritmo es una función pura (`RemoteAddr`, valores de la cabecera, `trusted`
  → `netip.Addr`, error) con el middleware como envoltorio fino.

**T-B220 — Implementar `httpx.ClientIP` y `httpx.ClientIPFrom`** (plan §11.1). Es el único lugar
del código que lee `X-Forwarded-For` y `RemoteAddr` (INV-25, T-B011).

**T-B203 [T] — Middlewares de seguridad y observabilidad** · ADR-006, plan §10.2–10.3, §10.7, §9.2, DD-12, DD-24, DD-28, DD-30, DD-32, INV-21, INV-25, H-7, H-9, H-10
- **Red** (unitario, `httptest` sobre el mux raíz de T-B005 con stub de SPA):

  | Caso | Esperado |
  |---|---|
  | `POST /api/v1/...` con `Sec-Fetch-Site: cross-site` | `403 application/problem+json` con `code: forbidden` (*deny handler*, H-9); el handler de la API no se ejecutó; log `security_event=csrf_rejected` con `ip`; `csrf_rejected_total` +1 |
  | `POST` con `Origin` de otro host y sin `Sec-Fetch-Site` | `403` problem+json `forbidden` |
  | `POST` con `Sec-Fetch-Site: same-origin` | pasa |
  | `POST` con `Origin: https://localhost:5173`, `Host: localhost:5173` y `Sec-Fetch-Site: same-origin` (proxy de Vite con HTTPS local, plan §10.5.1) | pasa |
  | `GET` con `Sec-Fetch-Site: cross-site` | pasa (método seguro) |
  | **Toda** respuesta de `/api/v1/*`: `200` JSON, `201`, `202`, `204`, `4xx` (incluido el `405`) y `5xx` problem+json (H-7) | `Cache-Control: no-store` |
  | Un handler de prueba registrado como `GET /api/v1/tenant/logo` que fija su propio `Cache-Control` | el middleware no lo pisa (la excepción de DD-23 queda en manos del handler del logo) |
  | Toda respuesta (API, ops y SPA) con configuración no local | `Strict-Transport-Security`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` |
  | Toda respuesta con configuración local | `X-Content-Type-Options` y `Referrer-Policy` presentes; `Strict-Transport-Security` **ausente** (DD-24) |
  | Respuesta del stub de SPA | **sin** `no-store` agregado por la API (la caché de la SPA la decide `web`, T-F007) |
  | Log de un request con body `{"password": "..."}` y cookie de sesión | el log no contiene la contraseña, el token ni el header `Cookie` |
  | Log de request | contiene `request_id`, `route` (patrón chi, `ops` o `spa`), `status`, `duration_ms` e `ip` |
  | Log de request con `TRUSTED_PROXIES` = rango del `RemoteAddr` y `X-Forwarded-For: 192.0.2.66` (tercera revisión) | `ip=192.0.2.66`; sin proxies de confianza, `ip` = la de `RemoteAddr` |
  | (Sexta revisión, DD-32) Mux raíz con `NewCommonMiddleware` (`TRUSTED_PROXIES` = rango del `RemoteAddr`) y `X-Forwarded-For: 192.0.2.66, basura` | una línea `WARN` `event=bad_forwarded_for` cuyo `request_id` es igual a la cabecera `X-Request-Id` de la respuesta y al `request_id` de la línea de request; ninguna línea contiene `basura` |
  | Handler de la API que devuelve `db.ErrCanceled` después de que el cliente cortó (contexto del request cancelado) | no se escribe cuerpo; log de request `level=INFO`, `event=client_canceled`, `status=499`; `http_client_canceled_total` +1; **ningún** log `ERROR` (tampoco del recover) |
  | El mismo handler con el contexto del request vivo | `503 service_unavailable` |
- **Green**: pasa la tabla.

**T-B204 — Implementar middlewares** (comunes, en el mux raíz, en este orden: request id →
`httpx.ClientIP` (DD-32, T-B220; sexta revisión: va después de request id y su aviso
`bad_forwarded_for` lleva `request_id`) → recover → logging (con `ip` y el `499` de las cancelaciones) →
security headers (HSTS según el modo, DD-24) → `CrossOriginProtection` con
`SetDenyHandler(httpx.CSRFDenyHandler)`; de la API, dentro de chi: `no-store` → rate limit por
ruta → autenticación por grupo). `CommonMiddleware` recibe el modo local y
`cfg.TrustedProxies`.

**T-B205 [T] — Tokens y contraseñas** · ADR-007, INV-09, DD-6
- **Red** (unitario):

  | Caso | Esperado |
  |---|---|
  | `securetoken.New()` 1000 veces | 1000 valores distintos; 43 caracteres base64url (32 bytes); `hash` = SHA-256 del raw (32 bytes) |
  | `securetoken.Hash(raw)` | igual al hash devuelto por `New` |
  | `Hasher.Hash("…")` | string PHC `$argon2id$v=19$m=19456,t=2,p=1$…` con sal distinta cada vez |
  | `Verify` correcto / incorrecto | `true` / `false`, sin error |
  | `Verify` con hash de parámetros menores (`m=8192`) | `ok=true, needsRehash=true` |
  | `Verify` con string PHC corrupto | error (no `false` silencioso) |
  | `VerifyDummy` | tarda del mismo orden que `Verify` (tolerancia amplia; test marcado `-short` skip) |
  | 20 `Hash` concurrentes con semáforo de 4 | nunca más de 4 en curso (contador instrumentado) |
  | Validación de contraseña: 9 caracteres / 10 / 128 / 129 / igual al email | `too_short` / ok / ok / `too_long` / `same_as_email` |
- **Green**: pasa la tabla.

**T-B206 — Implementar `platform/securetoken`, `platform/password` y la validación de contraseña**.

**T-B207 [T] — Matriz de permisos y middleware** · FR-007, ADR-013, INV-12
- **Red** (unitario):

  | Caso | Esperado |
  |---|---|
  | `Can(admin, p)` para los 15 permisos | `true` |
  | `Can(operator, p)` | `true` exactamente para `customers.manage`, `projects.manage`, `quotes.manage`, `customer_receipts.create`, `balances.view`, `suppliers.view_contact`; `false` para los otros 9 (una fila por renglón de FR-007) |
  | `Can(Role("otro"), p)` | `false` |
  | `Permissions(role)` | coincide con la tabla y con el enum `Permission` del contrato (el test carga el YAML) |
  | `RequirePermission(p)` sin principal en el contexto | `401 unauthenticated` |
  | `RequirePermission(settings.manage)` con operador | `403 forbidden`; el handler interno **no** se ejecuta |
  | `RequirePermission(settings.manage)` con admin | llama al handler |
- **Green**: pasa la tabla.

**T-B208 — Implementar `internal/authz`**.

**T-B209 [T] — Auditoría** · FR-008, INV-15, INV-32, INV-33, DD-37, DD-38
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | `Record` dentro de `InTenantTx(A)` | fila en `audit_log` con `tenant_id=A`, `request_id` del contexto |
  | `Record` con `Entry.TenantID = B` dentro de `InTenantTx(A)` | error (RLS); la transacción no deja fila |
  | `Record` y luego `ROLLBACK` de la operación | no queda fila (la auditoría es parte de la operación) |
  | `UPDATE`/`DELETE` sobre `audit_log` como rol de empresa | `42501` |
  | `Data` con clave `password` o `token` | error de programación (el Recorder las rechaza) |

  Sexta revisión (DD-37, INV-32). Todos los casos de credenciales corren **dentro de un
  `InTenantTx(A)` válido** y con un `Entry` válido salvo `Data` (el test actual pasa `tx = nil` y
  acepta cualquier error, así que no distingue el rechazo del secreto del de la transacción
  faltante). La función de la transacción **ignora** el error de `Record` y deja que la
  transacción haga `COMMIT`: así se demuestra que `Record` no insertó nada, no que la operación lo
  revirtió. Los valores secretos de prueba son largos y únicos (p. ej.
  `valor-secreto-de-prueba-<n>`) para poder buscarlos en el texto del error:

  | Caso | Esperado |
  |---|---|
  | Una clave por fila, con valor string: `password`, `new_password`, `passwd`, `passphrase`, `token`, `reset_token`, `Authorization`, `proxy-authorization`, `Cookie`, `Set-Cookie`, `secret`, `client_secret`, `api_key`, `X-API-Key`, `apiKey`, `private_key`, `credentials`, `signature`, `csrf_token`, `X-CSRF-Token`, `xsrf`, `auth`, `session`, `session_id`, `SID`, `otp`, `pin` | `errors.Is(err, audit.ErrSecretInData)`; el texto del error nombra la clave y **no** contiene el valor; 0 filas nuevas en `audit_log` después del `COMMIT` |
  | Anidadas: `{"headers": map[string]string{"Authorization": v}}`, `{"headers": map[string]any{"Set-Cookie": v}}`, `{"items": []any{map[string]any{"api_key": v}}}` | `ErrSecretInData`; el error incluye la ruta (`headers.Authorization`, `headers.Set-Cookie`, `items[0].api_key`) y no el valor |
  | Valores con clave inocua (`note`): `"Bearer abc.def"`, `"basic dXNlcjpwYXNz"`, `"  Digest username=x"`, un JWT (`eyJ…` en tres segmentos base64url), el `raw` de `securetoken.New()` | `ErrSecretInData` |
  | El `raw` de `securetoken.New()` como elemento de `{"fields": []string{"name", raw}}` | `ErrSecretInData` (las listas también se recorren) |
  | Tipos no admitidos: `struct{ A string }{}`, `[]byte("a")`, `json.RawMessage("{}")`, un `*string`, `map[string]int{}`, `float64(1)` | `errors.Is(err, audit.ErrUnsupportedData)`; 0 filas |
  | **Sin falsos positivos**: el `data` de cada acción del catálogo de `data-model.md` §2.6 con valores representativos (`{"industry_template_code": "generic"}`, `{"fields": []string{"name", "timezone"}}`, `{"content_type": "image/png"}`, `{"reason": "bad_password"}`, `{"sessions_revoked": int64(2)}`, `{"role": "operator", "trigger": "password_reset_request"}`, `{"from": "admin", "to": "operator", "status": "invited"}`, `{"to_status": "active"}`) y los tipos admitidos restantes (`bool`, `int`, `int32`, `uuid.UUID`, `time.Time`, `nil`) | una fila por caso |
  | `Data` limpio con `tx = nil` | `errors.Is(err, audit.ErrTxRequired)` |

  Séptima revisión (DD-37 (4), INV-32; *Accepted*): credencial **incrustada** en un texto bajo una
  clave inocua (el hueco que señaló la última revisión del PR #8). Se escriben primero y **fallan
  hoy** (la validación solo mira el valor entero), salvo la tabla "sin falsos positivos", que ya
  pasa y fija el comportamiento. `raw` es el de `securetoken.New()`.

  Integración, con el mismo esquema que los casos de la sexta revisión (`InTenantTx(A)` válido,
  `COMMIT` ignorando el error de `Record`):

  | `Data` | Esperado |
  |---|---|
  | `{"note": "see https://crm.example/reset-password#token=" + raw}` (el caso de la revisión) | `ErrSecretInData`; el error nombra `note` y no contiene `raw`, `token=` ni `://`; 0 filas después del `COMMIT` |
  | `{"items": []any{map[string]any{"note": "/accept-invitation#token=" + raw}}}` | `ErrSecretInData` con la ruta `items[0].note` |

  Unitarios (sin Docker; `package audit`, sobre la función no exportada que valida `Data`; un
  `map[string]any{"note": valor}` por fila salvo que diga otra cosa):

  | Valor | Esperado |
  |---|---|
  | `"token=valor-secreto-de-prueba-1"` | `ErrSecretInData` (clave incrustada; el valor no tiene forma de token) |
  | `"x reset_token = valor-secreto-de-prueba-2"` | `ErrSecretInData` (espacios antes del `=`) |
  | `"Authorization: Bearer valor-secreto-de-prueba-3"` | `ErrSecretInData` |
  | `"Cookie: __Host-crm_session=valor-secreto-de-prueba-4"` | `ErrSecretInData` |
  | `"X-API-Key:valor-secreto-de-prueba-5"` / `"pin: 1234"` | `ErrSecretInData` / `ErrSecretInData` (`pin` va por igualdad) |
  | `"reintentar con " + raw + " más tarde"` / `"(" + raw + ")"` | `ErrSecretInData` (secuencia de 43 incrustada) |
  | `{"fields": []string{"name", "ver " + raw}}` | `ErrSecretInData` con la ruta `fields[1]` |
  | En todas las filas anteriores | el texto del error no contiene `valor-secreto-de-prueba-<n>` ni `raw` |
  | **Sin falsos positivos**: `"password_reset_request"` (sin `=` ni `:`), `"Motivo: el cliente pidió anular el cobro de las 10:30"`, `"contraseña: cambiada"`, `"https://crm.example/settings?tab=users"`, `"sessions_revoked=2"`, `uuid.New().String()`, 22 dígitos seguidos (forma de un CBU), 64 caracteres hexadecimales (forma de un SHA-256), 42 y 44 caracteres base64url seguidos | sin error |

  Sexta revisión (DD-38, INV-33). Integración, `Record` dentro de `InTenantTx(A)`; después del
  `COMMIT` se lee `user_agent` de la fila:

  | `Entry.UserAgent` | Esperado |
  |---|---|
  | 513 caracteres ASCII | `Record` sin error (hoy falla el `CHECK` y revierte la operación); se guardan los primeros 512 |
  | 600 veces `ñ` (2 bytes cada una) | 512 runas (`char_length = 512`), UTF-8 válido |
  | `"Mozilla/5.0 \xff\xfe x"` (bytes que no son UTF-8) | sin error; se guarda `"Mozilla/5.0 �� x"`: un `U+FFFD` por **cada byte** inválido (DD-38; texto precisado en la séptima revisión, sin cambio de comportamiento) |
  | `"a\x00b"` | se guarda `"ab"` |
  | `""` | `user_agent IS NULL` |

  Unitario de `httpx.NormalizeUserAgent` (sexta revisión):

  | Entrada | Salida |
  |---|---|
  | `""` / 512 ASCII / 513 ASCII | `""` / igual / los primeros 512 |
  | 511 ASCII + `"ñ"` + `"x"` (513 runas) | 512 runas, terminando en `ñ` (nunca medio carácter) |
  | `"\xff"` / `"a\x00b"` | `"�"` / `"ab"` |
  | (Séptima revisión) `"a\xff\xfeb"` / `"\xe2\x82x"` (carácter de 3 bytes truncado a 2) | `"a��b"` / `"��x"`: un `U+FFFD` por byte, no por tramo (no es `strings.ToValidUTF8`). Caracteriza el comportamiento actual: pasa sin cambios de código |
  | Propiedad, sobre 1000 entradas aleatorias de bytes | `utf8.ValidString(out)`, sin `\x00`, `utf8.RuneCountInString(out) <= httpx.MaxUserAgentRunes` |
- **Green**: pasa la tabla.

**T-B210 — Implementar `platform/audit`**. Sexta revisión: `containsSecret` se reemplaza por la
validación de DD-37 (tipos admitidos, clave normalizada a minúsculas sin separadores contra las dos
listas, valores `Bearer`/`Basic`/`Digest`, JWT y formato de `securetoken`), que corre **antes** de
mirar `tx` y devuelve los centinelas de plan §11.1 envueltos con la ruta
(`fmt.Errorf("audit: data.%s: %w", ruta, ErrSecretInData)`), nunca con el valor; `ErrTxRequired`
reemplaza al `errors.New` actual; `Record` guarda `httpx.NormalizeUserAgent(e.UserAgent)` (DD-38),
función nueva de `platform/httpx` junto con la constante `MaxUserAgentRunes`. Séptima revisión
(DD-37 (4), *Accepted*): además de las reglas de valor de (3), cada `string` se recorre buscando
**(a)** toda secuencia maximal de letras, dígitos, `_` o `-` seguida de `=` o `:` (con espacios
opcionales en el medio), que se evalúa con la misma regla de claves de (2) (`isCredentialKey`); y
**(b)** toda secuencia maximal de **exactamente 43** caracteres de `[A-Za-z0-9_-]` (maximal: el
carácter anterior y el siguiente, si existen, no son de ese conjunto). La regla de (3) "son
exactamente 43 caracteres" queda como caso particular de (b) y se puede reemplazar por ella sin
cambiar ningún caso existente. El error es el mismo de (3) (`audit: data.<ruta>: ErrSecretInData`),
sin el valor ni la clave incrustada. Un recorrido por runas alcanza; si se usa `regexp`, se compila
una vez a nivel de paquete.

**T-B211 [T] — Outbox y worker** · ADR-010, ADR-024, ADR-025, INV-09, INV-16, INV-28, INV-29, INV-30, INV-31, DD-35, plan §9.4
- **Red** (integración, `Handler` falso configurable, `Clock` falso; los *hooks* de test no
  exportados del `Dispatcher` pueden forzar fallos en `GetMessage` y en el marcado):

  | Caso | Esperado |
  |---|---|
  | `Enqueue` en `InTenantTx(A)` + `COMMIT`, luego un ciclo del `Dispatcher` | el fake recibe 1 email; fila `sent`, `payload IS NULL`, `sent_at` puesto |
  | `Enqueue` + `ROLLBACK` | el worker no envía nada |
  | Fake devuelve `DeliveryError{Cause: network, Phase: data, Detail: "timeout"}` (quinta revisión) | `pending`, `attempts=1`, `next_attempt_at = now + 1 min`, `last_error = "network data: timeout"`; log `WARN` `outcome=retry` con `error_cause=network` y `smtp_phase=data` |
  | (Quinta revisión, M6) `Enqueue` con el `Clock` falso | `created_at` y `next_attempt_at` de la fila son exactamente `clock.Now()` (sin compensaciones de +1 s en los tests) |
  | Errores recuperables sucesivos | backoff 1, 5, 15, 60 min, 6 h (y 6 h en adelante) |
  | 8.º error recuperable | `failed`, `payload IS NULL`, `failed_at` puesto; log `ERROR` `outcome=failed`, `reason=max_attempts` |
  | (Quinta revisión) `DeliveryError{config, connection, 535, "5.7.8", "Authentication credentials invalid"}` | `pending`, `attempts=1`, `last_error = "config connection 535 5.7.8: Authentication credentials invalid"`; log **`ERROR`** `outcome=retry`, `error_cause=config`, `smtp_code=535`; **no** pasa a `failed` |
  | (Quinta revisión) Dos mensajes vencidos; el fake falla el primero en `Phase: connection` (cualquier causa) | el ciclo termina después del primero: el fake recibió **una** llamada; el segundo sigue intacto y lo toma el ciclo siguiente |
  | (Quinta revisión) Dos mensajes vencidos; el fake falla el primero con `DeliveryError{transient, rcpt_to, 452, "4.2.2", "Mailbox full"}` | recuperable con log `WARN`; el ciclo **sigue** y el segundo se envía |
  | (Quinta revisión) `DeliveryError{recipient, rcpt_to, 550, "5.1.1", "<persona@example.com>: User unknown"}` | `failed` al primer intento, `payload IS NULL`; `last_error = "recipient rcpt_to 550 5.1.1: [redacted] User unknown"`; log `WARN` `outcome=failed`; ni `last_error` ni el log contienen `@` |
  | (Quinta revisión) `DeliveryError{bug, compose, Detail: "unknown email template"}` | `failed` al primer intento; log **`ERROR`** |
  | (Quinta revisión) El fake devuelve `errors.New("x")` (no es `DeliveryError`) | recuperable con `last_error = "config unknown: unclassified handler error"`; log `ERROR` con `err_type`, sin el texto `x` |
  | (Quinta revisión) El fake devuelve `context.DeadlineExceeded` con el contexto del ciclo vivo | recuperable, `last_error = "network unknown: timeout"` |
  | (Quinta revisión) El fake captura el `ctx` que recibe | tiene *deadline* ≤ `now + outbox.SendBudget` (20 s) |
  | (Quinta revisión) Payload con JSON inválido (lo escribe el test como dueño de la tabla) | `failed`, `last_error = "bug compose: invalid payload"`; log **`ERROR`** (hoy sale en `WARN`) |
  | Mensaje con `next_attempt_at` futuro | no se toma |
  | Dos `Dispatcher` concurrentes con 20 mensajes | cada mensaje se envía **una** vez (`FOR UPDATE SKIP LOCKED`) |
  | Mensajes de `A` y `B` | cada uno se lee y marca bajo el rol de su empresa (`current_user` capturado por el fake vía hook de test) |
  | `UPDATE` que deja `sent` con payload | la base lo rechaza (`outbox_scrub_chk`) |
  | (Tercera revisión) El fake bloquea hasta que se cancela el contexto del `Dispatcher` y devuelve `context.Canceled` (apagado a mitad del envío) | el mensaje queda **exactamente** como estaba: `pending`, `attempts`, `next_attempt_at` y `last_error` sin cambios; log `INFO` con `outcome=canceled`; ningún log `ERROR`; en el próximo arranque se toma de nuevo |
  | (Quinta revisión, DD-35) El fake bloquea hasta que se cancela el contexto del `Dispatcher` y después devuelve **`nil`** (el envío terminó durante el apagado) | el mensaje queda `sent` (la transacción usa `context.WithoutCancel`); no se reenvía en el próximo arranque |
  | (Quinta revisión, DD-35) El contexto del `Dispatcher` ya está cancelado antes del ciclo | no toma ningún mensaje |
  | (Quinta revisión, INV-31) Mensaje viejo de una empresa `X` cuya fila de `tenants` existe pero su rol no (el test borra `crm_t_<X>` como superusuario del contenedor) + mensaje más nuevo de `A` | en el **primer** ciclo el de `A` queda `sent`; el de `X` sigue `pending`, con `attempts` y `last_error` sin cambios y `next_attempt_at = now + 10 s` (edad < 10 s); log `ERROR` `outcome=deferred`, `step=as_tenant`, con `err` |
  | (Quinta revisión) *Hook* que hace fallar `GetMessage` para un mensaje | aplazado como arriba, `step=get_message`; el lote sigue |
  | (Quinta revisión) *Hook* que hace fallar `MarkSent` con un error que no es `db.ErrUnavailable` | el mensaje queda `pending` y aplazado (`step=mark`); durante ese ciclo el fake recibió **una** sola llamada por ese mensaje |
  | (Quinta revisión) *Hook* que hace fallar `AsTenant` con `db.ErrUnavailable` | el ciclo termina; el mensaje **no** se aplaza (`next_attempt_at` sin cambios); log `ERROR` `event=outbox_cycle_failed` con `err` |
  | (Quinta revisión) Aplazar un mensaje que otro `Dispatcher` ya marcó `sent`, o cuyo `next_attempt_at` cambió | 0 filas, sin error |
  | (Quinta revisión) `Run` con un ciclo que falla | el log `outbox_cycle_failed` incluye `err` |
  | (Quinta revisión, INV-29) `SHOW idle_in_transaction_session_timeout` en una conexión de `crm_app` | ≥ `outbox.SendBudget + 10 s` |

  Unitarios (quinta revisión):

  | Caso | Esperado |
  |---|---|
  | `deferDelay(edad)` con edad −5 s / 0 / 9 s / 12 s / 14 min / 20 min | 10 s / 10 s / 10 s / 12 s / 14 min / 15 min |
  | `DeliveryError.LastError()` con y sin código, con y sin código extendido, y fase `unknown` | el formato exacto de ADR-024 §3 |
  | `Detail` con `\r\n`, tabulaciones y espacios repetidos | una sola línea, espacios simples, sin espacios en los extremos |
  | `Detail` con `<a@b.example>`, `a@b.example,` y `"a@b.example"` | cada token → `[redacted]`; el resultado no contiene `@` |
  | (Sexta revisión, ADR-025 §2) `Detail` con `https://crm.example/reset-password#token=<43 caracteres base64url>` | ese campo → `[redacted]`; el resultado no contiene `token=`, `://` ni el valor |
  | `Detail` con `#token=<43>` (sin esquema), con un token suelto de 43 caracteres seguido de `.`, y con un JWT | cada campo → `[redacted]` |
  | `Detail` `"id 0123456789abcdefghi"` (19 seguidos) y `"id 0123456789abcdefghij"` (20) | sin cambios / `"id [redacted]"` |
  | `Detail` `"Authentication credentials invalid"`, `"client host blocked"`, `"Mailbox full"` | sin cambios |
  | (Sexta revisión, ADR-025 §1) `DeliveryError{config, data, 554, "5.7.1", Detail: "Message rejected, URL https://crm.example/x#token=abc listed"}` | `LastError() = "config data 554 5.7.1: response text omitted"` |
  | `DeliveryError{recipient, data, 550, "5.1.1", Detail: "Recipient rejected"}` / `DeliveryError{network, data, 0, "", Detail: "timeout"}` | `"recipient data 550 5.1.1: response text omitted"` / `"network data: timeout"` (sin código SMTP, el detalle del vocabulario fijo se conserva) |
  | `Detail` de 2000 caracteres multibyte | exactamente 1000 runas, terminando en `…` |
  | `err.Error()` de un `*DeliveryError` | igual a `LastError()` |
  | `errors.As` sobre `fmt.Errorf("x: %w", deliveryErr)` | recupera el `*DeliveryError` |
  | `Cause.Permanent()` y `Cause.LogLevel()` para las 5 causas | `recipient` y `bug` definitivos; `config` y `bug` en `ERROR`; el resto `WARN` |
- **Green**: pasa la tabla. **Refactor**: backoff, `deferDelay` y la decisión "cancelación /
  recuperable / definitivo / aplazar / terminar el ciclo" como funciones puras con su propio test
  (plan §9.4); un único lugar arma los campos de log del worker.

**T-B212 — Implementar `platform/outbox` (`Enqueue`, `Dispatcher` con *polling* de 2 s y lote de 10, arranque y parada con `context`)**. Las queries del worker van en `internal/platform/outbox/store/worker.sql` (plan §4.4). Quinta revisión: `DeliveryError`, `Cause`, `Phase` y `SendBudget` según plan §9.3 (se borra `PermanentError`); `Enqueue` recibe el `clock.Clock` y fija `created_at` y `next_attempt_at` con `clock.Now()` (DD-18; resuelve M6); `LockDueMessage` devuelve también `next_attempt_at` y `created_at`; query `DeferMessage` (plan §9.4) y su nombre en `queryrules.DefaultExceptions`; `deferDelay`; transacción con `context.WithoutCancel` y `Handle` con `context.WithTimeout(…, SendBudget)` (DD-35). Sexta revisión (ADR-025 §1–§2): `LastError()` usa el detalle fijo `response text omitted` cuando `Phase == PhaseData && SMTPCode != 0`, y `sanitizeDetail` redacta además los campos con `://` o con 20 o más caracteres seguidos de `[A-Za-z0-9+/=_-]`.

**T-B213 [T] — Adaptador SMTP y plantillas** · ADR-010, ADR-024, ADR-025, DD-14, DD-24, DD-35, INV-28, INV-29, INV-30
- **Red** (integración contra Mailpit en contenedor; su API HTTP para leer lo recibido):

  | Caso | Esperado |
  |---|---|
  | Enviar `password_reset` con `{link}` y `APP_BASE_URL=https://crm.example` | llega a Mailpit con asunto en español, partes texto y HTML, enlace `https://crm.example/reset-password#token=…` (token en el **fragmento**, DD-14) |
  | Plantillas `email_verification` e `invitation` | enlaces `…/verify-email#token=…` y `…/accept-invitation#token=…`; nombre de empresa y rol en español ("Administrador"/"Operador"); sin campos vacíos |
  | `APP_BASE_URL=https://localhost:5173` | enlaces a `https://localhost:5173/…` (desarrollo con Vite y HTTPS local, plan §10.5.1) |
  | Destinatario con `\r\n` | `DeliveryError{recipient, compose}` antes de conectar |
  | SMTP inalcanzable (puerto cerrado) | `DeliveryError{network, connection}`, `LastError() = "network connection: connection refused"` |
- **Red** (quinta revisión, ADR-024; integración contra un **servidor SMTP falso en proceso**: un
  `net.Listener` que responde un guion fijo y registra los comandos recibidos; responde `250` a
  `NOOP`. Para los casos de AUTH, el servidor anuncia `AUTH CRAM-MD5` sin TLS, porque go-mail sin
  cifrado solo autodescubre SCRAM, NTLM o CRAM-MD5, y el test arma el adaptador con usuario y sin
  TLS con un constructor no exportado para tests; en producción la política TLS no cambia):

  | Guion del servidor falso | Esperado (`LastError()` del `*DeliveryError`) |
  |---|---|
  | Saludo `554 5.7.1 client host blocked` | `config connection 554 5.7.1: client host blocked`; `Cause.Permanent() == false` (el caso de la revisión) |
  | Saludo `421 4.3.2 Service not available` | `transient connection 421 4.3.2: Service not available` |
  | `EHLO` y `HELO` responden `550 not allowed` | causa `config`, fase `connection`, código `550` |
  | `AUTH` responde `535 5.7.8 Authentication credentials invalid` | `config connection 535 5.7.8: Authentication credentials invalid` (el caso de la revisión) |
  | `AUTH` responde `454 4.7.0 Temporary authentication failure` | `transient connection 454 4.7.0: Temporary authentication failure` |
  | Con usuario configurado y sin `AUTH` anunciado | causa `config`, fase `connection`, código 0, sin `@` |
  | `MAIL FROM` responde `550 5.7.1 Sender not authorized` | `config mail_from 550 5.7.1: Sender not authorized` |
  | `MAIL FROM` responde `451 4.3.0 Try again later` | `transient mail_from 451 4.3.0: Try again later` |
  | `RCPT TO` responde `550 5.1.1 <destinatario del mensaje>: User unknown`, con `ENHANCEDSTATUSCODES` anunciado | `recipient rcpt_to 550 5.1.1: [redacted] User unknown`; `Permanent() == true`; sin `@` |
  | `RCPT TO` responde `550 User unknown`, sin `ENHANCEDSTATUSCODES` | `recipient rcpt_to 550: User unknown` |
  | `RCPT TO` responde `554 5.7.1 Relay access denied` | `config rcpt_to 554 5.7.1: Relay access denied` (recuperable) |
  | `RCPT TO` responde `452 4.2.2 Mailbox full` | `transient rcpt_to 452 4.2.2: Mailbox full` |
  | Fin de datos responde `554 5.7.1 Message rejected` | `config data 554 5.7.1: response text omitted` (sexta revisión, ADR-025 §1: en la fase `data` no se usa el texto del proveedor) |
  | Fin de datos responde `550 5.1.1 Recipient rejected` | `recipient data 550 5.1.1: response text omitted` |
  | Fin de datos `250` y después cierra la conexión sin responder el `QUIT` | `nil` (entregado: `IsDelivered()`) |
  | Fin de datos `250` | el comando siguiente registrado es `QUIT`, no `RSET` (`WithoutRset`) |
  | Respuesta `5xx` de 3 líneas, 2000 caracteres y una dirección con `@` en el medio | `LastError()` en una línea, ≤ 1000 caracteres, sin `@` |
  | Acepta la conexión TCP y nunca manda el saludo | `network connection: timeout`, en ≤ 6 s (5 s por etapa + tolerancia) |
  | Responde bien hasta `DATA` (`354`) y nunca responde al fin de datos | `network data: timeout`, en ≤ 6 s después de enviar el contenido |
  | Contexto cancelado antes de llamar | `context.Canceled` (`errors.Is`), sin conectar |
  | Contexto cancelado mientras el servidor demora el saludo | cuando `Send` vuelve, devuelve `context.Canceled`, no un `DeliveryError` |
  | (Sexta revisión, DD-35) Contexto con *deadline* que vence mientras el servidor demora el saludo | `Send` devuelve `context.DeadlineExceeded` (`errors.Is`), no un `DeliveryError` (caracteriza el comportamiento actual: puede pasar sin cambios de código) |
  | (Sexta revisión, ADR-025 §3) `RCPT TO` responde `550 5.7.1 Relay access denied`, **sin** `ENHANCEDSTATUSCODES` | `config rcpt_to 550 5.7.1: Relay access denied`; `Permanent() == false` (hoy sale `recipient`) |
  | `RCPT TO` responde `550 5.1.1 <destinatario del mensaje>: User unknown`, sin `ENHANCEDSTATUSCODES` | `recipient rcpt_to 550 5.1.1: [redacted] User unknown` |
  | `RCPT TO` responde `554 5.1.1 Unknown user`, sin `ENHANCEDSTATUSCODES` | `recipient rcpt_to 554 5.1.1: Unknown user` |
  | `RCPT TO` responde `550 4.2.2 Mailbox full` (clase incoherente), sin `ENHANCEDSTATUSCODES` | `recipient rcpt_to 550: 4.2.2 Mailbox full` (el extendido se ignora; decide el `550`) |
  | Saludo `554 4.7.1 blocked` (clase incoherente) | `config connection 554: 4.7.1 blocked` |
  | (Sexta revisión, ADR-025 §1) Fin de datos responde `554 5.7.1 Message rejected, URL <el enlace del mensaje, con #token=…> listed` | `config data 554 5.7.1: response text omitted`; `LastError()` no contiene el token ni `token=` |
  | (Sexta revisión, ADR-025 §2) `MAIL FROM` responde `550 5.7.1 Sender not verified, see https://provider.example/help?id=1` | `config mail_from 550 5.7.1: Sender not verified, see [redacted]` |

  Unitarios (quinta revisión):

  | Caso | Esperado |
  |---|---|
  | Constantes del adaptador: 4 × timeout por etapa | ≤ `outbox.SendBudget` |
  | Regla del destinatario (fase, código, extendido): `rcpt_to 550 ""`, `rcpt_to 551 ""`, `rcpt_to 553 ""`, `rcpt_to 554 ""`, `rcpt_to 550 5.1.1`, `rcpt_to 550 5.2.2`, `rcpt_to 550 5.7.1`, `data 550 5.1.1`, `data 550 ""`, `mail_from 550 5.1.1`, `connection 550 5.1.1` | `recipient` solo para `rcpt_to 550 ""`, `rcpt_to 551 ""`, `rcpt_to 553 ""`, `rcpt_to 550 5.1.1`, `rcpt_to 550 5.2.2` y `data 550 5.1.1`; todos los demás `config` |
  | Errores de la fase de conexión: `net.Error` con `Timeout()`, `*net.DNSError`, `ECONNREFUSED`, `io.EOF`, error de certificado x509 | `network` (`timeout`, `dns lookup failed`, `connection refused`, `connection closed`) y `config` (`tls handshake failed`) |
  | Handler de plantillas con plantilla desconocida, token faltante o rol inválido | `DeliveryError{bug, compose}` con texto fijo; el texto no contiene el token |
  | (Sexta revisión, ADR-025 §3) Lectura del código extendido del texto, `(código, texto)` → `(extendido, resto)`: `(550, "5.1.1 User unknown")`, `(452, "4.2.2 Mailbox full")`, `(550, "4.2.2 Mailbox full")`, `(550, "User unknown 5.1.1")`, `(550, "5.1.1")`, `(550, "5.1.1234 x")`, `(550, "")` | `("5.1.1", "User unknown")`, `("4.2.2", "Mailbox full")`, `("", "4.2.2 Mailbox full")`, `("", "User unknown 5.1.1")`, `("5.1.1", "")`, `("", "5.1.1234 x")`, `("", "")` |
- **Green**: pasa la tabla.

**T-B214 — Implementar `platform/mailer` (go-mail) y las plantillas `identity/emails` (es-AR)**. Quinta revisión: clasificación de ADR-024 §1–§3 en el adaptador (`gomail.WithTimeout(5 * time.Second)`, `gomail.WithoutRset()`, `nil` si `Msg.IsDelivered()`); el detalle de un `SendError` se arma como dice ADR-024 §3 (sin `affected recipient(s)`); destinatario inválido → `recipient`, asunto inválido → `bug`, remitente inválido → `config`, todos en fase `compose`; el handler de plantillas devuelve `DeliveryError{Cause: CauseBug, Phase: PhaseCompose}` con textos fijos. Sexta revisión (ADR-025 §3, DD-35): una sola función lee el código extendido del comienzo del texto (forma `d.d.d` seguida de espacio o fin, y misma clase que el código básico) y la usan la fase `connection` y `classifySendError` cuando `EnhancedStatusCode()` está vacío; el comentario de `Mailer.Send` que señala la diferencia con DD-35 se reemplaza por una referencia a DD-35. La omisión del texto en la fase `data` y la redacción ampliada viven en `outbox.DeliveryError.LastError()` (T-B212), no en el adaptador.

**T-B215 [T] — Adaptador S3** · ADR-011, DD-36
- **Red** (integración contra MinIO en contenedor, con la imagen `containers.MinIO` de DD-36):

  | Caso | Esperado |
  |---|---|
  | `Put` + `Get` | mismos bytes y `ContentType` |
  | `Get` de clave inexistente | `objectstore.ErrNotFound` |
  | `Delete` de clave inexistente | sin error (idempotente) |
  | Endpoint caído | error envuelto clasificado como no disponible |
  | (Quinta revisión) Test unitario de `internal/testsupport/containers` que lee `compose.yaml` | el `image:` de los servicios `postgres`, `mailpit` y `minio` es exactamente `containers.Postgres`, `containers.Mailpit` y `containers.MinIO` |
  | (Quinta revisión) `containers.MinIO` | contiene `ghcr.io/coollabsio/minio:RELEASE.2025-10-15T17-29-55Z@sha256:`; ningún archivo del repo fuera de la documentación nombra `quay.io/minio`, `minio/minio:` ni `bitnamilegacy/` |
- **Green**: pasa la tabla.

**T-B216 — Implementar `platform/objectstore` (minio-go)**. Quinta revisión (DD-36): paquete `internal/testsupport/containers` con las constantes `Postgres` (`postgres:18`), `Mailpit` (`axllent/mailpit:v1.27`) y `MinIO` (`ghcr.io/coollabsio/minio:RELEASE.2025-10-15T17-29-55Z@sha256:<digest>`); `pgtest`, los dos tests que levantan Mailpit y el de MinIO usan esas constantes; `compose.yaml` usa las mismas referencias, mantiene `command: server /data --console-address ":9001"` y pierde el comentario sobre `quay.io`. El digest es el del **índice multiplataforma** de ese tag (lo obtiene el desarrollador con `docker buildx imagetools inspect`). Si ese tag no se puede descargar sin login, no se elige otra imagen por cuenta propia: se usa el respaldo de DD-36 (`cgr.dev/chainguard/minio` por digest) y se avisa al arquitecto para actualizar DD-36 y la nota de ADR-011. (Séptima revisión: este ajuste **no** se aplicó en la rama de la Fase 2; por decisión del usuario del 2026-10-02 se aplica, obligatorio, al comienzo de la Fase 3: paso 4 de la tabla de "Estado de la implementación".)

**T-B217 [T] — Rate limiter** · DD-9, DD-32, INV-25
- **Red** (unitario, reloj falso): 5 pedidos permitidos y el 6.º rechazado con `RetryAfter > 0`;
  claves independientes; recarga con el tiempo; limpieza de claves inactivas (sin crecer sin
  límite). Además (tercera revisión):

  | Caso | Esperado |
  |---|---|
  | `ratelimit.IPKey(192.0.2.7)` | `192.0.2.7/32` |
  | `IPKey(2001:db8:1:2:aaaa::1)` y `IPKey(2001:db8:1:2:bbbb::9)` | el mismo prefijo `2001:db8:1:2::/64` |
  | `IPKey(2001:db8:1:2::1)` y `IPKey(2001:db8:1:3::1)` | prefijos distintos |
  | `IPKey(::ffff:192.0.2.7)` | igual a `IPKey(192.0.2.7)` |
  | Middleware de rate limit (httptest con `ClientIP` delante): 5 requests desde `2001:db8:1:2::1` y el 6.º desde `2001:db8:1:2::ffff` | el 6.º recibe `429` (mismo /64) |
  | 5 requests desde `2001:db8:1:2::1` y 1 desde `2001:db8:1:3::1` | el último pasa (otro /64) |
  | Requests con el mismo `RemoteAddr` de un proxy de confianza y `X-Forwarded-For` de dos clientes distintos | cupos independientes (la clave sale de `ClientIPFrom`, nunca de `RemoteAddr`) |
- **Green**: pasa la tabla.

**T-B218 — Implementar `platform/ratelimit` (con `IPKey`) y el middleware por ruta** (`429
rate_limited` + `Retry-After`). La clave por IP es `IPKey(httpx.ClientIPFrom(ctx))` (DD-32). El
middleware consume el cupo **antes** de ejecutar el handler, así cuentan tanto los éxitos como
los rechazos (DD-9).

**Checkpoint Fase 2**: `make check` en verde. Quinta revisión: además, `docker compose up -d` levanta MinIO con la imagen de DD-36 sin login a ningún registro.

---

### Fase 3 — Historia 1: Registrar una empresa (P1) — primer slice vertical

**Objetivo**: un visitante se registra, queda logueado y ve su sesión en `/me`; el email de
verificación llega a Mailpit.

**Prueba independiente**: `POST /api/v1/auth/signup` → `201` + cookie → `GET /api/v1/me` con esa
cookie → `200` con rol `admin`, los 15 permisos y `email_verified: false`; en Mailpit hay un email
de verificación. Repetir el registro con el mismo email → `409 email_already_registered` con
`suggested_action: password_reset`.

Antes de T-B301: los cuatro ajustes de la séptima revisión (tabla "Fase 3" de "Estado de la
implementación"), en el orden de esa tabla.

**T-B311 [T] — Validador del contrato para los tests HTTP** · ADR-012, ADR-014 (nota 2026-10-02), principio VI, DD-17
(Numeración agregada en la séptima revisión, *Accepted* (aprobada por el usuario el 2026-10-02); se
hace **antes** de T-B301, porque T-B301, T-B305 y T-B309 validan contra el contrato con este
helper. Cierra el hueco diferido de T-B201/T-B004.)
- **Red** (unitario, sin Docker, en `internal/testsupport/contract`; servidores y respuestas de
  prueba armados a mano con `httptest`, sin el mux real; los casos usan los métodos que devuelven
  `error`, así no hace falta un `testing.TB` falso):

  | Caso | Esperado |
  |---|---|
  | `contract.Load()` desde el directorio del paquete | encuentra la raíz del módulo subiendo hasta el `go.mod`, carga `specs/001-empresas-usuarios/contracts/openapi.yaml` y `ValidateDocument()` no reporta errores (el contrato v0.4.1 es OpenAPI 3.1 válido) |
  | `GET /api/v1/industry-templates` → `200` `application/json` con un `items` válido | `CheckResponse` devuelve `nil` (el prefijo `/api/v1` de `servers` se quita para encontrar la operación) |
  | La misma respuesta con una propiedad de más en un item | error que nombra la propiedad (el esquema tiene `additionalProperties: false`) |
  | `POST /api/v1/auth/signup` → `409` `application/problem+json` con un cuerpo válido de `EmailAlreadyRegisteredProblem` | `nil`; el mismo cuerpo sin `suggested_action` → error |
  | `POST /api/v1/auth/signup` → `503` problem+json `service_unavailable` con `Retry-After: 2` | `nil` (la operación declara `5XX`); con `Retry-After: 0` → error (`minimum: 1`) |
  | `POST /api/v1/auth/signup` → `413` problem+json `payload_too_large` | `nil` (declarado desde el contrato v0.4.1); el mismo `413` en `GET /api/v1/industry-templates` → error (no declarado: esa operación no tiene body) |
  | `POST /api/v1/auth/signup` → `418` | error: status no declarado para la operación |
  | `POST /api/v1/auth/signup` → `409` con `Content-Type: application/json` | error: tipo de contenido no declarado para ese status |
  | `POST /api/v1/auth/logout` → `204` sin cuerpo | `nil` |
  | `DELETE /api/v1/industry-templates` → `405` problem+json `method_not_allowed` con `Allow: GET` | `nil` (respuesta global, contra `Problem`); sin `Allow` → error; con status `200` → error (método fuera del contrato) |
  | `GET /api/v1/no-existe` → `404` problem+json `not_found` | `nil` (respuesta global, contra `Problem`); un cuerpo sin `code` → error; con status `200` → error (ruta fuera del contrato) |
  | `CheckSchema("ValidationProblem", body)` con `errors` de `{field, code}` válidos / con un `code` fuera de `FieldErrorCode` | `nil` / error |
  | `CheckSchema("NoExiste", body)` | error que nombra el esquema |
  | Después de `CheckResponse` y de `CheckRequest` | el test puede leer los cuerpos completos (el helper los restituye) |
  | `CheckResponse`, `CheckRequest` y `Transport` reemplazan un body original | cierran el `ReadCloser` original después de copiarlo, también si la lectura falla; la copia sigue disponible para el caller |
  | `Transport` sobre un servidor de prueba: `POST /api/v1/auth/signup` con un cuerpo válido → `201` con `SessionInfo` y `Set-Cookie` | la función de reporte no se llamó |
  | `Transport`: el servidor de prueba acepta con `201` un request con un campo de más | se reporta un error **del request** (un `2xx` exige un request válido) |
  | `Transport`: el mismo request con campo de más → `400 malformed_request` | sin error del request (un no-`2xx` no valida el request); la respuesta sí se valida |
  | 20 validaciones concurrentes sobre el mismo `Validator` (`t.Parallel`) | sin carreras con `-race` |
- **Green**: pasa la tabla.
- **Refactor**: una sola función decide "operación del contrato o respuesta global (`404`/`405`)";
  los mensajes de error incluyen método, ruta y status.

**T-B312 — Implementar `internal/testsupport/contract`** (firmas en plan §11.1; séptima revisión,
*Accepted*).
- Dependencias: `github.com/pb33f/libopenapi` y `github.com/pb33f/libopenapi-validator`, ya
  elegidas en ADR-012 y ADR-014; la última versión estable al implementar (la de
  `libopenapi-validator` era v0.15.0, publicada el 2026-09-29) fijada en `go.mod` y reportada en
  el PR. Solo las importa este paquete: regla `depguard` nueva en `.golangci.yml` que prohíbe
  `github.com/pb33f/libopenapi` (y sus subpaquetes) en todo el repo salvo
  `**/internal/testsupport/contract/**`.
- Construcción: `libopenapi.NewDocument` + `validator.NewValidator(doc,
  config.WithFormatAssertions())`. Sin `WithStrictServerMatching` (el `servers` relativo `/api/v1`
  se resuelve quitando el prefijo), sin `WithoutResponseStatusValidation` y sin
  `WithoutSecurityValidation`. El documento se carga una vez por proceso (`sync.OnceValues`) y las
  validaciones se serializan con un `sync.Mutex`: no se asume que el validador sea seguro para uso
  concurrente.
- `CheckResponse`: lee, cierra el body original y restituye `resp.Body` (incluso cuando falla la
  lectura); si la librería informa que la ruta o el método no
  están en el contrato y el status es `404` o `405` con `application/problem+json`, valida el
  cuerpo con `CheckSchema("Problem", …)` y, en el `405`, exige `Allow`; en cualquier otro caso
  devuelve los errores de la librería unidos.
- `Transport`: copia y cierra el cuerpo original del request antes de enviarlo; con la respuesta, `CheckResponse`
  siempre y `CheckRequest` (sobre un clon con la copia del cuerpo) solo si el status es `2xx`.
  `Wrap(t, client)` reemplaza `client.Transport` por `Transport(client.Transport, t.Errorf…)`.
- `CheckSchema`: valida contra `components/schemas/<nombre>` con el validador de esquemas de la
  misma librería (paquete `schema_validation`).
- Si una respuesta que cumple el contrato falla por una limitación de la librería (p. ej. un
  `format`), **no** se apaga la opción ni se agrega una excepción: se frena y se avisa al
  arquitecto con el caso.
- **Ajuste de T-B201 y T-B004** (hueco diferido; tercer paso de la tabla de la Fase 3): en
  `internal/platform/httpx`, cada `WriteProblem` de T-B201 (incluido el `413` de un body JSON de
  más de 65 536 bytes) se valida con `CheckSchema("Problem")`, el `422` con
  `CheckSchema("ValidationProblem")` y el `409` de registro con
  `CheckSchema("EmailAlreadyRegisteredProblem")`; en `internal/app`, los `404` y `405` de T-B004
  con `CheckResponse`. `TestProblemCodesMatchContract` (`httpx`) y el test de `Permission` de
  `authz` se mantienen: controlan los enums sin armar respuestas (`payload_too_large` sigue en el
  enum `ErrorCode` y en `httpx.CodePayloadTooLarge`; el contrato v0.4.1 no cambia el enum).

**T-B301 [T] — Catálogo de plantillas y `GET /industry-templates`** · FR-002, DD-3
- **Red**:

  | Caso | Esperado |
  |---|---|
  | `industrytemplate.All()` | contiene al menos `aluminum_carpentry` ("Carpintería de aluminio") y `generic` ("Genérico"), cada uno con versión ≥ 1 |
  | `Lookup("inexistente")` | `false` |
  | `GET /api/v1/industry-templates` | `200`, valida contra el contrato (`contract`, T-B311), sin cookie |
- **Green**: pasa la tabla. El catálogo se lee de un archivo de datos embebido, no de constantes
  por rubro en el código (principio II).

**T-B302 — Implementar `internal/industrytemplate` (catálogo + `Seeder` sin efecto) y el handler**.
Séptima revisión: el catálogo es `internal/industrytemplate/catalog.json`, embebido con
`//go:embed` y leído con `encoding/json` (`DisallowUnknownFields`) una sola vez; no se usa YAML en
código de producción. Un catálogo inválido (código vacío o repetido, nombre vacío, versión < 1)
hace fallar un test unitario del paquete, y en el binario produce un *panic* al primer uso (es un
error de compilación del catálogo, no de runtime).

**T-B303 [T] — Servicio de registro** · US-1, FR-001, FR-002, FR-008, INV-14, INV-16, INV-26, INV-32, INV-33, DD-2, DD-4, DD-13, DD-15, DD-27, DD-33, DD-37, DD-38, H-6
- **Red** (integración, `Mailer` real no interviene: se verifica el outbox):

  | Caso | Esperado |
  |---|---|
  | Datos válidos | Una empresa con `base_currency`, `timezone` (del request o default), `industry_template_code/version`; rol `crm_t_<hex>` creado; un usuario `admin`/`active` con email normalizado, hash argon2id y `email_verified_at` nulo; un token `email_verification` (48 h); un mensaje `email_verification` pendiente; una sesión; auditoría `tenant.registered` |
  | Email con mayúsculas y espacios | se guarda en minúsculas y sin espacios |
  | Email ya existente en otra empresa, con el usuario en cada estado (`invited`, `active`, `disabled`) | `identity.ErrEmailTaken` en los tres casos; **no** queda empresa, usuario, token, mensaje, sesión ni **rol** nuevos |
  | Dos registros concurrentes con el mismo email | exactamente uno tiene éxito; el otro `ErrEmailTaken` |
  | Plantilla inexistente | error de validación `unknown_template`; nada creado |
  | `Seeder` falso que falla | error; nada creado (incluido el rol) |
  | `timezone` `America/Argentina/Cordoba` / `Asia/Kathmandu` | se guarda tal cual (la base IANA embebida las conoce) |
  | `timezone` ausente, vacía, `Marte/Olympus`, de 64 caracteres inventados o `Local` (décima revisión, DD-27 y DD-43: `time.LoadLocation` acepta `Local` pero no es un nombre IANA; hoy se guarda `Local`) | la empresa se crea con `America/Argentina/Buenos_Aires`; log `event=signup_timezone_defaulted`; **sin** error (DD-27) |
  | `base_currency` fuera de ARS/USD | validación `invalid_value` |
  | Contraseña de 9 caracteres o igual al email | validación; **no** se calcula hash ni se abre transacción |
  | (Tercera revisión, DD-33) **10 registros concurrentes** con emails distintos | los 10 tienen éxito (ninguno `ErrUnavailable`): 10 empresas y 10 roles |
  | Una transacción de prueba (superusuario del contenedor) ejecuta `GRANT crm_tenant TO <rol de prueba>` y queda abierta 3 s; mientras, un `Register` | `Register` devuelve un error con `errors.Is(err, db.ErrUnavailable)` en ~2 s (entre 2 y 3 s; **nunca** los 5 s de `statement_timeout` ni un `ErrPrivilege`); no queda empresa, usuario, token, mensaje, sesión ni **rol** nuevos; después del `ROLLBACK` de la transacción de prueba, el mismo `Register` tiene éxito |
  | Dentro de la transacción de registro (observado con el `Seeder` falso como *hook*) | `current_setting('lock_timeout')` = `2s` |
  | Durante todo `Register` | los fakes de `Mailer` y `ObjectStorage` no registran **ninguna** llamada (el email sale por el outbox; R-b de DD-33) |
  | (Sexta revisión, DD-38; precisado en la séptima) `Register` con `RequestMeta.UserAgent` de 600 caracteres que incluye un byte `\xff` y un `\x00` | éxito; `sessions.user_agent` y el `audit_log.user_agent` de `tenant.registered` son **iguales** a `httpx.NormalizeUserAgent(entrada)`: 512 runas, UTF-8 válido, sin `NUL` |
  | (Séptima revisión, DD-38) `RequestMeta.UserAgent = ""` | `sessions.user_agent IS NULL` y `audit_log.user_agent IS NULL` |
  | (Séptima revisión, INV-32, catálogo de `data-model.md` §2.6) Datos válidos; se leen las filas de `audit_log` de la empresa nueva en `InTenantTx` | exactamente **una** fila: `action = tenant.registered`, `actor_user_id` = el admin creado, `target_type = tenant`, `target_id` = la empresa, `ip` = `RequestMeta.IP`, `request_id` = el del contexto (el test lo obtiene pasando por `httpx.RequestID`, como T-B209); `data` decodificado como `map[string]any` es **igual** a `{"industry_template_code": <código del request>}`: mismas claves y ninguna de más (se compara el mapa, no el texto JSON). `CreateSession` no audita (`auth.login_succeeded` es de `Login`, Fase 4) |
  | (Séptima revisión, INV-32) La misma fila | ni `data::text` ni `user_agent` contienen el `RawToken` de la sesión, el token de verificación en claro (leído del `payload` del mensaje pendiente), la contraseña ni el email |
  | (Séptima revisión, DD-37) `Register` con cada plantilla de `industrytemplate.All()` | éxito en todas; `data.industry_template_code` = su código (ningún código del catálogo choca con la política de DD-37) |
- **Green**: pasa la tabla.
- **Refactor**: el servicio de `tenant` no conoce SQL de `identity`; solo usa la interfaz
  `AdminOnboarding` (plan §11.1). Séptima revisión: la lectura de las filas de `audit_log` de una
  empresa queda como función del test; cuando la Fase 4 también la necesite, se mueve a
  `internal/testsupport/audittest` (no antes).

**T-B304 — Implementar `tenant.Service.Register` y en `identity`: `CreateFirstAdmin`,
`CreateSession`, `IssueEmailVerification`**. Sexta revisión (DD-38, INV-33): `CreateSession` guarda `httpx.NormalizeUserAgent(meta.UserAgent)` en `sessions.user_agent`. Tercera revisión (DD-33, INV-26): validación y hash
argon2 **antes** de abrir la transacción; al abrirla, `lock_timeout` con `tenant.SignupLockTimeout`
mediante `SELECT set_config('lock_timeout', @timeout, true)` (equivale a `SET LOCAL`), antes de
`provision_tenant_role`; ambas queries en `internal/tenant/store/provisioning.sql` (plan §4.4);
desde el aprovisionamiento hasta el `COMMIT`, solo SQL (nada de SMTP, S3 ni otra E/S de red).
Séptima revisión (INV-32, DD-37 (5)): `Register` audita `tenant.registered` con actor = el admin
creado, target `tenant` = la empresa, `Data` = un mapa con la única clave `industry_template_code`
(el código de la plantilla aplicada), e `IP` y `UserAgent` tomados de `meta` sin modificar (la
normalización es de `Record`). Un error de `Record` se propaga y revierte el registro (es un bug:
`500`). `CreateSession` no escribe auditoría.

**T-B305 [T] — `POST /auth/signup`** · US-1, SC-001, P-4, P-5, DD-9, DD-19, DD-21, DD-24, DD-27, DD-28, DD-33, DD-38, INV-20, INV-23, INV-26, ADR-014
- **Red** (integración HTTP con `apitest` (HTTPS + `cookiejar`), contrato **v0.4.1** validado en
  cada respuesta: séptima revisión, con `contract.Default(t).Wrap(t, srv.Client)` (T-B311), que
  valida cada respuesta contra su operación y, si es `2xx`, también el request; los casos de la
  tabla no repiten la aserción):

  | Caso | Esperado |
  |---|---|
  | Payload válido | `201` `SessionInfo`; `Set-Cookie: __Host-crm_session=…; Path=/; HttpOnly; Secure; SameSite=Lax; Max-Age=604800` con la configuración por defecto, **sin** `Domain` (aserción explícita de cada atributo, INV-23); `Cache-Control: no-store` |
  | `SESSION_ABSOLUTE` menor o mayor que 7 días | La cookie usa el vencimiento real de `SessionResult.ExpiresAt`, sin fijar siete días |
  | `name` o `company_name` con `\u0000` | `422 invalid_value` para ese campo; no se calcula el hash ni se abre la transacción |
  | Registro válido y luego `GET /me` con el mismo cliente | `200`: la cookie viajó sola por HTTPS (el `cookiejar` la reenvió) |
  | Email existente | `409`, `code: email_already_registered`, `suggested_action: password_reset`, `title` "Ya existe un usuario con ese email", `detail` "Ya existe un usuario con ese email. ¿Querés recuperar la contraseña?"; valida contra `EmailAlreadyRegisteredProblem` |
  | Email existente de un usuario `invited`, `active` y `disabled` (tres cuentas distintas) | los tres cuerpos son **idénticos byte a byte** salvo `instance` (INV-20) |
  | Cuerpo del `409` | no contiene el nombre ni el id de la otra empresa, ni el nombre, id, rol o estado del usuario existente (búsqueda de esos valores en el cuerpo crudo) |
  | `409` | log con `security_event=signup_email_exists` e `ip`, sin el email en claro; `signup_email_exists_total` +1 |
  | `timezone: "Marte/Olympus"` | `201` (no `422`); `GET /tenant` muestra `America/Argentina/Buenos_Aires` |
  | Campo faltante / campo extra | `422` con `errors[].field` / `400 malformed_request` |
  | Sin `Content-Type` JSON | `415` |
  | (Séptima revisión, contrato v0.4.1) Body de **65 537 bytes**: un `SignupRequest` válido con espacios agregados al final | `413 payload_too_large` con `Cache-Control: no-store`; valida contra la operación (el `413` está declarado desde v0.4.1); no queda empresa, usuario ni rol nuevos. El mismo cuerpo con un espacio menos (**65 536 bytes**) → `201` (el límite es inclusivo) |
  | 6.º registro en una hora desde la misma IP | `429 rate_limited` con `Retry-After` |
  | 5 registros rechazados con `409` desde la misma IP y un 6.º con email nuevo | el 6.º recibe `429` (los rechazos consumen cupo, DD-9) |
  | (Tercera revisión) Con el lock de `crm_tenant` retenido como en T-B303 | `503` `code: service_unavailable` con `Retry-After: 2` y `Cache-Control: no-store`; valida contra `ServerError` del contrato v0.4.0; sin cookie; log `event=signup_lock_timeout` (sin email); `signup_lock_timeout_total` +1 |
  | (Séptima revisión, DD-38) `User-Agent` de 600 caracteres con un byte `\xff` | `201`; el handler pasa `r.UserAgent()` sin tocarlo (normalizan los escritores). Si el cliente o el servidor de Go rechazan esa cabecera, el caso se arma con `httptest.NewRequest` sobre el handler raíz, sin `apitest` |
- **Green**: pasa la tabla.

**T-B306 — Implementar el handler de signup** (mapeo `identity.ErrEmailTaken` →
`409 email_already_registered` + `SuggestedPasswordReset`, DD-21; `db.ErrUnavailable` que viene
de un `55P03` → `503` con `RetryAfter(tenant.SignupLockTimeout)`, DD-33).

**T-B307 [T] — Resolución de sesión (`identity.Authenticate`)** · ADR-006, INV-09, INV-11, DD-10, P-5
- **Red** (integración, `Clock` falso):

  | Caso | Esperado |
  |---|---|
  | Cookie válida | `Principal{TenantID, UserID, SessionID, Role}` en el contexto |
  | Sin cookie en ruta protegida | `401 unauthenticated` |
  | Token desconocido | `401` + cookie borrada |
  | Sesión revocada | `401` |
  | Uso cada 12 h durante 7 días (nunca 24 h sin uso) | sigue válida hasta los 7 días; a los **7 días + 1 s** desde el login → `401` |
  | **24 h + 1 s** sin uso | `401` |
  | 23 h 59 min sin uso y luego un request | válida; `last_seen_at` actualizado |
  | Uso dentro de los 5 min de `last_seen_at` | no escribe en la base |
  | Uso pasados 5 min | actualiza `last_seen_at` |
  | `expires_at` de una sesión nueva | `created_at + 7 días` |
  | Usuario `disabled` con sesión no revocada (fixture) | `401` (doble verificación) |
  | Rol cambiado en la base | el siguiente request trae el rol nuevo |
  | La base guarda solo el hash del token | `SELECT` como dueño: ningún valor de `token_hash` coincide con el raw |
- **Green**: pasa la tabla.

**T-B308 — Implementar `Authenticate` y `ResolveSession`**.

**T-B309 [T] — `GET /me`** · US-1, FR-007, P-2
- **Red**: admin → `200` con 15 permisos; operador → 6 permisos; `user.email_verified` refleja
  `email_verified_at`; sin cookie → `401`. Validado contra el contrato (`contract`, T-B311).

**T-B310 — Implementar `/me` y el cableado en `internal/app`** (grupos de rutas públicas y
autenticadas dentro de chi, worker arrancado por `serve`).

**Checkpoint Fase 3**: `make check` en verde + la prueba independiente ejecutada contra
`docker compose up -d` **completo** (todos los servicios, MinIO incluido con la imagen de DD-36
aplicada en el paso 4 de la séptima revisión, sin login a ningún registro): registro por `curl`,
email visible en Mailpit.

---

### Fase 4 — Historia 2: Iniciar y cerrar sesión, bloqueo (P1)

**Objetivo**: login con bloqueo 5/15 sin enumeración de cuentas; logout.

**Prueba independiente**: 5 contraseñas incorrectas → el 6.º intento (aun correcto) recibe
`429`; pasados 15 minutos, la contraseña correcta entra.

**T-B401 [T] — Política de bloqueo (función pura)** · US-2.2, DD-7
- **Red** (unitario): `nextThrottle(state, outcome, now) (state, locked bool, retryAfter)`

  | Estado previo | Resultado | Esperado |
  |---|---|---|
  | sin estado | fallo | `failed_count=1`, sin bloqueo |
  | 4 fallos | fallo | `failed_count=5`, `locked_until = now + 15 min` |
  | bloqueado hasta `t` | cualquier intento antes de `t` | rechazado, `retryAfter = t - now`, **no** extiende |
  | bloqueado hasta `t` | fallo en `t + 1 s` | contador reiniciado: `failed_count=1` |
  | 3 fallos | éxito | estado borrado |
- **Green**: pasa la tabla.

**T-B402 [T] — Servicio de login** · US-2.1, US-2.2, FR-003, FR-008, INV-09, INV-13, DD-8, DD-19
- **Red** (integración). El login **sigue siendo no enumerable** aunque el registro confirme la
  existencia de un email (DD-19):

  | Caso | Esperado |
  |---|---|
  | Email y contraseña correctos, usuario activo | sesión creada (`expires_at = now + 7 días`); auditoría `auth.login_succeeded`; `login_throttles` sin fila para ese email |
  | Contraseña incorrecta | `ErrInvalidCredentials`; auditoría `auth.login_failed`; contador +1 |
  | Email inexistente | `ErrInvalidCredentials` (mismo error); contador +1 para ese HMAC; se llamó a `VerifyDummy` |
  | 5 fallos seguidos con email **existente** y luego contraseña correcta | el 6.º devuelve `*LockedError` con `RetryAfter ≈ 15 min`; auditoría `auth.login_locked` al quinto |
  | 5 fallos seguidos con email **inexistente** y 6.º intento | mismo `*LockedError` (INV-13) |
  | Usuario `invited` (sin contraseña) | `ErrInvalidCredentials` |
  | Usuario `disabled`, contraseña correcta | `ErrAccountDisabled`; sin sesión |
  | Usuario `disabled`, contraseña incorrecta | `ErrInvalidCredentials` |
  | Hash con parámetros viejos y login correcto | `password_hash` reescrito con los parámetros actuales |
  | 10 intentos incorrectos **concurrentes** para el mismo email | exactamente 5 verifican contraseña; el resto recibe bloqueo (lock `FOR UPDATE`) |
  | Email con mayúsculas | normalizado antes del HMAC y la búsqueda |
- **Green**: pasa la tabla.

**T-B403 — Implementar `Login` y `Logout`**.

**T-B404 [T] — `POST /auth/login` y `POST /auth/logout`** · US-2, INV-13, INV-23
- **Red** (HTTP + contrato; los casos con cookie usan `apitest`):

  | Caso | Esperado |
  |---|---|
  | Login correcto | `200` `SessionInfo` + cookie con `Max-Age` calculado desde `SessionResult.ExpiresAt` (`604800` por defecto), `Secure`, `HttpOnly`, `SameSite=Lax`, `Path=/`, sin `Domain` (INV-23) |
  | Incorrecto / inexistente | `401 invalid_credentials`, **cuerpos idénticos byte a byte** salvo `instance` |
  | Bloqueado | `429 login_locked` + `Retry-After` en segundos |
  | Desactivado con contraseña correcta | `403 account_disabled` |
  | Logout con sesión | `204`, `Set-Cookie` que borra la cookie (`Max-Age=0`, mismos atributos, `Secure`); la cookie anterior da `401` en `/me`; auditoría `auth.logout` |
  | Logout sin cookie o con cookie inválida | `204` |
  | 21.º login en un minuto desde la misma IP | `429 rate_limited` |
- **Green**: pasa la tabla.

**T-B405 — Implementar handlers de login y logout**.

**Checkpoint Fase 4**: `make check` en verde.

---

### Fase 5 — Historia 2: Recuperar contraseña y verificar email (P1)

**Objetivo**: reset por email con enlace de un solo uso de 1 h (o reemisión de invitación para
invitados); verificación de email que no bloquea nada (P-2).

**Prueba independiente**: pedir reset → enlace en Mailpit → nueva contraseña → las sesiones
anteriores dan `401` y la nueva contraseña entra. Para un invitado, pedir reset → llega un email
de invitación nuevo.

**T-B501 [T] — Pedido de reset** · US-2.3, FR-004, INV-13, DD-19, DD-20, S-9
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | Usuario activo | token `password_reset` (vence en 1 h) + mensaje `password_reset` pendiente con enlace; auditoría `auth.password_reset_requested` |
  | Segundo pedido de un activo | el token anterior queda revocado; hay uno solo vigente |
  | Usuario `invited` | invitación reemitida: token `invitation` anterior revocado, uno nuevo de 7 días con `created_by_user_id` nulo, mensaje `invitation` pendiente; auditoría `user.invitation_reissued` con actor `NULL` y `trigger: password_reset_request`; **no** se crea token de reset (DD-20) |
  | Usuario `disabled` o email inexistente | no se crea nada |
  | En todos los casos anteriores | el servicio devuelve `nil` (mismo resultado, INV-13) |
  | 4.º pedido seguido para el mismo email (sin esperar la reposición: 3 de ráfaga y 1 cada 20 min, DD-9) | se ignora (rate limit por email) sin cambiar la respuesta |

  Fuera del alcance (octava revisión, DD-39): el tiempo de respuesta. No se escribe un test de
  tiempos: la decisión no promete igualarlo.
- **Green**: pasa la tabla.

**T-B502 [T] — Confirmación de reset** · US-2.3, INV-09, INV-11
- **Red** (integración, `Clock` falso):

  | Caso | Esperado |
  |---|---|
  | Token válido + contraseña válida | hash cambiado; token `used_at`; **todas** las sesiones revocadas (`password_reset`); `login_throttles` del email borrado; auditoría |
  | Token vencido (1 h + 1 s) | `ErrTokenInvalid` |
  | Token usado o revocado | `ErrTokenInvalid` |
  | Token de otro propósito (invitación, verificación) | `ErrTokenInvalid` |
  | Token inexistente | `ErrTokenInvalid` |
  | Usuario desactivado después de pedir el reset | `ErrTokenInvalid` (el token fue revocado al desactivar) |
  | Contraseña inválida | validación; el token **no** se consume |
  | Dos confirmaciones concurrentes del mismo token | exactamente una tiene éxito |
- **Green**: pasa la tabla.

**T-B503 — Implementar `RequestPasswordReset` (con DD-20) y `ConfirmPasswordReset`**. La
reemisión reutiliza la misma función interna que la reinvitación del Administrador (DD-5). Novena revisión (DD-40): `GetTokenFlowUser` (que también usan la verificación y su reenvío, T-B505) bloquea al usuario con `FOR NO KEY UPDATE`, nunca `FOR UPDATE`; el ajuste se aplica en el PR de la Fase 6 (Estado de la implementación) con la regresión L2 de T-B604.

**T-B504 [T] — Verificación de email** · US-1.3, DD-13, P-2
- **Red**: confirmar con token válido → `email_verified_at` puesto y auditoría; repetir con el
  mismo token → `ErrTokenInvalid`; token vencido (48 h) → `ErrTokenInvalid`; `resend` con email
  ya verificado → no crea nada; `resend` sin verificar → revoca el anterior y crea uno nuevo. Un
  usuario sin verificar puede usar todos los endpoints de 001 que su rol permite (P-2: no bloquea
  nada).

**T-B505 — Implementar verificación y reenvío**.

**T-B506 [T] — Endpoints de reset y verificación** · contrato, INV-13
- **Red**: `POST /auth/password-reset` → `202` sin cuerpo, con respuestas **idénticas** (status,
  headers relevantes y cuerpo vacío) para email de un usuario activo, invitado, desactivado e
  inexistente; `confirm` → `204` / `400 token_invalid` / `422`; `email-verification/confirm` →
  `204` / `400`; `resend` → `202` / `401` sin cookie. Todos validados contra el contrato. "Idénticas" no incluye el tiempo de respuesta (DD-39, octava revisión).

**T-B507 — Implementar los handlers**.

**Checkpoint Fase 5**: `make check` en verde + prueba independiente contra `docker compose`: **verificado el 2026-10-04** (pasos (a) a (h) en verde con el binario real; detalle y desvíos locales en "Estado de la implementación", "Ajustes posteriores a la Fase 5").

---

### Fase 6 — Historia 3: Invitar operadores y administrar usuarios (P2)

**Objetivo**: invitar, reinvitar (con cambio de rol si corresponde), aceptar, cambiar rol de
invitados y activos, desactivar (cerrando sesiones) y reactivar (P-3, confirmado), sin dejar
nunca la empresa sin Administrador y con todo auditado.

**Prueba independiente**: el admin invita a un operador; lo reinvita como administrador; el
invitado acepta y entra como Administrador; como otro operador, `/users` da `403`; el admin
desactiva a un usuario y el siguiente request de ese usuario da `401`; el admin lo reactiva y
vuelve a entrar con su contraseña.

**T-B601 [T] — Invitar y reinvitar** · US-3.1, FR-005, FR-008, INV-10, INV-16, DD-1, DD-5, DD-26, DD-40, H-5
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | Email nuevo, rol `operator` | usuario `invited`, `name` y `password_hash` nulos; token `invitation` de 7 días con `created_by_user_id`; mensaje `invitation` pendiente; auditoría `user.invited` |
  | Mismo email ya `invited` en la misma empresa, **mismo** rol | token anterior revocado, token nuevo; `reissued = true`; auditoría `user.invitation_reissued {role, trigger: admin}`; **sin** `user.role_changed`; sigue habiendo **un** usuario |
  | Mismo email ya `invited`, rol **distinto** (`operator` → `admin`) | además del caso anterior, `role = admin`; auditoría `user.role_changed {from: operator, to: admin, status: invited}` en la misma transacción (dos filas en total) |
  | Reinvitar como `operator` a un invitado `admin` siendo el creador el único admin activo | permitido (los invitados no cuentan para INV-10) |
  | Reinvitaciones concurrentes del mismo invitado con roles distintos (DD-26). Las serializa el lock del usuario (`LockManagedUser`): esta fila **no** detecta la falta del lock de la empresa en `Invite`; eso lo hace la siguiente | un invitado `operator` recibe dos reinvitaciones concurrentes, una como `operator` y otra como `admin` (20 repeticiones). En cada repetición: las dos tienen éxito; hay exactamente dos `user.invitation_reissued` (uno con `data.role = operator`, otro con `admin`) y un `user.role_changed` por cada cambio **real** de rol, con `from`/`to` encadenados según el orden en que se aplicaron. Si primero se aplicó la de `operator`: ningún `role_changed` por ella (mismo rol) y `operator → admin` por la segunda (3 filas en total). Si primero la de `admin`: `operator → admin` y después `admin → operator` (4 filas). El rol final es el de la última reinvitación aplicada; cada `user.role_changed` está en la misma transacción que el `user.invitation_reissued` de su reinvitación (p. ej., mismo `xmin`); queda **una** invitación abierta (la de la última) y la otra revocada. El orden se deduce de la cadena de auditoría y del rol final, no de `occurred_at` (con reloj falso puede coincidir, y con el default `now()` es el inicio de la transacción: la que esperó el lock pudo empezar antes) |
  | **Concurrencia, lock de la empresa en `Invite`** (INV-10, DD-40): dos `Invite` del mismo email **nuevo**, con el mismo rol, en la misma empresa. Mecanismo E de T-B604: `T_admin` toma la empresa con la query de INV-10 y la retiene; se lanzan las dos `Invite`; el test espera (sondeo de `pg_blocking_pids` cada 10 ms, contexto de 20 s) a que haya **dos** backends bloqueados por `T_admin`, y después `T_admin` hace `ROLLBACK`. Corre una vez: el mecanismo es determinista | Las dos quedan bloqueadas por `T_admin` antes de devolver (si alguna devuelve antes, el test falla con un mensaje que nombra INV-10 y DD-40: `Invite` no tomó el lock). Después, las dos tienen éxito y ninguna devuelve `ErrEmailTaken`: exactamente una con `reissued = false` y otra con `reissued = true` (`201` y `200` en HTTP, T-B606); **un** usuario `invited` con ese email y ese rol; auditoría: un `user.invited` y un `user.invitation_reissued {role, trigger: admin}`, sin `user.role_changed`; dos mensajes `invitation` pendientes (INV-16); **una** invitación abierta (la de la reemisión) y la de la creación, revocada. No se asume cuál de las dos crea: se identifica por el `reissued` que devolvió cada una. Sin el lock, las dos ven el email libre, las dos insertan y la segunda choca con `users_email_key` (`ErrEmailTaken`, un `409 email_taken` falso); con el lock tomado **después** de buscar el email, las dos quedan bloqueadas pero ya lo vieron libre y una termina en `ErrEmailTaken`: la fila falla en los dos casos |
  | Email de un usuario activo o desactivado de esta empresa | `ErrEmailTaken` |
  | Email de un usuario (cualquier estado) de **otra** empresa | `ErrEmailTaken` (sin datos de la otra empresa en el error) |
  | Rol `admin` en una invitación nueva | permitido |
  | Email inválido | validación |
- **Green**: pasa la tabla.

**T-B602 [T] — Ver y aceptar invitación** · US-3.2, DD-1, DD-4
- **Red** (integración, `Clock` falso):

  | Caso | Esperado |
  |---|---|
  | `Preview` con token vigente | nombre de la empresa, email, rol **actual** del usuario (el de la última reinvitación o cambio de rol), vencimiento |
  | `Accept` válido | usuario `active` con nombre y hash; `email_verified_at` puesto; token usado; sesión creada con el rol actual; auditoría `user.invitation_accepted` |
  | Token vencido (7 días + 1 s), reemplazado por reinvitación, o ya usado | `ErrTokenInvalid` (en `Preview` y en `Accept`) |
  | Token reemplazado por una reemisión disparada por pedido de reset (DD-20) | `ErrTokenInvalid`; el token nuevo sí funciona |
  | Usuario desactivado antes de aceptar | `ErrTokenInvalid` |
  | Contraseña inválida | validación; token no consumido |
  | Dos `Accept` concurrentes | uno solo tiene éxito |
- **Green**: pasa la tabla.

**T-B603 [T] — Cambio de rol y último Administrador** · US-3.4, FR-005, FR-008, INV-10, DD-26, H-5
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | Operador activo → admin | rol cambiado; auditoría `user.role_changed {from: operator, to: admin, status: active}` |
  | Invitado `operator` → `admin` y viceversa | permitido; auditoría con `status: invited`; la invitación vigente no cambia (mismo token y vencimiento) |
  | Usuario `disabled` → cualquier rol | `ErrInvalidTransition`; sin cambios ni auditoría |
  | Admin → operador habiendo otro admin activo | permitido |
  | Único admin activo → operador (incluido sobre sí mismo) | `ErrLastAdmin`; sin cambios |
  | Único admin activo + un invitado `admin`; bajar al activo | `ErrLastAdmin` (el invitado no cuenta) |
  | Dos admins activos + uno desactivado; bajar a uno de los activos | permitido; bajar al otro después → `ErrLastAdmin` (el desactivado no cuenta) |
  | Mismo rol que ya tiene | sin cambios, sin auditoría, sin error |
  | Id de otra empresa o inexistente | `ErrUserNotFound` |
  | **Concurrencia**: empresa con 2 admins; cada uno baja al otro a la vez (20 repeticiones) | en todas, exactamente una operación tiene éxito y la otra `ErrLastAdmin`; siempre queda ≥ 1 admin activo |
- **Green**: pasa la tabla.

**T-B604 [T] — Desactivar y reactivar** · US-3.3, US-3.4, FR-005, FR-008, INV-10, INV-11, P-3, DD-40, DD-41
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | Desactivar un operador con 2 sesiones abiertas y, en `user_tokens` (los arma el fixture), un `password_reset` y un `email_verification` pendientes, un `password_reset` ya usado y uno ya revocado; además, un `password_reset` pendiente de **otro** usuario de la empresa | `disabled`; ambas sesiones con `revoked_reason = user_disabled`; los dos tokens pendientes con `revoked_at` puesto, afirmado sobre la fila (INV-11: que el token deje de funcionar no alcanza, porque el estado `disabled` ya lo rechaza); el usado, intacto (`used_at` igual y `revoked_at` nulo: tocarlo violaría `NOT (used_at IS NOT NULL AND revoked_at IS NOT NULL)` y la desactivación fallaría); el ya revocado conserva su `revoked_at`; el del otro usuario, intacto; auditoría `user.deactivated {sessions_revoked: 2}` |
  | El siguiente request con cualquiera de sus cookies | `401` |
  | Desactivar al único admin activo | `ErrLastAdmin` |
  | Un admin se desactiva a sí mismo habiendo otro admin | permitido; su sesión actual queda revocada |
  | Desactivar un usuario ya `disabled` | `ErrInvalidTransition` |
  | Desactivar un `invited` | `disabled`; su invitación abierta con `revoked_at` puesto, afirmado sobre la fila de `user_tokens` (INV-11; `PreviewInvitation` ya la rechaza por el estado, así que no prueba la revocación); auditoría `user.deactivated {sessions_revoked: 0}` |
  | **Concurrencia**: 2 admins se desactivan mutuamente a la vez | uno solo tiene éxito |
  | Reactivar un `disabled` con contraseña | `active`; auditoría `user.reactivated {to_status: active}` con actor = admin, en la **misma** transacción; sus sesiones viejas **siguen** revocadas; puede iniciar sesión con su contraseña |
  | Reactivar un `disabled` que nunca tuvo contraseña | `invited` con invitación nueva de 7 días (`created_by_user_id` = admin) y mensaje `invitation` encolado; auditoría `user.reactivated {to_status: invited}` **y** `user.invitation_reissued {trigger: reactivation}` (FR-008: toda invitación se audita) |
  | Reactivar un `disabled` sin contraseña **no** puede dejarlo `active` | la base lo impide (`users_active_complete_chk`) aun si el código lo intentara (test directo contra la constraint) |
  | Reactivar un `active` o `invited` | `ErrInvalidTransition`; sin auditoría |
  | Reactivar un id de otra empresa o inexistente | `ErrUserNotFound` |
  | Reactivar un admin desactivado | vuelve a contar como admin activo para INV-10 |
  | **Concurrencia**: un admin reactiva a X mientras otro desactiva a X (20 repeticiones) | las operaciones se serializan por el lock de `tenants`; el estado final es consistente con una de las dos órdenes y hay exactamente una fila de auditoría por operación exitosa |
  | Cualquier operación fallida | ninguna fila de auditoría |

- **Red, regresión de los locks de empresa y usuario** (novena revisión, DD-40, DD-41, INV-10, INV-11; integración contra PostgreSQL real). Reemplaza al test diagnóstico `TestProposedTenantLockConflictsWithPasswordReset` de la rama de la Fase 6, que exigía un `40P01` y se borra. Los flujos de las Fases 4 y 5 corren **reales** (`identity.Service`); el único cambio en ellos es el modo de `GetTokenFlowUser`. La operación de administración es una transacción del test, `T_admin` (`InTenantTx`), armada con las mismas queries de `store` que usa el servicio y en el mismo orden: la query de INV-10 (`LockUsersTenant`), `LockManagedUser` (antes `GetManagedUserForUpdate`) y las escrituras de la operación (desactivar: `SetManagedUserStatus` a `disabled`, `RevokeDisabledUserSessions`, `RevokeDisabledUserTokens`; cambiar rol: `SetManagedUserRole`). Así el test controla cuándo toma cada lock y conoce su `pg_backend_pid()`. Los mecanismos son deterministas y el camino que pasa no tiene `sleep`; "bloqueado por P" se detecta sondeando cada 10 ms, con un contexto de 20 s, `SELECT pid FROM pg_stat_activity WHERE P = ANY(pg_blocking_pids(pid))`:
  - **E (la empresa primero)**: `T_admin` toma la empresa con la query de INV-10 y la retiene; el flujo corre en otra goroutine. El test espera a que el flujo devuelva o a que aparezca un backend bloqueado por `T_admin`; lo segundo es un fallo inmediato. Con el flujo terminado, `T_admin` bloquea al usuario (no debe esperar), aplica la operación y hace `COMMIT`.
  - **U (el usuario primero)**: la intercalación del diagnóstico. El flujo se detiene con el hasher instrumentado (`Hash` bloquea hasta que el test lo libera) **después** de bloquear al usuario y antes de su primera inserción con FK. `T_admin` toma la empresa (no debe esperar), pide al usuario y el test confirma que espera (`wait_event_type = 'Lock'` de su pid). Después libera el hasher.
  - **S (la sesión primero)**: el cierre de sesión corre real con un `audit.Recorder` instrumentado que, en el `Record` de `auth.logout`, guarda `pg_backend_pid()` con la `tx` que recibe y bloquea hasta que el test lo libera; los demás `Record` pasan directo. El cierre queda detenido con la fila de su sesión actualizada (`RevokeSession`) y antes de la inserción de la auditoría (`KEY SHARE` sobre el usuario).
  - **V (el login detenido)**: el login se detiene en `Verify` del hasher instrumentado: ya leyó al usuario sin lock y todavía no insertó la sesión.

  Solo la confirmación de reset llama al hasher entre el lock del usuario y sus inserciones. El pedido de reset no hashea, y el login hashea **antes** de tocar al usuario. Por eso esos flujos usan E, que no necesita *hooks* y prueba algo más fuerte: el flujo termina sin esperar mientras la empresa está tomada.

  | Caso | Flujo real | Mecanismo y `T_admin` | Esperado |
  |---|---|---|---|
  | R0 | — (regla estática, unitario: junto a la regla de T-B112 o en `internal/identity/store`) | — | En las queries de `internal/*/store/*.sql`, una cláusula de lock sin `OF` en un `JOIN` cuenta para todas sus tablas. **(a)** Las únicas queries que bloquean filas de `app.tenants` son la de INV-10 (`LockUsersTenant`) y, desde la Fase 7, `LockTenant` (`internal/tenant/store`; INV-34, décima revisión), y las dos usan exactamente `FOR NO KEY UPDATE`: cualquier otra, o una de esas con otro modo, hace fallar la regla. **(b)** Toda query que bloquea filas de `app.users` usa exactamente `FOR NO KEY UPDATE` (ni `FOR UPDATE`, ni `FOR SHARE`, ni `FOR KEY SHARE`) |
  | R1 | `RequestPasswordReset` de un operador `active` | E; desactivar a ese usuario | el pedido devuelve `nil` sin que ningún backend quede bloqueado por `T_admin`; después `T_admin` termina con `nil`. Estado final: usuario `disabled`; el token `password_reset` que creó el pedido, revocado; un mensaje `password_reset` encolado; una auditoría `auth.password_reset_requested` |
  | R2 | `RequestPasswordReset` de un invitado `operator` (reemite la invitación, DD-20) | E; cambiar su rol a `admin` | el pedido devuelve `nil` sin bloqueo; `T_admin` termina con `nil`. Estado final: `invited` con `role = admin`; **una** invitación abierta (la reemitida; la anterior, revocada), que el cambio de rol no toca (T-B603); un mensaje `invitation` encolado con el rol que leyó el pedido (`operator`; `Preview` muestra el actual, T-B602); una auditoría `user.invitation_reissued {role: operator, trigger: password_reset_request}` |
  | R3 | `ConfirmPasswordReset` de un usuario `active` con una sesión abierta | U; desactivar a ese usuario | las dos terminan con `nil`, ninguna con `40P01`. Estado final: contraseña nueva (`Verify` la acepta); token usado; la sesión previa revocada con `revoked_reason = password_reset` (el reset terminó primero; `RevokeDisabledUserSessions` de `T_admin` afecta 0 filas); usuario `disabled`; sin fila de `login_throttles` para ese email; una auditoría `auth.password_reset_completed` |
  | R4 | `Login` exitoso de un operador `active` (sin rehash) | E; desactivar a ese usuario | el login devuelve la sesión sin bloqueo; `T_admin` termina con `nil`. Estado final: usuario `disabled`; la sesión creada por el login, revocada con `revoked_reason = user_disabled`; `ResolveSession` con ese token → no autenticado; una auditoría `auth.login_succeeded` |
  | R5 | (caracteriza DD-41; pasa desde el principio, no es Red) `Login` exitoso de un operador `active` (sin rehash) | V; `T_admin` desactiva y hace `COMMIT` sin esperar (el login solo tiene el throttle); el test libera `Verify` | las dos terminan con `nil`. Estado final: usuario `disabled`; la sesión del login **sin revocar**; `ResolveSession` con ese token → no autenticado; una auditoría `auth.login_succeeded`. Si después otra transacción del test lo reactiva (`SetManagedUserStatus` a `active`) dentro de las 24 h, `ResolveSession` lo autentica (consecuencia aceptada en DD-41) |
  | L1 | `Logout` de un operador `active` con una sesión | S; desactivar a ese usuario | `T_admin` toma la empresa y al usuario sin esperar, cambia el estado y su `RevokeDisabledUserSessions` queda bloqueado por el cierre (el test lo confirma con el pid del cierre); el test libera el *recorder*. Las dos terminan con `nil`, ninguna con `40P01`. Estado final: usuario `disabled`; la sesión revocada con `revoked_reason = logout` (la revocó el cierre; `RevokeDisabledUserSessions` no la vuelve a tocar); una auditoría `auth.logout` |
  | L2 | `Logout` de un usuario `active` con una sola sesión, y `ConfirmPasswordReset` real del mismo usuario | S; sin `T_admin` | con el cierre detenido, la confirmación bloquea throttle, usuario y token, hashea y su `RevokeUserSessions` queda bloqueado por el cierre (el test lo confirma con el pid del cierre); el test libera el *recorder*. Las dos terminan con `nil`, ninguna con `40P01`. Estado final: contraseña nueva; token usado; la sesión revocada con `revoked_reason = logout`; auditorías `auth.logout` y `auth.password_reset_completed {sessions_revoked: 0}` |

  **Si alguna de esas queries vuelve a `FOR UPDATE`, el test falla** (ese es su valor de regresión):
  - **Query de INV-10:** falla R0 (a). R1, R2 y R4 fallan porque la primera inserción del flujo con FK a `tenants` (`InsertUserToken`, `InsertSession`) espera `FOR KEY SHARE` detrás de `T_admin`: el test lo detecta, falla con un mensaje que nombra DD-40 y hace `ROLLBACK` de `T_admin` para no dejar goroutines colgadas (sin esa detección, R1 y R2 terminarían en `40P01` al pedir `T_admin` al usuario). R3 falla porque una de las dos recibe `40P01`.
  - **`LockManagedUser` o `GetTokenFlowUser`:** falla R0 (b), y L1 o L2 respectivamente terminan en `40P01`. El cierre pide `KEY SHARE` sobre un usuario bloqueado con `FOR UPDATE`, cuyo dueño espera la sesión del cierre.

  Cada caso corre una vez: el mecanismo es determinista. Al escribirlos, con las queries todavía en `FOR UPDATE`, R0 a R4, L1 y L2 tienen que fallar así (Red).
- **Green** (regresión): pasan R0 a R5, L1 y L2 con la query de INV-10, `LockManagedUser` y `GetTokenFlowUser` en `FOR NO KEY UPDATE`, sin otros cambios en los flujos de las Fases 4 y 5.
- **Green**: pasa la tabla.

**T-B605 — Implementar `Invite`, `PreviewInvitation`, `AcceptInvitation`, `ListUsers`,
`ChangeRole`, `Deactivate`, `Reactivate`**. Las transiciones de estado se expresan como una
tabla (`estado actual × acción → estado nuevo | error`) con su test unitario (incluye las dos
salidas de `reactivate` según tenga o no contraseña y el rechazo de `changeRole` en `disabled`),
y cada operación de cambio de rol o estado (incluida `Invite`, que toma el lock **antes** de buscar el email porque todavía no sabe si va a crear o a reemitir; lo verifica la fila de concurrencia del email nuevo de T-B601) empieza con el lock de la
fila `tenants` con la query de INV-10, `FOR NO KEY UPDATE` y nunca `FOR UPDATE` (DD-40, novena
revisión), y recién después bloquea al usuario, también con `FOR NO KEY UPDATE` (`LockManagedUser`, antes `GetManagedUserForUpdate`) (orden empresa → usuario →
tokens). `AcceptInvitation` no lo necesita (solo puede subir la cuenta de Administradores
activos); si lo toma, lo toma antes que al usuario. Ninguna query nueva bloquea `users` ni `tenants` con `FOR UPDATE` (regla R0 de T-B604). De las Fases 4 y 5 solo cambia el modo de `GetTokenFlowUser` (T-B503). `ListUsers` informa `invitation_expires_at` desde la invitación abierta
de cada invitado, aunque haya vencido (DD-25).

**T-B606 [T] — Endpoints de usuarios e invitaciones** · contrato, FR-005, FR-007, P-3, DD-25, DD-26, H-4, H-5
- **Red** (HTTP + contrato):

  | Caso | Esperado |
  |---|---|
  | Admin: `GET /users` | `200` con invitados (con `invitation_expires_at`), activos y desactivados (con `invitation_expires_at: null`), en orden de alta |
  | Invitado cuya invitación venció hace 3 días (reloj falso) | `invitation_expires_at` = esa fecha pasada (no `null`) |
  | Invitado cuya invitación venció hace 40 días, después de correr la limpieza (T-B901) | **Diferida a T-B901** (novena revisión: la limpieza no existe hasta la Fase 9). Esperado allí: sigue informando la fecha (la limpieza conserva la invitación abierta) |
  | Operador en `GET /users`, `POST /users/invitations`, `PUT /users/{id}/role`, `POST .../deactivate`, `POST .../reactivate` | `403 forbidden` en todos; ningún cambio en la base ni en la auditoría |
  | Admin, `userId` inexistente o de otra empresa | `404 not_found` (idénticos) en `role`, `deactivate` y `reactivate` |
  | `userId` que no es UUID | `404` (no `400`: no revela formato de ids; el test valida solo la respuesta contra el contrato) |
  | Invitar nuevo / reinvitar con el mismo rol / reinvitar con otro rol | `201` / `200` / `200` con el rol nuevo en el cuerpo |
  | Invitar un email existente | `409 email_taken` **sin** `suggested_action` (DD-21) |
  | `PUT /users/{id}/role` sobre un invitado | `200` con el rol nuevo y `status: invited` |
  | `PUT /users/{id}/role` sobre un desactivado | `409 invalid_state` |
  | `POST /users/{id}/reactivate` sobre `disabled` con contraseña | `200` con `status: active` |
  | `POST /users/{id}/reactivate` sobre `disabled` sin contraseña | `200` con `status: invited` e `invitation_expires_at` a 7 días |
  | `POST /users/{id}/reactivate` sobre `active` o `invited` | `409 invalid_state` |
  | `409 last_admin`, `409 invalid_state` en `role`/`deactivate` | según T-B603..T-B604 |
  | `POST /auth/invitations/preview` y `accept` | `200` / `201` + cookie / `400 token_invalid` |
  | Cupo compartido (DD-9): desde una misma IP, 10 `preview` y 10 `accept` intercalados con un token inválido, y después un 21.º `preview` | los 20 primeros, `400 token_invalid`; el 21.º, `429 rate_limited` con `Retry-After` (con cupos separados, el 21.º pasaría) |
- **Green**: pasa la tabla.

**T-B607 — Implementar los handlers**.

**Checkpoint Fase 6**: `make check` en verde + prueba independiente.

---

### Fase 7 — Historia 4: Datos de la empresa y logo (P2)

**Objetivo**: el admin edita los datos de la empresa y sube el logo; cualquier usuario los ve; el
logo nunca se muestra desde la caché de otra empresa o de una versión anterior; la subida respeta
los límites exactos de DD-31 sea cual sea el cliente.

**Prueba independiente**: el admin sube un PNG y edita el CUIT; el operador ve el logo en
`GET /tenant/logo` y recibe `403` al intentar `PATCH /tenant`; con `curl`, un segundo `GET` con
`If-None-Match` responde `304`; tras reemplazar el logo, el mismo `If-None-Match` responde `200`;
con `curl`, un archivo de 2 097 153 bytes recibe `413`.

**T-B701 [T] — Validaciones puras de la empresa** · DD-16, DD-15
- **Red** (unitario):

  | Entrada | Esperado |
  |---|---|
  | CUIT con dígito verificador válido, con y sin guiones | normalizado a 11 dígitos |
  | CUIT con dígito verificador inválido | `invalid_tax_id` |
  | CUIT de 10 dígitos o con letras | `invalid_format` |
  | Casos del módulo 11 donde el resto da 10 o 11 | según la regla oficial (tabla de casos en el test, con fuente citada) |
  | `timezone` `America/Argentina/Cordoba` / `Marte/Olympus` | ok / `invalid_timezone` |
  | **Décima revisión** (DD-43): `timezone` `Local` o `""` (`time.LoadLocation` los acepta: `Local` es la zona del servidor y `""` es UTC) | `invalid_timezone`: no son nombres IANA. La misma función la usa el registro (T-B303) |
  | `name` vacío | `required` |
- **Green**: pasa la tabla.

**T-B702 [T] — Actualización de datos** · US-4, FR-008, DD-27, DD-40, DD-43, INV-34
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | `PATCH` parcial | solo cambian los campos presentes; `updated_at` avanza; auditoría `tenant.updated` con la **lista de nombres** de los campos cuyo valor cambió (no los valores), en el orden de columnas de `data-model.md` §2.1 |
  | `null` en un campo opcional | lo borra (`NULL`) |
  | `base_currency` en el payload | `400 malformed_request` (campo desconocido) |
  | `timezone: "Marte/Olympus"` | validación `invalid_timezone` (acá **sí** es error, a diferencia del registro) |
  | **Décima revisión** (DD-43): `timezone: "Local"` | `invalid_timezone`; sin cambios ni auditoría |
  | **Décima revisión** (DD-43; Red): `legal_name`, `tax_id`, `address`, `phone` o `email` con `""` o solo espacios | se guarda `NULL`, igual que con `null` (nunca una cadena vacía); `tax_id: ""` no da `422` |
  | **Décima revisión** (DD-43; Red): `PATCH` cuyos valores, ya normalizados, son iguales a los guardados (p. ej. el mismo CUIT escrito con guiones) | ningún `UPDATE`: `updated_at` igual, **ninguna** auditoría; devuelve el `Tenant` vigente |
  | **Décima revisión** (DD-43; Red): `PATCH` con un campo igual al guardado y otro distinto | `fields` lista solo el distinto |
  | **Décima revisión, concurrencia** (INV-34, DD-40): dos `PATCH` en paralelo de la misma empresa, uno con `phone` y otro con `address`. Mecanismo E de T-B604: `T_admin` retiene la fila con `LockTenant` (o con la query de INV-10: mismo modo); el test espera (sondeo de `pg_blocking_pids` cada 10 ms, contexto de 20 s) a que haya **dos** backends bloqueados por `T_admin` de forma directa o transitiva (aclaración de la Fase 6) y hace `ROLLBACK`. Corre una vez | los dos tienen éxito y **los dos** valores quedan guardados; dos auditorías `tenant.updated` (`[phone]` y `[address]`). Sin `LockTenant`, o con la lectura antes del lock, los dos leen la fila vieja, el `UPDATE` de cada uno espera a `T_admin` y, si el `UPDATE` reescribe todas las columnas de datos (leer-mezclar-reescribir), el segundo pisa el campo del primero con el valor viejo: la fila falla. Pasa hoy: mutación y se restaura |
- **Green**: pasa la tabla.

**T-B703 [T] — Logo (servicio)** · US-4, DD-11, DD-23, DD-31, DD-40, DD-42, INV-17, INV-24, INV-34, H-2, H-11
- **Red** (integración con `ObjectStorage` falso instrumentado; `SetLogo` recibe `[]byte`):

  | Caso | Esperado |
  |---|---|
  | PNG válido de 1 MB | objeto guardado con clave `tenants/{A}/logo/{uuid}.png`; `logo_object_key` y `logo_content_type` actualizados; `updated_at` avanza; auditoría `tenant.logo_updated` |
  | PNG válido de **exactamente 2 097 152 bytes** (relleno con un chunk auxiliar) | aceptado |
  | Datos de **2 097 153 bytes** | `ErrLogoTooLarge`; el fake de S3 **no** recibió `Put` |
  | Reemplazar logo | objeto nuevo guardado **antes** del `COMMIT`; el viejo borrado **después**; clave (y por lo tanto `ETag`) distinta |
  | JPEG válido | aceptado |
  | JPEG válido con segmento EXIF (orientación y coordenadas ficticias) | aceptado; el `Put` recibió **exactamente** los mismos bytes (el servidor no modifica ni quita metadatos, DD-11) |
  | GIF, WebP, SVG (aunque la extensión o el `Content-Type` digan `png`) | `ErrLogoUnsupportedType` |
  | PNG de 2001×10 o con encabezado que declara 50000×50000 | `ErrLogoInvalidImage` (sin decodificar la imagen entera) |
  | PNG de 2000×2000 | aceptado |
  | Bytes truncados después de la firma PNG | `ErrLogoInvalidImage` |
  | `Put` falla | `503`; la fila de la empresa sin cambios |
  | La transacción falla **dentro** de su función después del `Put` (p. ej. el `audit.Recorder` del test devuelve error) | `ROLLBACK` seguro: el objeto nuevo se borra con un contexto propio (`WithoutCancel` + 5 s); la fila conserva la clave anterior y su objeto existe; ninguna auditoría. Si ese `Delete` falla: log `WARN` `event=logo_delete_failed`, `reason=compensation` (huérfano aceptado) |
  | **Décima revisión, `COMMIT` confirmado con error** (DD-42, INV-17; Red): un `db.TxRunner` del test delega en el real y, cuando el real confirma, cancela el contexto del request y devuelve un error de commit (`context.Canceled` envuelto) | la fila apunta a `K_nuevo` y **su objeto existe** (con la regla anterior se borraba: referencia rota); `K_viejo` borrado; `SetLogo` devuelve el `Tenant` con la clave nueva; log `WARN` `event=logo_commit_uncertain`, `outcome=committed` |
  | **Décima revisión, `COMMIT` que no ocurrió** (DD-42; Red): el runner del test ejecuta la función del servicio, fuerza el `ROLLBACK`, cancela el contexto del request y devuelve un error de commit | la relectura (con contexto propio) ve `K_viejo`: `K_nuevo` se borra; la fila y `K_viejo`, intactos; `SetLogo` devuelve el error; `outcome=rolled_back`. Si la relectura usara el contexto del request (cancelado), fallaría y `K_nuevo` quedaría huérfano: la fila falla |
  | **Décima revisión, relectura que falla** (DD-42; Red): como "`COMMIT` confirmado con error", y además la segunda llamada al runner (la relectura) devuelve `db.ErrUnavailable` | no se borra **nada**: la fila apunta a `K_nuevo` y su objeto existe; `K_viejo` queda huérfano; `SetLogo` devuelve el error original; `outcome=unknown` |
  | **Décima revisión, `COMMIT` en vuelo** (DD-42; Red): el runner del test corre la transacción real en otra goroutine; cuando la función del servicio termina, guarda el `pg_backend_pid()` de esa transacción (`P`), deja la transacción abierta (con la fila bloqueada) y le devuelve al servicio un error de commit | el test espera (sondeo de `pg_blocking_pids` cada 10 ms, contexto de 20 s) a que haya un backend bloqueado por `P` (la relectura en `LockTenant`) y recién entonces deja confirmar a la goroutine. Esperado: la relectura ve `K_nuevo`, su objeto existe, `outcome=committed`. Si `SetLogo` devuelve antes de que aparezca el bloqueo, el test falla con un mensaje que nombra DD-42 (la relectura no tomó el lock: vería `K_viejo` y borraría `K_nuevo` justo antes de que el `COMMIT` lo haga visible). Al terminar, la goroutine siempre se libera |
  | Borrar el objeto viejo falla | la operación igual responde éxito; log `WARN` `event=logo_delete_failed`, `reason=replaced` (objeto huérfano aceptado) |
  | **Décima revisión, limpieza con el contexto cancelado** (DD-42 (3)): el `Delete` del fake falla si `ctx.Err() != nil` y registra el *deadline* que recibió; el runner del test cancela el contexto del request apenas el real confirma y devuelve `nil` | `K_viejo` borrado: el `Delete` recibió un contexto vivo con *deadline* de 5 s como máximo. Pasa hoy: mutación (pasarle al `Delete` el contexto del request → falla) y se restaura |
  | **Décima revisión, subidas concurrentes** (INV-34, DD-40): empresa con logo `K_0`; 3 `SetLogo` en paralelo con PNG distintos. Mecanismo E de T-B604: `T_admin` retiene la fila con `LockTenant` (o con la query de INV-10: mismo modo); el test espera (sondeo cada 10 ms, contexto de 20 s) a que haya **3** backends bloqueados por `T_admin` de forma directa o transitiva y hace `ROLLBACK`. Corre una vez | las 3 tienen éxito; 3 auditorías `tenant.logo_updated`; en el storage queda **exactamente un** objeto bajo `tenants/{A}/logo/`, el que referencia la fila (`K_0` y los otros dos, borrados). Sin `LockTenant` en `SetLogo` (clave anterior leída sin lock), las tres leen `K_0` y lo borran, y quedan dos huérfanos: la fila falla. Pasa hoy: mutación y se restaura |
  | **Décima revisión, subida y baja concurrentes** (INV-34): empresa con logo `K_0`; con `T_admin` reteniendo la fila (mecanismo E) se lanza `SetLogo` y el test espera a que quede bloqueado; recién entonces lanza `RemoveLogo` y espera a que haya 2 bloqueados (directos o transitivos); `ROLLBACK` de `T_admin` | se aplican en orden de llegada (subida, baja): la fila sin logo y **ningún** objeto bajo `tenants/{A}/logo/`; auditorías `tenant.logo_updated` y `tenant.logo_removed`. Sin `LockTenant` en `RemoveLogo`, la baja lee `K_0` antes de esperar, pone `NULL` y borra `K_0`: el objeto de la subida queda huérfano y la fila falla. Supuesto a validar en Red: PostgreSQL atiende la espera por la fila en orden de llegada; si no resulta determinista, aviso al arquitecto en vez de agregar `sleep` |
  | `RemoveLogo` | columnas en `NULL`, `updated_at` avanza, objeto borrado, auditoría `tenant.logo_removed`; repetir → sin error, **sin** auditoría y sin mover `updated_at` (décima revisión, DD-42 (4): sin logo no hay `content_type` que auditar) |
  | `GetLogo` con `ifNoneMatch` vacío | `ETag = "<uuid de la clave>"`, cuerpo del objeto |
  | `GetLogo` con `ifNoneMatch` igual al vigente | `NotModified = true`; el fake de S3 **no** recibió `Get` |
  | `GetLogo` con un `ifNoneMatch` viejo o de otra empresa | cuerpo completo con el `ETag` vigente |
  | **Décima revisión** (plan §11.1): `GetLogo` sin logo | `ErrLogoNotFound` (no `db.ErrNotFound`) |
  | **Décima revisión, objeto ausente** (DD-42 (6), INV-17; Red): la fila tiene clave pero el test borró el objeto del fake | `ErrLogoNotFound`; log `ERROR` `event=logo_object_missing` con `tenant_id` y `object_key` (el test captura el logger) |
  | **Décima revisión, reemplazo entre la lectura y el `Get`** (DD-42 (6); Red): el fake, en el primer `Get`, corre un `SetLogo` real de la misma empresa (confirma y borra `K_viejo`) y después devuelve `ErrNotFound` | cuerpo de `K_nuevo` con su `ETag`; **sin** log `ERROR` |
  | Lo mismo con un `RemoveLogo` en lugar del `SetLogo` | `ErrLogoNotFound`, sin log `ERROR` |
- **Green**: pasa la tabla.

**T-B704 — Implementar `tenant.Service.Update`, `SetLogo`, `RemoveLogo`, `GetLogo`** y las
constantes `LogoMaxBytes`, `LogoMaxBodyBytes`, `LogoMaxSide` (plan §11.1). Novena y décima revisión (DD-40 corregida, DD-42, DD-43, INV-17, INV-34): `Update`, `SetLogo` y `RemoveLogo` empiezan su transacción con `LockTenant` (`FOR NO KEY UPDATE`, nunca `FOR UPDATE`) y leen bajo ese lock lo que mezclan o la clave del logo anterior; después hacen el `UPDATE` de la fila antes de cualquier otra escritura (`Update` no escribe las columnas del logo; `SetLogo` y `RemoveLogo`, solo esas y `updated_at`) y por último la auditoría. `SetLogo` resuelve un fallo de la transacción según DD-42: si el error lo devolvió su propia función, el `ROLLBACK` es seguro y borra `K_nuevo`; si apareció después (el `COMMIT`), relee la fila con `LockTenant` en otra transacción con contexto propio (`context.WithoutCancel` + 5 s) y, según la clave, sigue el camino confirmado (borra `K_viejo`, devuelve el `Tenant`) o el de `ROLLBACK` (borra `K_nuevo`); si la relectura falla, no borra nada. `RemoveLogo` no borra nada ante un `COMMIT` incierto y, sin logo, no escribe ni audita. Todo `Delete` posterior a la transacción usa ese contexto propio y loguea `WARN` `event=logo_delete_failed` si falla. `GetLogo` devuelve `ErrLogoNotFound` (no `db.ErrNotFound`) y, ante `ErrNotFound` del storage, relee la clave una vez (DD-42 (6)). `Update` aplica DD-43; la validación de la zona horaria es la misma función que usa `Register` (T-B303).

**T-B705 [T] — Endpoints de empresa** · contrato, FR-007, DD-23, DD-28, DD-31, INV-21, INV-24, H-2, H-7, H-11
- **Red** (HTTP + contrato):

  | Caso | Esperado |
  |---|---|
  | `GET /tenant` como admin y como operador | `200`; `Cache-Control: no-store` |
  | `PATCH /tenant` | `200` admin, `403` operador, `422` CUIT inválido o `timezone` desconocida; `no-store` |
  | **Décima revisión** (DD-43; Red): `PATCH /tenant` con `name: null` o con `timezone: null` | `400 malformed_request` sin `errors` (el contrato no los admite: es un error de tipo, como `name: 123`); sin cambios |
  | **Décima revisión** (DD-43; Red): `PATCH /tenant` con `phone: ""` | `200` con `phone: null` |
  | **Décima revisión** (DD-43): `PATCH /tenant` con `timezone: "Local"` | `422` con `errors: [{field: timezone, code: invalid_timezone}]` |
  | `PUT /tenant/logo` multipart con un PNG válido | `200` (el `Tenant` devuelto tiene `updated_at` nuevo); `403` para el operador |
  | Parte `file` de **exactamente 2 097 152 bytes** con un nombre de archivo de 200 caracteres | `200` (el límite es del archivo, no del cuerpo) |
  | Parte `file` de **2 097 153 bytes** | `413 payload_too_large`; el handler dejó de leer: un lector instrumentado del cuerpo registra como máximo `LogoMaxBodyBytes` bytes leídos |
  | Cuerpo de más de 2 162 688 bytes (una parte `file` chica precedida de relleno en su encabezado o en el preámbulo) | `413 payload_too_large` |
  | **Décima revisión** (plan §9.2): parte `file` chica (PNG válido de 1 KB), *boundary* de cierre y un **epílogo** que lleva el cuerpo a `LogoMaxBodyBytes` + 1 bytes | `413 payload_too_large`; el fake de S3 **no** recibió `Put` (el handler lee el cuerpo hasta el final antes de llamar al servicio). Con un epílogo que lo deja en exactamente `LogoMaxBodyBytes` → `200`. Pasa hoy: mutación (quitar la lectura del resto del cuerpo → `200` en el primer caso) y se restaura |
  | Sin parte `file`, o con la parte `file` vacía | `422` con `errors: [{field: file, code: required}]` |
  | Parte `file` más otra parte `foo`, o dos partes `file` | `400 malformed_request` |
  | Cuerpo multipart mal formado (sin *boundary* de cierre) | `400 malformed_request` |
  | `Content-Type: application/json` | `415` |
  | GIF con `Content-Type: image/png` en la parte | `415 unsupported_media_type` |
  | PNG de 2001×10 | `422` con `errors: [{field: file, code: invalid_value}]` |
  | `GET /tenant/logo` | `200` con `Content-Type` exacto, `X-Content-Type-Options: nosniff`, `Cache-Control: private, no-cache`, `ETag` entre comillas; **sin** `no-store` |
  | `GET /tenant/logo?v=cualquier-cosa` | misma respuesta que sin `v` (el parámetro se ignora y es válido para el contrato) |
  | `GET /tenant/logo` con `If-None-Match` igual al `ETag` | `304` sin cuerpo, con `ETag` y `Cache-Control: private, no-cache` |
  | Reemplazar el logo y repetir con el `If-None-Match` anterior | `200` con los bytes nuevos y otro `ETag` |
  | Sin logo | `404` problem+json con `no-store` |
  | **Décima revisión** (plan §9.1, DD-42): `GET /tenant/logo` con el storage caído (el fake devuelve `objectstore.ErrUnavailable` en `Get`) | `503 service_unavailable` problem+json con `no-store` |
  | **Décima revisión** (plan §9.2; Red): el cliente corta la conexión durante la descarga (un `ResponseWriter` del test cancela el contexto del request y falla en `Write`) | ningún log `WARN` ni `ERROR`; log de request con `status=499` y `event=client_canceled` en `INFO` |
  | `DELETE /tenant/logo` | `204` con `no-store`; un `GET` posterior con el `If-None-Match` viejo → `404` (nunca `304`) |
- **Green**: pasa la tabla.

**T-B706 — Implementar los handlers** (en `tenant/http.go`: `http.MaxBytesReader` con
`LogoMaxBodyBytes`, `r.MultipartReader()` con exactamente una parte `file` leída con tope de
`LogoMaxBytes + 1` bytes, y el mapeo de errores de DD-31 y plan §9.2). Décima revisión: después de la parte `file`, el handler lee el resto del cuerpo hasta el final (con el mismo `MaxBytesReader`) **antes** de llamar al servicio: una parte más → `400`, superar `LogoMaxBodyBytes` → `413`. La descarga del logo trata un error de escritura con el contexto del request cancelado como `client_canceled` (`INFO`, `499`) y cualquier otro error a mitad de la copia como `WARN` `event=logo_stream_failed` (plan §9.2). El decodificador del `PATCH` distingue campo ausente de `null` y responde `400` ante `null` en `name` o `timezone` (DD-43).

**T-B707 — Bucket de desarrollo en `compose.yaml`** (décima revisión; plan §10.5.1). El servicio `minio` suma un *healthcheck* en forma exec que crea el bucket del modo local: `mc mb --ignore-existing local/${S3_BUCKET:-crm-dev}`, con `MC_HOST_local` en el `environment` del servicio, armado con `MINIO_ROOT_USER` y `MINIO_ROOT_PASSWORD` (los mismos defaults) y `127.0.0.1:9000`. No se agrega ningún servicio ni imagen: el test de `internal/testsupport/containers` no cambia. Primero se confirma que la imagen de DD-36 trae `mc` (`docker run --rm --entrypoint mc <referencia> --version`); si no lo trae, no se elige otra imagen: aviso al arquitecto. En el mismo cambio, el README ("Levantar todo": `--wait` deja el bucket creado) y `.env.example` (comentario de `S3_BUCKET`). **Verificación**: `docker compose down -v`, `docker compose up -d --wait` sin login a ningún registro, y en la prueba independiente de la Fase 7 la subida del logo responde `200` (antes, `503`).

**Checkpoint Fase 7**: `make check` en verde + prueba independiente con MinIO de `docker compose`.

---

### Fase 8 — Aislamiento entre empresas de punta a punta (SC-002, principio III)

**Objetivo**: demostrar con tests automáticos **0 accesos cruzados en todos los endpoints**, y que
el test falle solo si mañana se agrega un endpoint sin cubrir.

**Prueba independiente**: `go test -tags=integration -run Isolation ./...` en verde, con el
reporte de cobertura de rutas al 100 %.

**Fixture común** (`internal/testsupport/fixture`): empresas `A` y `B`, cada una con un admin, un
operador, un invitado, un desactivado con contraseña, un desactivado sin contraseña, logo y datos
completos; cookies de sesión de cada usuario (Revisión del PR #16: las de los dos desactivados se crean con `CreateSession` **después** de `Deactivate`, como la del invitado, y el fixture verifica que la fila de cada una tenga `revoked_at` nulo y el usuario esté `disabled`: así el rechazo depende del chequeo de estado de `ResolveSession` y no de la revocación; es el estado que deja DD-41, INV-11); tokens vigentes de reset, verificación e invitación
de cada empresa.

**T-B801 [T] — Matriz HTTP de aislamiento con cobertura de rutas** · SC-002, FR-006, INV-12, INV-21
- **Red**:
  - El test obtiene **todas** las rutas registradas con `chi.Walk` **sobre el router de la API**
    (el que `internal/app` monta en `/api/`; la SPA y ops no están en él, DD-22) y las compara con
    una tabla declarada `ruta → caso de aislamiento`. **Si existe una ruta sin caso, el test
    falla** (así se protege a las specs futuras). El mismo listado se compara con los `paths` del
    contrato (sin `/healthz` ni `/readyz`, que están en el mux raíz).
  - Casos por ruta (como admin de `A`, salvo la fila de lecturas del operador):

    | Tipo de ruta | Ataque | Esperado |
    |---|---|---|
    | Con `{userId}` (`role`, `deactivate`, `reactivate`) | usar ids de usuarios de `B` (cada estado) | `404`, cuerpo **y cabeceras** idénticos byte a byte a los de un id inexistente (Revisión del PR #16: todas las cabeceras, nombres y valores; se comparan sobre el router de la API con el mismo contexto de request, así que ninguna varía legítimamente; si la comparación pasara por un servidor real, la única excepción es `Date`, que escribe `net/http`); `B` sin cambios ni filas de auditoría nuevas (verificado como `B`) |
    | Sin id, lectura (`/me`, `/tenant`, `/users`, `/tenant/logo`) | — | solo datos de `A`: ningún centinela de `B` (ver abajo) aparece en el cuerpo ni en las cabeceras |
    | `GET /tenant/logo` con cookie de `A` e `If-None-Match` = `ETag` del logo de `B` | reuso de caché entre empresas en un dispositivo compartido | `200` con el logo de `A` (nunca `304`) (INV-21) |
    | Sin id, escritura (`PATCH /tenant`, `PUT/DELETE /tenant/logo`, `POST /users/invitations`) | — | modifica solo `A`; `B` sin cambios; la clave del logo empieza con `tenants/{A}/` |
    | Públicas con token (`/auth/*`) | token de `B` usado junto con la cookie de `A` | la operación afecta solo a `B` (la cookie no cambia la empresa del token); ninguna fila de `A` cambia |
    | `POST /auth/signup` con el email de un usuario de `B` | — | `409 email_already_registered` sin ningún dato de `B` (INV-20) |
    | Revisión del PR #16: lectura como **operador** de `A` (toda ruta `GET` que la tabla de T-B802 declara con éxito para el operador; hoy `/me`, `/tenant`, `/tenant/logo` y el catálogo) | el logo, también con `If-None-Match` = `ETag` de `B` | lo mismo que las filas de lectura de admin: solo datos de `A` y ningún centinela de `B` en cuerpo ni cabeceras (con la excepción del catálogo); el logo, `200` con los bytes y el `ETag` de `A` (nunca `304`). El conjunto se **deriva** de la tabla de T-B802, no de una lista aparte (una lectura nueva del operador queda cubierta sin editar esta fila), y el test falla si sale vacío |
  - **Centinelas de "ningún dato de la empresa ajena"** (Revisión del PR #16). La empresa ajena de cada fila es la que la operación no debe tocar ni mostrar: `B` en las filas autenticadas como `A` y en el registro con el email de `B`; `A` en las públicas con token o credenciales de `B` (incluido el login de un usuario de `B`), que viajan con la cookie de `A`. Centinelas de una empresa: su id; nombre, zona horaria, moneda, plantilla, fechas y datos de contacto; ids, emails y nombres no nulos de sus usuarios; la clave del logo, su `ETag` y el uuid del logo **sin comillas**; sus tokens crudos de reset, verificación e invitación; el valor de la cookie de cada uno de sus usuarios; y el sufijo común que el fixture agrega a todos sus textos (el fixture lo expone). Se buscan en bytes crudos en el cuerpo **y** en los nombres y valores de todas las cabeceras (incluidas `Set-Cookie`, `Location` y `ETag`) de **toda** respuesta de T-B801, también las `202` y `204` sin cuerpo y los `404` de ids ajenos. Única excepción: la respuesta del catálogo de plantillas (`/industry-templates`, pública y sin datos de empresa) contiene por construcción el código de plantilla de `B`; ahí se buscan todos los centinelas menos ese. **Centinelas cortos** (duodécima revisión): un valor de menos de 8 bytes (hoy la moneda, p. ej. `ARS`, y el código de plantilla `generic`; cualquier otro que el fixture agregue con menos de 8 bytes entra en la misma regla) se busca solo como palabra completa: el byte anterior y el siguiente están fuera de `[A-Za-z0-9_-]` (el alfabeto de base64url y de los UUID), o son el principio o el fin de la parte buscada (el cuerpo, un nombre de cabecera o un valor de cabecera, cada uno por separado). Los de 8 bytes o más se siguen buscando como subcadena. Motivo: un valor de 3 bytes aparece por azar dentro de los tokens aleatorios de `Set-Cookie` (falso acceso cruzado en alrededor de 1 de cada 3 200 corridas); uno de 8 bytes de base64url, en la práctica nunca. El sufijo común del fixture es el centinela que se busca **dentro** de otros textos: tiene al menos 8 bytes (el fixture ya lo cumple) y el test del helper lo comprueba. El helper tiene su propio test con respuestas armadas: un centinela en una cabecera, otro en el cuerpo, el uuid del logo sin comillas y el sufijo, cada uno detectado; y, para un centinela corto, detectado como valor JSON (`"ARS"`), como valor completo de una cabecera, como elemento de una lista (`ARS, USD`) y entre espacios en un texto, y **no** detectado dentro de un token base64url (`…xARSy…`, `…-ARS_…`) ni pegado a letras o dígitos.
  - **Conteo de accesos cruzados** (Revisión del PR #16): cada verificación de aislamiento de T-B801 (centinela encontrado, empresa ajena modificada, `404` distinto del de un id inexistente, sesión creada en otra empresa, logo o `ETag` de la otra empresa) suma 1 a un contador **cuando detecta**. El reporte imprime ese contador y las verificaciones ejecutadas **separadas por tipo** (duodécima revisión): `sentinel_checks` (cada búsqueda de un centinela en el cuerpo o en una cabecera) y `state_checks` (snapshots de la empresa ajena, comparaciones de los `404`, empresa de las sesiones creadas, logo y `ETag` propios, prefijo de la clave del logo), con el formato `Isolation: covered routes=N/M; cross-company accesses=D; sentinel_checks=S; state_checks=E`. Se imprime siempre, también si el test falla; el test falla si el contador no es 0, o si `sentinel_checks` o `state_checks` es 0 (un total alto, dominado por búsquedas de texto, no prueba que se haya verificado algún estado). Ningún número del reporte es un texto fijo.
  - **El router de `chi.Walk` es el que sirve `crm serve`, y solo en `HTTP_ADDR`** (Revisión del PR #16; reglas reescritas en la duodécima revisión). Test estático sobre **todos** los `.go` no-test de `cmd/crm` (no solo `serve.go`), con el paquete verificado por `go/types` (`types.Config.Check` con `importer.ForCompiler(fset, "source", nil)`; solo librería estándar): cada identificador se resuelve a su `types.Object` (`Info.Defs`, `Info.Uses`, `Info.Selections`), nunca por su nombre ni con `ast.Object` (deprecado; el test no lleva `nolint`). Si el chequeo de tipos devuelve cualquier error, el test falla: nunca decide sobre un paquete a medio resolver. *Variable de un solo uso*: variable local (declarada con `:=` o `var` dentro de una función; en una asignación de dos valores, la primera) cuyo objeto aparece exactamente una vez fuera de su declaración (una reasignación, un `_ = x`, un alias o una captura extra cuentan como apariciones), y esa aparición está en la posición que la regla permite. Reglas:
    (1) `app.BuildAPIRouter` se llama exactamente una vez; su resultado es directamente el valor de la clave `API` de un literal `app.RootDeps`, o una variable de un solo uso que aparece en esa clave. Todo literal de tipo `app.RootDeps` es **directamente** el primer argumento de `app.NewRootHandler` (sin variable intermedia), y ningún objeto declarado en `cmd/crm` (variable, parámetro, resultado o campo) tiene tipo `app.RootDeps` ni `*app.RootDeps`: nadie conserva una referencia al router después de entregarlo.
    (2) Ningún archivo no-test de `cmd/crm` importa `net/http` ni sus subpaquetes (`net/http/pprof`, `net/http/httptest`, `net/http/httputil`, …), `expvar`, ni `github.com/go-chi/chi/v5` ni sus subpaquetes. Ninguno los necesita: handlers y servidores los arma `internal/app`, y las métricas se publican en el paquete que las produce. Reemplaza la lista anterior (`http.NewServeMux`, `http.Handle`, `http.HandleFunc`) y además impide `http.ListenAndServe`, `http.Serve`, `http.DefaultServeMux` y handlers propios.
    (3) `app.NewRootHandler` se llama exactamente una vez y su resultado llega al segundo argumento de `app.NewServer`, directo o en una variable de un solo uso, sin envolverlo. `app.NewServer` se llama exactamente una vez y su `*http.Server` es una variable de un solo uso que aparece como argumento `srv` de una llamada a `app.Serve`. En `cmd/crm` no hay ningún literal compuesto de tipo `net/http.Server` (tampoco a través de un alias) ni ninguna selección del campo `net/http.Server.Handler`, de lectura o de escritura.
    (4) Servidor de métricas (T-B903, T-B904, plan §12.1): `app.NewMetricsServer` se llama exactamente una vez y su resultado es una variable de un solo uso que aparece como argumento `srv` de otra llamada a `app.Serve`. Hay exactamente dos llamadas a `app.Serve` y cada una recibe un par coherente: (servidor de `app.NewServer`, listener de `HTTP_ADDR`) o (servidor de `app.NewMetricsServer`, listener de `METRICS_ADDR`). El listener es una variable de un solo uso asignada desde una llamada al campo `listen` de un valor de tipo `env` (decimotercera revisión; antes, desde `(*net.ListenConfig).Listen` o `net.Listen`), cuyo argumento de dirección es directamente la selección del campo `HTTPAddr` o `MetricsAddr` de `config.Config`, respectivamente. Fuera de esas dos llamadas, el campo `env.listen` aparece una sola vez en los archivos no-test de `cmd/crm`: en el literal de `env` de `run`, con valor exactamente `(&net.ListenConfig{}).Listen` o `new(net.ListenConfig).Listen`, sin ninguna otra asignación ni lectura. Es la costura de T-B911: los tests de `runServe` arman su propio `env`.
    (5) (decimotercera revisión) Las cinco funciones que nombran las reglas (1), (3) y (4) (`app.BuildAPIRouter`, `app.NewRootHandler`, `app.NewServer`, `app.NewMetricsServer` y `app.Serve`) solo se usan como llamada directa: cada aparición de su `types.Func` en `Info.Uses` es exactamente el `Fun` de un `ast.CallExpr` (la selección `app.X`, o el identificador si el import es con punto), sin paréntesis de por medio. Cualquier otro uso es una violación (asignarla a una variable, pasarla como argumento, guardarla en un campo o en un literal, `reflect`), aunque las reglas (1) a (4) no vean ninguna llamada fuera de lugar. `go app.Serve(…)` y `defer app.Serve(…)` son llamadas directas.
    Cada violación nombra archivo, línea y número de regla. Mutaciones que tienen que hacerlo fallar: las cinco de la undécima revisión (alias `r := api` y `r.Get(...)`, función local que recibe el router, `API: wrap(app.BuildAPIRouter(...))`, un router chi nuevo en otro archivo de `cmd/crm`, `app.NewServer(cfg, wrap(root))`); y, de la duodécima: (a) `deps := app.RootDeps{API: app.BuildAPIRouter(...), …}` con `deps.API.(interface{ Get(string, http.HandlerFunc) }).Get("/api/v1/backdoor", h)` antes de `app.NewRootHandler(deps, …)` (reglas (1) y (2)); (a') la misma idea sin `net/http`, `deps.Liveness = deps.API` (solo regla (1)); (b) `_ = srv` y `app.Serve(ctx, &http.Server{Handler: otro}, ln, logger)` (reglas (2), (3) y (4)); (c) los listeners intercambiados entre las dos llamadas a `app.Serve` (regla (4)); (d) `metricsSrv.Handler = …` (reglas (3) y (4)); y, de la decimotercera: (e) `mk := app.NewMetricsServer` y `mk(cfg)` (reglas (5) y (4)); (f) `serve := app.Serve` y `serve(serverCtx, srv, ln, logger)` (regla (5)); (g) `(app.Serve)(…)` (regla (5)); (h) un listener de `net.Listen` o de `(*net.ListenConfig).Listen` directo en `runServe` (regla (4)); (i) `e.listen = otro` en `serve.go`, o un valor distinto de `(&net.ListenConfig{}).Listen` en el literal de `env` de `run` (regla (4)). Siguen en verde: renombrar variables, cambiar el alias del import de `app` y mover el cableado a otro archivo de `cmd/crm`. Si el importer `"source"` no resuelve algún paquete (p. ej. con cgo bajo `-race`) o su duración no es aceptable en `make check`, se usa el importer `"gc"` con un `lookup` que abre los archivos de export de `go list -export -deps -json` (también solo librería estándar); las reglas no cambian. Si una fase posterior cambia cómo arranca `crm serve` (p. ej. T-B909), estas reglas se actualizan en el mismo PR.
  - **Nota para las specs siguientes** (H3, Revisión del PR #16): los casos con id de esta tabla son de **usuarios** (`{userId}`). Cuando la spec 002 sume rutas con id de otro recurso, cada recurso necesita su propio caso (ids de `B` en cada estado del recurso → `404` idéntico en cuerpo y cabeceras, `B` sin cambios), no reusar el de usuarios: la cobertura de rutas obliga a declarar una fila, no a que el caso elegido corresponda al recurso. Un caso de id ajeno que falle si la ruta no tiene el parámetro que espera lo haría verificable.
- **Green**: todas las filas pasan y la cobertura de rutas es total.

**T-B802 [T] — Matriz de permisos por endpoint** · FR-007, INV-12
- **Red**: para cada ruta de la tabla de T-B801 × {anónimo, operador, admin}: el status esperado
  (`401` / `403` / éxito) declarado en la tabla; el `403` se obtiene **aunque el id sea de otra
  empresa o no exista** (la autorización no depende de la existencia del recurso).
  - **Revisión del PR #16**: cada ruta protegida (las que la tabla declara `401` para el anónimo), con la cookie de cada usuario no activo de cada empresa (invitado, desactivado con contraseña y desactivado sin contraseña), responde `401` como el anónimo. Las sesiones de los dos desactivados existen **sin revocar** (fixture común): es el estado que deja un login concurrente con la desactivación (DD-41), y solo lo rechaza el chequeo de estado de `ResolveSession` (INV-11). Mutación: cambiar `row.Status != "active"` por `row.Status == "invited"` → fallan las cookies de los dos desactivados (y R5 de T-B604).

**T-B803 [T] — Aislamiento en flujos sin empresa conocida** · plan §4.4
- **Red**:

  | Caso | Esperado |
  |---|---|
  | Login de un usuario de `B` | la sesión creada tiene `tenant_id = B` y `/me` devuelve `B` |
  | Reset de un usuario de `B` | cambia solo ese usuario; sesiones de `A` intactas |
  | Pedido de reset del invitado de `B` | la invitación reemitida y su auditoría quedan en `B`; nada cambia en `A` |
  | Worker con mensajes de `A` y `B` | cada `UPDATE` de **marcado** corre bajo el rol de su empresa; ningún mensaje de `B` pasa por una transacción de `A` |
  | Revisión del PR #16: aplazamiento (`DeferMessage`, ADR-024 §6, INV-31) | El observador reconoce cada `UPDATE` por el nombre de la query de sqlc (el comentario inicial), contra una lista cerrada: las de marcado del outbox (rol de la empresa y `tenant_id` explícito, como en la fila anterior) y `DeferMessage`. Para `DeferMessage` exige `current_user` = `current_setting('role')` = `crm_worker`, `app.current_tenant_id()` nulo y la transacción sin empresa asociada; no cuenta como marcado ni entra en el control de empresa por transacción. Un `UPDATE` fuera de la lista hace fallar el test. Para ejercitarlo, el *wrapper* de test hace fallar el primer `AsTenant` hacia `B` (solo en el *wrapper*, sin *hooks* en producción): ese mensaje se aplaza **una** vez como `crm_worker`, queda `pending` con `attempts` y `last_error` sin cambios y no se marca en la corrida; los demás se marcan una vez cada uno |
  | `ResolveSession` con token de `B` | `Principal.TenantID = B` |

**T-B804 [T] — Defensa en profundidad demostrada** · INV-01, INV-04
- **Red**: una query **de test** idéntica a la de `ListUsers` pero **sin** `WHERE tenant_id`,
  ejecutada en `InTenantTx(A)`, devuelve solo usuarios de `A` (la RLS sostiene aunque falte el
  filtro); y la query real, ejecutada con un `tenant_id` de `B` como parámetro pero en
  `InTenantTx(A)`, devuelve 0 filas (el filtro y la RLS se combinan, no se contradicen).

**T-B805 — Correcciones** que surjan de T-B801..T-B804. Toda falla es un bug crítico (principio
III): se corrige antes de cerrar la fase, con su test de regresión.

**Checkpoint Fase 8**: `make check` en verde; el test T-B801 imprime el conteo de rutas cubiertas
(= total de rutas) y 0 accesos cruzados, los dos como conteos reales junto con las verificaciones ejecutadas por tipo, `sentinel_checks` y `state_checks`, ambas mayores que 0 (Revisión del PR #16 y duodécima revisión, ver T-B801).

---

### Fase 9 — Robustez y operación

**Objetivo**: el sistema se puede operar: limpia lo vencido, expone salud y métricas, se
restaura sin perder los roles, y su rendimiento con muchas empresas (y con registros
concurrentes) está medido.

**Prueba independiente**: con 10.000 empresas cargadas, T-B910 reporta los targets de `plan.md`
§13 (decimotercera revisión) con su veredicto; borrar un rol de empresa y correr
`crm tenants reprovision-roles` restablece el acceso. Decimotercera revisión: antes decía "los
targets de §13 se cumplen"; cumplirlos pasa a ser condición para salir a producción, no para
cerrar la fase.

**T-B901 [T] — Limpieza periódica** · `data-model.md` §3.4, DD-25, ADR-001, plan §4.4
- **Red** (integración): sesiones y tokens vencidos hace > 30 días, mensajes terminales de > 30
  días y `login_throttles` de > 24 h sin bloqueo vigente se borran **como `crm_worker`**; nada
  vigente o reciente se borra; un `DELETE` de `crm_worker` sobre una sesión vigente afecta 0 filas
  (la política lo impide aunque el código se equivoque). Invitaciones:

  | Token `invitation` vencido hace > 30 días | ¿Se borra? |
  |---|---|
  | Abierto (sin usar ni revocar) de un usuario `invited` | **No** (DD-25); un `DELETE` directo como `crm_worker` afecta 0 filas |
  | Revocado por una reinvitación | Sí |
  | Usado (invitación aceptada) | Sí |
  | Revocado por desactivación | Sí |

  **Novena revisión**, caso diferido de T-B606 (HTTP + contrato): con un invitado cuya invitación venció hace 40 días, después de correr la limpieza, `GET /users` sigue informando su `invitation_expires_at` con esa fecha (DD-25).

  Tareas periódicas (tercera revisión, `outbox.PeriodicTask`):

  | Caso | Esperado |
  |---|---|
  | `Dispatcher` con una `PeriodicTask` falsa de `Every() = 1h` y reloj falso | corre una vez por hora; entre medio el `Dispatcher` sigue enviando mensajes |
  | Una tarea que devuelve error | log `ERROR` con `task=<Name()>`; el `Dispatcher` sigue y la tarea vuelve a correr en el próximo período |
  | Apagado (contexto cancelado) durante una tarea | la tarea termina con `context.Canceled`, su transacción hace `ROLLBACK`, log `INFO` (no `ERROR`) |
  | `identity.Cleanup` | limpia `sessions`, `user_tokens` y `login_throttles` con las queries de `internal/identity/store/cleanup.sql` |
  | Limpieza de `outbox_messages` terminales | con las queries de `internal/platform/outbox/store/worker.sql` (ningún archivo de `outbox` nombra tablas de `identity`) |

**T-B902 — Implementar la limpieza** (tercera revisión: repartida por dueño de las tablas, ADR-001).
`identity.Cleanup` satisface `outbox.PeriodicTask` (una vez por hora) con sus queries en
`internal/identity/store/cleanup.sql`; la limpieza de mensajes terminales es parte del
`Dispatcher`, con sus queries en `internal/platform/outbox/store/worker.sql`; `internal/app`
registra las tareas. Las cuatro rutas de sistema ya están en `queryrules.DefaultExceptions`
(T-B112).

**T-B903 [T] — `/readyz` y métricas** · plan §12
- **Red**: base caída → `503 {"status":"unavailable"}`; versión de migración de la base ≠ la
  embebida → `503`; ok → `200`; `/debug/vars` solo escucha en `METRICS_ADDR` y expone las
  métricas de §12.1 (incluidas `signup_email_exists_total`, `csrf_rejected_total`,
  `signup_lock_timeout_total`, `http_client_canceled_total`, `set_role_retry_total` y, desde la
  quinta revisión, `outbox_delivery_errors_total{cause}` y `outbox_deferred_total`, ADR-024);
  `tenants_total`, `tenant_roles_total` (roles `crm_t_*` del clúster) y `tenant_roles_missing`
  (empresas sin rol), muestreadas como `crm_worker` con las queries de
  `internal/tenant/store/provisioning.sql` (duodécima revisión: antes `tenant_roles_total` era la
  cantidad de empresas, y un rol faltante no se veía; plan §12.4).
  - **`/readyz`** (duodécima revisión, plan §12.2). Integración contra PostgreSQL 18; los casos que bloquean o revocan algo son secuenciales (sin `t.Parallel()`) y restauran en `t.Cleanup`:

    | Caso | Esperado |
    |---|---|
    | Base migrada; `expected` = `migrations.ExpectedVersion(migrations.FS)` | `200 {"status":"ok"}` con `no-store` |
    | `expected` = versión real + 1 (binario más nuevo que la base) y real − 1 (base más nueva) | `503 {"status":"unavailable"}`; log `WARN` `event=readiness_failed`, `reason=schema_mismatch`, `db_version`, `expected_version`; ninguna versión en la respuesta |
    | Base caída (runner contra un puerto cerrado) | `503`, `reason=unavailable` |
    | `public.goose_db_version` con `LOCK TABLE … IN ACCESS EXCLUSIVE MODE` en otra transacción del test | `503`, `reason=timeout`, antes de 3 s (el plazo es 1 s, incluida la espera de una conexión del pool) |
    | `REVOKE SELECT (version_id) ON public.goose_db_version FROM crm_auth` (el estado de una base sin la migración `00009`) | `503`, nunca `500`; log `ERROR` `reason=privilege` con `security_event=rls_violation` (INV-19 precisada) |
    | El cliente corta la conexión durante el chequeo | como §9.2: `499`, `INFO` `client_canceled`; sin `WARN` ni `ERROR` |
    | `db.SchemaVersion` como `crm_auth` | igual a `GetDBVersion` del provider de goose como `crm_owner` |
    | `db.SchemaVersion` con la tabla vacía (en una transacción del test: `DELETE` como `crm_owner`, `SET LOCAL ROLE crm_auth`, llamada, `ROLLBACK`) | `ok=false`; el handler lo responde como `503` `reason=schema_unknown` |
    | `SELECT max(version_id) FROM public.goose_db_version` como `crm_app` sin `SET ROLE`, `crm_worker`, `crm_signup` y un `crm_t_*` | `42501` en los cuatro |
    | Privilegios de los roles de sistema (T-B109 ampliado al esquema `public`; Red: hoy el snapshot solo mira `app`) | `crm_auth`: además de lo de `data-model.md` §3.4, `USAGE` en `public` y `public.goose_db_version.version_id:SELECT`; `crm_worker`, `crm_signup`, `crm_app` y `PUBLIC`: nada en `public`. Las funciones `SECURITY DEFINER` siguen siendo solo `provisioning.provision_tenant_role` (T-B107 sin cambios, INV-08) |
    | `migrations.ExpectedVersion` | con `migrations.FS`, igual a la versión que registra el `Up` del harness; con un `fstest.MapFS` sin `.sql` o con un nombre sin número, error |

    Mutaciones: comparar con `<` en lugar de `≠` → falla la fila de base más nueva; quitar el plazo de 1 s → falla la fila del `LOCK`; responder `500` ante `db.ErrPrivilege` → falla la fila del `REVOKE`.
  - **Métricas que leen la base** (duodécima revisión, plan §12.1): nunca se consultan durante un scrape.

    | Caso | Esperado |
    |---|---|
    | `outbox.Stats.Run` con pendientes de `A` y `B` y mensajes terminales, reloj falso | `outbox_pending` = cantidad de pendientes; `outbox_oldest_pending_seconds` = reloj − `created_at` del pendiente más viejo, **calculado al leer** `/debug/vars` (avanzar el reloj sin correr la tarea lo aumenta); 0 sin pendientes |
    | `tenant.RoleInventory.Run`, test secuencial: se borra (como superusuario) el rol de una empresa del fixture, se corre la tarea, se vuelve a llamar a `provision_tenant_role` y se corre otra vez | `tenant_roles_missing` sube en 1 y `tenant_roles_total` baja en 1; después, los dos vuelven a su valor; `tenants_total` = filas de `tenants` de la base del paquete |
    | Antes de la primera muestra, o cuando la última falló (base caída) | los cinco gauges valen `-1`; log `ERROR` `task=<Name()>` (T-B901); `GET /debug/vars` responde sin esperar a la base (el test lo acota en 1 s) |
    | `Every()` de las dos tareas | 60 s |
    | Queries nuevas | `PendingStats`, `CountTenants` y `CountTenantsWithoutRole` figuran por nombre en `queryrules.DefaultExceptions` (T-B112 falla si falta una); `CountTenantRoles` solo lee `pg_catalog` y no necesita excepción |
  - **Servidor de métricas** (duodécima revisión; se adelanta al paso 0 de esta fase, ver "Estado de la implementación"). Tests de `internal/app` sobre el handler real, sin red:

    | Caso | Esperado |
    |---|---|
    | `app.NewMetricsServer(cfg)` | `Addr` = `cfg.MetricsAddr`; `Handler` no nil y distinto de `http.DefaultServeMux`; los timeouts de `NewServer` (plan §10.3); sin `TLSConfig` |
    | `GET /debug/vars` en su handler | `200` JSON con las variables publicadas (al menos `cmdline`, `memstats` y las de plan §12.1 que ya existan) |
    | Cada ruta de `chi.Walk` sobre `app.BuildAPIRouter` (parámetros reemplazados por un UUID) con su método; `GET /healthz`, `GET /readyz`, `GET /` | `404` del mux de métricas (`Content-Type` distinto de `application/problem+json`), nunca la respuesta de la API, de ops ni de la SPA |
    | `POST /debug/vars` | `405` |
    | Una ruta que el test registra en `http.DefaultServeMux` (sonda) | `404`: el handler de métricas no delega en el mux global (donde el `init` de `expvar` registra `/debug/vars` y donde caería `net/http/pprof`) |
    | `GET /debug/vars` en el mux raíz (`NewRootHandler` con el router real y la SPA *stub*) | llega a la SPA (el *stub* lo registra); la respuesta no contiene `"memstats"` ni `"cmdline"` |
    | `app.NewServer(cfg, nil)` | error, antes de escuchar (con `Handler` nil, `net/http` usaría `http.DefaultServeMux`, que en este binario ya expone `/debug/vars`) |
    | `crm serve` (función `run`) con `METRICS_ADDR` ocupado por un listener del test y `HTTP_ADDR` libre | no arranca: error que nombra `METRICS_ADDR` |

    Mutaciones (se aplican, el caso falla, se restauran): el handler de métricas monta un router de la API en `/api/` o registra `/healthz` → falla la fila de rutas; `Handler: nil` en `NewMetricsServer` → falla la sonda; `NewRootHandler` registra `/debug/vars` → falla la fila del mux raíz; en `serve.go`, los listeners intercambiados entre las dos llamadas a `app.Serve` → falla la regla (4) de T-B801.
  - `/readyz` sigue en el mux raíz (`HTTP_ADDR`, plan §12.2), nunca en el servidor de métricas: `app.ReadinessHandler` reemplaza a `app.ReadinessPlaceholder()` en `RootDeps.Readiness` sin cambiar ninguna regla del guard de T-B801.

**T-B904 — Implementar `/readyz` y `expvar`**. Duodécima revisión:
- **Servidor de métricas**: lo arma `internal/app` (plan §11.1, §12.1): `app.NewMetricsServer(cfg)` con un `http.ServeMux` propio que solo registra `GET /debug/vars` → `expvar.Handler()`, sin la cadena común; `app.NewServer` rechaza un handler nil. `serve.go` solo lo invoca: escucha `HTTP_ADDR` y después `METRICS_ADDR` antes de arrancar el worker (el error nombra la variable) y llama a `app.Serve` una vez por servidor, con el par de la regla (4) de T-B801. Si una de las dos llamadas a `app.Serve` termina con error mientras la otra corre, `crm serve` cancela el contexto común (se apagan el otro servidor y el worker) y devuelve ese error. Las métricas se publican en el paquete que las produce (como `login_failed_total` en `identity`): `cmd/crm` no importa `expvar` ni `net/http` (regla (2) de T-B801).
- **`/readyz`**: migración `00009_readiness_schema_version.sql` (`GRANT USAGE ON SCHEMA public` y `GRANT SELECT (version_id) ON public.goose_db_version` a `crm_auth`; el `Down` los revoca; `data-model.md` §3.5, §6); `migrations.ExpectedVersion`, `db.SchemaVersion` y `app.ReadinessHandler` (plan §11.1, §12.2), que reemplaza a `app.ReadinessPlaceholder()` en `RootDeps.Readiness`. `serve.go` calcula la versión esperada al arrancar con `migrations.ExpectedVersion(migrations.FS)`; si falla, no arranca.
- **Métricas que leen la base**: `outbox.Stats` (query `PendingStats` en `internal/platform/outbox/store/worker.sql`) y `tenant.RoleInventory` (`CountTenants`, `CountTenantsWithoutRole` y `CountTenantRoles` en `internal/tenant/store/provisioning.sql`; el nombre del rol se deriva igual que en `provision_tenant_role`, y la fila de T-B903 que borra un rol lo verifica), registradas por `internal/app` como tareas del `Dispatcher` (T-B902). Los nombres nuevos sobre `outbox_messages` y `tenants` se agregan a `queryrules.DefaultExceptions` en el mismo cambio (plan §4.4). No hace falta ninguna regla nueva de `queryrules` ni de `reporules`: la lectura de la versión es SQL constante de `platform/db` y no toca tablas de empresa; lo que la acota es el test de privilegios de T-B109 ampliado al esquema `public`.

**T-B905 — Benchmark con 10.000 empresas** · R-2, R-3, R-15, R-17, ADR-005, DD-33, DD-34, INV-26
- Script de carga (test con build tag `bench`, fuera de `make check`) que aprovisiona 10.000
  empresas en un contenedor con la versión exacta de producción y mide p95 de: `SET LOCAL ROLE`
  (incluida la lectura de catálogo de DD-34, que va en el mismo viaje), `GET /me`,
  `GET /tenant/logo` con `304`, `POST /auth/login`, `POST /auth/signup` y el tiempo de conexión de
  `crm_app`.
- **Registros concurrentes** (tercera revisión, DD-33): con los 10.000 roles creados, tandas de
  **10** y de **50** registros simultáneos. Se mide la duración de la transacción de registro con
  el hash de contraseña precalculado (argon2 fuera de la medición), que acota por arriba el tiempo
  que se retiene el lock de `crm_tenant`. Target: **p95 < 250 ms y 0 `503` con 10 concurrentes**;
  con 50 se reporta la cantidad de `503`.
- Al final de la corrida se reporta `set_role_retry_total` (esperado: 0; cualquier valor mayor se
  analiza con el runbook de plan §12.3).
- Comparar con `plan.md` §13. **Si algún target no se cumple, frenar y volver al arquitecto** (se
  reabre ADR-005; opciones ya analizadas en research R-04c y R-28).
- **Decimotercera revisión**: ejecutada el 2026-10-08 (corrida 1; ver "Estado de la
  implementación"). El target de registros concurrentes medía la espera en el lock más la
  retención, no la retención: queda como dato reportado, sin target. Los targets nuevos y el
  desglose los mide T-B910, que reemplaza este test y quita su `t.Fatal` por `registration_tx_10`.

**T-B906 [T] — Reaprovisionamiento de roles** · plan §12.4, DD-33 (R-d)
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | Con dos empresas, se borra el rol de `A` (como superusuario del contenedor) | los requests de `A` fallan con `500` y log que nombra el rol; `crm tenants reprovision-roles` recrea el rol y sus membresías; los requests de `A` vuelven a funcionar; `B` nunca se ve afectada |
  | Correrlo de nuevo | no cambia nada (idempotente) |
  | Tres empresas sin rol; la del medio falla (forzado con un *hook* de test en el paso por empresa) | la corrida sigue: las otras dos quedan con su rol (cada una en **su propia transacción**, ya confirmada); `ReprovisionReport{Tenants: 3, Failed: [<id del medio>]}`; el comando sale con código ≠ 0 y nombra el id que falló |
  | Dos ejecuciones simultáneas | la segunda termina enseguida con "otra reprovisión en curso" y código ≠ 0; la primera no se ve afectada |
  | Un registro (T-B303) lanzado mientras se reprovisionan 50 empresas | el registro tiene éxito: cada transacción de la reprovisión suelta el lock de `crm_tenant` al terminar |

**T-B907 — Implementar `crm tenants reprovision-roles` y `tenant.Service.Reprovision`** (plan
§11.1, §12.4): lista `tenants.id` como `crm_worker` con la query de
`internal/tenant/store/provisioning.sql` (plan §4.4) y llama a `provision_tenant_role` como
`crm_signup` en **una transacción por empresa**; durante toda la corrida sostiene un *advisory
lock* de sesión con una clave constante declarada en `internal/tenant`. Como el lock de sesión
necesita una conexión dedicada y las conexiones las maneja `platform/db`, la operación que lo
toma vive en `platform/db`: si hace falta una firma nueva, el desarrollador la propone en el PR
de la fase y se agrega a plan §11.1 en el mismo cambio (matriz §16).

**T-B908 [T] — Apagado ordenado** · plan §9.4, research R-27
- **Red**: SIGTERM con un request en curso y un mensaje en envío → el request termina, el mensaje
  queda `sent` o `pending` (nunca a medio actualizar), el proceso sale con código 0 antes del
  timeout de apagado. Tercera revisión: si el apagado cancela el envío, el mensaje queda `pending`
  con `attempts`, `next_attempt_at` y `last_error` sin cambios, log `INFO` `outcome=canceled`; un
  request cuyo contexto cancela el propio servidor (no el cliente) recibe `503`; en todo el
  apagado no hay logs `ERROR` por cancelaciones. Quinta revisión (DD-35): si el envío termina con
  éxito durante el apagado, el mensaje queda `sent` (no se reenvía al volver a arrancar), y el
  proceso espera un envío en curso hasta `outbox.SendBudget` antes de salir. Duodécima revisión: la
  misma señal apaga también el servidor de métricas (deja de atender al empezar el apagado y no
  espera al worker), sin logs `ERROR` por su cierre, y el proceso sale con código 0.

**T-B909 — Implementar el apagado ordenado del servidor y del worker**. Quinta revisión (DD-35): el timeout de apagado es ≥ 25 s (`outbox.SendBudget` + 5 s), porque go-mail no corta una conversación SMTP en curso al cancelarse el contexto.

**T-B910 — Re-medición con 10.000 empresas: retención del lock, ráfagas y desglose** · R-2, R-3, R-15, R-17, ADR-005, DD-33 (R-b, R-e), DD-34, INV-26 · decimotercera revisión
- **Alcance**: solo código de test en `internal/testsupport/phase9bench/` (build tag `bench`, fuera de `make check`). Reemplaza a `TestPhase9Benchmark` de T-B905 y conserva sus mediciones: `registration_tx_10` y `registration_tx_50` se siguen reportando, sin target. No cambia el código de producción, `lock_timeout`, el pool de runtime, los targets ni el diseño. Si algo no se puede medir sin tocar producción, se reporta "no medido" y se sigue.
- **Entorno**:
  - El de la corrida 1: imagen por digest, `server_version` 18.6 verificada, 10.000 empresas con `Register` real, hash precalculado salvo en login y signup HTTP.
  - La medición usa un pool propio del bench como `crm_app`, con `pool_max_conns=8` **explícito** en el DSN.
  - En `ConnConfig.Tracer` va un tracer del bench que implementa `pgx.QueryTracer` y `pgxpool.AcquireTracer`. En pgx v5.11, `BEGIN` y `COMMIT` pasan por `Conn.Exec` y también se trazan; pgxpool usa el `AcquireTracer` si el tracer de la conexión lo implementa.
  - El log registra digest, `server_version`, Go, SO/arquitectura, CPU, memoria de Docker y `pool_max_conns`.
- **Instrumentación**:
  - Id de muestra: el bench lo pone en el contexto antes de llamar a `Register`. El tracer registra por evento el id de muestra, el PID del backend, una etiqueta y tiempos monotónicos.
  - Etiquetas: el nombre de sqlc (`-- name: X`); `set_role:crm_signup`, `set_role:crm_auth` y `set_role:tenant`, por el texto de `setRole`; `begin`, `commit` y `acquire`.
  - Intervalos por muestra:
    - `pool_wait`: lo que tarda el `acquire`;
    - `pre_callback`: del fin del `acquire` al fin de `set_role:crm_signup`;
    - `provision`: la sentencia `ProvisionTenantRole`, que con contención incluye la espera por el lock;
    - **`hold`**: del inicio de `ProvisionTenantRole` al fin del `commit`. Es la cota superior de la retención, porque incluye el `CREATE ROLE` que va antes del `GRANT`;
    - `callback`: como en la corrida 1;
    - `end_to_end`: de la llamada a `Register` a su retorno.
  - Autocontroles (el test falla si no se cumplen): toda sentencia de una muestra tiene etiqueta conocida y el id de muestra; cada muestra tiene exactamente un `begin`, un `ProvisionTenantRole` y un `commit`; en M-2, los `hold` de muestras distintas no se solapan.
- **Mediciones**:

  | Id | Qué | Cómo | Se reporta |
  |---|---|---|---|
  | M-1 | Curva de escala | Durante la carga de las 10.000, por bloque de 1.000 registros | p50/p95 de `hold`, `provision`, `set_role:tenant`, `pre_callback` y `callback` por bloque; duración de cada bloque |
  | M-2 | Registro aislado con 10.000 roles | 100 `Register` secuenciales después de la carga, sin otra actividad | p50/p95/máx de cada intervalo y de cada sentencia etiquetada |
  | M-3 | Cuerpo de `provision_tenant_role` | 50 transacciones por el pool de superusuario: `SET LOCAL ROLE crm_provisioner`; luego `CREATE ROLE`, `GRANT crm_tenant … WITH INHERIT TRUE, SET FALSE` y `GRANT <rol> TO crm_app WITH INHERIT FALSE, SET TRUE`, con los mismos atributos que la función y cada uno medido; `ROLLBACK` | p50/p95 por sentencia (aproximación: sin plpgsql ni `SECURITY DEFINER`) |
  | M-4 | Primer uso en otra conexión después de un registro | 20 veces: el bench toma 7 conexiones del pool de medición con `Acquire` y corre un `Register` (usa la 8.ª). En cada una de las 7 abre una transacción, mide dos veces seguidas el `setRole` de `crm_auth` (mismo texto que `platform/db`): la primera y la "caliente". Hace `ROLLBACK` y la libera. Además, 20 `GET /api/v1/me` por el router real, cada uno inmediatamente después de un registro | p50/p95/máx de la primera y de la caliente (140 muestras cada una); p95 del `GET /me` posterior a un registro |
  | M-5 | Ráfagas | 10 y 50 `Register` simultáneos (barrera como en la corrida 1), **5 repeticiones** de cada una. Entre repeticiones se espera a que el pool no tenga conexiones tomadas. Durante cada ráfaga de 50, una goroutine hace `GET /api/v1/me` secuenciales por el router real (mismo pool) desde la barrera hasta que vuelve el último `Register` | Por tamaño: muestras, cantidad de `503` y p50/p95/máx de `end_to_end`, `pool_wait`, `provision`, `hold` y `callback`. Espera estimada en el lock = `provision` − p50 de `provision` en M-2. `GET /me` durante la ráfaga: muestras y p50/p95/máx |
  | M-6 | Lo de la corrida 1 | Mismo método que T-B905 | Los mismos ocho valores; para `SET LOCAL ROLE`, además, las 10 muestras de calentamiento por separado; `set_role_retry_total` |

- **Corridas**: dos completas, cada una con un contenedor nuevo. Cada target se evalúa en las dos. No se agregan corridas para desempatar ni se repite una buscando aprobar.
- **Targets** (`plan.md` §13, decimotercera revisión). El test falla si alguno no se cumple en alguna de las dos corridas:

  | Id | Target |
  |---|---|
  | T-1 | `hold` de M-2: p95 < 140 ms |
  | T-2 | Ráfaga de 10 (M-5): 0 `503` en las 50 muestras |
  | T-3 | M-6, p95: `GET /me` < 50 ms, logo `304` < 50 ms, login < 400 ms, signup < 1000 ms |
  | — | `set_role_retry_total` = 0 (si no, runbook de plan §12.3) |

- **Reglas que se reportan sin hacer fallar el test** (línea `BENCH rule <id> triggered=<bool>`):

  | Id | Condición | Consecuencia (plan §18, decimotercera tanda) |
  |---|---|---|
  | D-a | Algún `503` con 50 simultáneos | Solo se reporta (DD-33 R-a) |
  | D-b | `GET /me` durante la ráfaga de 50: p95 > 1 s | Se le propone al usuario un semáforo de registros en el proceso |
  | D-c | M-4: primer `setRole` después de un registro con p95 > 250 ms | Volver al arquitecto |
  | D-d | M-1: el p95 de `hold` del bloque 9.001–10.000 dividido el del bloque 4.001–5.000 da > 2,5 | Volver al arquitecto (crecimiento más que lineal) |

- **Reporte**: va en la entrada de la Fase 9 de "Estado de la implementación": una tabla por medición y por corrida, y el veredicto por target y por regla. El desarrollador no decide ni cambia el diseño. Si T-1, T-2 o T-3 no se cumplen, o se dispara D-c o D-d, completa igual el checkpoint de la fase y lo reporta.

**T-B911 [T] — Tests de `runServe`: falla de un servidor y espera del worker** · T-B904, T-B908, T-B909, DD-35, plan §9.4 · decimotercera revisión
- **Costura**: `env` (`cmd/crm/main.go`) suma `listen func(ctx context.Context, network, address string) (net.Listener, error)`. `run` lo inicializa con `(&net.ListenConfig{}).Listen`, y `runServe` escucha `HTTP_ADDR` y `METRICS_ADDR` solo con `e.listen` (regla (4) de T-B801). Nada más cambia en `runServe`.
- **Red** (integración en `cmd/crm`, contra el PostgreSQL del harness, con un SMTP de prueba propio del test y sin `t.Parallel()`):

  | Caso | Esperado |
  |---|---|
  | El `listen` del test envuelve listeners reales. Con los dos servidores atendiendo, el `Accept` de `HTTP_ADDR` pasa a devolver un error permanente | `runServe` vuelve antes de `app.ShutdownTimeout` con un error que envuelve el del `Accept` (`errors.Is`). Después, `METRICS_ADDR` no acepta conexiones y el `Dispatcher` terminó |
  | Lo mismo con el `Accept` de `METRICS_ADDR` | Simétrico: `HTTP_ADDR` deja de aceptar, el worker terminó y `runServe` devuelve el error |
  | Se cancela el contexto (equivalente a SIGTERM) con un mensaje del outbox en envío, contra un SMTP de prueba que acepta la conexión y retiene la conversación hasta que el test lo libera (menos que los 5 s por etapa de DD-35) | `runServe` **no** vuelve mientras el SMTP retiene (se verifica durante al menos 500 ms). Al liberarlo, vuelve con `nil` antes de `app.ShutdownTimeout`. El mensaje queda `sent` o `pending`, nunca a medio actualizar (T-B908) |
  | Se cancela el contexto sin trabajo en curso | `nil` |

- **Mutaciones** (se aplican, el caso falla y se restauran):
  - quitar el `stopServers()` después del `app.Serve` de la API → falla la fila 1 (el test acota la espera);
  - lo mismo con el de métricas → falla la fila 2;
  - quitar la espera de `workerDone` → falla la fila 3;
  - devolver `nil` en lugar de `serveErr` → fallan las filas 1 y 2.
- **Green**: las cuatro filas pasan. El guard de T-B801, con las reglas (4) y (5) nuevas, pasa sobre el código real y falla con sus mutaciones (e) a (i). `make check` en verde.

**Checkpoint Fase 9** (decimotercera revisión; antes decía "`make check` en verde + resultado de T-B905 reportado con los números"):
- `make check` en verde.
- La prueba independiente con `crm serve` real **sobre el HEAD final**: `/readyz` con migrate down/up, dos ciclos de muestreo después de borrar y restaurar un rol, y SIGTERM con un request en curso.
- T-B905 (corrida 1, ya reportada) y T-B910 reportada con su veredicto. El veredicto de T-B910 **no** bloquea el cierre de la fase: decide si 001 puede salir a producción con ADR-005 tal como está.

**Condición de salida a producción** (no es una condición de la fase; viene de ADR-005, consecuencia 1): T-1, T-2 y T-3 de T-B910 cumplidos en las dos corridas, sin que se disparen D-c ni D-d. Si no, tiene que estar implementado el ADR que reemplace a ADR-005, o una decisión del usuario que lo evite.

---

### Trazabilidad

| Requisito / criterio / decisión | Tareas |
|---|---|
| FR-001 Registro autónomo | T-B303, T-B305 |
| FR-002 Plantillas | T-B301, T-B303 |
| FR-003 Email + contraseña, hash robusto | T-B205, T-B402, T-B404 |
| FR-004 Recuperar contraseña | T-B501, T-B502, T-B506 |
| FR-005 Invitar, desactivar, cambiar rol (+ reactivar, P-3) | T-B601..T-B606 |
| FR-006 Aislamiento | T-B101, T-B103, T-B107..T-B112, T-B801..T-B804 |
| FR-007 Matriz de permisos | T-B207, T-B309, T-B606, T-B705, T-B802 |
| FR-008 Auditoría (y reactivación, P-3) | T-B209, T-B303, T-B402, T-B501, T-B601, T-B603, T-B604 |
| US-1 (1, 2, 3) | T-B303 (1, 2), T-B305 (2), T-B303/T-B504 (3) |
| US-2 (1, 2, 3) | T-B402 (1, 2), T-B501/T-B502 (3) |
| US-3 (1, 2, 3, 4) | T-B601 (1), T-B602 (2), T-B604 (3), T-B603/T-B604 (4) |
| US-4 (1) | T-B702, T-B703, T-B705 |
| Casos borde (404 de otra empresa, 403 de operador) | T-B606, T-B801, T-B802 |
| SC-001 Panel en < 3 min | T-B305 (un request), T-B905 (latencia de signup, también con registros concurrentes), T-B910 (de punta a punta en ráfagas) |
| SC-002 0 accesos cruzados | T-B110, T-B801..T-B804 |
| P-2 Verificación no bloqueante | T-B309, T-B504 |
| P-3 Reactivación | T-B604, T-B605, T-B606, T-B801 |
| P-4 `email_already_registered` + login/reset no enumerables (tiempo del reset fuera, DD-39) | T-B201, T-B305, T-B402, T-B404, T-B501, T-B506, T-B801 |
| P-5 Sesión 24 h / 7 días | T-B002, T-B305, T-B307, T-B402, T-B404 |
| H-1 Mux raíz (DD-22, INV-22) | T-B004, T-B005, T-B011, T-B801; T-F007, T-F008 |
| H-2 Caché del logo (DD-23, INV-21) | T-B703, T-B705, T-B801 |
| H-3 / H-10 HTTPS en todos los entornos, TLS local, sin HSTS en modo local (DD-24, INV-23) | T-B002, T-B004, T-B005, T-B013, T-B014, T-B203, T-B204, T-B213, T-B305, T-B404; T-F006, T-F701 |
| H-4 `invitation_expires_at` vencida (DD-25) | T-B605, T-B606, T-B901 |
| H-5 Rol de invitados (DD-26) | T-B601, T-B603, T-B605, T-B606 |
| H-6 Zona horaria del registro (DD-27) | T-B001, T-B303, T-B305, T-B702 |
| H-7 `no-store` en la API (DD-28) | T-B203, T-B305, T-B705 |
| H-8 Cabeceras y gzip de la SPA (DD-29) | T-B004 (cabeceras comunes); T-F007, T-F008 (CSP, caché, gzip) |
| H-9 CSRF en problem+json (DD-30) | T-B203, T-B204, T-B903 |
| H-11 Límites exactos del logo (DD-31, INV-24) | T-B703, T-B704, T-B705, T-B706 |
| DD-40 / DD-42 / INV-17 / INV-34 Lock de la fila de `tenants` en la Fase 7, `COMMIT` incierto y lectura del logo (décima revisión) | T-B604 (R0), T-B702, T-B703, T-B704, T-B705, T-B706 |
| DD-43 / DD-27 Semántica de `PATCH /tenant` y `Local` (décima revisión; contrato v0.4.3) | T-B303, T-B701, T-B702, T-B705 |
| Bucket de desarrollo (plan §10.5.1) | T-B707 |
| JPEG re-codificado en el navegador (DD-F21): el backend no depende de eso (DD-11, INV-24) | T-B703 |
| Rutas de la SPA en inglés (DD-14) | T-B002, T-B213 |
| `405` con `method_not_allowed` y `Allow` (contrato v0.4.0, research R-26) | T-B004, T-B005, T-B201, T-B203; T-F004, T-F101 (frontend) |
| DD-32 / INV-25 IP del cliente detrás de proxies (research R-25) | T-B002, T-B004, T-B011, T-B014, T-B203, T-B204, T-B217, T-B218, T-B219, T-B220 |
| Cancelaciones: `db.ErrCanceled`, `499` en el log, worker sin intento (research R-27) | T-B103, T-B105, T-B106, T-B202, T-B203, T-B211, T-B901, T-B908 |
| DD-33 / INV-26 Lock de `GRANT crm_tenant` (R-a..R-e; research R-04c; notas en ADR-005) | T-B105 (`55P03`), T-B201, T-B303, T-B304, T-B305, T-B306, T-B903, T-B905, T-B906, T-B907, T-B910 |
| DD-34 / INV-27 / R-17 Primer `SET ROLE` desde otra conexión: lectura de catálogo y reintento único (research R-28; nota (b) en ADR-005) | T-B103, T-B104, T-B903, T-B905, T-B910 |
| Arranque y apagado de `crm serve`: dos servidores, listeners, worker, guard de composición (decimotercera revisión) | T-B801, T-B903, T-B904, T-B908, T-B909, T-B911 |
| Rutas exactas de las queries de sistema (plan §4.4, INV-04, ADR-001) | T-B112, T-B212, T-B304, T-B902, T-B903, T-B907 |
| FK `tenant_id → tenants(id)` de `sessions` y `user_tokens` (`data-model.md` §2.3/§2.4) | T-B108, T-B111 |
| ADR-024 / INV-28 / INV-30 Clasificación de fallos de entrega y `last_error` saneado (research R-29) | T-B211, T-B212, T-B213, T-B214, T-B903 |
| ADR-025 / INV-30 `last_error` sin texto del proveedor en `data`, redacción ampliada y código extendido leído del texto | T-B211, T-B212, T-B213, T-B214 |
| DD-37 / INV-32 Credenciales en `audit_log.data` | T-B209, T-B210 |
| DD-37 (4) y (5) / INV-32 Credencial incrustada en un texto y `data` exacto del catálogo (séptima revisión) | T-B209, T-B210, T-B303, T-B304 |
| DD-38 / INV-33 `user_agent` acotado en cada escritura | T-B209, T-B210, T-B303, T-B304, T-B305 |
| DD-35 / INV-29 Presupuesto del envío frente a `idle_in_transaction_session_timeout` | T-B211, T-B213, T-B214, T-B908, T-B909 |
| INV-31 Aislamiento de fallos por mensaje del `Dispatcher` | T-B112, T-B211, T-B212 |
| DD-36 Imagen de MinIO en desarrollo y tests (research R-30) | T-B215, T-B216 (paso 4 obligatorio al comienzo de la Fase 3, séptima revisión) |
| ADR-014 (nota 2026-10-02) / principio VI Validación de request y response contra el contrato (séptima revisión) | T-B311, T-B312, T-B004, T-B201, T-B301, T-B305, T-B309 (y todo test HTTP de las Fases 4 a 8) |
| Contrato v0.4.1: `413 payload_too_large` en las operaciones con body JSON (séptima revisión) | T-B201, T-B305, T-B311; T-F004 (tipos regenerados, frontend) |

---

## Frontend

Autor: `frontend-architect`. Implementa: `frontend-developer`, **una fase por invocación** (modo
en `.claude/dev-mode`). Diseño: [`ui.md`](ui.md) · ADR-015 a ADR-023.
**Revisión 2026-09-29**: respuestas del usuario a P-F1 a P-F5 y resolución de H-1 a H-9 por el
backend (plan §18). Tareas afectadas: T-F002 (piso Chrome 112), T-F006 (proxy sin `changeOrigin`,
S-F3 refutado para Chrome), T-F007/T-F008 (firmas `web.DistFS`/`web.NewHandler`, stub, gzip
aprobado), T-F202 (sin reintento por zona horaria), T-F502/T-F503/T-F504 (invitación vencida, rol
de invitados, reinvitación con otro rol), Fase F6 (preparación del logo: T-F603 a T-F605 nuevas o
reescritas), Fase F7 (T-F706 nueva: logo en navegadores reales; correcciones pasan a T-F707).
**Segunda revisión 2026-09-29**: decisiones del usuario sobre H-10 (HTTPS local con mkcert), H-11
(archivo ≤ 2 097 152 bytes) y P-F6 (todo JPEG se re-codifica), incorporadas por el backend
(plan DD-24, DD-31, §10.5.1; contrato v0.3.1). Tareas afectadas: comandos (`npm run dev` y
`npm run e2e` sobre HTTPS), tabla de dependencias, T-F004 (contrato v0.3.1), T-F006 (Vite con
`server.https`, proxy a `https://localhost:8443`, spike de la cookie en Chrome, Firefox y Safari),
T-F007 (HSTS según el modo), T-F009 (README y job de E2E con mkcert), T-F101 (errores `400`/`422`
del logo), T-F603/T-F604 (límite exacto 2 097 152), F7 (sin precondición de H-10; T-F701 con la
receta de CI y el supuesto S-12), T-F702/T-F703 (cookie real, respaldo `ignoreHTTPSErrors`),
T-F706, pruebas independientes de F0, F1, F2 y F6 (sin Firefox ni `http://`). Hallazgo nuevo H-12
(prueba en un celular real) en `ui.md` §27.2: no bloquea.

### Convenciones de esta sección

- `[T]` = tarea de test. Va **antes** de la tarea de código que la hace pasar, y el test debe
  fallar por la razón correcta antes de implementar (Red).
- Trazabilidad: `US-n` (historia de la spec), `FR-`, `SC-`, `P-` (plan), `DD-` (plan), `BR-F`,
  `INV-F`, `DD-F`, `NFR-F`, `H-` (hallazgos) y secciones de `ui.md`; `ADR-`.
- **Se prueba lo que el usuario percibe**: consultas por rol y nombre accesible
  (`getByRole('button', { name: 'Crear cuenta' })`), nunca por clase CSS ni estructura del DOM;
  nada de estado interno ni conteo de renders.
- Tests de pantalla con el **árbol de rutas real** (`createMemoryRouter`), un `QueryClient` nuevo
  por test y la red interceptada con **MSW**; handlers y fixtures tipados con los tipos generados
  del contrato y con datos ficticios.
- **Qué se sustituye y qué no** (ADR-022):

  | Se sustituye | Nunca se sustituye |
  |---|---|
  | La red, con MSW (`onUnhandledRequest: 'error'`) | Hooks de datos, `QueryClient`, router, React Hook Form, Zod |
  | El reloj (`vi.useFakeTimers` / `now` inyectado) donde hay horas | Componentes de shadcn/Radix (se usan a través de su accesibilidad) |
  | `window.location.reload` (espía) en el test de `vite:preloadError` | El backend en los E2E (Fase F7) |
  | `Intl.DateTimeFormat().resolvedOptions().timeZone` en el test del registro | Las funciones puras de preparación del logo (firma, dimensiones, plan) |
  | El adaptador de canvas del logo (`decodeImage`, `encodeBitmap`): como parámetro de `prepareLogo` en sus tests y con `vi.mock` del módulo `features/tenant/logo/canvas.ts` en los de pantalla (jsdom no tiene `createImageBitmap` ni canvas) | El adaptador de canvas real en los E2E (T-F706) |
  | — | El certificado y la cookie en los E2E: HTTPS real con la CA de mkcert (`ui.md` §21.3) |

- En jsdom la barra inferior y la lateral están ambas en el DOM (no hay CSS aplicado): los tests
  eligen con `within(getAllByRole('navigation', …)[0])` o equivalente.
- Los tests con MSW **no** necesitan el backend ni certificados. La **prueba independiente** de
  cada fase sí: columna "Backend necesario" de la tabla de dependencias, y la preparación de HTTPS
  local de `ui.md` §21.3 (`mkcert -install`, `make dev-certs`, `NODE_EXTRA_CA_CERTS`). Cualquier
  navegador del piso de NFR-F02 sirve para desarrollar.
- Versiones: la última estable de cada librería al arrancar la Fase F0, fijadas en
  `package-lock.json`; T-F001 reporta las versiones elegidas.

### Comandos de validación (los crea la Fase F0)

Se corren en `web/` salvo los de `make`.

| Comando | Qué corre |
|---|---|
| `npm run dev` | Vite en `https://localhost:5173` (`server.https` con `../.certs/`, `strictPort`) con *proxy* de `/api` a `https://localhost:8443` (sin `changeOrigin`, con verificación de certificado). Requiere `make dev-certs`, `crm serve` en HTTPS local con `APP_BASE_URL=https://localhost:5173` y `NODE_EXTRA_CA_CERTS` exportada (`ui.md` §21.3) |
| `npm run gen:api` | `openapi-typescript` con `redocly.yaml` → `src/api/generated/NNN.ts` |
| `npm run lint` | ESLint + `prettier --check` + verificación de que `gen:api` no produce diferencias |
| `npm run typecheck` | `tsc -b` |
| `npm test` | `vitest run --typecheck` (unitarios, pantallas con MSW y tests de tipos `*.test-d.ts`) |
| `npm run build` | `tsc -b && vite build` (imprime tamaños gzip de cada chunk) |
| `npm run check` | `lint` + `typecheck` + `test` + `build`. **Es el checkpoint de cada fase** (no necesita certificados) |
| `npm run e2e` | Playwright contra el binario en `https://localhost:8443` (`make build`, `make dev-certs`, `docker compose up`, `crm serve` en el modo "Binario completo" de plan §10.5.1; `NODE_EXTRA_CA_CERTS` exportada) |
| `make web-build` | `cd web && npm ci && npm run build` |
| `make web-check` | `cd web && npm ci && npm run check` |
| `make build` | `web-build` y después `go build` (binario con la SPA embebida) |
| `make check-all` | `make check` (backend) + `make web-check` |
| `make dev-certs` | (del backend, T-B014) genera `.certs/localhost.pem` y `.certs/localhost-key.pem` con mkcert |

### Dependencias con el backend

| Fase | Necesita del backend (prueba independiente) | Hallazgos |
|---|---|---|
| F0 | T-B005 (mux raíz con `RootDeps.SPA`, TLS local en `serve`) y T-B204 (middlewares comunes) para T-F007/T-F008; T-B014 (`make dev-certs`) para T-F006; T-B013 para T-F009 | H-1, H-8 y H-10 resueltos (DD-22, DD-29, DD-24) |
| F1 | T-B310 (`/me`) | — |
| F2 | T-B302, T-B306, T-B310 | H-3/H-10 resueltos (DD-24): HTTPS local en cualquier navegador |
| F3 | T-B405 | — |
| F4 | T-B507; `APP_LINK_*` con los defaults de DD-14 | — |
| F5 | T-B607 | H-4, H-5 resueltos (DD-25, DD-26) |
| F6 | T-B706 (límites de DD-31) | H-2 y H-11 resueltos (DD-23, DD-31); **H-12 abierto** (prueba en un celular real): no bloquea |
| F7 | Todas las fases del backend hasta la 7 + `compose.yaml` + T-B014 y la receta de CI de plan §10.5.1 | H-7, H-9, H-10 resueltos (DD-28, DD-30, DD-24); supuesto S-12 se valida en T-F701 |

---

### Fase F0 — Esqueleto del frontend (Setup)

**Objetivo**: existe `web/` con Vite + React + TypeScript, Tailwind + shadcn, router, TanStack
Query, tipos generados del contrato, cliente HTTP con errores normalizados, harness de tests con
MSW, lint; el desarrollo corre sobre HTTPS local; el binario Go embebe y sirve la SPA con las
cabeceras de `plan.md` §10.7; CI corre todo.

**Prueba independiente**: `make check-all` en verde en CI; `make build` + `crm serve` en el modo
"Binario completo" (plan §10.5.1) y abrir `https://localhost:8443/login` sin aviso de certificado
muestra la pantalla provisoria de Ingresar; `npm run dev` y abrir `https://localhost:5173/login`
muestra lo mismo con recarga en caliente; `curl -I` (sin `-k`) sobre `https://localhost:8443/`,
`/settings/users`, un asset con hash, un asset inexistente y `/api/v1/no-existe` devuelve las
respuestas de `ui.md` §21.1–21.2 (sin `Strict-Transport-Security`, porque es modo local).

**T-F001 — Proyecto `web/`, TypeScript y lint** · ADR-015, `ui.md` §9.1, §22
- Plantilla Vite React + TypeScript en `web/`; React 19; `strict` y `noUncheckedIndexedAccess`;
  alias `@/*`; `engines.node` y `.nvmrc` con Node LTS; `npm`.
- ESLint (typescript-eslint, `react-hooks`, `jsx-a11y`, `no-console`) + Prettier. Reglas
  `no-restricted-imports` de `ui.md` §9.1: `components/**` no importa `@/api`, `@/features`,
  `@/app`; `features/X` no importa `features/Y` salvo `features/auth/session`; `fetch` solo en
  `src/api/**` y `features/tenant/api.ts` (subida del logo); `createImageBitmap` y canvas solo en
  `features/tenant/logo/canvas.ts`.
- `index.html` base: `lang="es-AR"`, viewport sin bloquear zoom, `theme-color`, `manifest`,
  `apple-touch-icon`, `<noscript>` (`ui.md` §19.3).
- `.gitignore`: `web/node_modules`, `web/dist/*` salvo `web/dist/.gitkeep`, `web/.api-bundle`
  (`.certs/` ya lo ignora T-B014).
- Reportar las versiones elegidas de Vite, React, TypeScript, Tailwind, CLI de shadcn, React
  Router, TanStack Query, openapi-fetch, openapi-typescript, React Hook Form, Zod,
  `@hookform/resolvers` (≥ 5.1), Vitest, MSW, Playwright (RF-1).
- **Verificación**: un archivo de fixture en `components/` que importa `@/api/client` hace fallar
  `npm run lint` (se borra después de comprobarlo).

**T-F002 — Tailwind v4, shadcn/ui y tokens** · ADR-016, `ui.md` §19, NFR-F02, NFR-F07, NFR-F08
- `shadcn init` (base Radix, íconos lucide, alias). Variables de `ui.md` §19.1 (incluidas
  `--success*` y `--warning*`), fuente del sistema, `--radius`, regla global de
  `prefers-reduced-motion`, alto mínimo de 44 px en botones, inputs e ítems.
- Componentes: `button`, `input`, `label`, `field`, `radio-group`, `select`, `alert`,
  `alert-dialog`, `dropdown-menu`, `badge`, `card`, `skeleton`, `separator`, `sonner`.
- `build.target` explícito: `chrome112`, `edge112`, `firefox128`, `safari16.4`, `ios16.4`
  (NFR-F02: Chrome 112 por `createImageBitmap` con orientación EXIF).

**T-F003 [T] — Harness de tests con MSW** · ADR-022, supuesto S-F2
- **Red**:

  | Caso | Esperado |
  |---|---|
  | Un `Request` con URL absoluta (`${window.location.origin}/api/v1/me`) enviado con `fetch` en jsdom (lo que hace openapi-fetch) | MSW lo intercepta y devuelve el JSON del handler |
  | Un request sin handler | el test falla con un mensaje que nombra método y URL |
  | Dos tests seguidos | caché de Query, handlers de MSW (`resetHandlers`) y `sessionStorage` limpios entre tests |
  | `user-event` sobre un botón de shadcn | el click llega (smoke de la integración con Radix) |
- **Green**: `vitest.config` (jsdom, `setupFiles`), `src/test/setup.ts` (jest-dom, ciclo de MSW),
  `src/test/msw/server.ts` y `handlers.ts` (vacío), `src/test/fixtures.ts` con *builders* tipados
  (`buildSessionInfo`, `buildCurrentUser`, `buildTenant`, `buildUser`, `problem(code, status,
  extra)`) con datos ficticios.
- **Refactor**: ningún fixture con datos reales; *builders* con valores por defecto y
  `overrides`.
- Si S-F2 falla: probar `happy-dom` y reportar (no cambia código de la app).

**T-F004 [T] — Tipos generados del contrato y paso multi-spec** · ADR-018, ADR-014, DD-17, INV-F05, supuesto S-F1
- **Red** (tests de tipos `src/api/schema.test-d.ts` y un spike), contra el contrato **v0.3.1**:

  | Caso | Esperado |
  |---|---|
  | Tipo de la respuesta `200` de `paths['/me']['get']` | igual a `SessionInfo` |
  | `ErrorCode` | incluye `email_already_registered`, `email_taken`, `last_admin`, `token_invalid`, `payload_too_large`, `unsupported_media_type` |
  | `User['name']` | `string \| null` |
  | Parámetros de query de `paths['/tenant/logo']['get']` | incluyen `v?: string` |
  | Body de `paths['/tenant/logo']['put']` | `multipart/form-data` con la propiedad `file` (el test documenta el tipo generado; si no acepta un `File`, `putTenantLogo` usa `fetch` nativo, `ui.md` §11.3) |
  | `FieldErrorCode` | incluye `required` e `invalid_value` (los que usa el `422` del logo con `field: file`) |
  | Utilidad de tipos `AssertDisjoint<keyof Paths001, keyof PathsOtra>` con claves disjuntas | compila |
  | La misma utilidad con una ruta repetida (fixture de tipos) | error de compilación (`@ts-expect-error`) |
  | **Spike S-F1**: contrato de prueba `web/test-fixtures/openapi/999.yaml` con `$ref` externo a `Problem` de 001 generado con `gen:api` | el tipo generado tiene `code: ErrorCode`; si no, aplicar el respaldo `redocly bundle` (`ui.md` §11.4) y reportar |
  | Editar a mano `src/api/generated/001.ts` | `npm run lint` falla |
- **Green**: `web/redocly.yaml` (entrada `001` → `src/api/generated/001.ts`), script `gen:api`,
  `src/api/schema.ts` (`paths` combinados), `src/api/types.ts` (alias de `ui.md` §11.2), chequeo
  de actualización en `lint`. Archivos generados versionados.
- **Refactor**: el fixture del spike queda fuera de `redocly.yaml` de producción.

**T-F005 [T] — Cliente HTTP y normalización de errores** · ADR-018, ADR-009, `ui.md` §11.3
- **Red** (unitario sobre `toApiError`/`unwrap`/`isRetryable` y cliente con MSW):

  | Respuesta o situación | Esperado |
  |---|---|
  | `409` problem+json `email_already_registered` con `suggested_action` | `kind: 'problem'`, `status: 409`, `code`, `suggestedAction: 'password_reset'` |
  | `422` `ValidationProblem` con 2 errores | `fieldErrors` con los 2, en orden |
  | `429 login_locked` con `Retry-After: 900` | `retryAfterSeconds: 900` |
  | `429` sin `Retry-After` | `retryAfterSeconds: null` |
  | `500` con `instance` | `requestId` = `instance` |
  | `403` problem+json `forbidden` (como el rechazo de CSRF, DD-30) | `kind: 'problem'`, `code: 'forbidden'` |
  | `502` con cuerpo HTML | `kind: 'unexpected'`, `status: 502`, `code: null` |
  | problem+json con un `code` que no está en el enum | `kind: 'problem'`, `code: null`, `problem` conservado |
  | `fetch` rechaza (sin red) | `kind: 'network'`, `status: null` |
  | `unwrap` con `data` | devuelve `data` |
  | `isRetryable` | `true` para red y `503`; `false` para `500`, `429` y demás `4xx` |
  | `api.GET('/me')` con MSW | URL `${origin}/api/v1/me`, respuesta tipada |
  | `api.POST('/auth/login', …)` | `Content-Type: application/json`, body serializado |
- **Green**: `src/api/client.ts`, `errors.ts`, `queryKeys.ts` (`ui.md` §10.3).
- **Refactor**: `toApiError` es una función pura sin dependencias de React.

**T-F006 — Esqueleto de rutas, providers y desarrollo con HTTPS local** · ADR-017, ADR-019, DD-24, DD-F23, `ui.md` §6.1, §10.2, §21.3, H-10
- Depende de T-B005 (TLS local en `serve`) y T-B014 (`make dev-certs`).
- `src/app/router.tsx` con el árbol completo de `ui.md` §6.1 y **pantallas provisorias** (solo su
  `<h1>`), guards provisorios que dejan pasar, `lazy` por grupo de rutas; `RootLayout` con
  `<Toaster/>` y `<ScrollRestoration/>`; `createAppQueryClient` con los defaults de `ui.md` §10.2
  (sin los manejadores globales, que llegan en F1); `main.tsx`; `src/test/renderApp.tsx`
  (`createMemoryRouter` con el mismo árbol).
- Test de humo: cada ruta del árbol muestra su `<h1>` provisorio; una ruta inexistente muestra
  "No encontramos esta página."
- `vite.config.ts` según la tabla de `ui.md` §21.3: `server.https` con `../.certs/localhost.pem` y
  `../.certs/localhost-key.pem`; si faltan, `npm run dev` termina con "Faltan los certificados de
  desarrollo: corré `make dev-certs` (ver README)."; `server.port: 5173` con `strictPort`;
  *proxy* de `/api` a `https://localhost:8443` **sin** `changeOrigin` (el `Host` sigue siendo
  `localhost:5173`, como espera T-B203) y **sin** `secure: false`.
- **Verificación manual**: sin `NODE_EXTRA_CA_CERTS`, `npm run dev` arranca pero el *proxy* falla
  con un error de certificado (se ve en la consola de Vite); con la variable exportada,
  `fetch('/api/v1/healthz')`… no existe bajo `/api` → `404` problem+json desde la consola del
  navegador en `https://localhost:5173` (demuestra que el *proxy* llega al backend por HTTPS).
- **Spike de la cookie** (manual, cuando el backend tenga T-B306; si no, en el checkpoint de F2):
  con `crm serve` en HTTPS local y `APP_BASE_URL=https://localhost:5173`, en
  `https://localhost:5173` registrarse con `fetch('/api/v1/auth/signup', …)` desde la consola y
  verificar que `fetch('/api/v1/me')` responde `200`, en **Chrome**, **Firefox** y, si hay una Mac,
  **Safari** (esperado: sí en los tres, S-12 del plan). En DevTools, la cookie
  `__Host-crm_session` aparece con `Secure`, `HttpOnly` y `SameSite=Lax`. Reportar el resultado de
  cada navegador; si alguno falla, revisar la tabla de diagnóstico de `ui.md` §21.3 antes de
  volver al arquitecto.

**T-F007 [T] — Handler Go de la SPA** · ADR-019, plan DD-22, DD-24, DD-29, §10.7, INV-22, DD-14, NFR-F10, `ui.md` §21
- Implementa: `backend-developer` (código Go, `testing` nativo, table-driven, ADR-012). Depende de
  T-B005 (mux raíz con `RootDeps.SPA`) y T-B204 (middlewares comunes). Firmas (plan §11.1):
  `func DistFS() fs.FS` y `func NewHandler(dist fs.FS) http.Handler` en el paquete `web` (raíz del
  repo). El handler recibe un `fs.FS` para probarlo con `fstest.MapFS` sin compilar el frontend.
- **Red** (unitario con `fstest.MapFS` + integración del mux raíz con `httptest`, usando
  `app.NewRootHandler` con `RootDeps.SPA = web.NewHandler(fs)`):

  | Request | Esperado |
  |---|---|
  | `GET /` | `200` `text/html`, `index.html`, `Cache-Control: no-cache`, CSP exacta de §10.7 |
  | `GET /settings/users`, `GET /reset-password` | `200` con `index.html` |
  | `GET /assets/index-abc123.js` (existe) | `200`, tipo JavaScript, `Cache-Control: public, max-age=31536000, immutable` |
  | `GET /assets/viejo-def456.js` (no existe) | `404` `text/plain`, `Cache-Control: no-store`, el cuerpo no contiene `<html` |
  | `GET /sw.js` / `GET /manifest.webmanifest` / `GET /offline.html` | `no-cache`; tipos `text/javascript` / `application/manifest+json` / `text/html` con CSP |
  | `GET /icons/icon-192.png` | `public, max-age=86400`; **sin** `Content-Encoding` aunque se pida gzip |
  | `HEAD /` | `200` sin cuerpo |
  | `POST /login` | `405` |
  | `GET /api/v1/no-existe` (mux completo) | `404` problem+json `code: not_found` (nunca `index.html`, INV-22) |
  | `GET /api/v1/me` con `Accept-Encoding: gzip` (mux completo) | **sin** `Content-Encoding: gzip` (la API no se comprime, DD-29) |
  | `GET /healthz` (mux completo) | `200` `{"status":"ok"}` del backend |
  | `dist` sin `index.html` (solo `.gitkeep`) | `503` `text/plain` "La interfaz no está compilada (correr `make web-build`)" |
  | Toda respuesta de la SPA con configuración **no local** (`APP_BASE_URL=https://crm.example`) | `X-Request-Id`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `Strict-Transport-Security` (middlewares comunes) |
  | Toda respuesta de la SPA con configuración **local** (`APP_BASE_URL=https://localhost:8443`) | las mismas cabeceras **sin** `Strict-Transport-Security` (DD-24) |
  | `Accept-Encoding: gzip` en un asset JS, en `index.html` y en el manifest (de más de 1 KB) | `Content-Encoding: gzip` y `Vary: Accept-Encoding` (supuesto 6 del `research.md` del backend) |
  | `chi.Walk` sobre el router de la API | no incluye rutas de la SPA (T-B801 y el test de rutas contra el contrato siguen igual) |
- **Green**: pasa la tabla.
- **Refactor**: las reglas de caché en una tabla (extensión/prefijo → cabecera), no en `if`
  dispersos.

**T-F008 — Implementar el paquete `web` y conectarlo al mux raíz** · ADR-019, plan DD-22, DD-29
- Implementa: `backend-developer`. `web/embed.go` (`//go:embed all:dist`, `DistFS`), el handler
  (`NewHandler`, envuelto con `gzhttp` con el umbral por defecto), `web/dist/.gitkeep`; en
  `cmd/crm`/`internal/app`, reemplazar el stub de `RootDeps.SPA` (T-B005) por
  `web.NewHandler(web.DistFS())`. `depguard`: `web` solo importa la librería estándar y `gzhttp`
  (T-B011).

**T-F009 — Makefile, CI y README del frontend** · ADR-019, ADR-022, DD-24
- Targets `web-build`, `web-check`, `build`, `check-all` (tabla de comandos). CI: job de frontend
  con Node LTS y caché de npm que corre `make web-check` (sin certificados); el job de backend no
  necesita el frontend (usa el marcador de `dist`); un job de build que corre `make build` y guarda
  el binario como artefacto para los E2E (el job de E2E con mkcert lo agrega T-F701). Depende de
  T-B013.
- `README`, sección de desarrollo (complementa lo que escribe T-B014: mkcert, `make dev-certs`,
  los tres modos): cómo correr `npm run dev` en `https://localhost:5173` con `crm serve` en
  `:8443` y `APP_BASE_URL=https://localhost:5173`; exportar `NODE_EXTRA_CA_CERTS` antes de
  `npm run dev` y `npm run e2e`; cómo correr el E2E local contra `https://localhost:8443`; la
  tabla de diagnóstico de `ui.md` §21.3. Sin recomendar un navegador en particular.

**Checkpoint Fase F0**: `make check-all` en verde local y en CI; la prueba independiente ejecutada
(navegador y `curl` sobre HTTPS); resultados de los spikes S-F1 y S-F2 reportados (y el de la
cookie de T-F006, si el backend ya tiene T-B306).

---

### Fase F1 — Fundacional: errores, sesión, guards y shell

**Objetivo**: toda pantalla con sesión está protegida; un `401` en cualquier lado vuelve al login
con aviso; los permisos ocultan lo que no corresponde; el shell es navegable por teclado y la PWA
es instalable.

**Prueba independiente**: con el backend (T-B310) en HTTPS local, en cualquier navegador con la CA
de mkcert instalada: abrir `https://localhost:5173/settings/users` sin sesión redirige a
`/login?next=%2Fsettings%2Fusers`; con una sesión creada por `curl` (cookie pegada en el
navegador) se ve el shell; borrar la sesión en la base y navegar vuelve al login con "Tu sesión se
cerró"; Chrome ofrece "Instalar app" sobre el binario en `https://localhost:8443`.

**T-F101 [T] — Mensajes por `code` y por campo** · ADR-009, INV-F05, `ui.md` §12.2–12.3
- **Red**:

  | Caso | Esperado |
  |---|---|
  | Test de tipos: el mapa de mensajes es `Record<ErrorCode, …>` | un `ErrorCode` agregado en un fixture de tipos sin mensaje no compila |
  | `messageForError` para cada `code` de §12.2 en contexto `generic` | título y descripción de la tabla |
  | `token_invalid` en `resetConfirm`, `verifyEmail`, `invitationPreview` | los tres textos de §12.2 |
  | `unsupported_media_type`, `payload_too_large` y `malformed_request` en `logo` | textos del logo ("No pudimos subir el logo." con `requestId` para el `400`) |
  | `rate_limited` con `retryAfterSeconds` 61 / 1800 / `null` | "2 minutos" / "30 minutos" / "unos minutos" |
  | `internal` con `requestId` | la descripción incluye el código |
  | `kind: 'network'` | "No hay conexión." y `retryable: true` |
  | `kind: 'unexpected'` con `502` / `418` | como `service_unavailable` / como `internal` |
  | `fieldErrorMessage` para cada `FieldError.code` y cada variante por campo de §12.3, incluidas `file` + `required` y `file` + `invalid_value` | textos de la tabla |
  | `logoRejectMessage` para cada `LogoRejectReason` | textos de §13.11.1 |
  | `applyServerFieldErrors` con un campo conocido y uno desconocido | `setError` en el conocido; devuelve el desconocido |
- **Green**: pasa la tabla.

**T-F102 — Implementar `api/errorMessages.ts`** (`messageForError`, `fieldErrorMessage`,
`logoRejectMessage`, `applyServerFieldErrors`).

**T-F103 [T] — `safeNextPath`** · INV-F07, BR-F07
- **Red**:

  | Entrada | Salida |
  |---|---|
  | `null`, `''` | `/` |
  | `/settings/users`, `/settings/users?x=1` | igual |
  | `//evil.example`, `/\evil.example`, `https://evil.example`, `javascript:alert(1)` | `/` |
  | `settings` (sin barra inicial) | `/` |
  | `/%2F%2Fevil.example` | `/` |
- **Green**: pasa la tabla.

**T-F104 [T] — Sesión, guards y manejadores globales** · ADR-017, ADR-018, INV-F01, INV-F02, INV-F03, INV-F06, BR-F04, `ui.md` §6.4, §12.4
- **Red** (pantallas con MSW):

  | Caso | Esperado |
  |---|---|
  | Ruta con sesión y `/me` pendiente | pantalla de arranque con `aria-busy`; no se ve el formulario de login |
  | `/me` → `401` al entrar a `/settings/users` | `/login?next=%2Fsettings%2Fusers`, **sin** aviso de sesión vencida |
  | `/me` → `200` en `/login` | redirige a `/`; con `?next=/settings/users`, a `/settings/users`; con `?next=//evil.example`, a `/` |
  | `/me` → `503` en una ruta con sesión | estado de error con "Reintentar"; al reintentar con `200` se ve la pantalla |
  | Operador en `/settings/users` | "No tenés acceso a esta sección" + "Volver al inicio"; **no** se pidió `GET /users` |
  | Admin en `/settings/users` y `GET /users` → `401 unauthenticated` | caché vacía; `/login?next=%2Fsettings%2Fusers&reason=session_expired` con "Tu sesión se cerró. Ingresá de nuevo para seguir." |
  | Dos queries responden `401` a la vez | una sola navegación |
  | Una mutación responde `401 unauthenticated` | mismo comportamiento |
  | `POST /auth/login` → `401 invalid_credentials` | no navega ni limpia la caché (INV-F02) |
  | Una query responde `403 forbidden` | se vuelve a pedir `/me`; con el rol nuevo sin `settings.manage`, el guard muestra "Sin permiso" |
  | Una mutación responde `403 forbidden` (p. ej. rechazo de CSRF, DD-30) | mensaje "No tenés permiso para hacer esto."; se vuelve a pedir `/me` |
  | `['session']` pasa de un usuario a `null` en un refetch al enfocar la ventana | igual que un `401` global (`reason=session_expired`) |
  | Tras el `401`, botón atrás del navegador | se vuelve a pedir `/me`; no se ven datos anteriores |
- **Green**: pasa la tabla.
- **Refactor**: un solo lugar decide "¿es `401 unauthenticated`?"; los guards no conocen la red.

**T-F105 — Implementar sesión y guards**: `features/auth/session.ts` (`useSession`,
`useCurrentSession`, `useCan`), `RequireSession`, `PublicOnly`, `RequirePermission`,
`ForbiddenState`, manejadores de `createAppQueryClient` inyectados desde `main.tsx`,
`lib/safeNextPath.ts`.

**T-F106 [T] — Shell, Ajustes y accesibilidad base** · BR-F04, INV-F10, NFR-F06, `ui.md` §13.8, §13.13, §18
- **Red**:

  | Caso | Esperado |
  |---|---|
  | Admin en `/` | navegación con "Inicio" y "Ajustes" (ícono y texto); en Ajustes, su nombre, email y "Administrador", y los enlaces "Datos de la empresa" y "Usuarios" |
  | Operador en `/settings` | sin la sección "Tu empresa" |
  | Primer Tab al cargar | enfoca "Saltar al contenido"; Enter mueve el foco a `<main>` |
  | Navegar de Inicio a Ajustes | foco en el `<h1>` "Ajustes"; `document.title` = "Ajustes · {Empresa}" |
  | Ítem de navegación actual | `aria-current="page"` |
  | Encabezado | nombre de la empresa de `['tenant']`; mientras carga, `session.tenant.name`; sin logo, iniciales |
  | Evento `offline` / `online` | aparece / desaparece "Sin conexión. Revisá tu internet." (`role="status"`) |
  | Una pantalla que lanza un error al renderizar | "Algo salió mal al mostrar esta pantalla" con "Recargar" e "Ir al inicio" |
  | `vite:preloadError` la primera vez / la segunda | llama a `reload` (espía) y marca `sessionStorage` / muestra "Hay una versión nueva de la app" + "Actualizar" |
  | Ruta inexistente | "No encontramos esta página." + "Ir al inicio" |
- **Green**: pasa la tabla.

**T-F107 — Implementar el shell**: `RootLayout` (foco al `<h1>` al cambiar de ruta),
`AppShell`, navegación inferior y lateral, `SettingsPage` (sin "Cerrar sesión", que llega en F3),
`NotFoundPage`, `ErrorBoundary` raíz, manejo de `vite:preloadError`, `PageHeader`, `FormAlert`,
`ErrorState`, `EmptyState`, `AppBrand`, `OfflineBanner`, `useOnlineStatus`, `useDocumentTitle`,
`useTenant` (lectura).

**T-F108 — PWA base** · ADR-020, `ui.md` §20
- `public/manifest.webmanifest` (`name`/`short_name` "CRM"), íconos genéricos provisorios (P-F1),
  `public/sw.js` y `public/offline.html` según ADR-020; registro en `main.tsx` solo en producción.
  Verificación manual en el binario en `https://localhost:8443` (Chrome DevTools → Application:
  manifest sin errores, SW activo, Cache Storage con solo `crm-offline-v1`); la verificación
  automática es T-F703.

**Checkpoint Fase F1**: `npm run check` en verde + prueba independiente contra el backend.

---

### Fase F2 — Historia 1: Registrar una empresa (P1) — primer slice vertical

**Objetivo**: un visitante se registra desde el celular en una sola pantalla y llega al panel con
su empresa, rubro y moneda base.

**Prueba independiente**: con el backend (T-B306, T-B310) en HTTPS local y Mailpit, en cualquier
navegador con la CA de mkcert instalada: en `https://localhost:5173/signup` (o el binario en
`https://localhost:8443/signup`), completar el formulario → panel con "Hola, {nombre}" y el rubro
elegido; el email de verificación está en Mailpit con un enlace a `https://localhost:5173/…`;
repetir con el mismo email → "Ya existe un usuario con ese email." y "Recuperar contraseña". Spike
de la cookie (T-F006) reportado para Chrome, Firefox y, si hay, Safari.

**T-F201 [T] — Esquema del registro** · ADR-021, DD-6, `ui.md` §13.1, §16
- **Red** (unitario sobre el esquema Zod):

  | Entrada | Esperado |
  |---|---|
  | Todo válido | ok; email en minúsculas y sin espacios; textos con `trim` |
  | `name`/`company_name` vacíos o de 121 caracteres | `required` / `too_long` |
  | Email `ana@` | `invalid_format` |
  | Contraseña de 9 / 10 / 128 / 129 caracteres | `too_short` / ok / ok / `too_long` |
  | Contraseña igual al email (sin distinguir mayúsculas) | `same_as_email` |
  | Sin `industry_template_code` | `required` |
  | Test de tipos: el esquema produce un `SignupRequest` | compila; un campo renombrado en el contrato no compila |
- **Green**: pasa la tabla.

**T-F202 [T] — Pantalla de registro** · US-1.1, US-1.2, FR-001, FR-002, SC-001, P-4, DD-19, DD-21, DD-27, DD-F10, DD-F18, BR-F10, `ui.md` §13.1, §14.2
- **Red**:

  | Interacción | Resultado observable |
  |---|---|
  | Rubros pendientes | *skeletons* en "Rubro de tu empresa"; "Crear cuenta" deshabilitado |
  | Rubros → `503`, "Reintentar" → `200` | aparecen "Carpintería de aluminio" y "Genérico" |
  | Enviar vacío | un error debajo de cada campo requerido (textos de §12.3); foco en "Tu nombre"; ningún request |
  | Datos válidos | `POST /auth/signup` con el body del contrato (email normalizado, `base_currency: 'ARS'` por defecto, `timezone` = la del navegador tal cual, incluso si es un valor raro como `Etc/Unknown`); luego el panel, toast "¡Listo! Tu empresa quedó creada."; atrás no vuelve al formulario |
  | `409 email_already_registered` | alerta "Ya existe un usuario con ese email." con "Recuperar contraseña" (→ `/forgot-password` con el email prellenado) y "Usar otro email" (enfoca el email) |
  | `422` con `errors: [{field: 'company_name', code: 'too_long'}]` | error debajo de "Nombre de la empresa" y foco ahí |
  | `429 rate_limited` con `Retry-After: 1800` | "Hiciste muchos intentos seguidos. Esperá 30 minutos y probá de nuevo." |
  | `503` / sin red | mensaje con "Reintentar"; los valores siguen cargados |
  | Doble click en "Crear cuenta" | un solo `POST`; botón "Creando cuenta…" deshabilitado |
  | Completar todo con teclado (Tab, flechas en los radios, Enter) | se envía |
  | Visitante con sesión vigente | redirige a `/` |
  | Campos | todos encontrables por `getByLabelText`; `autocomplete` `name`, `email`, `new-password`, `organization`; ayuda de moneda base visible |
- **Green**: pasa la tabla.

**T-F203 — Implementar el registro**: `SignupPage`, `IndustryTemplatePicker`, `CurrencyPicker`,
`PasswordInput`, `SubmitButton`, `useSignup`, `useIndustryTemplates`, esquema de T-F201.

**T-F204 [T] — Panel inicial** · US-1 (prueba independiente), BR-F04, `ui.md` §13.7
- **Red**:

  | Caso | Esperado |
  |---|---|
  | Admin | "Hola, {nombre}"; tarjeta con la empresa, "Rubro: Carpintería de aluminio", "Moneda base: Pesos (ARS)"; "Primeros pasos" con "Completá los datos de tu empresa" y "Invitá a tu equipo" |
  | Operador | sin "Primeros pasos" |
  | `/tenant` pendiente | saludo visible; tarjeta en *skeleton* |
  | `/tenant` → `503` | "No pudimos cargar los datos de tu empresa." + "Reintentar" |
  | Código de rubro que no está en el catálogo | se muestra el código |
- **Green**: pasa la tabla.

**T-F205 — Implementar `DashboardPage`**.

**Checkpoint Fase F2**: `npm run check` en verde + prueba independiente (registro medido: < 3
min en una prueba manual, SC-001).

---

### Fase F3 — Historia 2: Ingresar, bloqueo y cerrar sesión (P1)

**Objetivo**: ingresar con mensajes que no revelan si un email existe, bloqueo explicado con la
hora, y cierre de sesión que deja el dispositivo limpio.

**Prueba independiente**: con el backend (T-B405): 5 contraseñas incorrectas → el 6.º intento
muestra "Por seguridad, pausamos el ingreso…" con la hora; "Cerrar sesión" desde Ajustes vuelve al
login con "Cerraste sesión." y atrás no muestra datos.

**T-F301 [T] — Pantalla Ingresar** · US-2.1, US-2.2, FR-003, INV-13, INV-F02, DD-F17, `ui.md` §13.2, §14.1
- **Red**:

  | Interacción | Resultado observable |
  |---|---|
  | Credenciales correctas sin `next` / con `next=/settings/users` / con `next=//evil.example` | `/` / `/settings/users` / `/`; atrás no vuelve al login |
  | `401 invalid_credentials` | "El email o la contraseña no son correctos."; email conservado; contraseña vacía y enfocada; sin navegación ni limpieza de caché |
  | `403 account_disabled` | "Tu usuario está desactivado." + "Pedile a un administrador de tu empresa que lo reactive." |
  | `429 login_locked` con `Retry-After: 900` y reloj en 14:20 | "…Podés volver a intentar a las 14:35 (en 15 minutos)…"; "Restablecer contraseña" lleva a `/forgot-password` con el email; "Volver a intentar" muestra el formulario |
  | `429 rate_limited` | minutos de espera |
  | `reason=session_expired` / `password_changed` / `logged_out` / otro valor | el aviso correspondiente / ninguno |
  | "¿Olvidaste tu contraseña?" con un email escrito | `/forgot-password` con ese email |
  | `503` / sin red | mensaje con "Reintentar"; datos conservados |
  | Campos | `autocomplete` `email` y `current-password`; se puede pegar en ambos |
  | Envío con Enter desde el campo contraseña | envía |
- **Green**: pasa la tabla.

**T-F302 — Implementar `LoginPage`, `LockedNotice`, `useLogin`, `formatRetryAt`**.

**T-F303 [T] — Cerrar sesión** · US-2, INV-F03, `ui.md` §13.8, §17
- **Red**:

  | Interacción | Resultado observable |
  |---|---|
  | "Cerrar sesión" en Ajustes → `204` | `POST /auth/logout`; `/login` con "Cerraste sesión."; al ir a `/` se vuelve a pedir `/me` (la caché quedó vacía) |
  | "Cerrar sesión" en el menú del usuario (navegación lateral) | mismo resultado |
  | Sin red | toast "No pudimos cerrar la sesión. Revisá tu conexión."; sigue en Ajustes con la sesión |
  | Mientras envía | "Cerrando sesión…" deshabilitado |
- **Green**: pasa la tabla.

**T-F304 — Implementar `useLogout` y los dos puntos de salida**.

**Checkpoint Fase F3**: `npm run check` en verde + prueba independiente.

---

### Fase F4 — Historia 2.3 y 1.3: Recuperar contraseña y confirmar email (P1)

**Objetivo**: los enlaces de email funcionan de punta a punta sin exponer el token en la URL, y el
aviso de verificación permite reenviar el email.

**Prueba independiente**: con el backend (T-B507) y los `APP_LINK_*` por defecto (DD-14): pedir el
restablecimiento, abrir el enlace de Mailpit, definir la contraseña nueva e ingresar; reabrir el
mismo enlace → "Este enlace ya no sirve."; confirmar el email desde su enlace y ver desaparecer el
aviso.

**T-F401 [T] — Token de los enlaces (`useLinkToken`)** · DD-14, DD-F2, INV-F08, `ui.md` §6.2
- **Red**:

  | Caso | Esperado |
  |---|---|
  | Montar `/reset-password#token=abc-_123` | devuelve `abc-_123`; `location.hash` vacío; `location.state.linkToken` = `abc-_123` |
  | Volver a montar con ese `state` (recarga en la misma pestaña) | devuelve `abc-_123` |
  | Sin fragmento ni `state` / `#token=` vacío / `#foo=bar` | `null` |
  | En ningún momento | el token no aparece en `location.search`; `console` no fue llamado |
- **Green**: pasa la tabla.

**T-F402 [T] — Olvidé mi contraseña** · US-2.3, FR-004, INV-13, DD-20, `ui.md` §13.3
- **Red**:

  | Interacción | Resultado observable |
  |---|---|
  | Llegar con `state.email` | campo prellenado |
  | Enviar → `202` | "Revisá tu email" (foco ahí) con el texto neutro de §13.3; siempre igual |
  | Email inválido | error en el campo; sin request |
  | `429` | minutos de espera |
  | "¿No llegó? Pedilo de nuevo" | formulario con el email cargado |
  | Sin red | mensaje con "Reintentar" |
  | Con sesión vigente | redirige a `/` |
- **Green**: pasa la tabla.

**T-F403 [T] — Restablecer contraseña** · US-2.3, BR-F06, DD-F16, `ui.md` §13.4
- **Red**:

  | Interacción | Resultado observable |
  |---|---|
  | Sin token | "Este enlace está incompleto." + "Pedir un enlace nuevo" |
  | Contraseña de 9 caracteres | error; sin request |
  | Válida → `204` | body `{ token, password }` con el token del enlace; caché vacía; `/login?reason=password_changed` con "Listo, cambiaste tu contraseña. Ingresá con la nueva." |
  | `400 token_invalid` | "Este enlace ya no sirve." + "Vence a la hora o ya se usó. Pedí uno nuevo." + "Pedir un enlace nuevo" → `/forgot-password` |
  | `422 same_as_email` | error debajo del campo |
  | Campo | `autocomplete="new-password"`; "Mostrar"/"Ocultar" con `aria-pressed` |
- **Green**: pasa la tabla.

**T-F404 [T] — Confirmar email** · US-1.3, DD-13, DD-F3, `ui.md` §13.5
- **Red**:

  | Interacción | Resultado observable |
  |---|---|
  | Montar la pantalla con token | ningún `POST` hasta tocar "Confirmar mi email" |
  | Confirmar sin sesión → `204` | "¡Listo! Tu email quedó confirmado." + "Ingresar" |
  | Confirmar con sesión → `204` | "Ir al inicio"; `/me` pedido de nuevo (`email_verified: true`) |
  | `400 token_invalid` con sesión | mensaje de §12.2 + "Reenviar email de confirmación" → `POST …/resend` → "Listo, te lo reenviamos…" |
  | `400 token_invalid` sin sesión | mensaje + "Ingresar" |
  | Sin token | "Este enlace está incompleto." |
- **Green**: pasa la tabla.

**T-F405 [T] — Aviso "Confirmá tu email"** · US-1.3, P-2, BR-F05, `ui.md` §13.13
- **Red**:

  | Caso | Esperado |
  |---|---|
  | `email_verified: false` | aviso con el email en todas las pantallas con sesión |
  | `email_verified: true` | sin aviso |
  | "Reenviar" → `202` | "Listo, te lo reenviamos. Revisá también el correo no deseado." anunciado en `aria-live` |
  | "Reenviar" → `429` con `Retry-After: 300` | "Ya te lo mandamos hace poco. Esperá 5 minutos." |
  | "Ocultar" | desaparece y sigue oculto al navegar; con `sessionStorage` vacío vuelve a aparecer |
  | Teclado | "Reenviar" y "Ocultar" alcanzables con Tab y con nombre accesible |
- **Green**: pasa la tabla.

**T-F406 — Implementar** `useLinkToken`, `ForgotPasswordPage`, `ResetPasswordPage`,
`VerifyEmailPage`, `EmailVerificationBanner` y sus hooks.

**Checkpoint Fase F4**: `npm run check` en verde + prueba independiente.

---

### Fase F5 — Historia 3: Usuarios e invitaciones (P2)

**Objetivo**: el Administrador invita, reenvía, cambia roles (también de invitados), desactiva y
reactiva desde el celular sin poder dejar la empresa sin administrador; el invitado acepta en una
pantalla.

**Prueba independiente**: con el backend (T-B607): invitar a un operador, reinvitarlo como
Administrador (el toast lo dice), abrir su enlace en otra ventana privada, aceptar y ver el panel
con "Primeros pasos"; como un operador, `/settings/users` muestra "Sin permiso"; como admin,
desactivar a un usuario y ver que su ventana vuelve al login con "Tu sesión se cerró"; reactivarlo.

**T-F501 [T] — Reglas de acciones y fechas** · BR-F01, BR-F02, BR-F03, INV-F12, DD-26, ADR-023
- **Red** (unitario):

  | Entrada | Esperado de `availableUserActions` |
  |---|---|
  | `active`+`admin`, otro, 2 admins activos | `makeOperator` y `deactivate` habilitadas, con confirmación |
  | `active`+`admin`, 1 admin activo (él) | ambas deshabilitadas con `disabledReason: 'last_admin'` |
  | `active`+`admin`, es el usuario actual, 2 admins | `makeOperator` con `selfWarning: 'lose_settings_access'`; `deactivate` con `selfWarning: 'end_own_session'` |
  | `active`+`operator` | `makeAdmin`, `deactivate` |
  | `invited`+`operator` | `resendInvitation` (sin confirmación), `makeAdmin`, `deactivate` |
  | `invited`+`admin`, siendo el usuario actual el único admin activo | `makeOperator` **habilitada** (los invitados no cuentan para `last_admin`, DD-26), `resendInvitation`, `deactivate` |
  | `disabled` | solo `reactivate` (sin cambio de rol: el backend responde `invalid_state`) |

  | Entrada | Esperado |
  |---|---|
  | `countActiveAdmins` con admins `active`, `invited` y `disabled` | cuenta solo `active` + `admin` |
  | `formatDateTime('2026-10-06T17:30:00Z', 'America/Argentina/Buenos_Aires', now=2026-09-29)` | "6/10, 14:30" |
  | Misma fecha con `now` en otro año | incluye el año |
- **Green**: pasa la tabla.

**T-F502 [T] — Lista de usuarios** · US-3, FR-005, FR-007, DD-25, `ui.md` §13.9
- **Red**:

  | Caso | Resultado observable |
  |---|---|
  | `GET /users` pendiente | 3 *skeletons* con `aria-busy` |
  | Admin (vos), invitado vigente, desactivado | una tarjeta por usuario con nombre (o email si `name` es `null`), email, rol y estado **en texto**; "(vos)" en la propia; "La invitación vence el 6/10, 14:30" en la zona de la empresa |
  | Invitado con `invitation_expires_at` en el pasado | "La invitación venció el {fecha}. Reenviala para que pueda entrar." |
  | Invitado con `invitation_expires_at: null` (fuera de contrato) | sin texto de vencimiento; la tarjeta se ve igual |
  | Solo el usuario actual | "Todavía sos el único usuario." + "Invitar" |
  | `500` con `instance` | "No pudimos cargar los usuarios." + código + "Reintentar" |
  | Nombre de 120 caracteres | el texto completo está en el DOM (sin truncar) |
- **Green**: pasa la tabla.

**T-F503 [T] — Acciones sobre usuarios** · US-3.3, US-3.4, FR-005, P-3, DD-5, DD-26, DD-F7, DD-F8, INV-F04, INV-F10, `ui.md` §13.9
- **Red**:

  | Interacción | Resultado observable |
  |---|---|
  | Abrir el menú de un invitado con teclado (Enter, flechas) | "Reenviar invitación", "Hacer Administrador", "Desactivar"; nombre accesible "Acciones para {email}" |
  | Único admin activo (vos) | "Cambiar a Operador" y "Desactivar" deshabilitados con "Tiene que quedar al menos un administrador activo" |
  | "Hacer Administrador" a un activo → confirmar → `200` | `PUT /users/{id}/role` `{ role: 'admin' }`; toast "{nombre} ahora es Administrador"; la tarjeta muestra el rol nuevo; foco de vuelta en el botón de acciones de esa fila |
  | "Hacer Administrador" a un invitado → confirmar → `200` con `status: invited` | el diálogo decía "Cuando acepte la invitación, va a poder…"; toast "{email} ahora es Administrador"; sigue "Invitado" con el mismo vencimiento |
  | Menú de un desactivado | solo "Reactivar" (no hay cambio de rol) |
  | Cancelar o Esc en el diálogo | sin request; foco de vuelta en el botón |
  | `409 last_admin` | toast con el mensaje de §12.2; se vuelve a pedir la lista |
  | "Desactivar" a otro → `200` | toast "Desactivaste a {nombre}"; estado "Desactivado" |
  | Desactivarse a sí mismo (hay otro admin) → `200` | el diálogo advertía "Se va a cerrar tu sesión."; el request siguiente da `401` y se ve el login con aviso |
  | Bajarse a Operador → `200` | el diálogo advertía "Vas a perder el acceso a Ajustes."; `/me` pedido de nuevo; "Sin permiso" |
  | "Reactivar" → `200` `active` / `200` `invited` | "Reactivaste a {nombre}" / "Le enviamos una invitación nueva a {email}" |
  | "Reenviar invitación" → `200` | `POST /users/invitations` con su email y su rol actual; "Reenviamos la invitación a {email}" |
  | `404 not_found` / `409 invalid_state` | mensaje de §12.2 y lista pedida de nuevo |
  | `403 forbidden` | `/me` pedido de nuevo; "Sin permiso" |
  | Doble confirmación rápida | un solo request |
- **Green**: pasa la tabla.

**T-F504 [T] — Invitar** · US-3.1, DD-5, DD-21, DD-26, BR-F08, BR-F11, DD-F6, `ui.md` §13.10, §14.3
- **Red**:

  | Interacción | Resultado observable |
  |---|---|
  | Abrir la pantalla | "Operador" seleccionado; descripción de cada rol visible |
  | Email inválido | error; sin request |
  | Enviar → `201` | vuelve a Usuarios; toast "Invitación enviada a {email}. Vence el {fecha}."; la lista incluye al invitado |
  | Enviar → `200` con el mismo rol que tenía en la lista | toast "{email} ya estaba invitado: le reenviamos la invitación." |
  | Enviar como Administrador → `200` a un invitado que figuraba como Operador | toast "{email} ya estaba invitado: le reenviamos la invitación como Administrador."; la lista muestra el rol nuevo |
  | `200` sin `['users']` en caché | texto neutro "{email} ya estaba invitado: le reenviamos la invitación." |
  | `409 email_taken` | "Ese email ya tiene un usuario en el sistema." en el campo; si es de un desactivado de la lista, "Es de {nombre}, que está desactivado. Podés reactivarlo desde la lista." |
  | "Cancelar" | vuelve a Usuarios |
- **Green**: pasa la tabla.

**T-F505 [T] — Aceptar invitación** · US-3.2, DD-1, DD-4, DD-20, BR-F12, INV-F03, `ui.md` §13.6, §14.3
- **Red**:

  | Interacción | Resultado observable |
  |---|---|
  | Sin token | "Este enlace está incompleto." |
  | Vista previa pendiente | *skeleton*; el `POST …/preview` lleva el token en el body y el token no se ve en la URL |
  | Vista previa `200` | "Te invitaron a {empresa} como Operador", vencimiento, email de solo lectura |
  | Vista previa `400 token_invalid` | "Esta invitación ya no es válida…" + "pedí una nueva con tu email" (→ `/forgot-password`) + "Ingresá" |
  | Contraseña igual al email de la vista previa | error; sin request |
  | Aceptar → `201` | caché vacía y sesión nueva; `/` con "¡Bienvenido/a a {empresa}!" |
  | Aceptar → `400 token_invalid` | estado "Esta invitación ya no es válida" |
  | Aceptar → `422` | errores por campo |
  | Con otra sesión abierta | "Tenés una sesión abierta como {email}. Si aceptás, se va a cerrar." |
- **Green**: pasa la tabla.

**T-F506 — Implementar** `UsersPage`, `UserListItem`, `UserStatusBadge`, `ConfirmDialog`,
`InviteUserPage`, `AcceptInvitationPage`, `features/users/rules.ts`, `lib/format.ts`
(`formatDateTime`) y los hooks de `features/users/api.ts` y de invitación.

**Checkpoint Fase F5**: `npm run check` en verde + prueba independiente.

---

### Fase F6 — Historia 4: Datos de la empresa y logo (P2)

**Objetivo**: el Administrador edita los datos fiscales y de contacto y sube el logo, incluso
desde una foto del celular (que se achica y se endereza en el navegador), y el encabezado se
actualiza sin recargar.

**Prueba independiente**: con el backend (T-B706) y MinIO, en HTTPS local: cargar un CUIT válido;
subir una foto JPEG grande (> 2 MiB, > 2000 px, sacada con un celular y copiada a la computadora) y
ver "Preparando la imagen…", luego "Logo actualizado. Lo achicamos para que pese menos." y el logo
nuevo en el encabezado; subir un PNG con transparencia chico y verlo tal cual; cerrar sesión,
entrar con otra empresa en el mismo navegador y verificar que **no** se ve el logo anterior.
**En un celular real** (Android y, si hay, iPhone), elegir una foto de la galería: necesita un
origen HTTPS accesible desde el teléfono (hallazgo H-12, `ui.md` §27.2): un entorno de *staging*
si existe; si no, en Android, reenvío de puertos de Chrome (`chrome://inspect` → *Port
forwarding* `8443` → `localhost:8443`) con la CA de mkcert instalada en el teléfono (supuesto
S-F10). Valida S-F8 y NFR-F13; si no hay forma de hacerla, se reporta como pendiente.

**T-F601 [T] — CUIT** · DD-16, `ui.md` §13.11
- **Red**: la **misma tabla de casos que T-B701** (con la misma fuente citada): válido con y sin
  guiones → `true`; dígito verificador inválido → `false`; 10 dígitos o letras → `false`; casos
  del módulo 11 con resto 10 u 11 según la regla oficial.
- **Green**: `lib/cuit.ts` (`isValidCuit`) pasa la tabla.

**T-F602 [T] — Datos de la empresa** · US-4, DD-15, DD-27, BR-F09, BR-F10, INV-F04, `ui.md` §13.11
- **Red**:

  | Interacción | Resultado observable |
  |---|---|
  | `GET /tenant` pendiente / `503` | *skeleton* / "No pudimos cargar los datos de tu empresa." + "Reintentar" |
  | Datos cargados | campos con sus valores; "Moneda base: Pesos (ARS)" + "No se puede cambiar" (no editable) |
  | Sin cambios | "Guardar cambios" deshabilitado |
  | Cambiar solo el teléfono → Guardar | `PATCH /tenant` con `{ phone }` solamente |
  | Vaciar la razón social → Guardar | `{ legal_name: null }` |
  | CUIT con dígito verificador inválido | error en el campo; sin request |
  | `422 invalid_tax_id` / `422 invalid_timezone` del servidor | error en "CUIT" / en "Zona horaria" |
  | `200` | toast "Guardamos los datos de la empresa."; el encabezado muestra el nombre nuevo |
  | Salir con cambios sin guardar | "Tenés cambios sin guardar. ¿Salir igual?"; "Salir" navega, "Quedarme" no |
  | Zona horaria | `America/Argentina/*` primero en el selector |
  | `503` al guardar | mensaje con "Reintentar"; valores intactos |
- **Green**: pasa la tabla.

**T-F603 [T] — Logo en la pantalla y en el encabezado** · US-4, DD-11, DD-23, DD-31, DD-F11, DD-F12, DD-F22, BR-F13, INV-F14, `ui.md` §11.1, §13.11, §14.4
- **Red** (pantalla con MSW; el módulo `features/tenant/logo/canvas.ts` con `vi.mock`: `decodeImage`
  devuelve un bitmap falso con el tamaño pedido y `encodeBitmap` un `Blob` del tamaño que fija cada
  caso):

  | Interacción | Resultado observable |
  |---|---|
  | `has_logo: false` | iniciales + "Todavía no subiste un logo"; encabezado con iniciales |
  | `has_logo: true` | `<img alt="Logo de {empresa}">` con `src` = `/api/v1/tenant/logo?v=` + `encodeURIComponent(id-updated_at)` en S-11 y en el encabezado |
  | Elegir un PNG chico dentro de los límites | sin "Preparando…" visible más de un instante; `PUT /tenant/logo` multipart con **una sola** parte `file` con los **mismos bytes** del archivo; toast "Logo actualizado." |
  | Elegir un PNG de 1000×1000 de **exactamente 2 097 152 bytes** (encabezado real + relleno) | se sube tal cual (mismos bytes); los falsos del canvas no se llamaron |
  | Elegir un JPEG de 5 MB y 4000×3000 (bytes de fixture con encabezado real) | "Preparando la imagen…" anunciado; "Cambiar logo" y "Quitar" deshabilitados; luego "Subiendo logo…"; `PUT` con un archivo `image/jpeg` ≤ 2 097 152 bytes; toast "Logo actualizado. Lo achicamos para que pese menos."; `src` con el `updated_at` nuevo en S-11 y en el encabezado |
  | Elegir un SVG / un GIF / un WebP | mensaje de §13.11.1 para cada uno; **ningún** request |
  | Elegir un archivo con bytes que no son imagen | "No pudimos leer la imagen. Probá con otro archivo PNG o JPG."; ningún request |
  | El adaptador falla al decodificar | mismo mensaje de "no pudimos leer"; ningún request |
  | Ningún paso de la escalera entra (el adaptador falso devuelve siempre 3 MB) | "No pudimos achicar la imagen lo suficiente…"; ningún request |
  | `413` / `415` del servidor | mensajes de §12.2 en el control del logo |
  | `422` con `errors: [{field: 'file', code: 'invalid_value'}]` / `[{field: 'file', code: 'required'}]` | "No pudimos leer la imagen o mide más de 2000 × 2000 px. Probá con otra imagen del logo." / "Elegí una imagen para el logo." en el control del logo (no en un campo del formulario de datos) |
  | `400 malformed_request` con `instance` | "No pudimos subir el logo." + "Recargá la página y probá de nuevo." + código |
  | `503` / sin red al subir | mensaje con "Reintentar" en el control del logo |
  | Salir mientras se prepara o sube | pide confirmación (BR-F10) |
  | "Quitar" → confirmar → `204` | iniciales; toast "Quitamos el logo." |
  | La imagen falla al cargar | iniciales |
  | Control de archivo | etiqueta accesible "Cambiar logo"; `accept="image/png,image/jpeg"`; operable con teclado |
- **Green**: pasa la tabla.

**T-F604 [T] — Preparación del logo: funciones puras y orquestación** · DD-11, DD-31, DD-F20, DD-F21, DD-F22, INV-F13, H-11, `ui.md` §13.11.1
- **Red** (unitario; bytes de prueba armados en el test, sin imágenes reales):

  | `sniffImageType` | Esperado |
  |---|---|
  | Firma PNG | `png` |
  | `FF D8 FF E0` y `FF D8 FF E1` (JFIF y EXIF) | `jpeg` |
  | `GIF89a` / `RIFF….WEBP` / `….ftypheic` | `gif` / `webp` / `heic` |
  | Texto `<svg …>`, con BOM y espacios antes, y `<?xml … ?><svg` | `svg` |
  | Bytes arbitrarios con `declaredType = 'image/svg+xml'` | `svg` |
  | Bytes arbitrarios, archivo vacío | `unknown` |

  | `readImageSize` | Esperado |
  |---|---|
  | PNG con `IHDR` de 3000×2000 | `{ width: 3000, height: 2000 }` |
  | JPEG con `APP1` (EXIF de 20 KB) antes de un `SOF0` de 4000×3000 | `{ width: 4000, height: 3000 }` |
  | JPEG progresivo (`SOF2`) | dimensiones correctas |
  | JPEG con un `DHT` (`C4`) antes del `SOF` | no lo confunde con un `SOF` |
  | Encabezado truncado | `null` |

  | `planLogoProcessing` | Esperado |
  |---|---|
  | PNG de 800×400 y 300 KB | `upload_original` |
  | PNG de 1000×1000 y **2 097 152** bytes | `upload_original` (el límite es inclusivo, igual que en el servidor) |
  | PNG de 1000×1000 y **2 097 153** bytes | `reencode` `image/png` con la escalera PNG |
  | PNG de 3000×3000 (o de 1500×1500 y 2,5 MB) | `reencode` `image/png` con la escalera PNG; ningún `maxSide` mayor al lado original |
  | JPEG de 1200×800 y 200 KB | `reencode` `image/jpeg` (DD-F21) con primer paso (1200, 0,90) |
  | JPEG de 4000×3000 y 5 MB | `reencode` `image/jpeg` con la escalera JPEG completa |
  | Cualquier imagen de más de 25 MP o archivo de más de 20 MB | `reject` `too_large_to_process` |
  | `svg` / `gif` / `webp` / `heic` / `unknown` | `reject` `svg` / `unsupported_format` (los cuatro restantes) |
  | PNG o JPEG con `width: null` | `reject` `unreadable` |

  | `fitWithin` | Esperado |
  |---|---|
  | (4000, 3000, 2000) / (3000, 4000, 2000) / (800, 400, 2000) | (2000, 1500) / (1500, 2000) / (800, 400) |

  | `prepareLogo` (con `decodeImage` y `encodeBitmap` falsos inyectados) | Esperado |
  |---|---|
  | PNG dentro de los límites | `ok`, mismo `File`, `reencoded: false`; los falsos no se llamaron |
  | JPEG grande; el falso devuelve 2,4 MB, 2,1 MB y después 1,5 MB | `ok`, `resized: true`, archivo `image/jpeg` de 1,5 MB; `decodeImage` llamado **una** vez; `encodeBitmap` tres veces con los pasos de la escalera en orden |
  | JPEG grande; el falso devuelve **2 097 153** bytes y después **2 097 152** | `ok` con el segundo (el borde exacto entra) |
  | Ningún paso entra | `{ ok: false, reason: 'cannot_shrink' }` |
  | `decodeImage` rechaza | `{ ok: false, reason: 'unreadable' }` |
  | En todos los caminos con decodificación | el bitmap se cierra (`close` del falso llamado) |
  | Constante | `LOGO_TARGET_MAX_BYTES === 2_097_152` (igual a `LogoMaxBytes` del backend, DD-31) |
- **Green**: pasa la tabla.
- **Refactor**: las constantes (`limits.ts`) en un solo lugar; `prepareLogo` sin ramas de formato
  que no estén en `planLogoProcessing`.

**T-F605 — Implementar** `CompanyPage`, `LogoUploader`, `useUpdateTenant`, `useUploadLogo`
(`putTenantLogo`, con una sola parte `file`), `useDeleteLogo`, `logoUrl`, `lib/cuit.ts`,
`features/tenant/logo/` (`limits.ts`, `sniff.ts`, `plan.ts`, `canvas.ts`, `prepareLogo.ts`); el
encabezado pasa a mostrar el logo.

**Checkpoint Fase F6**: `npm run check` en verde + prueba independiente (la prueba en un celular
real, con su resultado para S-F8, S-F10 y NFR-F13, o reportada como pendiente por H-12).

---

### Fase F7 — End-to-end, PWA, accesibilidad y performance (verificación)

**Objetivo**: los flujos críticos funcionan de punta a punta sobre el binario real servido por
HTTPS local; la PWA, la CSP, la cookie de sesión real, la preparación del logo y la accesibilidad
están verificadas; los objetivos de performance están medidos. Esta fase **verifica y cierra
huecos**: cada pantalla ya llegó con sus estados y su accesibilidad.

**Prueba independiente**: `npm run e2e` en verde en CI (Chromium, contra `https://localhost:8443`
con la CA de mkcert del runner); corrida en WebKit reportada; reporte con tamaños de bundle,
métricas de Lighthouse y el resultado del supuesto S-12.

**T-F701 — Playwright y job de E2E** · ADR-022, DD-24, plan §10.5.1, S-12
- `playwright.config.ts`: Chromium en cada PR; WebKit en la corrida previa a liberar;
  `use.baseURL = 'https://localhost:8443'`; **sin** `ignoreHTTPSErrors` (ver respaldo abajo).
- *Global setup*: verifica, con Node y `NODE_EXTRA_CA_CERTS`, que `https://localhost:8443/healthz`
  responde `200` **sin** error de certificado y que `docker compose` está arriba; si falla, un
  mensaje que remite a `make dev-certs`, `mkcert -install` y la tabla de diagnóstico de `ui.md`
  §21.3.
- Helpers: leer emails y extraer el enlace desde la API HTTP de Mailpit (los enlaces apuntan a
  `https://localhost:8443/…`, porque `APP_BASE_URL` es ese origen); axe (`@axe-core/playwright`,
  reglas WCAG 2.x AA); captura de eventos `securitypolicyviolation` y errores de consola.
- **Job de E2E en CI** (receta de plan §10.5.1): instalar mkcert de una versión fijada con checksum
  verificado y `certutil`; `mkcert -install`; `make dev-certs`; `docker compose up`;
  `crm migrate up`; `crm serve` con `HTTP_ADDR=:8443`, `TLS_CERT_FILE`, `TLS_KEY_FILE` y
  `APP_BASE_URL=https://localhost:8443` (el binario viene del artefacto de T-F009);
  `NODE_EXTRA_CA_CERTS` para el proceso de Playwright. La CA se crea en el runner y muere con él;
  nunca se sube como artefacto.
- **Supuesto S-12** (se valida en la primera corrida): Chromium y WebKit de Playwright abren
  `https://localhost:8443` sin error de certificado. Si alguno falla: respaldo
  `ignoreHTTPSErrors: true` **solo para ese navegador** y **solo si** T-F703 muestra que el
  service worker se registra igual; si no, frenar y volver al arquitecto. El resultado va al
  reporte de la fase.
- Fixtures de imágenes en `web/e2e/fixtures/` generadas para la prueba (sin fotos reales de
  personas ni lugares): JPEG 4000×3000 de ~5 MB con EXIF de orientación 6 y coordenadas GPS
  ficticias; PNG 3000×3000 con zonas transparentes; PNG 800×400 < 1 MB; PNG ≤ 2000 px de entre
  1 900 000 y 2 097 152 bytes; SVG; GIF.

**T-F702 [T] — Flujos críticos** · SC-001, US-1..US-4, NFR-F01, NFR-F06, NFR-F10, P-5, INV-23 (plan)
- **Red**:

  | Flujo | Resultado observable |
  |---|---|
  | Registro → panel → cerrar sesión → ingresar → panel | cada paso visible; registro completo < 10 s automatizado (SC-001) |
  | Tras el registro, las cookies del contexto (`context.cookies()`) | `__Host-crm_session` con `secure: true`, `httpOnly: true`, `sameSite: 'Lax'`, `path: '/'` y dominio `localhost` sin punto inicial (es la cookie real, H-10) |
  | Registro con un email existente → "Recuperar contraseña" → enviar | "Revisá tu email"; llega el email |
  | Admin invita → enlace de Mailpit → invitado acepta → panel de Operador → `/settings/users` | "Sin permiso" |
  | Admin reinvita a un invitado con otro rol → el invitado acepta | entra con el rol nuevo |
  | Olvidé mi contraseña → enlace → contraseña nueva → ingresar; reabrir el enlace | éxito; "Este enlace ya no sirve." |
  | Admin desactiva al operador (otro contexto de navegador) → el operador navega | login con "Tu sesión se cerró." |
  | 5 contraseñas incorrectas → 6.º intento | estado Bloqueado con la hora |
  | En cada pantalla visitada | axe sin violaciones *serious*/*critical*; a 320 px de ancho, `scrollWidth ≤ innerWidth`; 0 violaciones de CSP; 0 errores de consola |
  | Cabeceras de `/` en el binario local | CSP de §10.7 presente; `Strict-Transport-Security` **ausente** (modo local, DD-24) |
- **Green**: todos los flujos pasan en Chromium; WebKit reportado.

**T-F703 [T] — PWA y service worker** · ADR-020, INV-F09, NFR-F09, NFR-F11
- **Red**:

  | Caso | Esperado |
  |---|---|
  | `manifest.webmanifest` | campos de `ui.md` §20 (`name` "CRM"); los íconos responden `200` |
  | Tras cargar `/` en el binario | SW activo con `scope` `/` |
  | Contexto sin conexión → navegar a `/settings` | se ve "Sin conexión. Para usar la app necesitás internet." |
  | Cache Storage | solo `crm-offline-v1` con `offline.html`; ninguna URL `/api/` ni `/assets/` |
  | Sin conexión, `fetch('/api/v1/me')` desde la página | falla (el SW no lo sirve) |
  | Después de cerrar sesión | `localStorage` e IndexedDB sin datos; `sessionStorage` solo con marcas de UI |
  | Solo si se aplicó el respaldo de S-12 (`ignoreHTTPSErrors`) en algún navegador | en ese navegador el SW también queda activo; si no, el respaldo no es válido y se vuelve al arquitecto |
- **Green**: pasa la tabla.

**T-F704 — Performance** · NFR-F03, NFR-F04, NFR-F05
- Reportar los tamaños gzip de `vite build` por chunk frente al presupuesto de NFR-F05 y
  Lighthouse *mobile* sobre el binario en `https://localhost:8443` en `/login` y `/` (LCP, CLS; INP
  en un Android de gama media a mano, con el origen de H-12). **Si un objetivo no se cumple, frenar
  y volver al arquitecto** con los números.

**T-F705 — Verificación manual de accesibilidad** · NFR-F06, NFR-F07, NFR-F08, `ui.md` §18
- Checklist: todos los flujos solo con teclado; TalkBack (Chrome Android) y VoiceOver (Safari
  iOS) en registro, ingresar, usuarios y datos de la empresa (incluida la subida del logo y el
  anuncio "Preparando la imagen…"), con el origen HTTPS de H-12 (si no hay, VoiceOver de macOS y
  NVDA en escritorio, y el celular queda pendiente); zoom al 200 %; `prefers-reduced-motion`;
  contraste de cada par de tokens de §19.1 con una herramienta; objetivos táctiles ≥ 44 px; foco
  nunca tapado por la barra inferior. Resultado en el reporte de la fase, con cada hallazgo y su
  corrección.

**T-F706 [T] — Preparación del logo en navegadores reales** · DD-F20, DD-F21, DD-F22, INV-F13, NFR-F13, S-F7, S-F9, `ui.md` §13.11.1
- **Red** (Playwright, Chromium y WebKit; cada caso sube por la UI y después descarga
  `/api/v1/tenant/logo` desde la página para inspeccionar lo que guardó el servidor):

  | Fixture | Esperado |
  |---|---|
  | JPEG 4000×3000, ~5 MB, EXIF orientación 6 y GPS | se sube; el logo guardado es JPEG ≤ 2 097 152 bytes (`LOGO_TARGET_MAX_BYTES`), **vertical** (alto > ancho: se aplicó la orientación), lado mayor ≤ 2000, **sin** segmento `APP1`/`Exif` (sin GPS); tiempo de preparación reportado (NFR-F13) |
  | PNG 3000×3000 con transparencia | se sube como PNG ≤ 2000 px y ≤ 2 097 152 bytes; el píxel de una esquina transparente tiene alfa 0 (se lee dibujando el logo guardado en un canvas de la página) |
  | PNG 800×400 < 1 MB | los bytes guardados son **idénticos** a los del archivo |
  | PNG ≤ 2000 px de entre 1 900 000 y 2 097 152 bytes | los bytes guardados son **idénticos** (sin margen: DD-F22) |
  | SVG / GIF | mensaje de §13.11.1; ningún `PUT` en la red |
- **Green**: pasa en Chromium y WebKit; si S-F7 falla en algún navegador (foto horizontal), frenar
  y volver al arquitecto con el resultado (respaldo previsto: leer la orientación del EXIF y rotar
  en el canvas).

**T-F707 — Correcciones** que surjan de T-F702..T-F706, cada una con su test de regresión.

**Checkpoint Fase F7**: `npm run check` y `npm run e2e` en verde (Chromium en CI; WebKit
reportado); reportes de T-F704, T-F705 y T-F706 adjuntos; resultado de S-12 (y del respaldo, si se
usó) reportado.

---

### Trazabilidad (frontend)

| Requisito / criterio / decisión | Tareas |
|---|---|
| FR-001 Registro autónomo | T-F201, T-F202, T-F702 |
| FR-002 Plantillas | T-F202, T-F204 |
| FR-003 Email + contraseña | T-F301 |
| FR-004 Recuperar contraseña | T-F402, T-F403, T-F702 |
| FR-005 Invitar, desactivar, cambiar rol (también de invitados), reactivar | T-F501..T-F504, T-F702 |
| FR-006 Aislamiento (del lado del cliente: caché vacía entre sesiones, logo por empresa) | T-F104, T-F303, T-F505, T-F603, T-F703 |
| FR-007 Matriz de permisos (UX) | T-F104, T-F106, T-F502, T-F702 |
| US-1 (1, 2, 3) | T-F202 (1, 2), T-F404/T-F405 (3) |
| US-2 (1, 2, 3) | T-F301 (1, 2), T-F402/T-F403 (3), T-F303 (cerrar sesión) |
| US-3 (1, 2, 3, 4) | T-F504 (1), T-F505 (2), T-F503 (3, 4) |
| US-4 (1) | T-F602, T-F603, T-F604, T-F706 |
| Casos borde (404 de otra empresa, 403 de operador) | T-F503 (404), T-F104, T-F502, T-F702 (403) |
| SC-001 Panel en < 3 min | T-F202, T-F702, checkpoint F2 |
| P-2 Verificación no bloqueante | T-F405 |
| P-3 Reactivación | T-F503 |
| P-4 `email_already_registered` | T-F202, T-F702 |
| P-5 Sesión 24 h / 7 días (401 global) | T-F104, T-F702 |
| P-F2 Logo achicado en el navegador (DD-F11, DD-F20) | T-F603, T-F604, T-F706 |
| P-F6 (resuelta) / DD-F21 JPEG siempre re-codificado | T-F604, T-F706 |
| DD-14 Token en el fragmento | T-F401, T-F007 |
| DD-17 *Bundle* multi-spec | T-F004 |
| DD-23 Caché del logo (`v`, `ETag`) | T-F004, T-F603 |
| DD-25 / DD-26 Invitación vencida, rol de invitados | T-F501, T-F502, T-F503, T-F504 |
| DD-27 Zona horaria del registro | T-F202 |
| DD-30 `403` de CSRF | T-F005, T-F104 |
| ADR-006 (401 en cualquier request vuelve al login) | T-F104 |
| ADR-019 (SPA embebida, cabeceras, CSP, gzip) | T-F007, T-F008, T-F702 |
| ADR-020 (PWA sin offline) | T-F108, T-F703 |
| H-10 (resuelto) HTTPS local con mkcert, sin HSTS en modo local (DD-24, INV-23, DD-F23) | T-F006, T-F007, T-F009, T-F701, T-F702, T-F703 |
| H-11 (resuelto) Límites exactos del logo (DD-31, DD-F22) | T-F004, T-F101, T-F603, T-F604, T-F706 |
| H-12 Prueba en un celular real (abierto, no bloquea) | Checkpoint F6, T-F704, T-F705 |
| NFR-F01..F13 | T-F002 (F02, F07, F08), T-F702 (F01, F06, F10, F12), T-F703 (F09, F11), T-F704 (F03..F05), T-F705 (F06..F08), T-F706 (F13) |
