# Tasks: Configuración por empresa

**Spec**: [`spec.md`](spec.md) · **Plan**: [`plan.md`](plan.md) · **Modelo**: [`data-model.md`](data-model.md)
· **Contrato**: [`contracts/openapi.yaml`](contracts/openapi.yaml)
**Estado**: Propuesto (2026-10-23). **No se implementa nada** hasta que el usuario apruebe
`spec.md` (P-1 a P-10) y el plan. Las tareas asumen las opciones recomendadas.

---

## Backend

Autor: `backend-architect`. Implementa: `backend-developer`, **una fase por invocación**, en modo
completo (`.claude/dev-mode`).

### Estado de la implementación (2026-10-23)

Ninguna fase empezada. Bloqueado por la aprobación de la spec y del plan.

### Convenciones de esta sección

Rigen las de la sección Backend de [`tasks.md` de 001](../001-empresas-usuarios/tasks.md)
("Convenciones de esta sección"): `[T]` antes de su tarea de código y fallando por la razón correcta;
`testing` nativo, *table-driven*, sin testify; integración con `//go:build integration`, PostgreSQL
real y conexión como `crm_app`; contrato validado en todo test HTTP; IPs de documentación; `now`
inyectado; **nombres por funcionalidad**, nunca por fase o tarea, con la trazabilidad en el
comentario de cada `Test*`. Además:

- **Identificadores** (plan DD-17): `T-002-Bnnn`. En los comentarios del código:
  `// T-002-B401 (002/INV-2): two concurrent deletions never leave the pipeline without a won stage.`
  Las decisiones e invariantes de 002 se citan `002/DD-5`, `002/INV-2`; las de 001, sin prefijo,
  como hasta ahora.
- **Una tarea por rama.** La unidad de rama es una tarea `[T]` **junto con** su tarea de
  implementación (Red → Green en la misma rama: un test rojo no se mergea solo); las tareas sin
  `[T]` van solas. Ramas desde `main`, con nombre por funcionalidad (p. ej.
  `feat/002-stage-pipeline`), nunca por fase. Las ramas de una fase se mergean antes de empezar la
  siguiente.
- **Archivos de test por funcionalidad**: `internal/settings/pipeline_test.go`,
  `stages_integration_test.go`, `categories_integration_test.go`, `catalog_integration_test.go`,
  `preferences_integration_test.go`, `seeder_integration_test.go`; en `internal/app`,
  `settings_stages_integration_test.go`, etc.; el escenario de SC-001 en
  `internal/app/configuration_scenario_integration_test.go`.
- **Qué se sustituye y qué no** (ADR-012):

  | Se sustituye | Nunca se sustituye |
  |---|---|
  | `clock.Clock` (tiempo controlado para `updated_at`) | PostgreSQL: RLS, privilegios por columna, `CHECK`, índices parciales y locks son lo que se prueba |
  | Un `db.Tx` envoltorio que **cuenta** sentencias o falla en la N-ésima, solo en los tests del sembrado (INV-10) | `settings.TemplateSeeder` en los tests de registro desde la Fase 3 (el `NoopSeeder` queda para los tests de 001 que no miran la configuración) |
  | `audit.Recorder` que falla, solo para probar que la operación se deshace | El router, los middlewares y el validador del contrato en los tests HTTP |

- **Concurrencia**: el "mecanismo E" de T-B604 de 001 (una transacción del test retiene el lock;
  el test sondea `pg_blocking_pids` cada 10 ms con contexto de 20 s hasta ver N backends bloqueados
  directa o transitivamente; `ROLLBACK`). Cada fila de concurrencia incluye la **mutación** que la
  hace fallar (se aplica, se ve fallar y se restaura), como en 001.

### Comandos de validación

Los de 001: `make generate` (sqlc), `make lint`, `make test`, `make test-int`, **`make check`
(checkpoint de cada fase)**. La medición de T-002-B307 corre con el build tag `bench`
(`internal/testsupport/perfbench`), fuera de `make check`, como T-B910 de 001.

### Coordinación con la sección Frontend

La sección Frontend y `ui.md` de 002 los escribe el `frontend-architect`. Lo que necesita del
backend:

| Necesita | Del backend | Notas |
|---|---|---|
| Contrato de 002 en la generación de tipos | `contracts/openapi.yaml` v0.1.0 (y **v0.6.0 de 001**, T-002-B202) | Una entrada nueva en `web/redocly.yaml` y un `& Paths002` en `src/api/schema.ts` (`ui.md` §11.4 de 001). Las rutas de 002 son disjuntas de las de 001 |
| Mapa de mensajes por `code` (T-F101 de 001) | `name_taken` (409), contrato de 001 v0.6.0 | El mapa exhaustivo deja de compilar hasta tener su texto (a propósito). `invalid_state` tiene un significado por endpoint (plan §8.3) |
| `Money` | Componente compartido de 001 v0.6.0 (ADR-029) | Primera spec con importes: entran las funciones `formatMoney` y `parseMoneyInput` de ADR-023 |
| Nombres personalizados en toda la UI | `GET /settings/labels` (cualquier rol) | Singular, plural y **género** (`masculine`/`feminine`) para concordar ("Nueva obra"). Se piden al arrancar, junto con `/me` |
| Colores de etapa | Tokens `gray`, `blue`, `teal`, `green`, `yellow`, `orange`, `red`, `purple` (DD-7) | La UI decide el color real; el rediseño visual no toca la base |
| Lecturas para el Operador | Todas las `GET` de 002 (DD-11) | Las escrituras devuelven `403` al Operador; `/me` ya trae `permissions` (`settings.manage`) |
| Registro | Sin cambios de contrato: elegir la plantilla ahora precarga la configuración | La UI puede mostrar qué trae cada plantilla con un texto propio (el contenido no se expone por `GET /industry-templates`) |

**Disponibilidad de las rutas por fase** (una ruta solo se consume después de mergeada su fase):

| Fase | Rutas |
|---|---|
| 4 | `GET/POST /stages`, `PATCH/DELETE /stages/{stageId}`, `POST /stages/{stageId}/move` |
| 5 | `GET/POST /categories`, `PATCH /categories/{categoryId}`, `POST /categories/{categoryId}/archive`, `POST /categories/{categoryId}/restore` |
| 6 | `GET/POST /catalog-items`, `GET/PATCH /catalog-items/{catalogItemId}`, `POST /catalog-items/{catalogItemId}/archive`, `POST /catalog-items/{catalogItemId}/restore` |
| 7 | `GET/PUT /settings/labels`, `GET/PUT /settings/quote-defaults` |

---

