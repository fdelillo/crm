# Tasks: Empresas, usuarios y roles

**Spec**: [`spec.md`](spec.md) · **Plan**: [`plan.md`](plan.md) · **Modelo**: [`data-model.md`](data-model.md)
· **Contrato**: [`contracts/openapi.yaml`](contracts/openapi.yaml)

---

## Backend

Autor: `backend-architect`. Implementa: `backend-developer`, **una fase por invocación**.

### Convenciones de esta sección

- `[T]` = tarea de test. Va **antes** de la tarea de código que la hace pasar, y el test debe
  fallar por la razón correcta antes de implementar (Red).
- Cada tarea lleva trazabilidad: historia (`US-n` = Historia n de la spec), `FR-`, `SC-`, `INV-`
  (plan §5), `DD-` (plan §6), `ADR-`.
- Tests: paquete `testing` nativo, *table-driven*, sin testify (ADR-012). Unitarios junto al
  código (`*_test.go`). Integración con build tag `integration` (`//go:build integration`) y
  PostgreSQL 18 real vía `internal/testsupport/pgtest`.
- **Qué se sustituye y qué no** (ADR-012):

  | Se sustituye (fake en memoria) | Nunca se sustituye |
  |---|---|
  | `mailer.Mailer` en tests de servicio y HTTP | PostgreSQL (RLS, roles, constraints son lo que se prueba) |
  | `objectstore.ObjectStorage` en tests de servicio y HTTP | El adaptador SMTP en su propio test (contra Mailpit) |
  | `clock.Clock` (tiempo controlado) | El adaptador S3 en su propio test (contra MinIO) |
  | `industrytemplate.Seeder` solo en el test de rollback (para forzar un fallo) | El router y los middlewares en los tests HTTP (`httptest` sobre el handler real) |

- Los tests de integración se conectan **como `crm_app`** (INV-18). Cada test crea sus propias
  empresas con UUID y emails aleatorios: pueden correr en paralelo (`t.Parallel()`) sobre el mismo
  contenedor sin limpiar tablas.
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

---

### Fase 0 — Esqueleto del monolito (Setup)

**Objetivo**: el binario compila, sirve `/healthz`, corre migraciones embebidas, genera código
sqlc y hay un harness que levanta PostgreSQL 18 real con los roles del proyecto. CI corre todo.

**Prueba independiente**: `make check` en verde en CI; `docker compose up` + `crm migrate up` +
`crm serve` y `curl /healthz` responde `200 {"status":"ok"}`.

**T-B001 — Módulo Go y estructura de directorios** · ADR-001
- `go mod init github.com/fdelillo/crm` con Go 1.27. Árbol de `plan.md` §11 con paquetes vacíos
  (`doc.go` que explique la responsabilidad de cada uno en una línea).
- `cmd/crm` con subcomandos `serve`, `migrate {up|down|status}`, `tenants reprovision-roles`
  (este último vacío hasta la Fase 9), usando `flag` de la librería estándar.

**T-B002 [T] — Configuración** · ADR-001, plan §10.5
- **Red**:

  | Entrada (env) | Resultado esperado |
  |---|---|
  | Todas las variables obligatorias válidas | `Config` poblado; defaults: `HTTP_ADDR=:8080`, `METRICS_ADDR=127.0.0.1:9090`, `SESSION_IDLE=168h`, `SESSION_ABSOLUTE=720h` |
  | Falta `DATABASE_URL` | error que nombra la variable |
  | `AUTH_HMAC_KEY` de menos de 32 bytes (decodificada) | error |
  | `APP_BASE_URL` sin esquema `https://` y `COOKIE_SECURE` ≠ `false` | error |
  | `SESSION_IDLE` > `SESSION_ABSOLUTE` | error |
  | Duración mal formada | error que nombra la variable |
  | El error nunca incluye el **valor** de una variable secreta | aserción sobre el texto |
- **Green**: `config.Load(getenv func(string) string) (Config, error)` pasa la tabla.
- **Refactor**: una sola función de validación por campo; nada de variables globales.

**T-B003 — Implementar `platform/config`**.

**T-B004 [T] — Servidor HTTP mínimo** · ADR-002, DD-12
- **Red**:

  | Caso | Esperado |
  |---|---|
  | `GET /healthz` | `200`, `application/json`, `{"status":"ok"}` |
  | Ruta inexistente | `404` `application/problem+json` con `code=not_found` |
  | Método no permitido | `405` problem+json |
  | Un handler que hace *panic* | `500` problem+json `code=internal`, el proceso sigue vivo, log `ERROR` con `request_id` |
  | Toda respuesta | header `X-Request-Id`; `X-Content-Type-Options: nosniff` |
