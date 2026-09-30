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

---

## Backend

Autor: `backend-architect`. Implementa: `backend-developer`, **una fase por invocación**.

### Estado de la implementación (2026-09-30)

- **Fase 0**: implementada y mergeada (PR fdelillo/crm#6).
- **Fase 1**: implementada en la rama, **en revisión**.
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
  con `context` al recibir SIGTERM.

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
  | `InTenantTx` de una empresa sin rol (`42704`) | sin reintento; el error nombra el rol (fila de arriba) |
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
  `tenants`). Excepciones **solo por ruta exacta** (tercera revisión), cada una con su motivo:

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
  | Una excepción cuyo archivo todavía no existe (`cleanup.sql` y `provisioning.sql` llegan en fases posteriores) | no es error |
- **Green**: `queryrules.DefaultExceptions` tiene exactamente las cuatro rutas; el detector
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
  | Body > 64 KB | `413 payload_too_large` |
  | Sin `Content-Type: application/json` | `415` |
  | `Content-Type: application/json; charset=utf-8` | aceptado |
- **Green**: pasa la tabla; las respuestas validan contra `Problem`/`ValidationProblem` del
  contrato.

**T-B202 — Implementar `platform/httpx` (problem+json con `suggested_action` y `Retry-After` opcionales, DecodeJSON, mapeo por defecto de errores, incluido `db.ErrCanceled` de plan §9.2)**.

**T-B219 [T] — IP del cliente (`httpx.ClientIP`)** · DD-32, INV-25, plan §10.3, research R-25
(Numeración agregada en la tercera revisión; se hace **antes** de T-B203, porque la cadena común
empieza con `ClientIP`.)
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
  | `198.51.100.10:4711` | `192.0.2.66, basura` | `198.51.100.10` + log `WARN` `event=bad_forwarded_for` (sin el valor de la cabecera) |
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
  | Handler de la API que devuelve `db.ErrCanceled` después de que el cliente cortó (contexto del request cancelado) | no se escribe cuerpo; log de request `level=INFO`, `event=client_canceled`, `status=499`; `http_client_canceled_total` +1; **ningún** log `ERROR` (tampoco del recover) |
  | El mismo handler con el contexto del request vivo | `503 service_unavailable` |
- **Green**: pasa la tabla.