### Fase 1 — Esquema y aislamiento de la configuración (fundacional A)

**Objetivo**: las cuatro tablas existen con RLS forzada, privilegios por columna, `CHECK` e índices
del modelo; las empresas previas quedan con la configuración mínima (DD-4); los barridos de 001 las
cubren sin tests nuevos de aislamiento; las reglas de fronteras y de locks las vigilan.

**Prueba independiente**: sobre una base con empresas creadas por el binario actual de `main`,
`crm migrate up` deja a cada una con 1 fila de `tenant_settings`, 5 categorías de sistema y 5 etapas;
`make check` en verde.

**T-002-B101 [T] — Tablas de configuración: catálogo, constraints, privilegios y *backfill*** ·
INV-1, INV-3 a INV-7, DD-4 a DD-10, ADR-028, ADR-029, checklist de `data-model.md` §6 de 001
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | `expectedTables` de T-B107 de 001 | incluye `tenant_settings`, `stages`, `categories`, `catalog_items`; RLS habilitada y **forzada** en las cuatro |
  | Políticas (test de políticas de 001) | exactamente `tenant_isolation` (`FOR ALL`, `USING` y `WITH CHECK` con `tenant_id`) en cada una; ninguna otra |
  | Barrido T-B110 de 001 con los *inserters* nuevos | el rol de A no ve, no actualiza, no borra ni mueve filas de B en las cuatro; `tablesWithCompanyDelete` sigue vacía; ninguna nueva en `tablesWithoutCompanyUpdate` |
  | T-B109 de 001 (privilegios de los roles de sistema) | sin cambios: `crm_auth`, `crm_worker`, `crm_signup` no tienen nada sobre las tablas nuevas |
  | INV-5, como rol de empresa: `UPDATE … SET type` y `SET system_key` en `categories`; `SET id`, `SET tenant_id`, `SET created_at` en cada tabla | `42501` en cada caso |
  | INV-3: `DELETE` en cada tabla | `42501` |
  | `categories` con `system_key = 'customer_receipts'` y `type = 'expense'` | `23514` en `categories_system_type_chk` |
  | `system_key = 'cash_count_adjustment'` (todavía no existe) | `23514` en `categories_system_key_chk` |
  | `UPDATE` de `archived_at` en una categoría de sistema | `23514` en `categories_system_not_archived_chk` |
  | Dos categorías con el mismo `system_key` en una empresa / en dos empresas | `23505` en `categories_system_key_key` / ok |
  | Dos etapas activas "Vidrios" y "vidrios" / con la primera eliminada (`deleted_at`) | `23505` en `stages_active_name_key` / ok |
  | Dos categorías "Flete": ambas `expense` activas / una `income` / una archivada | `23505` en `categories_active_name_key` / ok / ok |
  | `catalog_items` con precio sin moneda / moneda sin precio | `23514` (par) |
  | `reference_price_cents` = −1 / 9007199254740992 / 9007199254740991 / 0 | `23514` / `23514` / ok / ok |
  | `unit = 'other'` sin `unit_label` / `unit = 'hour'` con `unit_label` | `23514` / `23514` |
  | `color = 'pink'`, `type = 'closed'`, `unit = 'kg'`, `*_gender = 'neutral'`, `quote_validity_days` 0 y 366, `quote_terms = ''`, nombre de etapa de 61 caracteres | `23514` en cada caso |
  | Segunda fila de `tenant_settings` para la misma empresa | `23505` en `tenant_settings_tenant_id_key` |
  | ***Backfill*** (DD-4): con el `goose.Provider` de `db/migrations`, base migrada hasta `00009`, dos empresas creadas con `fixture.NewCompany` de antes de 002 (sin filas en las tablas nuevas), y después `UpTo` `00010` | cada empresa: 1 fila de `tenant_settings` (Proyecto/Proyectos, Cliente/Clientes, Presupuesto/Presupuestos, todos `masculine`; 15 días; `quote_terms` `NULL`); las 5 categorías de sistema con su tipo; 5 etapas "Genérico" en posiciones 1 a 5 con tipos `open, open, open, won, lost`; ninguna categoría propia ni ítem; `industry_template_version` sin cambios; `app.tenants` y las cuatro tablas con `FORCE` al terminar |
  | `DownTo` `00009` | las cuatro tablas no existen; `app.tenants` sigue con `FORCE` |
- **Green**: pasa la tabla.
- **Refactor**: que los nombres de constraints coincidan con `data-model.md` (los usa el mapeo de
  errores de las Fases 4 y 5).

**T-002-B102 — Migración `00010_company_configuration.sql`, *inserters* y listas esperadas** (según
`data-model.md` §2 a §4 y §8). Un *inserter* por tabla en `fixture.CompanyInserters` (una fila
válida cada uno; `tenant_settings` devuelve su `id`), `expectedTables` y la lista de políticas
esperadas. El *backfill* desactiva `FORCE` en `app.tenants` y en las cuatro tablas solo dentro de la
migración (ADR-004 §6). `make generate` no cambia todavía (sin queries).

**T-002-B103 [T] — Fronteras del módulo y modo de los locks** · DD-1, INV-8, ADR-001, DD-40 de 001
- **Red**:

  | Caso | Esperado |
  |---|---|
  | `TestRowLockRuleScopes` (`queryrules/rowlocks_test.go`): `SELECT … FROM app.stages FOR UPDATE`, `FOR SHARE`, `FOR KEY SHARE`; lo mismo con `categories`, `catalog_items` y `tenant_settings` | violación ("DD-40: configuration rows only FOR NO KEY UPDATE") |
  | Lo mismo con `FOR NO KEY UPDATE` | sin violación |
  | `TestStoreQueriesUseAllowedRowLocks` sobre el repositorio | sigue pasando |
  | `depguard` (en `make lint`): mutación manual, un archivo de `internal/settings` que importa `internal/identity`; otra en `internal/tenant` que importa `internal/settings`; otra en `internal/platform/db` que importa `internal/settings` | `make lint` falla en cada una; se restauran |
- **Green**: la regla de `rowLockErrors` cubre las cuatro tablas y las mutaciones de `depguard`
  fallan.

**T-002-B104 — Implementar la regla de locks y las reglas de `depguard`**: en `.golangci.yml`,
`settings` no importa `identity`, `tenant`, `app` ni stores ajenos; `identity`, `tenant`,
`platform/*`, `authz` e `industrytemplate` no importan `settings` ni `settings/store`; entradas de
"no-foreign-store" para el módulo nuevo (comentario del archivo: "una entrada por módulo").

**Checkpoint Fase 1**: `make check` en verde + prueba independiente con `make db-reset` sobre una
base con una empresa previa.