- **Green**: `app.NewRouter(deps) http.Handler` con chi; timeouts de §10.3 del plan; apagado
  ordenado con `context` al recibir SIGTERM.

**T-B005 — Implementar router raíz, logging `slog` JSON y `serve`**.

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

**T-B011 [T] — Reglas de repositorio** · INV-03, INV-04, ADR-001
- **Red** (test Go que recorre el árbol de fuentes):

  | Regla | Caso que debe fallar |
  |---|---|
  | `SET ROLE`, `SET LOCAL ROLE`, `RESET ROLE` o `set_config('role'` solo en `internal/platform/db` | Archivo de fixture en `testdata/` con `SET ROLE` en otro paquete → detectado |
  | Ningún `fmt.Sprintf`/concatenación que arme SQL fuera de `platform/db` | Fixture con `"SELECT " + x` → detectado |
- `.golangci.yml` con `depguard`: reglas de `plan.md` §4.1 (platform no importa dominio;
  `identity` no importa `tenant`; nadie importa `*/store` de otro módulo).
- **Green**: el test pasa sobre el código real y detecta los fixtures.

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
  `make check`. Cachés de módulos.

**Checkpoint Fase 0**: `make check` en verde local y en CI; resultados de los spikes T-B008 y
T-B012 reportados. **No avanzar a la Fase 1 si T-B012 falla.**

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

**T-B103 [T] — `TxRunner` y `Tx`** · INV-02, INV-03, ADR-005
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
- **Green**: `db.NewTxRunner(pool)` pasa la tabla.
- **Refactor**: `SET LOCAL ROLE` con `pgx.Identifier{...}.Sanitize()`; una sola función privada
  que cambia de rol.

**T-B104 — Implementar `platform/db` (`TxRunner`, `Tx`, `TenantRoleName`)**.

**T-B105 [T] — Mapeo de errores de PostgreSQL** · INV-19, plan §9.2
- **Red** (unitario, con `*pgconn.PgError` construidos a mano):

  | Error de entrada | Salida |
  |---|---|
  | `pgx.ErrNoRows` | `errors.Is(err, db.ErrNotFound)` |
  | SQLSTATE `23505`, constraint `users_email_key` | `db.ErrUniqueViolation` y `*db.ConstraintError{Constraint:"users_email_key"}` |
  | SQLSTATE `42501` | `db.ErrPrivilege` (nunca `ErrNotFound`) |
  | Error de conexión, `57014` (statement timeout), `context.DeadlineExceeded` | `db.ErrUnavailable` |
  | Otro SQLSTATE | se devuelve envuelto sin clasificar |
  | En la capa HTTP: `db.ErrPrivilege` | `500 internal` + log `ERROR` con `security_event=rls_violation` |
  | En la capa HTTP: `db.ErrUnavailable` | `503 service_unavailable` |
- **Green**: `db.MapError(error) error` y el mapeo por defecto de `httpx` pasan la tabla.

**T-B106 — Implementar `db.MapError`** (el mapeo HTTP por defecto va en T-B202).

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
  de `A` apuntando a un usuario de `B` → violación de FK.
- **Green**: pasa tras T-B111.

**T-B109 [T] — Privilegios de los roles de sistema** · INV-05, plan §4.4
- **Red**: *snapshot* de `information_schema.column_privileges` y `table_privileges` para
  `crm_auth`, `crm_worker`, `crm_signup` comparado con la tabla de `data-model.md` §3.4 declarada
  en el test. Además, como `crm_auth`: `SELECT password_hash FROM app.users` → `42501`; como
  `crm_worker`: `SELECT payload FROM app.outbox_messages` → `42501`.
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
  en `data-model.md`. Cada una con `Down`.

**T-B112 [T] — Queries con filtro explícito de empresa** · INV-04
- **Red**: test que parsea cada `internal/*/store/*.sql` y, para toda query que referencie una
  tabla de empresa, exige un predicado `tenant_id = @tenant_id` (o `id = @tenant_id` en
  `tenants`). Excepciones solo por **lista explícita** (`auth_lookup.sql` y las del worker), cada
  una con su motivo. Fixture en `testdata/` sin filtro → detectado.
- **Green**: el detector funciona sobre el fixture; se vuelve efectivo a medida que las fases
  siguientes agregan queries.