**T-B204 — Implementar middlewares** (comunes, en el mux raíz, en este orden: `httpx.ClientIP`
(DD-32, T-B220) → request id → recover → logging (con `ip` y el `499` de las cancelaciones) →
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

**T-B209 [T] — Auditoría** · FR-008, INV-15
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | `Record` dentro de `InTenantTx(A)` | fila en `audit_log` con `tenant_id=A`, `request_id` del contexto |
  | `Record` con `Entry.TenantID = B` dentro de `InTenantTx(A)` | error (RLS); la transacción no deja fila |
  | `Record` y luego `ROLLBACK` de la operación | no queda fila (la auditoría es parte de la operación) |
  | `UPDATE`/`DELETE` sobre `audit_log` como rol de empresa | `42501` |
  | `Data` con clave `password` o `token` | error de programación (el Recorder las rechaza) |
- **Green**: pasa la tabla.

**T-B210 — Implementar `platform/audit`**.

**T-B211 [T] — Outbox y worker** · ADR-010, INV-09, INV-16, plan §9.4
- **Red** (integración, `Mailer` falso configurable, `Clock` falso):

  | Caso | Esperado |
  |---|---|
  | `Enqueue` en `InTenantTx(A)` + `COMMIT`, luego un ciclo del `Dispatcher` | el fake recibe 1 email; fila `sent`, `payload IS NULL`, `sent_at` puesto |
  | `Enqueue` + `ROLLBACK` | el worker no envía nada |
  | Fake devuelve error recuperable | `pending`, `attempts=1`, `next_attempt_at = now + 1 min`, `last_error` sin datos personales |
  | Errores recuperables sucesivos | backoff 1, 5, 15, 60 min, 6 h (y 6 h en adelante) |
  | 8.º error recuperable | `failed`, `payload IS NULL`, `failed_at` puesto |
  | Fake devuelve `*outbox.PermanentError` | `failed` al primer intento |
  | Mensaje con `next_attempt_at` futuro | no se toma |
  | Dos `Dispatcher` concurrentes con 20 mensajes | cada mensaje se envía **una** vez (`FOR UPDATE SKIP LOCKED`) |
  | Mensajes de `A` y `B` | cada uno se lee y marca bajo el rol de su empresa (`current_user` capturado por el fake vía hook de test) |
  | `UPDATE` que deja `sent` con payload | la base lo rechaza (`outbox_scrub_chk`) |
  | (Tercera revisión) El fake bloquea hasta que se cancela el contexto del `Dispatcher` y devuelve `context.Canceled` (apagado a mitad del envío) | el mensaje queda **exactamente** como estaba: `pending`, `attempts`, `next_attempt_at` y `last_error` sin cambios; log `INFO` con `outcome=canceled`; ningún log `ERROR`; en el próximo arranque se toma de nuevo |
- **Green**: pasa la tabla. **Refactor**: la política de backoff es una función pura con su propio
  test unitario; la clasificación "cancelación / recuperable / definitivo" también (plan §9.4).

**T-B212 — Implementar `platform/outbox` (`Enqueue`, `Dispatcher` con *polling* de 2 s y lote de 10, arranque y parada con `context`)**. Las queries del worker van en `internal/platform/outbox/store/worker.sql` (plan §4.4).

**T-B213 [T] — Adaptador SMTP y plantillas** · ADR-010, DD-14, DD-24
- **Red** (integración contra Mailpit en contenedor; su API HTTP para leer lo recibido):

  | Caso | Esperado |
  |---|---|
  | Enviar `password_reset` con `{link}` y `APP_BASE_URL=https://crm.example` | llega a Mailpit con asunto en español, partes texto y HTML, enlace `https://crm.example/reset-password#token=…` (token en el **fragmento**, DD-14) |
  | Plantillas `email_verification` e `invitation` | enlaces `…/verify-email#token=…` y `…/accept-invitation#token=…`; nombre de empresa y rol en español ("Administrador"/"Operador"); sin campos vacíos |
  | `APP_BASE_URL=https://localhost:5173` | enlaces a `https://localhost:5173/…` (desarrollo con Vite y HTTPS local, plan §10.5.1) |
  | Destinatario con `\r\n` | error antes de conectar |
  | SMTP inalcanzable (puerto cerrado) | error recuperable (no `PermanentError`), respeta timeout de `context` |
  | Respuesta `5xx` simulada | `PermanentError` (si Mailpit no permite simularla, test unitario del clasificador de códigos) |
- **Green**: pasa la tabla.

**T-B214 — Implementar `platform/mailer` (go-mail) y las plantillas `identity/emails` (es-AR)**.

**T-B215 [T] — Adaptador S3** · ADR-011
- **Red** (integración contra MinIO en contenedor):

  | Caso | Esperado |
  |---|---|
  | `Put` + `Get` | mismos bytes y `ContentType` |
  | `Get` de clave inexistente | `objectstore.ErrNotFound` |
  | `Delete` de clave inexistente | sin error (idempotente) |
  | Endpoint caído | error envuelto clasificado como no disponible |
- **Green**: pasa la tabla.

**T-B216 — Implementar `platform/objectstore` (minio-go)**.

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

**Checkpoint Fase 2**: `make check` en verde.

---

### Fase 3 — Historia 1: Registrar una empresa (P1) — primer slice vertical

**Objetivo**: un visitante se registra, queda logueado y ve su sesión en `/me`; el email de
verificación llega a Mailpit.

**Prueba independiente**: `POST /api/v1/auth/signup` → `201` + cookie → `GET /api/v1/me` con esa
cookie → `200` con rol `admin`, los 15 permisos y `email_verified: false`; en Mailpit hay un email
de verificación. Repetir el registro con el mismo email → `409 email_already_registered` con
`suggested_action: password_reset`.

**T-B301 [T] — Catálogo de plantillas y `GET /industry-templates`** · FR-002, DD-3
- **Red**:

  | Caso | Esperado |
  |---|---|
  | `industrytemplate.All()` | contiene al menos `aluminum_carpentry` ("Carpintería de aluminio") y `generic` ("Genérico"), cada uno con versión ≥ 1 |
  | `Lookup("inexistente")` | `false` |
  | `GET /api/v1/industry-templates` | `200`, valida contra el contrato, sin cookie |
- **Green**: pasa la tabla. El catálogo se lee de un archivo de datos embebido, no de constantes
  por rubro en el código (principio II).

**T-B302 — Implementar `internal/industrytemplate` (catálogo + `Seeder` sin efecto) y el handler**.

**T-B303 [T] — Servicio de registro** · US-1, FR-001, FR-002, INV-14, INV-16, INV-26, DD-2, DD-4, DD-13, DD-15, DD-27, DD-33, H-6
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
  | `timezone` ausente, vacía, `Marte/Olympus` o de 64 caracteres inventados | la empresa se crea con `America/Argentina/Buenos_Aires`; log `event=signup_timezone_defaulted`; **sin** error (DD-27) |
  | `base_currency` fuera de ARS/USD | validación `invalid_value` |
  | Contraseña de 9 caracteres o igual al email | validación; **no** se calcula hash ni se abre transacción |
  | (Tercera revisión, DD-33) **10 registros concurrentes** con emails distintos | los 10 tienen éxito (ninguno `ErrUnavailable`): 10 empresas y 10 roles |
  | Una transacción de prueba (superusuario del contenedor) ejecuta `GRANT crm_tenant TO <rol de prueba>` y queda abierta 3 s; mientras, un `Register` | `Register` devuelve un error con `errors.Is(err, db.ErrUnavailable)` en ~2 s (entre 2 y 3 s; **nunca** los 5 s de `statement_timeout` ni un `ErrPrivilege`); no queda empresa, usuario, token, mensaje, sesión ni **rol** nuevos; después del `ROLLBACK` de la transacción de prueba, el mismo `Register` tiene éxito |
  | Dentro de la transacción de registro (observado con el `Seeder` falso como *hook*) | `current_setting('lock_timeout')` = `2s` |
  | Durante todo `Register` | los fakes de `Mailer` y `ObjectStorage` no registran **ninguna** llamada (el email sale por el outbox; R-b de DD-33) |
- **Green**: pasa la tabla.
- **Refactor**: el servicio de `tenant` no conoce SQL de `identity`; solo usa la interfaz
  `AdminOnboarding` (plan §11.1).

**T-B304 — Implementar `tenant.Service.Register` y en `identity`: `CreateFirstAdmin`,
`CreateSession`, `IssueEmailVerification`**. Tercera revisión (DD-33, INV-26): validación y hash
argon2 **antes** de abrir la transacción; al abrirla, `lock_timeout` con `tenant.SignupLockTimeout`
mediante `SELECT set_config('lock_timeout', @timeout, true)` (equivale a `SET LOCAL`), antes de
`provision_tenant_role`; ambas queries en `internal/tenant/store/provisioning.sql` (plan §4.4);
desde el aprovisionamiento hasta el `COMMIT`, solo SQL (nada de SMTP, S3 ni otra E/S de red).

**T-B305 [T] — `POST /auth/signup`** · US-1, SC-001, P-4, P-5, DD-9, DD-19, DD-21, DD-24, DD-27, DD-28, DD-33, INV-20, INV-23, INV-26
- **Red** (integración HTTP con `apitest` (HTTPS + `cookiejar`), contrato validado en cada
  respuesta):

  | Caso | Esperado |
  |---|---|
  | Payload válido | `201` `SessionInfo`; `Set-Cookie: __Host-crm_session=…; Path=/; HttpOnly; Secure; SameSite=Lax; Max-Age=604800`, **sin** `Domain` (aserción explícita de cada atributo, INV-23); `Cache-Control: no-store` |
  | Registro válido y luego `GET /me` con el mismo cliente | `200`: la cookie viajó sola por HTTPS (el `cookiejar` la reenvió) |
  | Email existente | `409`, `code: email_already_registered`, `suggested_action: password_reset`, `title` "Ya existe un usuario con ese email", `detail` "Ya existe un usuario con ese email. ¿Querés recuperar la contraseña?"; valida contra `EmailAlreadyRegisteredProblem` |
  | Email existente de un usuario `invited`, `active` y `disabled` (tres cuentas distintas) | los tres cuerpos son **idénticos byte a byte** salvo `instance` (INV-20) |
  | Cuerpo del `409` | no contiene el nombre ni el id de la otra empresa, ni el nombre, id, rol o estado del usuario existente (búsqueda de esos valores en el cuerpo crudo) |
  | `409` | log con `security_event=signup_email_exists` e `ip`, sin el email en claro; `signup_email_exists_total` +1 |
  | `timezone: "Marte/Olympus"` | `201` (no `422`); `GET /tenant` muestra `America/Argentina/Buenos_Aires` |
  | Campo faltante / campo extra | `422` con `errors[].field` / `400 malformed_request` |
  | Sin `Content-Type` JSON | `415` |
  | 6.º registro en una hora desde la misma IP | `429 rate_limited` con `Retry-After` |
  | 5 registros rechazados con `409` desde la misma IP y un 6.º con email nuevo | el 6.º recibe `429` (los rechazos consumen cupo, DD-9) |
  | (Tercera revisión) Con el lock de `crm_tenant` retenido como en T-B303 | `503` `code: service_unavailable` con `Retry-After: 2` y `Cache-Control: no-store`; valida contra `ServerError` del contrato v0.4.0; sin cookie; log `event=signup_lock_timeout` (sin email); `signup_lock_timeout_total` +1 |
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
  `email_verified_at`; sin cookie → `401`. Validado contra el contrato.

**T-B310 — Implementar `/me` y el cableado en `internal/app`** (grupos de rutas públicas y
autenticadas dentro de chi, worker arrancado por `serve`).

**Checkpoint Fase 3**: `make check` en verde + la prueba independiente ejecutada contra
`docker compose` (registro por `curl`, email visible en Mailpit).

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
  | Login correcto | `200` `SessionInfo` + cookie con `Max-Age=604800`, `Secure`, `HttpOnly`, `SameSite=Lax`, `Path=/`, sin `Domain` (INV-23) |
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
  | 4.º pedido en una hora para el mismo email | se ignora (rate limit por email) sin cambiar la respuesta |
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
reemisión reutiliza la misma función interna que la reinvitación del Administrador (DD-5).

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
  `204` / `400`; `resend` → `202` / `401` sin cookie. Todos validados contra el contrato.

**T-B507 — Implementar los handlers**.

**Checkpoint Fase 5**: `make check` en verde + prueba independiente contra `docker compose`.

---

### Fase 6 — Historia 3: Invitar operadores y administrar usuarios (P2)

**Objetivo**: invitar, reinvitar (con cambio de rol si corresponde), aceptar, cambiar rol de
invitados y activos, desactivar (cerrando sesiones) y reactivar (P-3, confirmado), sin dejar
nunca la empresa sin Administrador y con todo auditado.

**Prueba independiente**: el admin invita a un operador; lo reinvita como administrador; el
invitado acepta y entra como Administrador; como otro operador, `/users` da `403`; el admin
desactiva a un usuario y el siguiente request de ese usuario da `401`; el admin lo reactiva y
vuelve a entrar con su contraseña.

**T-B601 [T] — Invitar y reinvitar** · US-3.1, FR-005, FR-008, INV-10, INV-16, DD-1, DD-5, DD-26, H-5
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | Email nuevo, rol `operator` | usuario `invited`, `name` y `password_hash` nulos; token `invitation` de 7 días con `created_by_user_id`; mensaje `invitation` pendiente; auditoría `user.invited` |
  | Mismo email ya `invited` en la misma empresa, **mismo** rol | token anterior revocado, token nuevo; `reissued = true`; auditoría `user.invitation_reissued {role, trigger: admin}`; **sin** `user.role_changed`; sigue habiendo **un** usuario |
  | Mismo email ya `invited`, rol **distinto** (`operator` → `admin`) | además del caso anterior, `role = admin`; auditoría `user.role_changed {from: operator, to: admin, status: invited}` en la misma transacción (dos filas en total) |
  | Reinvitar como `operator` a un invitado `admin` siendo el creador el único admin activo | permitido (los invitados no cuentan para INV-10) |
  | La reinvitación toma el lock de `tenants` | dos reinvitaciones concurrentes del mismo email con roles distintos terminan con un rol consistente y dos pares de filas de auditoría coherentes (sin interleaving: 20 repeticiones) |
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

**T-B604 [T] — Desactivar y reactivar** · US-3.3, US-3.4, FR-005, FR-008, INV-10, INV-11, P-3
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | Desactivar un operador con 2 sesiones abiertas | `disabled`; ambas sesiones con `revoked_reason = user_disabled`; tokens pendientes revocados; auditoría `user.deactivated {sessions_revoked: 2}` |
  | El siguiente request con cualquiera de sus cookies | `401` |
  | Desactivar al único admin activo | `ErrLastAdmin` |
  | Un admin se desactiva a sí mismo habiendo otro admin | permitido; su sesión actual queda revocada |
  | Desactivar un usuario ya `disabled` | `ErrInvalidTransition` |
  | Desactivar un `invited` | `disabled`; su invitación queda revocada |
  | **Concurrencia**: 2 admins se desactivan mutuamente a la vez | uno solo tiene éxito |
  | Reactivar un `disabled` con contraseña | `active`; auditoría `user.reactivated {to_status: active}` con actor = admin, en la **misma** transacción; sus sesiones viejas **siguen** revocadas; puede iniciar sesión con su contraseña |
  | Reactivar un `disabled` que nunca tuvo contraseña | `invited` con invitación nueva de 7 días (`created_by_user_id` = admin) y mensaje `invitation` encolado; auditoría `user.reactivated {to_status: invited}` **y** `user.invitation_reissued {trigger: reactivation}` (FR-008: toda invitación se audita) |
  | Reactivar un `disabled` sin contraseña **no** puede dejarlo `active` | la base lo impide (`users_active_complete_chk`) aun si el código lo intentara (test directo contra la constraint) |
  | Reactivar un `active` o `invited` | `ErrInvalidTransition`; sin auditoría |
  | Reactivar un id de otra empresa o inexistente | `ErrUserNotFound` |
  | Reactivar un admin desactivado | vuelve a contar como admin activo para INV-10 |
  | **Concurrencia**: un admin reactiva a X mientras otro desactiva a X (20 repeticiones) | las operaciones se serializan por el lock de `tenants`; el estado final es consistente con una de las dos órdenes y hay exactamente una fila de auditoría por operación exitosa |
  | Cualquier operación fallida | ninguna fila de auditoría |
- **Green**: pasa la tabla.

**T-B605 — Implementar `Invite`, `PreviewInvitation`, `AcceptInvitation`, `ListUsers`,
`ChangeRole`, `Deactivate`, `Reactivate`**. Las transiciones de estado se expresan como una
tabla (`estado actual × acción → estado nuevo | error`) con su test unitario (incluye las dos
salidas de `reactivate` según tenga o no contraseña y el rechazo de `changeRole` en `disabled`),
y cada operación de cambio de rol o estado (incluida la reinvitación) empieza con el lock de la
fila `tenants` (INV-10). `ListUsers` informa `invitation_expires_at` desde la invitación abierta
de cada invitado, aunque haya vencido (DD-25).

**T-B606 [T] — Endpoints de usuarios e invitaciones** · contrato, FR-005, FR-007, P-3, DD-25, DD-26, H-4, H-5
- **Red** (HTTP + contrato):

  | Caso | Esperado |
  |---|---|
  | Admin: `GET /users` | `200` con invitados (con `invitation_expires_at`), activos y desactivados (con `invitation_expires_at: null`), en orden de alta |
  | Invitado cuya invitación venció hace 3 días (reloj falso) | `invitation_expires_at` = esa fecha pasada (no `null`) |
  | Invitado cuya invitación venció hace 40 días, después de correr la limpieza (T-B901) | sigue informando la fecha (la limpieza conserva la invitación abierta) |
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
  | `name` vacío | `required` |
- **Green**: pasa la tabla.

**T-B702 [T] — Actualización de datos** · US-4, FR-008, DD-27
- **Red** (integración): `PATCH` parcial solo cambia los campos presentes; `null` borra un campo
  opcional; `updated_at` avanza; auditoría `tenant.updated` con la **lista de nombres** de campos
  (no los valores); `base_currency` en el payload → `400 malformed_request` (campo desconocido);
  `timezone: "Marte/Olympus"` → validación `invalid_timezone` (acá **sí** es error, a diferencia
  del registro).

**T-B703 [T] — Logo (servicio)** · US-4, DD-11, DD-23, DD-31, INV-17, INV-24, H-2, H-11
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
  | La transacción falla después del `Put` | el objeto nuevo se borra (mejor esfuerzo, logueado si falla) |
  | Borrar el objeto viejo falla | la operación igual responde éxito; log `WARN` (objeto huérfano aceptado) |
  | `RemoveLogo` | columnas en `NULL`, `updated_at` avanza, objeto borrado, auditoría `tenant.logo_removed`; repetir → sin error |
  | `GetLogo` con `ifNoneMatch` vacío | `ETag = "<uuid de la clave>"`, cuerpo del objeto |
  | `GetLogo` con `ifNoneMatch` igual al vigente | `NotModified = true`; el fake de S3 **no** recibió `Get` |
  | `GetLogo` con un `ifNoneMatch` viejo o de otra empresa | cuerpo completo con el `ETag` vigente |
- **Green**: pasa la tabla.

**T-B704 — Implementar `tenant.Service.Update`, `SetLogo`, `RemoveLogo`, `GetLogo`** y las
constantes `LogoMaxBytes`, `LogoMaxBodyBytes`, `LogoMaxSide` (plan §11.1).

**T-B705 [T] — Endpoints de empresa** · contrato, FR-007, DD-23, DD-28, DD-31, INV-21, INV-24, H-2, H-7, H-11
- **Red** (HTTP + contrato):

  | Caso | Esperado |
  |---|---|
  | `GET /tenant` como admin y como operador | `200`; `Cache-Control: no-store` |
  | `PATCH /tenant` | `200` admin, `403` operador, `422` CUIT inválido o `timezone` desconocida; `no-store` |
  | `PUT /tenant/logo` multipart con un PNG válido | `200` (el `Tenant` devuelto tiene `updated_at` nuevo); `403` para el operador |
  | Parte `file` de **exactamente 2 097 152 bytes** con un nombre de archivo de 200 caracteres | `200` (el límite es del archivo, no del cuerpo) |
  | Parte `file` de **2 097 153 bytes** | `413 payload_too_large`; el handler dejó de leer: un lector instrumentado del cuerpo registra como máximo `LogoMaxBodyBytes` bytes leídos |
  | Cuerpo de más de 2 162 688 bytes (una parte `file` chica precedida de relleno en su encabezado o en el preámbulo) | `413 payload_too_large` |
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
  | `DELETE /tenant/logo` | `204` con `no-store`; un `GET` posterior con el `If-None-Match` viejo → `404` (nunca `304`) |
- **Green**: pasa la tabla.

**T-B706 — Implementar los handlers** (en `tenant/http.go`: `http.MaxBytesReader` con
`LogoMaxBodyBytes`, `r.MultipartReader()` con exactamente una parte `file` leída con tope de
`LogoMaxBytes + 1` bytes, y el mapeo de errores de DD-31 y plan §9.2).

**Checkpoint Fase 7**: `make check` en verde + prueba independiente con MinIO de `docker compose`.

---

### Fase 8 — Aislamiento entre empresas de punta a punta (SC-002, principio III)

**Objetivo**: demostrar con tests automáticos **0 accesos cruzados en todos los endpoints**, y que
el test falle solo si mañana se agrega un endpoint sin cubrir.

**Prueba independiente**: `go test -tags=integration -run Isolation ./...` en verde, con el
reporte de cobertura de rutas al 100 %.

**Fixture común** (`internal/testsupport/fixture`): empresas `A` y `B`, cada una con un admin, un
operador, un invitado, un desactivado con contraseña, un desactivado sin contraseña, logo y datos
completos; cookies de sesión de cada usuario; tokens vigentes de reset, verificación e invitación
de cada empresa.

**T-B801 [T] — Matriz HTTP de aislamiento con cobertura de rutas** · SC-002, FR-006, INV-12, INV-21
- **Red**:
  - El test obtiene **todas** las rutas registradas con `chi.Walk` **sobre el router de la API**
    (el que `internal/app` monta en `/api/`; la SPA y ops no están en él, DD-22) y las compara con
    una tabla declarada `ruta → caso de aislamiento`. **Si existe una ruta sin caso, el test
    falla** (así se protege a las specs futuras). El mismo listado se compara con los `paths` del
    contrato (sin `/healthz` ni `/readyz`, que están en el mux raíz).
  - Casos por ruta (como admin de `A`):

    | Tipo de ruta | Ataque | Esperado |
    |---|---|---|
    | Con `{userId}` (`role`, `deactivate`, `reactivate`) | usar ids de usuarios de `B` (cada estado) | `404`, cuerpo idéntico al de un id inexistente; `B` sin cambios ni filas de auditoría nuevas (verificado como `B`) |
    | Sin id, lectura (`/me`, `/tenant`, `/users`, `/tenant/logo`) | — | solo datos de `A`: ningún id, email o nombre de `B` aparece en el cuerpo |
    | `GET /tenant/logo` con cookie de `A` e `If-None-Match` = `ETag` del logo de `B` | reuso de caché entre empresas en un dispositivo compartido | `200` con el logo de `A` (nunca `304`) (INV-21) |
    | Sin id, escritura (`PATCH /tenant`, `PUT/DELETE /tenant/logo`, `POST /users/invitations`) | — | modifica solo `A`; `B` sin cambios; la clave del logo empieza con `tenants/{A}/` |
    | Públicas con token (`/auth/*`) | token de `B` usado junto con la cookie de `A` | la operación afecta solo a `B` (la cookie no cambia la empresa del token); ninguna fila de `A` cambia |
    | `POST /auth/signup` con el email de un usuario de `B` | — | `409 email_already_registered` sin ningún dato de `B` (INV-20) |
- **Green**: todas las filas pasan y la cobertura de rutas es total.

**T-B802 [T] — Matriz de permisos por endpoint** · FR-007, INV-12
- **Red**: para cada ruta de la tabla de T-B801 × {anónimo, operador, admin}: el status esperado
  (`401` / `403` / éxito) declarado en la tabla; el `403` se obtiene **aunque el id sea de otra
  empresa o no exista** (la autorización no depende de la existencia del recurso).

**T-B803 [T] — Aislamiento en flujos sin empresa conocida** · plan §4.4
- **Red**:

  | Caso | Esperado |
  |---|---|
  | Login de un usuario de `B` | la sesión creada tiene `tenant_id = B` y `/me` devuelve `B` |
  | Reset de un usuario de `B` | cambia solo ese usuario; sesiones de `A` intactas |
  | Pedido de reset del invitado de `B` | la invitación reemitida y su auditoría quedan en `B`; nada cambia en `A` |
  | Worker con mensajes de `A` y `B` | cada `UPDATE` corre bajo el rol de su empresa; ningún mensaje de `B` pasa por una transacción de `A` |
  | `ResolveSession` con token de `B` | `Principal.TenantID = B` |

**T-B804 [T] — Defensa en profundidad demostrada** · INV-01, INV-04
- **Red**: una query **de test** idéntica a la de `ListUsers` pero **sin** `WHERE tenant_id`,
  ejecutada en `InTenantTx(A)`, devuelve solo usuarios de `A` (la RLS sostiene aunque falte el
  filtro); y la query real, ejecutada con un `tenant_id` de `B` como parámetro pero en
  `InTenantTx(A)`, devuelve 0 filas (el filtro y la RLS se combinan, no se contradicen).

**T-B805 — Correcciones** que surjan de T-B801..T-B804. Toda falla es un bug crítico (principio
III): se corrige antes de cerrar la fase, con su test de regresión.

**Checkpoint Fase 8**: `make check` en verde; el test T-B801 imprime el conteo de rutas cubiertas
(= total de rutas) y 0 accesos cruzados.

---

### Fase 9 — Robustez y operación

**Objetivo**: el sistema se puede operar: limpia lo vencido, expone salud y métricas, se
restaura sin perder los roles, y su rendimiento con muchas empresas (y con registros
concurrentes) está medido.

**Prueba independiente**: con 10.000 empresas cargadas, los targets de `plan.md` §13 se cumplen;
borrar un rol de empresa y correr `crm tenants reprovision-roles` restablece el acceso.

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
  `signup_lock_timeout_total`, `http_client_canceled_total` y `set_role_retry_total`);
  `tenant_roles_total` = cantidad de empresas, leída como `crm_worker` con la query de
  `internal/tenant/store/provisioning.sql`.