---

### Fase 2 — Contratos, error nuevo y esqueleto del módulo (fundacional B)

**Objetivo**: el contrato de 001 está en v0.6.0 (`name_taken`, `Money`) con su código en `httpx`;
el validador de tests carga 001 y 002 resolviendo los `$ref` entre archivos; el módulo `settings`
existe y está cableado (todavía sin rutas); todas las operaciones de 002 figuran como pendientes.

**Prueba independiente**: `make check` en verde; el test de rutas contra el contrato lista las 20
operaciones de 002 como pendientes y ninguna de 001.

**T-002-B201 [T] — `name_taken` y `Money` compartidos** · DD-12, DD-13, ADR-029
- **Red**:

  | Caso | Esperado |
  |---|---|
  | `TestProblemCodesMatchContract` (existente) con `name_taken` en el enum del contrato de 001 | falla hasta que `httpx` tenga el código |
  | `httpx.WriteProblem(w, r, httpx.CodeNameTaken)` | `409`, `type` `/problems/name_taken`, `code` `name_taken`, `title` genérico en español |
  | `contract.CheckSchema("Money", …)` con `{amount_cents: 0, currency: ARS}` y `{9007199254740991, USD}` | ok |
  | Ídem con `9007199254740992`, `-1`, `1.5`, `"100"`, sin `currency`, `currency: "EUR"`, una propiedad extra | error en cada caso |
  | Test de `authz` contra el enum `Permission` del contrato | sin cambios |
- **Green**: pasa la tabla.

**T-002-B202 — Contrato de 001 v0.6.0 y `httpx.CodeNameTaken`**: el fragmento de plan §8.2
(versión, línea de historial, `name_taken` en `ErrorCode`, `Money`); `CodeNameTaken` con su status y
título. Sin otros cambios en 001.

**T-002-B203 [T] — Validador con varios contratos y operaciones pendientes** · DD-13, S-4, R-3
- **Red** (unitarios del helper, más el test de rutas de `internal/app`):

  | Caso | Esperado |
  |---|---|
  | `contract.Load()` | carga los dos documentos de `contract.Documents`; los dos pasan `ValidateDocument` |
  | `$ref` externos: respuesta `409` de `POST /stages` con `code: "name_taken"` / con `code: "foo"` | ok / error (el enum de 001 se aplica a través del `$ref`) |
  | Respuesta `200` de `GET /catalog-items` con `reference_price.amount_cents: 1.5` | error (`Money` de 001 a través del `$ref`) |
  | `Operations()` | las operaciones de 001 más las 20 de 002 (lista exacta de `operationId`) |
  | `Documents` de *testdata* con dos contratos que declaran la misma ruta y método | `Load` falla nombrando la ruta y los dos archivos |
  | Contrato de *testdata* con un `$ref` a un archivo inexistente | `Load` falla |
  | Ruta que no está en ningún contrato | igual que hoy (`404`/`405` globales contra `Problem`) |
  | Test de rutas contra el contrato (T-B801 de 001) con las 20 operaciones de 002 en `pendingOperations` | pasa |
  | Una operación en `pendingOperations` que sí tiene ruta en el router | falla ("remove it from pendingOperations") |
  | Una operación de 002 sin ruta y fuera de `pendingOperations` | falla ("contract route not implemented") |
- **Green**: pasa la tabla. Si libopenapi no resuelve los `$ref` entre archivos (S-4), se aplica el
  respaldo del plan (fusión en memoria de los componentes de 001) y se anota en el PR.

**T-002-B204 — Implementar `contract.Documents`, la carga múltiple y `pendingOperations`** (en el
test de rutas de `internal/app`, con las 20 operaciones de 002).

**T-002-B205 — Esqueleto del módulo `settings` y cableado** · DD-1. Paquete con `doc.go`,
`service.go` (`Service`, `NewService` que entra en pánico con una dependencia nil, `RequestMeta`,
errores centinela, `FieldErrors`) y `http.go` (`RegisterRoutes` que todavía no registra nada);
`internal/app/routes_settings.go`; `BuildAPIRouter` recibe `*settings.Service`; `cmd/crm/serve.go` lo
construye; se actualizan las llamadas existentes a `BuildAPIRouter` en los tests (con `nil` donde hoy
pasan `nil`) y el texto de la mutación de `serve_router_mutations_test.go` que reproduce la llamada,
sin cambiar las reglas del guard. Un test chico: `NewService` con cada dependencia nil entra en
pánico.

**Checkpoint Fase 2**: `make check` en verde.

---

### Fase 3 — Plantillas de rubro y sembrado en el registro (fundacional C; FR-006, Historia 1.1 de 001)

**Objetivo**: las reglas puras de la configuración existen y están probadas; las plantillas tienen
su contenido (versión 2) y lo cumplen; registrarse con cualquier plantilla deja la configuración
completa en la misma transacción; el costo está medido contra el presupuesto del lock de registro.

**Prueba independiente**: `POST /auth/signup` con `aluminum_carpentry` y, como el rol de esa
empresa, `psql` muestra 6 etapas, 14 categorías, 4 ítems y la fila de configuración; con `generic`,
5 etapas, 13 categorías y ningún ítem.