**Checkpoint Fase 1**: `make check` en verde. `go test -tags=integration -run
'Catalog|Isolation|TxRunner|Provision' ./internal/platform/db/... ./db/...` en verde.

---

### Fase 2 — Fundacional B: plataforma HTTP y servicios transversales

**Objetivo**: todo lo que las historias usan y no es de ningún dominio: problem+json,
middlewares de seguridad, tokens, hashing, autorización, auditoría, outbox con worker, email y
almacenamiento de objetos.

**Prueba independiente**: un mensaje encolado en una transacción de empresa llega a Mailpit;
un `POST` de origen cruzado es rechazado; la matriz de permisos coincide con FR-007.

**T-B201 [T] — problem+json y decodificación de JSON** · ADR-009, plan §9
- **Red** (unitario):

  | Caso | Esperado |
  |---|---|
  | `WriteProblem(w, r, code)` para cada `code` de §9.1 | status correcto, `Content-Type: application/problem+json`, `type=/problems/{code}`, `instance`=request id, `title` en español |
  | `ValidationError` con dos campos | `422`, `errors` con `{field, code}` en orden estable |
  | `DecodeJSON` con JSON válido | struct poblado |
  | Campo desconocido | `400 malformed_request` (`DisallowUnknownFields`) |
  | Dos objetos JSON concatenados | `400` |
  | Body > 64 KB | `413 payload_too_large` |
  | Sin `Content-Type: application/json` | `415` |
  | `Content-Type: application/json; charset=utf-8` | aceptado |
- **Green**: pasa la tabla; las respuestas validan contra `Problem`/`ValidationProblem` del
  contrato.

**T-B202 — Implementar `platform/httpx` (problem+json, DecodeJSON, mapeo por defecto de errores)**.