**T-B904 — Implementar `/readyz` y `expvar`**.

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
  apagado no hay logs `ERROR` por cancelaciones.

**T-B909 — Implementar el apagado ordenado del servidor y del worker**.

**Checkpoint Fase 9**: `make check` en verde + resultado de T-B905 reportado con los números.

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
| FR-008 Auditoría (y reactivación, P-3) | T-B209, T-B402, T-B501, T-B601, T-B603, T-B604 |
| US-1 (1, 2, 3) | T-B303 (1, 2), T-B305 (2), T-B303/T-B504 (3) |
| US-2 (1, 2, 3) | T-B402 (1, 2), T-B501/T-B502 (3) |
| US-3 (1, 2, 3, 4) | T-B601 (1), T-B602 (2), T-B604 (3), T-B603/T-B604 (4) |
| US-4 (1) | T-B702, T-B703, T-B705 |
| Casos borde (404 de otra empresa, 403 de operador) | T-B606, T-B801, T-B802 |
| SC-001 Panel en < 3 min | T-B305 (un request), T-B905 (latencia de signup, también con registros concurrentes) |
| SC-002 0 accesos cruzados | T-B110, T-B801..T-B804 |
| P-2 Verificación no bloqueante | T-B309, T-B504 |
| P-3 Reactivación | T-B604, T-B605, T-B606, T-B801 |
| P-4 `email_already_registered` + login/reset no enumerables | T-B201, T-B305, T-B402, T-B404, T-B501, T-B506, T-B801 |
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
| JPEG re-codificado en el navegador (DD-F21): el backend no depende de eso (DD-11, INV-24) | T-B703 |
| Rutas de la SPA en inglés (DD-14) | T-B002, T-B213 |
| `405` con `method_not_allowed` y `Allow` (contrato v0.4.0, research R-26) | T-B004, T-B005, T-B201, T-B203; T-F004, T-F101 (frontend) |
| DD-32 / INV-25 IP del cliente detrás de proxies (research R-25) | T-B002, T-B004, T-B011, T-B014, T-B203, T-B204, T-B217, T-B218, T-B219, T-B220 |
| Cancelaciones: `db.ErrCanceled`, `499` en el log, worker sin intento (research R-27) | T-B103, T-B105, T-B106, T-B202, T-B203, T-B211, T-B901, T-B908 |
| DD-33 / INV-26 Lock de `GRANT crm_tenant` (R-a..R-d; research R-04c; nota en ADR-005) | T-B105 (`55P03`), T-B201, T-B303, T-B304, T-B305, T-B306, T-B903, T-B905, T-B906, T-B907 |
| DD-34 / INV-27 / R-17 Primer `SET ROLE` desde otra conexión: lectura de catálogo y reintento único (research R-28; nota (b) en ADR-005) | T-B103, T-B104, T-B903, T-B905 |
| Rutas exactas de las queries de sistema (plan §4.4, INV-04, ADR-001) | T-B112, T-B212, T-B304, T-B902, T-B903, T-B907 |
| FK `tenant_id → tenants(id)` de `sessions` y `user_tokens` (`data-model.md` §2.3/§2.4) | T-B108, T-B111 |

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