**T-002-B301 [T] — Reglas puras de la configuración** · DD-5, DD-7 a DD-10, INV-2, INV-7, P-6 a P-10
- **Red** (unitarios del paquete `settings`, sin base):

  | Regla | Entrada | Esperado |
  |---|---|---|
  | Texto obligatorio (máx. N) | `"  Obra  "` | `"Obra"` |
  | | `""`, `"   "` | `required` |
  | | N + 1 caracteres (contando runas; `"ñ"` × (N + 1)) | `too_long` |
  | | `"Obra"` seguido de N espacios (largo medido sobre lo recibido, como DD-43 (2) de 001) | `too_long` |
  | | con `NUL` | `invalid_value` |
  | Texto opcional | `""`, `"   "` | `nil` (sin valor) |
  | `validatePipeline` | `[]`, `[open]`, `[open, lost]` | `ErrPipelineRule` (sin `won`) |
  | | `[won, open]`, `[lost, open, won]` | `ErrPipelineRule` (primera de cierre) |
  | | `[open, won]`, `[open, won, lost]`, `[open, lost, won]`, `[open, open, won, won]` | ok |
  | Insertar en posición `p` (n etapas) | `p` = 1 y `p` = n + 1 | orden esperado, posiciones 1..n+1 |
  | | `p` = 0, `p` = n + 2 | `position` → `invalid_value` |
  | Mover de `a` a `b` | `b` = `a` | sin cambios (marca "no-op") |
  | | `b` fuera de 1..n | `position` → `invalid_value` |
  | | casos de subir, bajar, a la primera y a la última | orden esperado, posiciones densas |
  | Quitar | la primera, una del medio, la última | posiciones densas 1..n−1 |
  | Enumerados | cada valor válido de `StageType`, `Color`, `CategoryType`, `Unit`, `Gender`, `SystemKey` | ok |
  | | `"Open"`, `"pink"`, `"kg"`, `"neutral"`, `""` | `invalid_value` |
  | `Money` | `(0, ARS)`, `(9007199254740991, USD)` | ok |
  | | `(9007199254740992, ARS)`, `(-1, ARS)` | `reference_price.amount_cents` → `invalid_value` |
  | | `(100, "ars")`, `(100, "EUR")`, `(100, "")` | `reference_price.currency` → `invalid_value` (o `required` con `""`) |
  | Unidad | `other` + `"jornada"` | ok |
  | | `other` sin `unit_label` | `unit_label` → `required` |
  | | `hour` + `"x"` | `unit_label` descartado (`nil`) |
  | | `other` + 21 caracteres | `unit_label` → `too_long` |
  | Condiciones | `validity_days` 1 y 365 / 0 y 366 | ok / `validity_days` → `invalid_value` |
  | | `terms` `""` y `"   "` / 2001 caracteres | `nil` / `terms` → `too_long` |
  | Nombres | singular o plural vacío / de 41 caracteres / `gender` inválido | `project.singular` → `required` / `too_long` / `project.gender` → `invalid_value` (campos con ruta `entidad.campo`) |
  | Categorías de sistema | la tabla `systemCategories` | exactamente las 5 claves con el tipo y el nombre de `data-model.md` §4.1 |
- **Green**: pasa la tabla.

**T-002-B302 — Implementar las reglas puras** (`text.go`, `pipeline.go`, y los tipos y validaciones
de `categories.go`, `catalog.go` y `preferences.go`, sin acceso a datos).

**T-002-B303 [T] — Contenido de las plantillas** · FR-006, DD-2, INV-9, P-4
- **Red**:

  | Caso | Esperado |
  |---|---|
  | `industrytemplate.All()` | dos plantillas en `version` 2 con **exactamente** el contenido de P-4 (tabla esperada en el test: nombres, tipos, colores y orden de etapas; categorías por tipo; ítems con unidad; condiciones y validez; nombres por defecto) |
  | `parseCatalog` con un campo desconocido dentro de una etapa / una plantilla sin `stages` / `version` 0 | error en cada caso |
  | `GET /api/v1/industry-templates` | idéntica a la actual: solo `code` y `name` (contrato de 001 sin cambios) |
  | (en `settings`) cada plantilla de `All()` pasa las reglas de T-002-B301: pipeline válido, enumerados, largos, condiciones; nombres únicos (etapas entre sí; categorías por tipo, **incluidas las de sistema**) | ok; una plantilla de *testdata* con la primera etapa `won`, o con una categoría "Cobros de clientes" de ingreso, falla con un mensaje que nombra la plantilla y la regla |
  | INV-9: ningún archivo `.go` fuera de `_test.go`, `testdata/` e `internal/testsupport/` contiene los literales `"generic"` ni `"aluminum_carpentry"` | regla nueva en `internal/testsupport/reporules` (con un caso de *testdata* que la viola) |
- **Green**: pasa la tabla.

**T-002-B304 — Implementar `Template` con contenido y `catalog.json` v2** (plan §11.1; valores de
P-4). Los enumerados son *strings* en `industrytemplate` (no importa `settings`).

**T-002-B305 [T] — Sembrado en el registro** · FR-006, DD-3, INV-1, INV-4, INV-10, INV-14 de 001
- **Red** (integración: `tenant.Service.Register` real con `settings.TemplateSeeder` real):

  | Caso | Esperado |
  |---|---|
  | Registrar con `aluminum_carpentry` | `tenant_settings`: nombres por defecto, 15 días, condiciones de P-4; 6 etapas con nombre, tipo, color y posiciones 1..6; 5 categorías de sistema + 9 de la plantilla con sus tipos; 4 ítems sin precio con su unidad; `tenant.registered` igual que antes; **ninguna** fila de auditoría de configuración |
  | Registrar con `generic` | 5 etapas; 5 + 8 categorías; 0 ítems; `quote_terms` `NULL` |
  | INV-10, con un `db.Tx` envoltorio que cuenta sentencias | **exactamente 4** en `Seed` con cada plantilla, y con una plantilla de *testdata* de 30 etapas y ningún ítem |
  | El envoltorio falla en la 3.ª sentencia | `Register` devuelve error; no existen la empresa, su rol, el usuario ni ninguna fila de configuración (INV-14 de 001) |
  | Las filas sembradas, vistas como el rol de otra empresa | no se ven (RLS) |
  | `GET /me` justo después de registrarse (otra conexión) | `200` (sin cambios de 001, DD-34 de 001) |
  | Test de rollback del registro de 001 (T-B303, *seeder* que falla) | sigue pasando |
- **Green**: pasa la tabla. **Supuesto S-3** (sqlc con arrays en `unnest`): se confirma acá; si no
  se cumple, `:copyfrom` y se ajusta la cuenta de INV-10 en el plan, en el mismo PR.

**T-002-B306 — Implementar `TemplateSeeder`** (`seeder.go`, `store/seed.sql`) y cablearlo:
`serve.go` reemplaza `NoopSeeder` por `settings.NewTemplateSeeder(c)`; los entornos de test que
recorren el registro de punta a punta (`newAuthServer`, el escenario `fixture/e2e`) usan el real.
`cmd/crm/tenants.go` (reprovisión) sigue con `NoopSeeder`: nunca registra.

**T-002-B307 [T] [bench] — Compuerta: costo del sembrado en la retención del lock de registro** ·
DD-3, R-1, DD-33 R-b/R-c de 001
- **Medición** (build tag `bench`, en `internal/testsupport/perfbench`, entorno de T-B910 de 001 con
  10.000 empresas; corre fuera de `make check`):

  | Medición | Cómo | Se registra |
  |---|---|---|
  | M-7: sembrado con 10.000 roles | 100 registros secuenciales sin otra actividad, alternando `TemplateSeeder` real (plantilla `aluminum_carpentry`, la más grande) y `NoopSeeder`; el *tracer* de T-B910 suma el intervalo `seed` | p50 y p95 de `seed` (solo los reales) y de `hold` por grupo |