**T-B203 [T] — Middlewares de seguridad y observabilidad** · ADR-006, plan §10.2–10.3, DD-12
- **Red** (unitario, `httptest`):

  | Caso | Esperado |
  |---|---|
  | `POST` con `Sec-Fetch-Site: cross-site` | `403` (CrossOriginProtection) |
  | `POST` con `Origin` de otro host y sin `Sec-Fetch-Site` | `403` |
  | `POST` con `Sec-Fetch-Site: same-origin` | pasa |
  | `GET` con `Sec-Fetch-Site: cross-site` | pasa (método seguro) |
  | Respuesta de `/api/v1/auth/*` y `/api/v1/me` | `Cache-Control: no-store` |
  | Toda respuesta | `Strict-Transport-Security`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer` |
  | Log de un request con body `{"password": "..."}` y cookie de sesión | el log no contiene la contraseña, el token ni el header `Cookie` |
  | Log de request | contiene `request_id`, `route` (patrón chi), `status`, `duration_ms` |
- **Green**: pasa la tabla.

**T-B204 — Implementar middlewares** (orden: request id → recover → logging → security headers →
CrossOriginProtection → rate limit por ruta → autenticación por grupo).

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
- **Green**: pasa la tabla. **Refactor**: la política de backoff es una función pura con su propio
  test unitario.

**T-B212 — Implementar `platform/outbox` (`Enqueue`, `Dispatcher` con *polling* de 2 s y lote de 10, arranque y parada con `context`)**.

**T-B213 [T] — Adaptador SMTP y plantillas** · ADR-010
- **Red** (integración contra Mailpit en contenedor; su API HTTP para leer lo recibido):

  | Caso | Esperado |
  |---|---|
  | Enviar `password_reset` con `{link}` | llega a Mailpit con asunto en español, partes texto y HTML, enlace con el token en el **fragmento** (DD-14) |
  | Plantillas `email_verification` e `invitation` (con nombre de empresa y rol en español: "Administrador"/"Operador") | renderizan sin campos vacíos |
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

**T-B217 [T] — Rate limiter** · DD-9
- **Red** (unitario, reloj falso): 5 pedidos permitidos y el 6.º rechazado con `RetryAfter > 0`;
  claves independientes; recarga con el tiempo; limpieza de claves inactivas (sin crecer sin
  límite).
- **Green**: pasa la tabla.

**T-B218 — Implementar `platform/ratelimit` y el middleware por ruta** (`429 rate_limited` +
`Retry-After`).

**Checkpoint Fase 2**: `make check` en verde.

---

### Fase 3 — Historia 1: Registrar una empresa (P1) — primer slice vertical

**Objetivo**: un visitante se registra, queda logueado y ve su sesión en `/me`; el email de
verificación llega a Mailpit.

**Prueba independiente**: `POST /api/v1/auth/signup` → `201` + cookie → `GET /api/v1/me` con esa
cookie → `200` con rol `admin` y los 15 permisos; en Mailpit hay un email de verificación.

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

**T-B303 [T] — Servicio de registro** · US-1, FR-001, FR-002, INV-14, INV-16, DD-2, DD-4, DD-13, DD-15
- **Red** (integración, `Mailer` real no interviene: se verifica el outbox):

  | Caso | Esperado |
  |---|---|
  | Datos válidos | Una empresa con `base_currency`, `timezone` (del request o default), `industry_template_code/version`; rol `crm_t_<hex>` creado; un usuario `admin`/`active` con email normalizado y hash argon2id; un token `email_verification` (48 h); un mensaje `email_verification` pendiente; una sesión; auditoría `tenant.registered` |
  | Email con mayúsculas y espacios | se guarda en minúsculas y sin espacios |
  | Email ya existente (en cualquier empresa) | `identity.ErrEmailTaken`; **no** queda empresa, usuario, token, mensaje, sesión ni **rol** nuevos |
  | Dos registros concurrentes con el mismo email | exactamente uno tiene éxito; el otro `ErrEmailTaken` |
  | Plantilla inexistente | error de validación `unknown_template`; nada creado |
  | `Seeder` falso que falla | error; nada creado (incluido el rol) |
  | Zona horaria inválida | validación `invalid_timezone` |
  | `base_currency` fuera de ARS/USD | validación `invalid_value` |
  | Contraseña de 9 caracteres o igual al email | validación; **no** se calcula hash ni se abre transacción |
- **Green**: pasa la tabla.
- **Refactor**: el servicio de `tenant` no conoce SQL de `identity`; solo usa la interfaz
  `AdminOnboarding` (plan §11.1).

**T-B304 — Implementar `tenant.Service.Register` y en `identity`: `CreateFirstAdmin`,
`CreateSession`, `IssueEmailVerification`**.

**T-B305 [T] — `POST /auth/signup`** · US-1, SC-001
- **Red** (integración HTTP, contrato validado en cada respuesta):

  | Caso | Esperado |
  |---|---|
  | Payload válido | `201` `SessionInfo`; `Set-Cookie: __Host-crm_session=…; Path=/; HttpOnly; Secure; SameSite=Lax; Max-Age=2592000` |
  | Email existente | `409 email_taken`; el cuerpo no contiene el nombre de la otra empresa ni datos del usuario existente |
  | Campo faltante / campo extra | `422` con `errors[].field` / `400 malformed_request` |
  | Sin `Content-Type` JSON | `415` |
  | 6.º registro en una hora desde la misma IP | `429 rate_limited` con `Retry-After` |
- **Green**: pasa la tabla.

**T-B306 — Implementar el handler de signup**.

**T-B307 [T] — Resolución de sesión (`identity.Authenticate`)** · ADR-006, INV-11, DD-10
- **Red** (integración, `Clock` falso):

  | Caso | Esperado |
  |---|---|
  | Cookie válida | `Principal{TenantID, UserID, SessionID, Role}` en el contexto |
  | Sin cookie en ruta protegida | `401 unauthenticated` |
  | Token desconocido | `401` + cookie borrada |
  | Sesión revocada | `401` |
  | Pasados 30 días desde el login (aun con uso diario) | `401` |
  | 7 días sin uso | `401` |
  | Uso dentro de los 5 min de `last_seen_at` | no escribe en la base |
  | Uso pasados 5 min | actualiza `last_seen_at` |
  | Usuario `disabled` con sesión no revocada (fixture) | `401` (doble verificación) |
  | Rol cambiado en la base | el siguiente request trae el rol nuevo |
  | La base guarda solo el hash del token | `SELECT` como dueño: ningún valor de `token_hash` coincide con el raw |
- **Green**: pasa la tabla.

**T-B308 — Implementar `Authenticate` y `ResolveSession`**.

**T-B309 [T] — `GET /me`** · US-1, FR-007
- **Red**: admin → `200` con 15 permisos; operador → 6 permisos; sin cookie → `401`. Validado
  contra el contrato.

**T-B310 — Implementar `/me` y el cableado en `internal/app`** (grupos de rutas públicas y
autenticadas, worker arrancado por `serve`).

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

**T-B402 [T] — Servicio de login** · US-2.1, US-2.2, FR-003, FR-008, INV-09, INV-13, DD-8
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | Email y contraseña correctos, usuario activo | sesión creada; auditoría `auth.login_succeeded`; `login_throttles` sin fila para ese email |
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

**T-B404 [T] — `POST /auth/login` y `POST /auth/logout`** · US-2
- **Red** (HTTP + contrato):

  | Caso | Esperado |
  |---|---|
  | Login correcto | `200` `SessionInfo` + cookie |
  | Incorrecto / inexistente | `401 invalid_credentials`, **cuerpos idénticos byte a byte** salvo `instance` |
  | Bloqueado | `429 login_locked` + `Retry-After` en segundos |
  | Desactivado con contraseña correcta | `403 account_disabled` |
  | Logout con sesión | `204`, cookie borrada, la cookie anterior da `401` en `/me`; auditoría `auth.logout` |
  | Logout sin cookie o con cookie inválida | `204` |
  | 21.º login en un minuto desde la misma IP | `429 rate_limited` |
- **Green**: pasa la tabla.

**T-B405 — Implementar handlers de login y logout**.

**Checkpoint Fase 4**: `make check` en verde.

---

### Fase 5 — Historia 2: Recuperar contraseña y verificar email (P1)

**Objetivo**: reset por email con enlace de un solo uso de 1 h; verificación de email.

**Prueba independiente**: pedir reset → enlace en Mailpit → nueva contraseña → las sesiones
anteriores dan `401` y la nueva contraseña entra.

**T-B501 [T] — Pedido de reset** · US-2.3, FR-004, INV-13, S-9
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | Usuario activo | token `password_reset` (vence en 1 h) + mensaje pendiente con enlace; auditoría `auth.password_reset_requested` |
  | Segundo pedido | el token anterior queda revocado; hay uno solo vigente |
  | Email inexistente, usuario `invited` o `disabled` | no se crea nada; **mismo** resultado (`nil`) |
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

**T-B503 — Implementar `RequestPasswordReset` y `ConfirmPasswordReset`**.

**T-B504 [T] — Verificación de email** · US-1.3, DD-13
- **Red**: confirmar con token válido → `email_verified_at` puesto y auditoría; repetir con el
  mismo token → `ErrTokenInvalid`; token vencido (48 h) → `ErrTokenInvalid`; `resend` con email
  ya verificado → no crea nada; `resend` sin verificar → revoca el anterior y crea uno nuevo.

**T-B505 — Implementar verificación y reenvío**.

**T-B506 [T] — Endpoints de reset y verificación** · contrato
- **Red**: `POST /auth/password-reset` → `202` sin cuerpo para email existente e inexistente
  (respuestas idénticas); `confirm` → `204` / `400 token_invalid` / `422`;
  `email-verification/confirm` → `204` / `400`; `resend` → `202` / `401` sin cookie. Todos
  validados contra el contrato.

**T-B507 — Implementar los handlers**.

**Checkpoint Fase 5**: `make check` en verde + prueba independiente contra `docker compose`.

---

### Fase 6 — Historia 3: Invitar operadores y administrar usuarios (P2)

**Objetivo**: invitar, reinvitar, aceptar, cambiar rol, desactivar (cerrando sesiones) y
reactivar (P-3), sin dejar nunca la empresa sin Administrador.

**Prueba independiente**: el admin invita a un operador; el operador acepta y entra; el operador
recibe `403` en `/users`; el admin lo desactiva y el siguiente request del operador da `401`.

**T-B601 [T] — Invitar y reinvitar** · US-3.1, FR-005, FR-008, INV-16, DD-1, DD-5
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | Email nuevo, rol `operator` | usuario `invited`, `name` y `password_hash` nulos; token `invitation` de 7 días con `created_by_user_id`; mensaje `invitation` pendiente; auditoría `user.invited` |
  | Mismo email ya `invited` en la misma empresa | token anterior revocado, token nuevo; `reissued = true`; auditoría `user.invitation_reissued`; sigue habiendo **un** usuario |
  | Email de un usuario activo o desactivado de esta empresa | `ErrEmailTaken` |
  | Email de un usuario (cualquier estado) de **otra** empresa | `ErrEmailTaken` (sin datos de la otra empresa en el error) |
  | Rol `admin` | permitido |
  | Email inválido | validación |
- **Green**: pasa la tabla.

**T-B602 [T] — Ver y aceptar invitación** · US-3.2, DD-1, DD-4
- **Red** (integración, `Clock` falso):

  | Caso | Esperado |
  |---|---|
  | `Preview` con token vigente | nombre de la empresa, email, rol, vencimiento |
  | `Accept` válido | usuario `active` con nombre y hash; `email_verified_at` puesto; token usado; sesión creada; auditoría `user.invitation_accepted` |
  | Token vencido (7 días + 1 s), reemplazado por reinvitación, o ya usado | `ErrTokenInvalid` (en `Preview` y en `Accept`) |
  | Usuario desactivado antes de aceptar | `ErrTokenInvalid` |
  | Contraseña inválida | validación; token no consumido |
  | Dos `Accept` concurrentes | uno solo tiene éxito |
- **Green**: pasa la tabla.

**T-B603 [T] — Cambio de rol y último Administrador** · US-3.4, FR-005, FR-008, INV-10
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | Operador → admin | rol cambiado; auditoría `user.role_changed {from: operator, to: admin}` |
  | Admin → operador habiendo otro admin activo | permitido |
  | Único admin activo → operador (incluido sobre sí mismo) | `ErrLastAdmin`; sin cambios |
  | Dos admins activos + uno desactivado; bajar a uno de los activos | permitido; bajar al otro después → `ErrLastAdmin` (el desactivado no cuenta) |
  | Mismo rol que ya tiene | sin cambios, sin auditoría, sin error |
  | Id de otra empresa o inexistente | `ErrUserNotFound` |
  | **Concurrencia**: empresa con 2 admins; cada uno baja al otro a la vez (20 repeticiones) | en todas, exactamente una operación tiene éxito y la otra `ErrLastAdmin`; siempre queda ≥ 1 admin activo |
- **Green**: pasa la tabla.

**T-B604 [T] — Desactivar y reactivar** · US-3.3, US-3.4, INV-10, INV-11, P-3
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
  | Reactivar un `disabled` con contraseña | `active`; auditoría `user.reactivated {to_status: active}`; sus sesiones viejas **siguen** revocadas |
  | Reactivar un `disabled` sin contraseña | `invited` con invitación nueva de 7 días y mensaje encolado |
  | Reactivar un `active` o `invited` | `ErrInvalidTransition` |
- **Green**: pasa la tabla.

**T-B605 — Implementar `Invite`, `PreviewInvitation`, `AcceptInvitation`, `ListUsers`,
`ChangeRole`, `Deactivate`, `Reactivate`**. Las transiciones de estado se expresan como una
tabla (`estado actual × acción → estado nuevo | error`) con su test unitario, y cada operación de
cambio de rol o estado empieza con el lock de la fila `tenants` (INV-10).

**T-B606 [T] — Endpoints de usuarios e invitaciones** · contrato, FR-007
- **Red** (HTTP + contrato):

  | Caso | Esperado |
  |---|---|
  | Admin: `GET /users` | `200` con invitados (con `invitation_expires_at`), activos y desactivados, en orden de alta |
  | Operador en `GET /users`, `POST /users/invitations`, `PUT /users/{id}/role`, `POST .../deactivate`, `POST .../reactivate` | `403 forbidden` en todos |
  | Admin, `userId` inexistente o de otra empresa | `404 not_found` (idénticos) |
  | `userId` que no es UUID | `404` (no `400`: no revela formato de ids) |
  | Invitar nuevo / reinvitar | `201` / `200` |
  | `409 last_admin`, `409 invalid_state`, `409 email_taken` | según T-B601..T-B604 |
  | `POST /auth/invitations/preview` y `accept` | `200` / `201` + cookie / `400 token_invalid` |
- **Green**: pasa la tabla.

**T-B607 — Implementar los handlers**.

**Checkpoint Fase 6**: `make check` en verde + prueba independiente.

---

### Fase 7 — Historia 4: Datos de la empresa y logo (P2)

**Objetivo**: el admin edita los datos de la empresa y sube el logo; cualquier usuario los ve.

**Prueba independiente**: el admin sube un PNG y edita el CUIT; el operador ve el logo en
`GET /tenant/logo` y recibe `403` al intentar `PATCH /tenant`.

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

**T-B702 [T] — Actualización de datos** · US-4, FR-008
- **Red** (integración): `PATCH` parcial solo cambia los campos presentes; `null` borra un campo
  opcional; `updated_at` avanza; auditoría `tenant.updated` con la **lista de nombres** de campos
  (no los valores); `base_currency` en el payload → `400 malformed_request` (campo desconocido).

**T-B703 [T] — Logo** · US-4, DD-11, INV-17
- **Red** (integración con `ObjectStorage` falso instrumentado):

  | Caso | Esperado |
  |---|---|
  | PNG válido de 1 MB | objeto guardado con clave `tenants/{A}/logo/{uuid}.png`; `logo_object_key` y `logo_content_type` actualizados; auditoría `tenant.logo_updated` |
  | Reemplazar logo | objeto nuevo guardado **antes** del `COMMIT`; el viejo borrado **después** |
  | JPEG válido | aceptado |
  | GIF, WebP, SVG (aunque la extensión o el `Content-Type` digan `png`) | `ErrLogoUnsupportedType` |
  | 2 MB + 1 byte | `ErrLogoTooLarge` (el body se corta: no se lee entero en memoria) |
  | PNG de 4000×10 px o con encabezado que declara 50000×50000 | `ErrLogoInvalidImage` (sin decodificar la imagen entera) |
  | `Put` falla | `503`; la fila de la empresa sin cambios |
  | La transacción falla después del `Put` | el objeto nuevo se borra (mejor esfuerzo, logueado si falla) |
  | Borrar el objeto viejo falla | la operación igual responde éxito; log `WARN` (objeto huérfano aceptado) |
  | `RemoveLogo` | columnas en `NULL`, objeto borrado, auditoría `tenant.logo_removed`; repetir → sin error |
- **Green**: pasa la tabla.

**T-B704 — Implementar `tenant.Service.Update`, `SetLogo`, `RemoveLogo`, `GetLogo`**.

**T-B705 [T] — Endpoints de empresa** · contrato, FR-007
- **Red** (HTTP + contrato): `GET /tenant` → `200` para admin y operador; `PATCH /tenant` → `200`
  admin, `403` operador, `422` CUIT inválido; `PUT /tenant/logo` multipart → `200` / `413` /
  `415` / `403`; `GET /tenant/logo` → `200` con `Content-Type` exacto, `nosniff`,
  `Cache-Control: private, max-age=300`, o `404` sin logo; `DELETE /tenant/logo` → `204`.

**T-B706 — Implementar los handlers**.

**Checkpoint Fase 7**: `make check` en verde + prueba independiente con MinIO de `docker compose`.

---

### Fase 8 — Aislamiento entre empresas de punta a punta (SC-002, principio III)

**Objetivo**: demostrar con tests automáticos **0 accesos cruzados en todos los endpoints**, y que
el test falle solo si mañana se agrega un endpoint sin cubrir.

**Prueba independiente**: `go test -tags=integration -run Isolation ./...` en verde, con el
reporte de cobertura de rutas al 100 %.

**Fixture común** (`internal/testsupport/fixture`): empresas `A` y `B`, cada una con un admin, un
operador, un invitado, un desactivado, logo y datos completos; cookies de sesión de cada usuario;
tokens vigentes de reset, verificación e invitación de cada empresa.

**T-B801 [T] — Matriz HTTP de aislamiento con cobertura de rutas** · SC-002, FR-006, INV-12
- **Red**:
  - El test obtiene **todas** las rutas registradas con `chi.Walk` y las compara con una tabla
    declarada `ruta → caso de aislamiento`. **Si existe una ruta sin caso, el test falla**
    (así se protege a las specs futuras).
  - Casos por ruta (como admin de `A`):

    | Tipo de ruta | Ataque | Esperado |
    |---|---|---|
    | Con `{userId}` | usar ids de usuarios de `B` (cada estado) | `404`, cuerpo idéntico al de un id inexistente; `B` sin cambios (verificado como `B`) |
    | Sin id, lectura (`/me`, `/tenant`, `/users`, `/tenant/logo`) | — | solo datos de `A`: ningún id, email o nombre de `B` aparece en el cuerpo |
    | Sin id, escritura (`PATCH /tenant`, `PUT/DELETE /tenant/logo`, `POST /users/invitations`) | — | modifica solo `A`; `B` sin cambios; la clave del logo empieza con `tenants/{A}/` |
    | Públicas con token (`/auth/*`) | token de `B` usado junto con la cookie de `A` | la operación afecta solo a `B` (la cookie no cambia la empresa del token); ninguna fila de `A` cambia |
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
restaura sin perder los roles, y su rendimiento con muchas empresas está medido.

**Prueba independiente**: con 10.000 empresas cargadas, los targets de `plan.md` §13 se cumplen;
borrar un rol de empresa y correr `crm tenants reprovision-roles` restablece el acceso.

**T-B901 [T] — Limpieza periódica** · `data-model.md` §3.4
- **Red** (integración): sesiones y tokens vencidos hace > 30 días, mensajes terminales de > 30
  días y `login_throttles` de > 24 h sin bloqueo vigente se borran **como `crm_worker`**; nada
  vigente o reciente se borra; un `DELETE` de `crm_worker` sobre una sesión vigente afecta 0 filas
  (la política lo impide aunque el código se equivoque).

**T-B902 — Implementar la limpieza** (en el mismo `Dispatcher`, una vez por hora).

**T-B903 [T] — `/readyz` y métricas** · plan §12
- **Red**: base caída → `503 {"status":"unavailable"}`; versión de migración de la base ≠ la
  embebida → `503`; ok → `200`; `/debug/vars` solo escucha en `METRICS_ADDR` y expone las
  métricas de §12.1; `tenant_roles_total` = cantidad de empresas.

**T-B904 — Implementar `/readyz` y `expvar`**.

**T-B905 — Benchmark con 10.000 empresas** · R-2, R-3, ADR-005
- Script de carga (test con build tag `bench`, fuera de `make check`) que aprovisiona 10.000
  empresas en un contenedor con la versión exacta de producción y mide p95 de: `SET LOCAL ROLE`,
  `GET /me`, `POST /auth/signup` y el tiempo de conexión de `crm_app`. Comparar con `plan.md` §13.
  **Si algún target no se cumple, frenar y volver al arquitecto** (se reabre ADR-005).

**T-B906 [T] — Reaprovisionamiento de roles** · plan §12.4
- **Red**: con dos empresas, se borra el rol de `A` (como superusuario del contenedor); los
  requests de `A` fallan con `500` y log que nombra el rol; `crm tenants reprovision-roles`
  recrea el rol y sus membresías; los requests de `A` vuelven a funcionar; correrlo de nuevo no
  cambia nada; `B` nunca se ve afectada.

**T-B907 — Implementar `crm tenants reprovision-roles`** (lista `tenants.id` como `crm_worker` y
llama a la función como `crm_signup`).

**T-B908 [T] — Apagado ordenado**
- **Red**: SIGTERM con un request en curso y un mensaje en envío → el request termina, el mensaje
  queda `sent` o `pending` (nunca a medio actualizar), el proceso sale con código 0 antes del
  timeout de apagado.

**T-B909 — Implementar el apagado ordenado del servidor y del worker**.

**Checkpoint Fase 9**: `make check` en verde + resultado de T-B905 reportado con los números.

---

### Trazabilidad

| Requisito / criterio | Tareas |
|---|---|
| FR-001 Registro autónomo | T-B303, T-B305 |
| FR-002 Plantillas | T-B301, T-B303 |
| FR-003 Email + contraseña, hash robusto | T-B205, T-B402, T-B404 |
| FR-004 Recuperar contraseña | T-B501, T-B502, T-B506 |
| FR-005 Invitar, desactivar, cambiar rol | T-B601..T-B606 |
| FR-006 Aislamiento | T-B101, T-B103, T-B107..T-B112, T-B801..T-B804 |
| FR-007 Matriz de permisos | T-B207, T-B309, T-B606, T-B705, T-B802 |
| FR-008 Auditoría | T-B209, T-B402, T-B601, T-B603 |
| US-1 (1, 2, 3) | T-B303 (1, 2), T-B305 (2), T-B303/T-B504 (3) |
| US-2 (1, 2, 3) | T-B402 (1, 2), T-B501/T-B502 (3) |
| US-3 (1, 2, 3, 4) | T-B601 (1), T-B602 (2), T-B604 (3), T-B603/T-B604 (4) |
| US-4 (1) | T-B702, T-B703 |
| Casos borde (404 de otra empresa, 403 de operador) | T-B606, T-B801, T-B802 |
| SC-001 Panel en < 3 min | T-B305 (un request), T-B905 (latencia de signup) |
| SC-002 0 accesos cruzados | T-B110, T-B801..T-B804 |

---

## Frontend

> **Pendiente: `frontend-architect`.** Esta sección la escribe el arquitecto de frontend a partir
> de `ui.md`. Insumos del backend que necesita: `contracts/openapi.yaml` (canónico), los `code`
> de error de `plan.md` §9.1, la lista `permissions` de `/me` (solo para UX; la autorización es
> del servidor) y los enlaces de email con el token en el fragmento (DD-14), cuyos *paths* debe
> fijar `ui.md`.