- **Regla de decisión** (fijada antes de medir): **se mantiene** si `seed` p95 ≤ 10 ms **y**
  `hold` p95 (real) − `hold` p95 (noop) ≤ 10 ms. Si además `hold` p95 (real) ≥ 140 ms, se informa al
  usuario (R-b de 001 en riesgo con o sin 002), sin bloquear la fase. **Si no se cumple**, la fase
  se detiene y vuelve al arquitecto: salida de R-c (research R-3), que necesita aprobación.
- **Green**: resultado y decisión anotados en el plan (estado de la implementación) con los números
  de las dos corridas.

**Checkpoint Fase 3**: `make check` en verde + T-002-B307 dentro de la regla (o la decisión del
usuario registrada).

---

### Fase 4 — Historia 1: Etapas de proyecto (P1) — primer slice vertical por HTTP

**Objetivo**: el Administrador crea, renombra, cambia tipo o color, mueve y elimina etapas por la
API; las reglas del pipeline valen también con pedidos concurrentes; el Operador las lee.

**Prueba independiente**: con `curl` y la cookie de un Administrador de una empresa "Genérico":
mover "En curso" a la posición 1, crear "Instalación" antes de "Finalizado", intentar eliminar
"Finalizado" (`409 invalid_state`) y ver el orden final con `GET /stages` como Operador.

**T-002-B401 [T] — Servicio de etapas** · Historia 1 (1, 2, 4), P-2, P-6, DD-5, DD-6, DD-15, DD-18,
INV-2, INV-6, INV-8, INV-11 a INV-13
- **Red** (integración; empresa sembrada con `generic`: Presupuesto `open`, Aprobado `open`, En curso
  `open`, Finalizado `won`, Perdido `lost`):

  | Caso | Esperado |
  |---|---|
  | `ListStages` como admin y como operador | las 5, posiciones 1..5 |
  | `CreateStage` "Instalación" `open`, sin posición | posición 6; auditoría `stage.created {type: open, position: 6}` |
  | `CreateStage` con `position` 3 | queda en 3; las siguientes suben uno; densas |
  | `CreateStage` con `position` 0 / 7 (n + 2) | `FieldErrors{position: invalid_value}`; nada cambia |
  | `CreateStage` "presupuesto" | `ErrNameTaken` |
  | `CreateStage` con el nombre de una etapa eliminada | ok |
  | `CreateStage` `won` o `lost` en posición 1 | `ErrPipelineRule`; nada cambia |
  | Cualquier escritura como operador | `authz.ErrForbidden`; nada cambia |
  | `UpdateStage` nombre y color | ok; `stage.updated {fields: [name, color]}` |
  | `UpdateStage` con los valores actuales | sin `UPDATE`, sin auditoría, `updated_at` igual (INV-13) |
  | `UpdateStage` tipo de Finalizado (única `won`) a `open` | `ErrPipelineRule` |
  | `UpdateStage` tipo de Presupuesto (primera) a `won` | `ErrPipelineRule` |
  | `UpdateStage` En curso a `won`; después Finalizado a `open` | ok los dos |
  | `UpdateStage` con el nombre de otra activa | `ErrNameTaken` |
  | `UpdateStage` de una eliminada / de otra empresa / inexistente | `ErrNotFound` en los tres (indistinguibles) |
  | `DeleteStage` Aprobado | `deleted_at` puesto; las activas quedan 1..4; no aparece en `ListStages`; `stage.deleted {}` |
  | `DeleteStage` Finalizado (única `won`) | `ErrPipelineRule` |
  | `DeleteStage` Presupuesto con Aprobado `open` segunda | ok (Aprobado queda inicial) |
  | `DeleteStage` de la primera con pipeline `[open, won, lost]` | `ErrPipelineRule` |
  | `DeleteStage` dos veces | la segunda `ErrNotFound` |
  | `MoveStage` Perdido a 1 | `ErrPipelineRule` |
  | `MoveStage` En curso de 3 a 1 | `[En curso, Presupuesto, Aprobado, Finalizado, Perdido]`; `stage.moved {from: 3, to: 1}`; devuelve la lista |
  | `MoveStage` a la posición actual | lista sin cambios; sin auditoría |
  | `MoveStage` a 6 (n + 1) | `FieldErrors{position: invalid_value}` |
  | **Concurrencia (INV-2)**: pipeline con dos `won` (Finalizado y "Entregado"); `DeleteStage` de cada una en paralelo. Mecanismo E: `T_admin` retiene `LockTenantSettings`; el test espera 2 backends bloqueados y hace `ROLLBACK`. Corre una vez | exactamente una tiene éxito y la otra `ErrPipelineRule`; queda una `won`. Mutación: sin `LockTenantSettings` las dos pasan y quedan cero `won` → la fila falla |
  | **Concurrencia**: dos `CreateStage` al final en paralelo (mecanismo E) | posiciones 6 y 7, sin repetidos ni huecos. Mutación: sin el lock, las dos leen 5 y quedan dos en 6 → falla |
  | Auditoría | `data` con claves **exactas**; actor, `target_type` `stage`, `target_id`, IP y *user agent* |
- **Green**: pasa la tabla.
- **Refactor**: un único esqueleto lock → lista → regla pura → escritura → auditoría compartido por
  las cuatro escrituras (sin duplicar la secuencia).

**T-002-B402 — Implementar los casos de uso de etapas** (`stages.go`; queries `LockTenantSettings`,
`ListActiveStages`, `InsertStage`, `UpdateStageFields`, `SetStagePositions` en una sola sentencia,
`SoftDeleteStage`; mapeo de `23505` en `stages_active_name_key` a `ErrNameTaken`).

**T-002-B403 [T] — Endpoints de etapas** · contrato, FR-007 de 001, DD-11, DD-12, INV-11
- **Red** (HTTP + contrato; `apitest`):

  | Caso | Esperado |
  |---|---|
  | `GET /stages` como admin y como operador | `200`, `Cache-Control: no-store` |
  | `POST /stages` válido | `201` con `Stage` |
  | `POST /stages` como operador, también con un body inválido | `403 forbidden` (el permiso se decide antes de leer el body) |
  | `POST /stages` con `name: ""` / `type: "closed"` / `color: "pink"` | `422` con `errors` `name`/`required`, `type`/`invalid_value`, `color`/`invalid_value` |
  | `POST /stages` con un campo desconocido / `position: "2"` | `400 malformed_request` |
  | `POST /stages` nombre repetido / `won` en posición 1 | `409 name_taken` / `409 invalid_state` |
  | `PATCH /stages/{id}` con `{}` / `{"name": null}` | `400 malformed_request` |
  | `PATCH` y `DELETE` con id de otra empresa / inexistente / `no-uuid` | `404` en los tres |
  | `DELETE /stages/{id}` válido / única `won` | `204` / `409 invalid_state` |
  | `POST /stages/{id}/move` válido / `{position: 0}` / `{position: 99}` | `200` con `StageList` / `422` / `422` (`position`, `invalid_value`) |
  | Body de más de 64 KiB / sin `Content-Type: application/json` | `413` / `415` |
  | `POST` de origen cruzado | `403 forbidden` (CSRF, DD-30 de 001) |
  | Barrido de aislamiento (T-B801 de 001): `isolationCases` para las 5 rutas | con ids de otra empresa, `404` y sin cambios en B |
  | Barrido de permisos (T-B802 de 001) | operador: `403` en `POST`, `PATCH`, `DELETE` y `move`; `200` en `GET` |
  | Test de rutas contra el contrato | las 5 operaciones de etapas salen de `pendingOperations` |
- **Green**: pasa la tabla.

**T-002-B404 — Implementar los handlers de etapas** (`settings/http.go`: DTOs, decodificación que
distingue ausente de `null`, mapeo de errores por endpoint según plan §8.3; `RegisterRoutes` agrupa
las escrituras con `RequirePermission(SettingsManage)`).

**Checkpoint Fase 4**: `make check` en verde + prueba independiente con `curl`.

---

### Fase 5 — Historia 3: Categorías de ingresos y egresos (P1)

**Objetivo**: el Administrador crea, renombra, archiva y restaura categorías; las de sistema se
renombran pero no se archivan; los nombres no se repiten por tipo.

**Prueba independiente**: con `curl`: renombrar "Cobros de clientes" a "Cobranzas", intentar
archivarla (`409 invalid_state`), crear "Combustible" de egreso, archivarla y verla solo con
`include_archived=true`.

**T-002-B501 [T] — Servicio de categorías** · Historia 3 (1, 2, 3), FR-003, FR-004, P-8, DD-8,
DD-15, INV-4 a INV-6, INV-13
- **Red** (integración; empresa `generic`):

  | Caso | Esperado |
  |---|---|
  | `ListCategories` sin filtros | no archivadas; orden: `income` primero; dentro de cada tipo, las de sistema y después las de la empresa, por nombre (lista exacta esperada) |
  | `ListCategories` con `type = expense` / con `IncludeArchived` | solo egresos / incluye archivadas con `Archived = true` |
  | `CreateCategory` "Combustible" `expense` | ok; `category.created {type: expense}` |
  | `CreateCategory` "flete" `expense` (existe "Flete") / "Flete" `income` / con el nombre de una archivada | `ErrNameTaken` / ok / ok |
  | `RenameCategory` "Cobros de clientes" → "Cobranzas" | ok; `SystemKey` igual; `category.renamed {}` |
  | `RenameCategory` al nombre de otra activa del mismo tipo / de una archivada | `ErrNameTaken` / ok |
  | `RenameCategory` de una archivada | ok |
  | `RenameCategory` con el mismo nombre | sin cambios ni auditoría |
  | `ArchiveCategory` propia | archivada; fuera de la lista por defecto; `category.archived {}` |
  | `ArchiveCategory` ya archivada | devuelve la categoría; sin auditoría |
  | `ArchiveCategory` de sistema | `ErrSystemCategory`; sin cambios |
  | `RestoreCategory` | activa; `category.restored {}` |
  | `RestoreCategory` cuyo nombre ya usa otra activa del mismo tipo | `ErrNameTaken` |
  | `RestoreCategory` de una activa | sin cambios ni auditoría |
  | Id de otra empresa / inexistente | `ErrNotFound` |
  | Escrituras como operador | `authz.ErrForbidden` |
  | **Concurrencia**: dos `CreateCategory` "Peajes" `expense` en paralelo | una ok, otra `ErrNameTaken` (lo resuelve el índice único, sin lock) |
  | **Concurrencia**: dos `ArchiveCategory` de la misma en paralelo (mecanismo E sobre `LockCategory`) | una sola auditoría (la segunda ve "ya archivada"). Mutación: leer sin `LockCategory` → dos auditorías → falla |
- **Green**: pasa la tabla.

**T-002-B502 — Implementar los casos de uso de categorías** (`categories.go`; queries
`ListCategories`, `InsertCategory`, `LockCategory`, `UpdateCategoryName`, `SetCategoryArchived`;
`23505` en `categories_active_name_key` → `ErrNameTaken`).

**T-002-B503 [T] — Endpoints de categorías** · contrato, DD-11, DD-12
- **Red** (HTTP + contrato):

  | Caso | Esperado |
  |---|---|
  | `GET /categories`, `?type=income`, `?include_archived=true` | `200` según el filtro |
  | `GET /categories?type=foo` / `?include_archived=maybe` | `400 malformed_request` |
  | `POST /categories` válido / nombre repetido / `type: "other"` / sin `name` | `201` / `409 name_taken` / `422` / `422` (`name`, `required`) |
  | `PATCH /categories/{id}` válido / `{}` / `{"name": null}` | `200` / `400` / `400` |
  | `POST …/archive` de sistema / propia / ya archivada | `409 invalid_state` / `200` / `200` |
  | `POST …/restore` con nombre tomado | `409 name_taken` |
  | Ids de otra empresa, inexistentes o `no-uuid` en las rutas con id | `404` |
  | Barridos de aislamiento y de permisos (T-B801/T-B802 de 001) para las 5 rutas | como en T-002-B403 |
  | Test de rutas contra el contrato | las 5 operaciones salen de `pendingOperations` |
- **Green**: pasa la tabla.

**T-002-B504 — Implementar los handlers de categorías.**

**Checkpoint Fase 5**: `make check` en verde + prueba independiente.

---

### Fase 6 — Historia 4: Catálogo de productos y servicios (P2)

**Objetivo**: el Administrador gestiona ítems con unidad y precio de referencia opcional en
centavos y moneda; editar es parcial y seguro ante ediciones concurrentes; archivar los saca de la
lista activa.

**Prueba independiente**: con `curl`: crear "Portón corredizo" (unidad, USD 4.500,00 →
`amount_cents: 450000`), cambiarle solo el precio, archivarlo y verlo solo con
`include_archived=true`; un `amount_cents` con decimales responde `400`.

**T-002-B601 [T] — Servicio de catálogo** · Historia 4 (1, 3), FR-005, P-9, DD-9, DD-15, INV-7,
INV-13, ADR-029
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | `CreateCatalogItem` sin precio, `square_meter` | ok; `ReferencePrice` nil; `catalog_item.created {unit: square_meter}` |
  | `CreateCatalogItem` con `{AmountCents: 123456, Currency: USD}` | guardado exacto (`reference_price_cents` 123456, `USD`) |
  | `CreateCatalogItem` con 9007199254740992 | `FieldErrors{"reference_price.amount_cents": invalid_value}` |
  | `CreateCatalogItem` `other` sin `UnitLabel` / `hour` con `UnitLabel` | `FieldErrors{unit_label: required}` / ok con `UnitLabel` nil |
  | `CreateCatalogItem` con descripción `""` | `Description` nil |
  | Dos ítems con el mismo nombre | ok los dos |
  | `UpdateCatalogItem` solo nombre | solo cambia el nombre; `catalog_item.updated {fields: [name]}` |
  | `UpdateCatalogItem` `Description` presente y nil / `ReferencePrice` presente y nil | se borran; `fields` las nombra |
  | `UpdateCatalogItem` precio `{0, ARS}` | guardado 0 (precio válido, no "sin precio") |
  | `UpdateCatalogItem` `unit` de `other` a `unit` | `unit_label` se borra; `fields: [unit, unit_label]` |
  | `UpdateCatalogItem` `unit` de `hour` a `other` sin `UnitLabel` | `FieldErrors{unit_label: required}` |
  | `UpdateCatalogItem` con los valores actuales | sin `UPDATE`, sin auditoría, `updated_at` igual |
  | `UpdateCatalogItem` con `Name` o `Unit` presentes y nulos | `ErrMalformedChanges` |
  | **Concurrencia**: dos `UpdateCatalogItem` en paralelo (uno el nombre, otro el precio). Mecanismo E: `T_admin` retiene `LockCatalogItem`; 2 bloqueados; `ROLLBACK` | se guardan **los dos** cambios; dos auditorías. Mutación: leer sin lock y reescribir todas las columnas → uno pisa al otro → falla |
  | `ArchiveCatalogItem` / otra vez / `RestoreCatalogItem` / otra vez | archivado con auditoría / sin auditoría / activo con auditoría / sin auditoría |
  | Ítem archivado | `GetCatalogItem` ok; `UpdateCatalogItem` ok; fuera de la lista por defecto |
  | Id de otra empresa / inexistente | `ErrNotFound` |
  | Escrituras como operador | `authz.ErrForbidden` |
- **Green**: pasa la tabla. (Historia 4.2, "cambiar el precio no modifica presupuestos", se verifica
  en 005: 002 no tiene presupuestos.)

**T-002-B602 — Implementar los casos de uso del catálogo** (`catalog.go`, `Patch[T]`; queries
`ListCatalogItems`, `GetCatalogItem`, `InsertCatalogItem`, `LockCatalogItem`,
`UpdateCatalogItemFields`, `SetCatalogItemArchived`).

**T-002-B603 [T] — Endpoints del catálogo** · contrato, ADR-029
- **Red** (HTTP + contrato):

  | Caso | Esperado |
  |---|---|
  | `POST /catalog-items` válido con y sin `reference_price` | `201`; `amount_cents` sale como entero JSON (sin decimales ni exponente) |
  | `amount_cents: 1.5` / `"100"` / `9223372036854775808` (no entra en `int64`) | `400 malformed_request` |
  | `amount_cents: 9007199254740992` | `422` (`reference_price.amount_cents`, `invalid_value`) |
  | `currency: "EUR"` / `reference_price` sin `currency` | `422` (`reference_price.currency`, `invalid_value` / `required`) |
  | `unit: "other"` sin `unit_label` | `422` (`unit_label`, `required`) |
  | `PATCH` con `{"reference_price": null}` / `{"name": null}` / `{}` | `200` sin precio / `400` / `400` |
  | `GET /catalog-items/{id}` de un archivado / de otra empresa | `200` / `404` |
  | `GET /catalog-items?include_archived=maybe` | `400` |
  | `POST …/archive` y `…/restore` | `200` (idempotentes) |
  | Barridos de aislamiento y de permisos para las 6 rutas | como en T-002-B403 |
  | Test de rutas contra el contrato | las 6 operaciones salen de `pendingOperations` |
- **Green**: pasa la tabla.

**T-002-B604 — Implementar los handlers del catálogo** (DTO de `Money` con campos puntero para
distinguir faltante de cero; nunca `float64` en la decodificación).

**Checkpoint Fase 6**: `make check` en verde + prueba independiente.

---

### Fase 7 — Historias 6 y 2: Condiciones comerciales y nombres personalizados (P2, P3)

**Objetivo**: el Administrador define la validez y las condiciones por defecto de los presupuestos y
los nombres de Proyecto, Cliente y Presupuesto con su género; cualquier usuario los lee.

**Prueba independiente**: con `curl`: `PUT /settings/labels` con Proyecto → "Obra"/"Obras"
femenino; `GET /settings/labels` como Operador lo devuelve; `PUT /settings/quote-defaults` con
30 días y condiciones vacías guarda `terms: null`.

**T-002-B701 [T] — Servicio de nombres y condiciones** · Historias 2 y 6, FR-002, FR-007, P-3,
P-10, DD-10, DD-15, INV-13
- **Red** (integración):

  | Caso | Esperado |
  |---|---|
  | `GetLabels` de una empresa recién registrada | los valores por defecto de la plantilla |
  | `ReplaceLabels` con `project = {Obra, Obras, feminine}` | guardado; `settings.labels_updated {entities: [project]}` |
  | `ReplaceLabels` con los valores actuales | sin `UPDATE` ni auditoría |
  | `ReplaceLabels` con un singular vacío / de 41 caracteres / género `neutral` | `FieldErrors` con `project.singular` `required` / `too_long` / `project.gender` `invalid_value` |
  | `GetQuoteDefaults` de `generic` / de `aluminum_carpentry` | `{15, nil}` / `{15, condiciones de P-4}` |
  | `ReplaceQuoteDefaults` `{30, "Seña del 50 %"}` | guardado; `settings.quote_defaults_updated {fields: [validity_days, terms]}` |
  | `ReplaceQuoteDefaults` con `terms` `"   "` | `terms` nil |
  | `ReplaceQuoteDefaults` con `validity_days` 0 | `FieldErrors{validity_days: invalid_value}` |
  | Escrituras como operador | `authz.ErrForbidden` |
  | **Concurrencia**: `ReplaceLabels` y `DeleteStage` de la misma empresa en paralelo (mismo lock, mecanismo E) | las dos confirman, una después de la otra; sin `40P01` |
- **Green**: pasa la tabla.

**T-002-B702 — Implementar los casos de uso de nombres y condiciones** (`preferences.go`; queries
`GetTenantSettings`, `UpdateLabels`, `UpdateQuoteDefaults`, con `LockTenantSettings` primero).

**T-002-B703 [T] — Endpoints de nombres y condiciones** · contrato
- **Red** (HTTP + contrato):

  | Caso | Esperado |
  |---|---|
  | `GET` de los dos recursos como admin y como operador | `200` |
  | `PUT /settings/labels` sin `customer` | `422` (`customer`, `required`) |
  | `PUT /settings/labels` con un campo desconocido | `400` |
  | `PUT /settings/quote-defaults` con `terms: null` / `validity_days: "30"` / `validity_days: 400` | `200` / `400` / `422` |
  | Barridos de aislamiento y de permisos para las 4 rutas | como en T-002-B403 (no tienen id: el barrido verifica que A nunca lee ni escribe la fila de B) |
  | Test de rutas contra el contrato | las 4 operaciones salen de `pendingOperations` (queda vacía) |
- **Green**: pasa la tabla.

**T-002-B704 — Implementar los handlers de nombres y condiciones.**

**Checkpoint Fase 7**: `make check` en verde + prueba independiente.

---

### Fase 8 — SC-001 de punta a punta y cierre

**Objetivo**: demostrar que una empresa de otro rubro queda configurada solo con la API, sin código
específico; dejar la documentación viva al día.

**Prueba independiente**: el escenario de T-002-B801 en verde.

**T-002-B801 [T] — Escenario "herrería" solo con la API** · SC-001, FR-001 a FR-007, INV-9, R-9
- **Red** (HTTP de punta a punta con `apitest`, un solo test):

  | Paso | Esperado |
  |---|---|
  | Registro con `generic` | `201` |
  | `PUT /settings/labels` con Proyecto → "Trabajo"/"Trabajos" masculino | `200` |
  | Renombrar "En curso" → "En taller"; crear "Colocación" `open` en la posición 4; cambiar colores; mover "Aprobado" | `GET /stages` devuelve el orden y los valores esperados exactos |
  | Crear categorías "Hierros", "Pintura" y "Electrodos" de egreso; archivar "Alquiler" | lista por defecto exacta |
  | Crear "Reja de ventana" (`square_meter`, sin precio), "Portón corredizo" (`unit`, USD 450000) y "Soldadura" (`other`, "jornada", ARS 2500000) | lista exacta |
  | `PUT /settings/quote-defaults` | `200` |
  | Un Operador invitado (flujo de 001) lee los seis `GET` / intenta cualquier escritura | `200` / `403` |
  | `pendingOperations` | vacía (y se borra la lista del test) |
- **Green**: pasa la tabla, sin ningún cambio de código de producción en esta tarea.

**T-002-B802 — Documentación viva** (plan §16, §17): términos de §17 del plan en
`docs/glosario.md`; en `docs/modelo-dominio.md` ("Configuración por empresa"), la etapa inicial, las
categorías de sistema por clave y el precio opcional del catálogo; estado de la implementación en el
plan y en esta sección; estado de ADR-028 y ADR-029 en `docs/adr/README.md` según lo que haya
aprobado el usuario; las vistas de `docs/diagramas/` que listen módulos o tablas, según el
procedimiento de `docs/diagramas/README.md`.

**Checkpoint Fase 8**: `make check` en verde.

---

### Trazabilidad

| Requisito / criterio / decisión | Tareas |
|---|---|
| FR-001 Etapas ordenadas con tipo y color | T-002-B101, T-002-B301, T-002-B401..B404 |
| FR-002 Nombres personalizados | T-002-B101, T-002-B301, T-002-B701..B704 |
| FR-003 Categorías: crear, renombrar, archivar | T-002-B101, T-002-B501..B504 |
| FR-004 Categorías de sistema | T-002-B101, T-002-B301, T-002-B305, T-002-B501 |
| FR-005 Catálogo con unidad, precio y moneda | T-002-B101, T-002-B301, T-002-B601..B604 |
| FR-006 Plantillas de rubro | T-002-B303..B307 |
| FR-007 Condiciones y validez por defecto | T-002-B101, T-002-B301, T-002-B701..B704 |
| Historia 1 (1, 2, 4) / (3, P-2) | T-002-B401, T-002-B403 / T-002-B401 (eliminar); mover proyectos: spec 004 |
| Historia 2 | T-002-B701, T-002-B703 |
| Historia 3 (1, 2, 3) | T-002-B501, T-002-B503 |
| Historia 4 (1, 3) / (2) | T-002-B601, T-002-B603 / spec 005 |
| Historia 6 (P-10) | T-002-B701, T-002-B703 |
| Historia 1.1 de 001 (precarga de la plantilla) | T-002-B305 |
| SC-001 | T-002-B801 |
| SC-002 de 001 (0 accesos cruzados) en las rutas nuevas | T-002-B101 (base), T-002-B403, B503, B603, B703 (HTTP) |
| INV-1 | T-002-B101, T-002-B305 |
| INV-2 | T-002-B301, T-002-B401 |
| INV-3, INV-5 | T-002-B101 |
| INV-4 | T-002-B101, T-002-B305, T-002-B501 |
| INV-6 | T-002-B101, T-002-B401, T-002-B501 |
| INV-7 / ADR-029 | T-002-B101, T-002-B201, T-002-B301, T-002-B601, T-002-B603 |
| INV-8 | T-002-B103, T-002-B401, T-002-B501, T-002-B601 |
| INV-9 | T-002-B303, T-002-B801 |
| INV-10 / DD-3 / DD-33 R-c de 001 | T-002-B305, T-002-B307 |
| INV-11 / DD-11 | T-002-B401, B403, B501, B503, B601, B603, B701, B703, T-002-B801 |
| INV-12 / DD-14 | Filas de auditoría de T-002-B401, B501, B601, B701 |
| INV-13 / DD-15 | T-002-B401, B501, B601, B701 |
| DD-4 *backfill* | T-002-B101 |
| DD-12 / DD-13 contratos y `name_taken` | T-002-B201..B204 |
| ADR-028 | T-002-B101, T-002-B401 (borrado lógico), T-002-B501 |
| DD-1 fronteras | T-002-B103, T-002-B205 |

---

## Frontend

**Pendiente**: la escribe el `frontend-architect` junto con `ui.md` de 002, con identificadores
`T-002-Fnnn`. Lo que necesita del backend está en "Coordinación con la sección Frontend" (arriba),
incluida la disponibilidad de cada ruta por fase y el cambio de v0.6.0 del contrato de 001
(`name_taken`, `Money`). No condiciona al backend: el rediseño visual previsto no cambia ninguna
decisión de este plan (los colores son tokens, DD-7).
