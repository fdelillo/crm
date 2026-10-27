# Tasks: Configuración por empresa

**Spec**: [`spec.md`](spec.md) · **Plan**: [`plan.md`](plan.md) · **Modelo**: [`data-model.md`](data-model.md)
· **Contrato**: [`contracts/openapi.yaml`](contracts/openapi.yaml)
**Estado**: Aprobado (2026-10-23), junto con la spec (P-1 a P-10 con las opciones recomendadas) y el
plan.

---

## Backend

Autor: `backend-architect`. Implementa: `backend-developer`, **una fase por invocación**, en modo
completo (`.claude/dev-mode`).

### Estado de la implementación (2026-10-23)

Ninguna fase empezada. Spec y plan aprobados: se puede empezar por la Fase 1. Quedan por verificar
al implementar (no son aprobaciones): la compuerta del sembrado (T-002-B307) y los supuestos S-3
(sqlc con arrays, T-002-B305) y S-4 (`$ref` entre archivos en libopenapi, T-002-B203).

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
  siguiente. **Excepción**: T-002-B201, T-002-B202 y la tarea frontend T-002-F001 van en **una sola
  rama** (el cambio del enum compartido rompe el mapa exhaustivo del frontend, y `make check-all` no
  puede quedar en rojo entre ramas).
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
(checkpoint de cada fase)**, que corre `lint`, `test` y `test-int` del backend y **no** incluye el
frontend: el typecheck y los tests del frontend los corre `make web-check`, y `make check-all` corre
las dos suites. La medición de
T-002-B307 corre con el build tag `bench` (`internal/testsupport/perfbench`), fuera de `make check`,
como T-B910 de 001.

### Coordinación con la sección Frontend

La sección Frontend y `ui.md` de 002 los escribe el `frontend-architect`. Lo que necesita del
backend:

| Necesita | Del backend | Notas |
|---|---|---|
| Contrato de 002 en la generación de tipos | `contracts/openapi.yaml` v0.1.0 y **v0.6.0 de 001** (T-002-B202) | **T-002-B202 es requisito previo**: el contrato 002 no se agrega a `web/redocly.yaml` ni se consume antes de que B202 esté mergeada y el checkpoint de la Fase 2 haya demostrado que todos sus `$ref` resuelven. Después, una entrada nueva en `web/redocly.yaml` y un `& Paths002` en `src/api/schema.ts` (`ui.md` §11.4 de 001). Las rutas de 002 son disjuntas de las de 001 |
| Mapa de mensajes por `code` (`Record<ErrorCode, …>`, T-F101 de 001) | `name_taken` (409), contrato de 001 v0.6.0 | **T-002-F001**, en la **misma rama** que T-002-B202: regenera los tipos de 001, agrega el mensaje de `name_taken` al mapa y su test, y deja el typecheck del frontend en verde. La detalla el `frontend-architect` en la sección Frontend (el texto del mensaje es una decisión de UI). `invalid_state` tiene un significado por endpoint (plan §8.3) |
| `Money` | Componente compartido de 001 v0.6.0 (ADR-029) | Primera spec con importes: entran las funciones `formatMoney` y `parseMoneyInput` de ADR-023 |
| Nombres personalizados en toda la UI | `GET /settings/labels` (cualquier rol) | Singular, plural y **género** (`masculine`/`feminine`) para concordar ("Nueva obra"). Se piden al arrancar, junto con `/me` |
| Colores de etapa | Tokens `gray`, `blue`, `teal`, `green`, `yellow`, `orange`, `red`, `purple` (DD-7) | La UI decide el color real; el rediseño visual no toca la base |
| Lecturas para el Operador | Todas las `GET` de 002 (DD-11) | Las escrituras devuelven `403` al Operador; `/me` ya trae `permissions` (`settings.manage`) |
| Condiciones comerciales | `GET/PUT /settings/quote-defaults` | Texto libre (un área de texto); las condiciones de pago con porcentajes son de 005 |
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

**Objetivo**: el contrato de 001 está en v0.6.0 (`name_taken`, `Money`) con su código en `httpx` y
el mapa de mensajes del frontend al día (T-002-F001); el validador de tests carga 001 y 002 y **todos
los `$ref` del contrato 002 resuelven**; el módulo `settings` existe y está cableado (todavía sin
rutas); todas las operaciones de 002 figuran como pendientes.

**Orden y dependencias**: T-002-B201 + T-002-B202 + T-002-F001 (una sola rama) → T-002-B203 +
T-002-B204 → T-002-B205. **T-002-B202 es requisito previo de cualquier validación o consumidor del
contrato 002** (T-002-B203, la generación de tipos del frontend): hasta que se mergea, el `$ref` a
`Money` del contrato 002 apunta a un componente que no existe en el contrato de 001 (v0.5.0).

**Prueba independiente**: `make check-all` en verde (backend y frontend, incluido el typecheck); el test de
resolución de referencias de T-002-B203 lista cero `$ref` sin resolver en el contrato 002; el test de
rutas contra el contrato lista las 20 operaciones de 002 como pendientes y ninguna de 001.

**T-002-B201 [T] — `name_taken` y `Money` compartidos** · DD-12, DD-13, ADR-029
- **Red**:

  | Caso | Esperado |
  |---|---|
  | `TestProblemCodesMatchContract` (existente) con `name_taken` en el enum del contrato de 001 | falla hasta que `httpx` tenga el código |
  | `httpx.WriteProblem(w, r, httpx.CodeNameTaken)` | `409`, `type` `/problems/name_taken`, `code` `name_taken`, `title` genérico en español |
  | `contract.CheckSchema("Money", …)` con `{amount_cents: 0, currency: ARS}` y `{9007199254740991, USD}` | ok |
  | Ídem con `9007199254740992`, `-1`, `1.5`, `"100"`, sin `currency`, `currency: "EUR"`, una propiedad extra | error en cada caso |
  | Test de `authz` contra el enum `Permission` del contrato | sin cambios |
  | Typecheck del frontend (`make web-check`) con los tipos de 001 regenerados | falla hasta T-002-F001 (el mapa `Record<ErrorCode, …>` no tiene `name_taken`) |
- **Green**: pasa la tabla.

**T-002-B202 — Contrato de 001 v0.6.0 y `httpx.CodeNameTaken`**: el fragmento de plan §8.2
(versión, línea de historial, `name_taken` en `ErrorCode`, `Money`); `CodeNameTaken` con su status y
título. Sin otros cambios en 001. **En la misma rama va T-002-F001** (sección Frontend): sin ella,
`make web-check` (y por lo tanto `make check-all`) queda en rojo. Es requisito previo de T-002-B203 y de cualquier consumidor del
contrato 002.

**T-002-B203 [T] — Validador con varios contratos, resolución completa de `$ref` y operaciones
pendientes** · DD-13, S-4, R-3. Depende de T-002-B202 mergeada.
- **Red** (unitarios del helper, más el test de rutas de `internal/app`):

  | Caso | Esperado |
  |---|---|
  | `contract.Load()` | carga los dos documentos de `contract.Documents`; los dos pasan `ValidateDocument` |
  | **Resolución completa**: un test recorre todas las referencias del contrato 002 cargado (internas y externas a 001: respuestas, `Problem`, `Money`, `securitySchemes`) | cada una resuelve a un nodo; la lista de `$ref` sin resolver está **vacía** |
  | Ídem con un contrato de 001 de *testdata* sin `Money` (como la v0.5.0) | `Load` falla nombrando la referencia sin resolver (`#/components/schemas/Money`) |
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
  respaldo del plan (fusión en memoria de los componentes de 001) y se anota en el PR; la fila de
  resolución completa vale igual sobre el documento fusionado.

**T-002-B204 — Implementar `contract.Documents`, la carga múltiple, la verificación de resolución de
`$ref` en `Load` y `pendingOperations`** (en el test de rutas de `internal/app`, con las 20
operaciones de 002).

**T-002-B205 — Esqueleto del módulo `settings` y cableado** · DD-1. Paquete con `doc.go`,
`service.go` (`Service`, `NewService` que entra en pánico con una dependencia nil, `RequestMeta`,
errores centinela, `FieldErrors`) y `http.go` (`RegisterRoutes` que todavía no registra nada);
`internal/app/routes_settings.go`; `BuildAPIRouter` recibe `*settings.Service`; `cmd/crm/serve.go` lo
construye; se actualizan las llamadas existentes a `BuildAPIRouter` en los tests (con `nil` donde hoy
pasan `nil`) y el texto de la mutación de `serve_router_mutations_test.go` que reproduce la llamada,
sin cambiar las reglas del guard. Un test chico: `NewService` con cada dependencia nil entra en
pánico.

**Checkpoint Fase 2**: las tres condiciones, en este orden:
1. `make check-all` en verde (`make check` del backend y `make web-check`, **incluido el typecheck y
   los tests del frontend**) con T-002-F001 (el mapa
   de mensajes tiene `name_taken`).
2. El test de resolución completa de T-002-B203 en verde: **cero `$ref` sin resolver** en el
   contrato 002. Recién con esto el contrato 002 se da por validado y puede consumirse (validador de
   los tests HTTP desde la Fase 4, `web/redocly.yaml`).
3. El test de rutas contra el contrato lista las 20 operaciones de 002 en `pendingOperations`.

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
  | `industrytemplate.All()` | dos plantillas en `version` 2 con **exactamente** el contenido de "Plantillas de rubro" de la spec (tabla esperada en el test: nombres, tipos, colores y orden de etapas; categorías por tipo; ítems con unidad; condiciones y validez; nombres por defecto) |
  | `parseCatalog` con un campo desconocido dentro de una etapa / una plantilla sin `stages` / `version` 0 | error en cada caso |
  | `GET /api/v1/industry-templates` | idéntica a la actual: solo `code` y `name` (contrato de 001 sin cambios) |
  | (en `settings`) cada plantilla de `All()` pasa las reglas de T-002-B301: pipeline válido, enumerados, largos, condiciones; nombres únicos (etapas entre sí; categorías por tipo, **incluidas las de sistema**) | ok; una plantilla de *testdata* con la primera etapa `won`, o con una categoría "Cobros de clientes" de ingreso, falla con un mensaje que nombra la plantilla y la regla |
  | INV-9: ningún archivo `.go` fuera de `_test.go`, `testdata/` e `internal/testsupport/` contiene los literales `"generic"` ni `"aluminum_carpentry"` | regla nueva en `internal/testsupport/reporules` (con un caso de *testdata* que la viola) |
- **Green**: pasa la tabla.

**T-002-B304 — Implementar `Template` con contenido y `catalog.json` v2** (plan §11.1; valores de
"Plantillas de rubro" de la spec). Los enumerados son *strings* en `industrytemplate` (no importa
`settings`).

**T-002-B305 [T] — Sembrado en el registro** · FR-006, DD-3, INV-1, INV-4, INV-10, INV-14 de 001
- **Red** (integración: `tenant.Service.Register` real con `settings.TemplateSeeder` real):

  | Caso | Esperado |
  |---|---|
  | Registrar con `aluminum_carpentry` | `tenant_settings`: nombres por defecto, 15 días, condiciones de la plantilla; 6 etapas con nombre, tipo, color y posiciones 1..6; 5 categorías de sistema + 9 de la plantilla con sus tipos; 4 ítems sin precio con su unidad; `tenant.registered` igual que antes; **ninguna** fila de auditoría de configuración |
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

- **Regla de decisión** (fijada antes de medir). La compuerta **pasa** solo si se cumplen las tres
  condiciones:
  1. `seed` p95 ≤ 10 ms;
  2. `hold` p95 (real) − `hold` p95 (noop) ≤ 10 ms;
  3. `hold` p95 (real) **< 140 ms** (target heredado de R-b de 001; R-c exige que el sembrado entre
     en ese presupuesto).

  **Si cualquiera falla**, la compuerta falla: la fase se detiene y no se avanza a la Fase 4 hasta
  que (a) se aplique la salida de R-c (research R-3, rediseño que vuelve al arquitecto y necesita
  aprobación), o (b) el usuario decida explícitamente otra cosa, con la decisión registrada en el
  plan. Ejemplo que **falla**: `seed` p95 = 8 ms, `hold` p95 (noop) = 133 ms y `hold` p95 (real) =
  141 ms (el delta cumple, el absoluto no).
- **Green**: resultado anotado en el plan (estado de la implementación) con los números de las dos
  corridas y la decisión (pasa, o la salida elegida).

**Checkpoint Fase 3**: `make check` en verde + T-002-B307 cumple las tres condiciones de su regla, o
la decisión explícita del usuario está registrada en el plan.

---

### Fase 4 — Historia 1: Etapas de proyecto (P1) — primer slice vertical por HTTP

**Objetivo**: el Administrador crea, renombra, cambia tipo o color, mueve y elimina etapas por la
API; las reglas del pipeline valen también con pedidos concurrentes; el Operador las lee.

**Prueba independiente**: con `curl` y la cookie de un Administrador de una empresa "Genérico":
mover "En curso" a la posición 1, crear "Instalación" antes de "Finalizado", intentar eliminar
"Finalizado" (`409 invalid_state`) y ver el orden final con `GET /stages` como Operador.

**T-002-B401 [T] — Servicio de etapas** · Historia 1 (1 a 5), P-2, P-6, DD-5, DD-6, DD-15, DD-18,
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

**T-002-B403 [T] — Endpoints de etapas** · contrato, FR-007 de 001, FR-008, DD-11, DD-12, INV-11
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

**T-002-B501 [T] — Servicio de categorías** · Historia 3 (1 a 4), FR-003, FR-004, P-8, DD-8,
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

**Objetivo**: el Administrador define la validez y las condiciones por defecto (texto libre) de los
presupuestos y los nombres de Proyecto, Cliente y Presupuesto con su género; cualquier usuario los
lee.

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
  | `GetQuoteDefaults` de `generic` / de `aluminum_carpentry` | `{15, nil}` / `{15, condiciones de la plantilla}` |
  | `ReplaceQuoteDefaults` `{30, "Seña del 50 %"}` | guardado como texto, sin interpretar; `settings.quote_defaults_updated {fields: [validity_days, terms]}` |
  | `ReplaceQuoteDefaults` con `terms` `"   "` | `terms` nil |
  | `ReplaceQuoteDefaults` con `validity_days` 0 | `FieldErrors{validity_days: invalid_value}` |
  | Escrituras como operador | `authz.ErrForbidden` |
  | **Concurrencia**: `ReplaceLabels` y `DeleteStage` de la misma empresa en paralelo (mismo lock, mecanismo E) | las dos confirman, una después de la otra; sin `40P01` |
- **Green**: pasa la tabla.

**T-002-B702 — Implementar los casos de uso de nombres y condiciones** (`preferences.go`; queries
`GetTenantSettings`, `UpdateLabels`, `UpdateQuoteDefaults`, con `LockTenantSettings` primero).

**T-002-B703 [T] — Endpoints de nombres y condiciones** · contrato, FR-008
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

**T-002-B801 [T] — Escenario "herrería" solo con la API** · SC-001, FR-001 a FR-008, INV-9, R-9
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
plan y en esta sección; las vistas de `docs/diagramas/` que listen módulos o tablas, según el
procedimiento de `docs/diagramas/README.md`.

**Checkpoint Fase 8**: `make check` en verde.

---

### Trazabilidad

| Requisito / criterio / decisión | Tareas |
|---|---|
| FR-001 Etapas ordenadas con tipo y color | T-002-B101, T-002-B301, T-002-B401..B404 |
| FR-002 Nombres personalizados | T-002-B101, T-002-B301, T-002-B701..B704 |
| FR-003 Categorías: crear, renombrar, archivar, restaurar | T-002-B101, T-002-B501..B504 |
| FR-004 Categorías de sistema | T-002-B101, T-002-B301, T-002-B305, T-002-B501 |
| FR-005 Catálogo con unidad, precio y moneda | T-002-B101, T-002-B301, T-002-B601..B604 |
| FR-006 Plantillas de rubro | T-002-B303..B307 |
| FR-007 Condiciones y validez por defecto | T-002-B101, T-002-B301, T-002-B701..B704 |
| FR-008 Lectura para todos, escritura solo Administrador | T-002-B403, B503, B603, B703, T-002-B801 |
| Historia 1 (1, 2, 4, 5) / (3, P-2) | T-002-B401, T-002-B403 / T-002-B401 (eliminar); mover proyectos: spec 004 |
| Historia 2 | T-002-B701, T-002-B703 |
| Historia 3 (1 a 4) | T-002-B501, T-002-B503 |
| Historia 4 (1, 3) / (2) | T-002-B601, T-002-B603 / spec 005 |
| Historia 6 | T-002-B701, T-002-B703 |
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
| INV-10 / DD-3 / DD-33 R-b y R-c de 001 (incluida la retención absoluta p95 < 140 ms) | T-002-B305, T-002-B307 |
| INV-11 / DD-11 | T-002-B401, B403, B501, B503, B601, B603, B701, B703, T-002-B801 |
| INV-12 / DD-14 | Filas de auditoría de T-002-B401, B501, B601, B701 |
| INV-13 / DD-15 | T-002-B401, B501, B601, B701 |
| DD-4 *backfill* | T-002-B101 |
| DD-12 / DD-13 contratos, `name_taken` y resolución de `$ref` | T-002-B201..B204, T-002-F001 (frontend, misma rama que T-002-B202) |
| ADR-028 | T-002-B101, T-002-B401 (borrado lógico), T-002-B501 |
| DD-1 fronteras | T-002-B103, T-002-B205 |

---

## Frontend

Autor: `frontend-architect`. Implementa: `frontend-developer`, **una fase por invocación**, en modo
completo (`.claude/dev-mode`). Diseño: [`ui.md`](ui.md) · ADR-030 (*Accepted*, 2026-10-26) · ADR-015
a ADR-023 y ADR-027 de 001.

### Estado (2026-10-26)

**Aprobado (2026-10-26)** junto con `ui.md`: el usuario aprobó P-F1 a P-F4 con las opciones
recomendadas ("ok sigamos con la recomendacion"; `ui.md` §24) y ADR-030 pasó a *Accepted*. Ninguna
fase empezada. Se empieza por **T-002-F001**, que va en la rama única de T-002-B201 y T-002-B202
(Fase 2 del backend). H-1 está resuelto del lado del backend (`ui.md` §25).

Registro de lo aprobado:

| Fecha | Aprobado | Efecto en esta sección |
|---|---|---|
| 2026-10-26 | P-F1: reordenar etapas sin arrastre ("Subir", "Bajar", "Mover a…") | T-002-F202 tal como está |
| 2026-10-26 | P-F2: corregir en los componentes compartidos los hallazgos 1, 2 y 3 de T-F705 de 001 | Entran en T-002-F103 (`ui.md` DD-F13) |
| 2026-10-26 | P-F3: separación ≥ 8 px en las listas nuevas y en Ajustes (NFR-F07 de 001 sin cambios) | T-002-F103 (`SettingsLinkList`) |
| 2026-10-26 | P-F4: los datos sembrados no se renombran solos | T-002-F502 (aclaración en S-18) |

### Convenciones de esta sección

Rigen las de la sección Frontend de [`tasks.md` de 001](../001-empresas-usuarios/tasks.md)
("Convenciones de esta sección"): `[T]` con Red que falla por la razón correcta; **se prueba lo que
el usuario percibe** (consultas por rol y nombre accesible, nunca por clase ni por estado interno);
pantallas con el árbol de rutas real (`createMemoryRouter`), un `QueryClient` nuevo por test y la red
con MSW (`onUnhandledRequest: 'error'`); handlers y fixtures tipados con los tipos generados y con
datos ficticios; **nombres por funcionalidad**, nunca por fase o tarea; versiones y clasificación de
dependencias como en 001 (002 no agrega dependencias). Además:

- **Identificadores** (plan DD-17): `T-002-Fnnn`. En el código:
  `// T-002-F202 (002/INV-F3): stage writes leave one at a time.` Las decisiones e invariantes de 002
  se citan `002/DD-F5`, `002/INV-F3`; las de 001, sin prefijo.
- **Una tarea por rama.** Cada tarea `[T]` incluye su implementación: Red → Green → Refactor en la
  misma rama, con nombre por funcionalidad desde `main` (p. ej. `feat/002-stages-screen`). Las ramas
  de una fase se mergean antes de empezar la siguiente. Excepción: **T-002-F001 va en la rama única
  de T-002-B201 y T-002-B202** (convención de la sección Backend).
- **Rutas del backend**: una fase que consume una ruta de 002 se puede desarrollar y probar con MSW
  antes, pero se **mergea** después de que su fase del backend está en `main` (tabla de dependencias).
- **Archivos de test por funcionalidad**: `lib/money.test.ts`, `lib/terminology.test.ts`,
  `lib/textLength.test.ts`, `lib/removalFocus.test.ts`, `features/configuration/stages/pipeline.test.ts`,
  `StagesPage.test.tsx`, `StagesOrder.test.tsx` (la cola de etapas), `CategoriesPage.test.tsx`,
  `catalog/schemas.test.ts`, `CatalogPage.test.tsx`, `CatalogItemPage.test.tsx`,
  `LabelsPage.test.tsx`, `QuoteDefaultsPage.test.tsx`, `app/terminologyLoading.test.tsx`,
  `test/terminologyLiterals.test.ts`; E2E en `web/e2e/company-configuration.spec.ts` y
  `web/e2e/configuration-accessibility.spec.ts`.
- **Qué se sustituye y qué no**: la tabla de 001 sin cambios. La red con MSW; nunca hooks de datos,
  `QueryClient`, router, React Hook Form, Zod ni Radix. Las funciones puras nuevas
  (`pipeline.ts`, `money.ts`, `terminology.ts`, `removalFocus.ts`) se usan **reales** en los tests de
  pantalla. Nada nuevo se sustituye.
- **Fixtures** (`src/test/configuration.ts`): empresa "Genérico" con las etapas Presupuesto (`open`,
  `blue`), Aprobado (`open`, `teal`), En curso (`open`, `yellow`), Finalizado (`won`, `green`) y
  Perdido (`lost`, `gray`); las 5 categorías de sistema, "Otros ingresos" y 7 de egreso; sin ítems;
  nombres por defecto; validez 15 y sin condiciones. Variante "renombrada": Proyecto → "Obra"/"Obras"
  femenino, Cliente → "Comitente"/"Comitentes" masculino, Presupuesto → "Cotización"/"Cotizaciones"
  femenino. Sesión de Administrador y de Operador.

### Comandos de validación

Los de 001, sin comandos nuevos: `npm run gen:api`, `npm run lint` (incluye la verificación de los
generados), `npm run typecheck`, `npm test`, `npm run build`, **`npm run check` (checkpoint de cada
fase)**, `npm run e2e`, `make web-check` y `make check-all`. Según la sección Backend, `make check`
también corre el typecheck y los tests del frontend.

### Dependencias con el backend

| Fase | Para mergear necesita en `main` | La prueba independiente necesita | Hallazgos |
|---|---|---|---|
| F0 | — (va en la rama única de T-002-B201 y T-002-B202) | — | H-1 **resuelto (2026-10-26)**: T-002-B202 nombra a T-002-F001 en la misma rama, y el punto 1 del checkpoint de la Fase 2 exige `make check` con el typecheck y los tests del frontend (`ui.md` §25) |
| F1 | Checkpoint de la Fase 2 del backend completo: contrato de 001 v0.6.0 (`002.ts` referencia su `Money`) y cero `$ref` sin resolver en el contrato de 002 (T-002-B203) | Backend de 001 | — |
| F2 | Fase 4 (rutas de etapas) | Fases 3 y 4 (empresa sembrada y etapas) | — |
| F3 | Fase 5 (categorías) | Fase 5 | — |
| F4 | Fase 6 (catálogo) | Fase 6 | — |
| F5 | Fase 7 (nombres y condiciones) | Fase 7 | H-2 (rutas con punto en `FieldError.field`): **abierto, no bloquea**, para el `backend-architect` |
| F6 | Fase 8 (`pendingOperations` vacía) | Backend de 002 completo | — |

---

### Fase F0 — Contrato de 001 v0.6.0: `name_taken` en el mapa de errores (viaja con T-002-B202)

**Objetivo**: con el contrato de 001 en v0.6.0, el frontend compila y el mapa exhaustivo de errores
tiene el texto de `name_taken`; `make check` (con el typecheck y los tests del frontend) y
`make check-all` siguen verdes en la rama única de T-002-B201, T-002-B202 y T-002-F001.

**Prueba independiente**: en esa rama, `make check` y `make check-all` en verde;
`web/src/api/generated/001.ts` trae `name_taken` en `ErrorCode` y el esquema `Money`.

**T-002-F001 [T] — `name_taken` en el mapa exhaustivo de errores** · plan DD-12, DD-13, §8.2;
INV-F05 de 001; `ui.md` §12.4, H-1 (resuelto)
- **Rama**: la rama única de T-002-B201 y T-002-B202, después del commit que sube el contrato de 001
  a v0.6.0.
- **Red**:

  | Caso | Esperado |
  |---|---|
  | `npm run gen:api` con el contrato de 001 en v0.6.0 | `generated/001.ts` con `name_taken` en `ErrorCode` y el esquema `Money`; `npm run typecheck` **falla** en `api/errorMessages.ts` porque `Record<ErrorCode, …>` no tiene `name_taken` (el rojo esperado, el mismo que la última fila de T-002-B201) |
  | `messageForError(<409 problem+json con code name_taken>, 'generic')` | título "Ese nombre ya está en uso.", descripción "Elegí otro nombre.", `retryable: false`, sin `requestId` |
  | El mismo error con un contexto de 001 (`'users'`, `'tenant'`) | el mismo texto (las variantes de 002 entran con sus pantallas, T-002-F101) |
  | `errorMessages.test-d.ts` (exhaustividad del mapa) | sigue pasando con la clave nueva |
  | Mutación: quitar la entrada `name_taken` | no compila (se ve fallar y se restaura) |
- **Green**: la entrada `name_taken` en `errorMessages` con ese texto; `generated/001.ts` regenerado y
  versionado; `npm run check`, `make check` y `make check-all` en verde. Ningún otro cambio de
  frontend en esta rama.
- **Refactor**: nada (cambio mínimo, a propósito).

**Checkpoint Fase F0**: `make check` (punto 1 del checkpoint de la Fase 2 del backend) y
`make check-all` en verde en la rama única de T-002-B201, T-002-B202 y T-002-F001.

---

### Fase F1 — Fundacional: contrato de 002 en el cliente, dinero, terminología pura y componentes compartidos

**Objetivo**: los tipos de 002 se generan y se combinan con los de 001; existen las piezas puras
(dinero, terminología con los nombres por defecto, largos, destino del foco), los textos de error por
contexto y los componentes de presentación que usan las pantallas nuevas, con las correcciones
aprobadas en P-F2 y P-F3. Todavía no se consume ninguna ruta de 002.

**Prueba independiente**: `npm run check` en verde y, en el navegador contra el backend, las
pantallas de 001 se operan igual que antes, con los radios de Registro e Invitar redondos, "+ Invitar"
de 44 px de alto, el menú de acciones de Usuarios dentro del viewport a 375 px y las filas de Ajustes
separadas (P-F2, P-F3).

**T-002-F101 [T] — Contrato de 002 en el cliente y piezas puras** · `ui.md` §9.2, §11.3, §12.2,
§12.3, §13.1, §13.3, §16.4; INV-F6, INV-F10, NFR-F5, DD-F3, DD-F11, ADR-030
- **Red**:

  | Pieza | Caso | Esperado |
  |---|---|---|
  | `schema.test-d.ts` | rutas de `Paths001` y `Paths002` | disjuntas (`AssertDisjoint`); `paths` tiene `/stages` y `/me` |
  | `types.test-d.ts` | `CatalogItem['reference_price']` | `Money \| null`, con `Money` de `generated/001` |
  | `queryKeys` | `settings.stages`, `settings.catalogItem('x')` | `['settings','stages']`, `['settings','catalog-items','x']` |
  | `countCodePoints` | `"Obra"`, `"ñ"` × 60, `"👷"`, `""` | 4, 60, 1, 0 |
  | `focusTargetAfterRemoval` | `[a,b,c]` sin `b` / sin `c` / `[a]` sin `a` / id que no está | `c` / `b` / `null` / `null` |
  | `midSentence` | `"Obra"`, `"Orden de trabajo"`, `"Ñandú"` | `"obra"`, `"orden de trabajo"`, `"ñandú"` |
  | | `"OT"`, `"PyME"`, `"obra"`, `""` | sin cambios |
  | `byGender` | `('feminine', { masculine: 'Nuevo', feminine: 'Nueva' })` | `"Nueva"` |
  | `terminologyFrom(DEFAULT_LABELS).project` | — | `singular` "Proyecto", `plural` "Proyectos", `lower` "proyecto"/"proyectos", artículos `el`, `los`, `un`, `del`, `al` |
  | `terminologyFrom` con Proyecto = Obra/Obras femenino | — | `lower` "obra"/"obras"; artículos `la`, `las`, `una`, `de la`, `a la` |
  | `DEFAULT_LABELS` | — | igual a "Comunes a las dos plantillas" de la spec (tres masculinos) |
  | `useTerminology()` provisorio (sin red hasta T-002-F501) | render dentro del shell de test | devuelve `terminologyFrom(DEFAULT_LABELS)` y **no** hace ningún request |
  | `messageForError` | `name_taken` con `'stage'` / `'category'` / `'categoryRestore'` | los textos de `ui.md` §13.1 |
  | | `invalid_state` con `'stage'` / `'category'` | los textos de §13.1 |
  | | `invalid_state` con `'users'` y con `'generic'` | el texto de 001, sin cambios |
  | | `not_found` con `'stage'`, `'category'`, `'catalogItem'` / con `'users'` | los textos de §13.1 / el de 001 |
  | `fieldErrorMessage` | `('name','too_long','stage')` / `('name','too_long','catalogItem')` | "…(máximo 60 caracteres)." / "…(máximo 120 caracteres)." |
  | | `('name','invalid_value')` sin *scope* | "Tiene que tener entre 1 y 120 caracteres." (001 sin cambios) |
  | | `('project.singular','too_long','labels')`, `('validity_days','invalid_value','quoteDefaults')`, `('terms','too_long','quoteDefaults')`, `('unit_label','required','catalogItem')`, `('reference_price.amount_cents','invalid_value','catalogItem')` | los textos de §13.3 |
  | `applyServerFieldErrors` | con *scope* `catalogItem` y `knownFields` que incluye `reference_price.amount_cents` | `setError` en esa ruta, tipo `server`, con el texto del *scope*; devuelve `[]` |
- **Mutación**: `midSentence` que pasa a minúscula sin mirar el resto de la palabra → falla "OT".
- **Green (implementación)**: entrada `'002'` en `web/redocly.yaml` y `generated/002.ts`;
  `schema.ts` con `Paths001 & Paths002`; alias de `types.ts` (§12.2); `queryKeys.settings`;
  `lib/textLength.ts`, `lib/removalFocus.ts`, `lib/terminology.ts`; `api/labels.ts` con
  `useTerminology()` provisorio (sin query: la conecta T-002-F501); contextos nuevos de
  `ErrorContext` y `FieldScope` en `api/errorMessages.ts`.
- **Refactor**: que los máximos por *scope* salgan de una sola tabla (sin repetir números en cada
  variante).

**T-002-F102 [T] — Dinero: carga y formato sin punto flotante** · ADR-023, ADR-029, INV-F5, DD-F8,
S-F2, `ui.md` §17.2
- **Red**:

  | Caso | Esperado |
  |---|---|
  | `parseMoneyInput`: cada fila de `ui.md` §17.2 | el resultado de la tabla, incluido `"90.071.992.547.409,91"` → 9007199254740991 y un centavo más → `too_large` |
  | `formatMoney(450000, 'USD')` | igual a `new Intl.NumberFormat('es-AR', { style: 'currency', currency: 'USD', currencyDisplay: 'code' }).format('4500.00')`; contiene "USD" y "4.500,00" (S-F2) |
  | `formatMoney(5, 'ARS')`, `formatMoney(0, 'ARS')` | contienen "0,05" y "0,00" |
  | `formatMoney(9007199254740991, 'ARS')` | contiene "90.071.992.547.409,91" (exacto) |
  | `formatMoney(-1, …)`, `formatMoney(1.5, …)`, `formatMoney(9007199254740992, …)` | lanza (error de programación: el contrato no lo admite) |
  | `formatMoneyInput(450050)` | `"4.500,50"` |
  | Ida y vuelta: `parseMoneyInput(formatMoneyInput(x))` para 0, 5, 99, 100, 123456, 9007199254740991 | `x` |
- **Mutación**: convertir con `Math.round(parseFloat(…) * 100)` → debe fallar al menos la fila de
  2^53 − 1; si no falla ninguna, se agrega una fila que lo haga antes de seguir.
- **Green (implementación)**: `lib/money.ts` (§16.4). Resultado de S-F2 (formato del código de
  moneda en Node del CI) anotado en el PR.
- **Refactor**: la conversión centavos ↔ texto decimal en una sola función interna.

**T-002-F103 [T] — Componentes compartidos y correcciones aprobadas en P-F2 y P-F3** · P-F2, P-F3
(aprobadas el 2026-10-26), NFR-F3, NFR-F4, DD-F12, DD-F13, RF-7, `ui.md` §14.1, §16.1, §18;
hallazgos 1, 2, 3 y 5 de T-F705 de 001
- **Red** (componentes con Testing Library; E2E con Playwright en Chromium):

  | Caso | Esperado |
  |---|---|
  | `FormDialog` abierto | `role="dialog"` con nombre = título; foco en el primer campo; Esc cierra y el foco vuelve al disparador |
  | `FormDialog` con `pending` | "Cancelar" y el envío deshabilitados; Esc no cierra; el botón dice el `pendingLabel` |
  | `BlockedActionDialog` | `role="alertdialog"` con título y descripción; un solo botón "Entendido", con el foco; al cerrar, el foco vuelve al disparador |
  | `ArchivedSection` con `count` 2 / 0 | botón "Ver archivadas (2)" con `aria-expanded="false"`; al activarlo, `true` y el contenido visible / nada en el DOM |
  | `SettingsLinkList` | una lista con un enlace por ítem, por nombre accesible |
  | `ReadOnlyNotice` | el texto visible, sin `role="status"` ni `role="alert"` |
  | E2E, 375 px: radios de rol (Invitar) y de moneda (Registro) | el control del radio mide ≤ 24 px de alto; su etiqueta, ≥ 44 px (hallazgo 2) |
  | E2E, 375 px: "+ Invitar" y "Cancelar" de Invitar (enlaces con aspecto de botón) | alto ≥ 44 px (hallazgo 1) |
  | E2E, 375 y 320 px: menú de acciones de un usuario con una acción deshabilitada | el menú cabe en el viewport (borde derecho ≤ ancho de la ventana) y su texto no se corta (hallazgo 3) |
  | E2E, 375 px: Ajustes como Administrador | "Datos de la empresa" y "Usuarios" miden ≥ 44 px y están separados ≥ 8 px (P-F3, hallazgo 5) |
- **Green (implementación)**: `components/ui/dialog.tsx` y `components/ui/textarea.tsx` con el CLI de
  shadcn; `FormDialog`, `BlockedActionDialog`, `ArchivedSection`, `SettingsLinkList`,
  `ReadOnlyNotice` (§16.1); S-08 usa `SettingsLinkList` en "Tu empresa"; P-F2 (DD-F13): el radio no
  hereda el mínimo de 44 px de `globals.css` (su objetivo es la tarjeta), `a[data-slot="button"]`
  entra en la regla de 44 px, y `DropdownMenuContent` limita su ancho al disponible y deja saltar el
  texto.
- **Refactor**: que ninguna de las correcciones use clases de la paleta de Tailwind (INV-F23 de 001).

**Checkpoint Fase F1**: `npm run check` y `npm run e2e` en verde + prueba independiente.

---

### Fase F2 — Historia 1: Etapas (P1) — primer slice vertical

**Objetivo**: el Administrador ve el pipeline y agrega, edita (nombre, tipo, color), mueve y elimina
etapas con teclado y en el celular; las acciones que rompen las reglas se explican antes de enviar;
el Operador ve la lista en solo lectura. Ajustes muestra la sección "Configuración" con su primer
enlace.

**Prueba independiente**: con el backend (Fases 3 y 4), registrar una empresa "Genérico" y, solo con
teclado: Ajustes → "Etapas de los proyectos"; mover "En curso" dos veces hacia arriba (se anuncia la
posición y el foco sigue en su menú); agregar "Instalación" antes de "Finalizado"; intentar eliminar
"Finalizado" (aparece la explicación); cambiar "Perdido" a "Cierre ganado" y entonces eliminar
"Finalizado". Como un Operador invitado, la misma lista sin "+ Agregar etapa" ni menús. A 320 px, sin
scroll horizontal.

**T-002-F201 [T] — Reglas del pipeline en el cliente** · Historia 1.4 y 1.5, P-6, plan DD-5 y
T-002-B301, INV-F4, INV-F9, DD-F5, `ui.md` §16.4
- **Red** (unitarios; tipos abreviados `o`/`w`/`l`):

  | Función | Entrada | Esperado |
  |---|---|---|
  | `pipelineViolation` | `[]`, `[o]`, `[o, l]` | `no_won` |
  | | `[w, o]`, `[l, o, w]`, `[l]` | `first_not_open` (se evalúa primero la primera etapa) |
  | | `[o, w]`, `[o, w, l]`, `[o, l, w]`, `[o, o, w, w]` | `null` |
  | `afterDelete` (fixture Genérico) | Finalizado / Aprobado | `no_won` / `null`, con las demás en orden |
  | | Presupuesto (Aprobado `o` segunda) | `null` |
  | | la primera de `[o, w, l]` | `first_not_open` |
  | `afterMove` | Perdido a 1 / En curso de 3 a 1 / Finalizado a 5 | `first_not_open` / `null` con `[En curso, Presupuesto, Aprobado, Finalizado, Perdido]` / `null` |
  | `afterTypeChange` | Finalizado a `o` / Presupuesto a `w` / En curso a `w` | `no_won` / `first_not_open` / `null` |
  | `afterCreate` | `w` en 1 / `o` al final / `o` en 4 | `first_not_open` / `null` / `null` con la nueva en 4 |
  | `moveOptions` (Genérico, En curso) | — | 5 opciones; la 3 marcada `current`; etiquetas "1 (al principio)", "2 (después de «Presupuesto»)", …, "5 (al final)" |
  | `suggestStageColor` | colores del Genérico (`blue`, `teal`, `yellow`, `green`, `gray`) / los 8 usados | `orange` (el primero libre en el orden del contrato) / `gray` |
  | `stageChanges` | mismos valores / solo el nombre con espacios en los extremos / nombre y color | `{}` / `{ name }` sin espacios / `{ name, color }` (nunca `null`) |
  | `stageTypeLabels`, `stageColorNames` | cada valor del enum | su texto (`Record`: un valor nuevo no compila) |
- **Green (implementación)**: `features/configuration/stages/pipeline.ts` y `labels.ts`; tokens
  `--stage-*` en `globals.css` (`ui.md` §19.2) incluidos en `colorTokens.test.ts`.
- **Refactor**: que las cuatro funciones `after*` compartan el reordenamiento a posiciones 1..n.

**T-002-F202 [T] — Pantalla de etapas y entrada en Ajustes** · Historia 1 (1 a 5), FR-001, FR-008,
BR-F1 a BR-F5, BR-F12, INV-F3, INV-F6, INV-F8, INV-F19 de 001, DD-F1, DD-F4 a DD-F6, DD-F10,
`ui.md` §14.1, §14.2, §15.1, §15.2
- **Red** (pantalla con MSW, sesión de Administrador salvo que diga Operador):

  | Interacción | Resultado observable |
  |---|---|
  | Ajustes como Administrador / como Operador | sección "Configuración" con el enlace "Etapas de los proyectos"; el Operador la ve también, con "Podés consultarla; solo un administrador la cambia." |
  | `GET /stages` pendiente | región "Cargando las etapas" con `aria-busy` |
  | `200` (Genérico) | `<h1>` "Etapas de los proyectos"; lista ordenada de 5; "Presupuesto" con "Abierta · Etapa inicial"; "Finalizado" con "Cierre ganado"; el color no es el único dato |
  | `500` con `instance` | "No pudimos cargar las etapas." + código + "Reintentar" |
  | Operador | sin "+ Agregar etapa" ni botones "Acciones para …"; aviso "Solo un administrador puede cambiar las etapas."; nunca "Sin permiso" (INV-F8) |
  | Menú de la primera / de la última etapa (teclado: Enter, flechas) | sin "Subir" / sin "Bajar"; nombre accesible "Acciones para {nombre}" |
  | "Subir" en En curso → `200` con la lista nueva | la fila pasa a la posición 2; durante la espera, "Moviendo…" en la fila; la región viva (presente y vacía antes) dice "«En curso» quedó en la posición 2 de 5."; el foco queda en "Acciones para En curso" |
  | "Subir" en Finalizado hasta la posición 1 (con En curso, Aprobado y Presupuesto arriba, hasta que la regla se rompería) | al intentar dejarla primera: diálogo "No se puede mover «Finalizado» ahí" con "Entendido" (foco); ningún request |
  | "Mover a otra posición…" → posición 1 para En curso → "Mover" | `POST …/move {position: 1}`; mismo resultado observable |
  | "Mover a otra posición…" sin cambiar la posición actual → "Mover" | ningún request; el diálogo se cierra |
  | "Eliminar" en Finalizado | diálogo "No se puede eliminar «Finalizado»" con el texto de `no_won`; ningún `DELETE` |
  | "Eliminar" en Aprobado → "Eliminar" → `204` | confirmación "¿Eliminar la etapa «Aprobado»?"; toast "Eliminaste la etapa «Aprobado»."; la lista se vuelve a pedir; el foco va a "Acciones para En curso" (la siguiente, INV-F6) |
  | Eliminar la última etapa de la lista (con otra `won` antes) | el foco va a la anterior |
  | "+ Agregar etapa" | diálogo "Nueva etapa": foco en "Nombre"; "Abierta" elegida; color "Naranja" (el primero libre); ubicación "Antes de «Finalizado»" |
  | Agregar "Instalación" → `201` | `POST /stages` con `{name, type: 'open', color: 'orange', position: 4}`; toast "Agregaste la etapa «Instalación»."; lista releída; foco de vuelta en "+ Agregar etapa" |
  | Agregar con nombre "presupuesto" | error "Ya hay una etapa con ese nombre." en "Nombre", sin request |
  | Agregar `won` en la posición 1 | error en "Tipo": "La primera etapa tiene que ser abierta…"; sin request |
  | Agregar → `409 name_taken` (otra persona lo creó) | error en "Nombre" con el texto de `stage`; foco ahí; lista releída |
  | Agregar con 61 caracteres (emoji cuenta 1) | error "…(máximo 60 caracteres)."; sin request |
  | "Editar" en Perdido → tipo "Cierre ganado" → `200` | `PATCH` con `{type: 'won'}` solamente; toast "Guardamos la etapa «Perdido»."; la fila dice "Cierre ganado" |
  | "Editar" sin cambios → "Guardar etapa" | ningún request; el diálogo se cierra |
  | "Editar" Finalizado (única ganada) → tipo "Abierta" | error en "Tipo": "Tiene que quedar al menos una etapa de cierre ganado."; sin request |
  | Eliminar → `409 invalid_state` (lista vieja) | diálogo cerrado; toast "No pudimos hacer ese cambio en las etapas." con la descripción de §13.1; lista releída |
  | Cualquier acción → `404` | toast "Esa etapa ya no existe. Actualizamos la lista."; lista releída |
  | Agregar → red caída / `503` | `FormAlert` en el diálogo con el texto de 001; lo escrito se conserva |
  | Agregar → `403 forbidden` (le bajaron el rol) | `/me` pedido de nuevo; con el permiso perdido, la pantalla queda en solo lectura (sin "Sin permiso") |
  | Doble activación de "Agregar etapa" en el diálogo | un solo `POST` |
- **Red — cola de etapas** (`StagesOrder.test.tsx`, INV-F3; `test/gate.ts` de 001 retiene respuestas):

  | Caso | Esperado |
  |---|---|
  | "Subir" en En curso y, con esa respuesta retenida, "Bajar" en Presupuesto | el segundo `POST` sale recién cuando respondió el primero; la lista final es la de la segunda respuesta |
  | `GET /stages` en vuelo (foco de la pestaña) que responde después de un `move` | la lista queda con la respuesta del `move` (la lectura se canceló antes de escribir) |
  | Mutación: quitar el `scope` de `useMoveStage` | falla la primera fila |
- **Green (implementación)**: `StagesPage`, `StageListItem`, `StageForm`, `StageColorOption`,
  `features/configuration/stages/api.ts` (cola `STAGE_CHANGES_SCOPE`, §11.3); ruta
  `settings/stages`; sección "Configuración" de S-08 con su primer enlace. Los textos con
  "proyectos" usan `useTerminology()` (todavía los nombres por defecto).
- **Refactor**: que el manejo de foco tras quitar o mover use `focusTargetAfterRemoval` y una sola
  función de refoco por id; que `StagesPage` no tenga más de un estado de diálogo por tipo.

**Checkpoint Fase F2**: `npm run check` en verde + prueba independiente.

---

### Fase F3 — Historia 3: Categorías (P1)

**Objetivo**: el Administrador agrega, renombra, archiva y restaura categorías de ingreso y egreso;
las del sistema se renombran y no se archivan; el Operador las consulta.

**Prueba independiente**: con el backend (Fase 5): renombrar "Cobros de clientes" a "Cobranzas" (su
menú no ofrece "Archivar"), agregar "Combustible" desde la sección Egresos, archivarla y verla en
"Ver archivadas", restaurarla. Crear otra "Combustible" mientras la primera está archivada y después
intentar restaurar la archivada: aparece el aviso de nombre repetido.

**T-002-F301 [T] — Reglas de presentación de categorías** · Historia 3, BR-F6, BR-F7, INV-F7,
DD-F7
- **Red** (unitarios):

  | Función | Caso | Esperado |
  |---|---|---|
  | `categorySections(categories)` | lista del servidor (Genérico con una archivada) | `{ income: [...], expense: [...], archived: [...] }`, cada una en el orden recibido |
  | `isDuplicateActiveName(categories, name, type, exceptId?)` | "flete" contra "Flete" activa de egreso / de ingreso / archivada / la misma categoría (renombrar sin cambiar) | `true` / `false` / `false` / `false` |
  | `categoryActions(category)` | propia activa / del sistema / archivada | `['rename','archive']` / `['rename']` / `['restore','rename']` |
  | `categoryActions` con `system: true` y `system_key` que el cliente no conoce (simulado) | — | `['rename']` (decide por `system`, INV-F7) |
  | `categoryTypeLabels` | `income`, `expense` | "Ingreso"/"Ingresos", "Egreso"/"Egresos" (`Record`) |
- **Green (implementación)**: `features/configuration/categories/rules.ts` y `labels.ts`.

**T-002-F302 [T] — Pantalla de categorías** · Historia 3 (1 a 4), FR-003, FR-004, FR-008, BR-F1,
BR-F6 a BR-F8, BR-F12, INV-F6, INV-F7, INV-F8, DD-F6, DD-F7, DD-F9, `ui.md` §14.3
- **Red**:

  | Interacción | Resultado observable |
  |---|---|
  | Ajustes | enlace "Categorías de ingresos y egresos" en "Configuración" |
  | `GET /categories?include_archived=true` pendiente / `500` | "Cargando las categorías" / "No pudimos cargar las categorías." + "Reintentar" |
  | `200` (Genérico, "Alquiler" archivada) | `<h2>` "Ingresos" y "Egresos" con sus listas; "Del sistema" en las 5; "Ver archivadas (1)" cerrado |
  | Operador | sin "+ Agregar" ni menús; aviso de solo lectura; "Ver archivadas" se puede abrir |
  | Menú de "Cobros de clientes" | solo "Renombrar" |
  | "+ Agregar" de Egresos | diálogo "Nueva categoría de egreso" con "El tipo no se puede cambiar después."; sin selector de tipo; nombre accesible del botón "Agregar categoría de egreso" |
  | Agregar "Combustible" → `201` | `POST /categories {name: 'Combustible', type: 'expense'}`; toast "Agregaste la categoría «Combustible»."; lista releída; foco en "+ Agregar" de Egresos |
  | Agregar "flete" (existe "Flete" de egreso) | error "Ya hay otra categoría activa del mismo tipo con ese nombre." sin request |
  | Agregar → `409 name_taken` | el mismo error en el campo; foco ahí; lista releída |
  | Renombrar "Cobros de clientes" a "Cobranzas" → `200` | `PATCH {name: 'Cobranzas'}`; toast "Cambiaste el nombre a «Cobranzas»."; foco en el menú de esa categoría (por id, aunque cambie de lugar) |
  | "Archivar" en "Materiales" → `200` | sin confirmación; toast "Archivaste «Materiales». La encontrás en Archivadas."; "Ver archivadas (2)"; el foco va al menú de la categoría siguiente de Egresos (INV-F6) |
  | "Restaurar" en "Alquiler" (archivadas abiertas) → `200` | toast "Restauraste «Alquiler»."; vuelve a Egresos; con Archivadas vacía, la sección desaparece y el foco va a "+ Agregar" de Egresos |
  | "Restaurar" → `409 name_taken` | toast "No pudimos restaurarla: ya hay otra categoría activa del mismo tipo con ese nombre." + "Renombrá una de las dos y probá de nuevo."; sigue archivada |
  | "Archivar" → `409 invalid_state` (caché vieja: era del sistema) | toast "Las categorías del sistema no se pueden archivar."; lista releída |
  | Acción → `404` | toast "Esa categoría ya no está. Actualizamos la lista." |
  | Una categoría `system: true` con una clave nueva (006) | se muestra con "Del sistema" y solo "Renombrar"; ningún error (INV-F7) |
- **Green (implementación)**: `CategoriesPage`, `CategoryListItem`, diálogo de categoría,
  `features/configuration/categories/api.ts`; ruta `settings/categories`; enlace en S-08.
- **Refactor**: compartir con etapas el patrón de fila con menú (sin un componente genérico si
  obliga a más de dos *props* de configuración).

**Checkpoint Fase F3**: `npm run check` en verde + prueba independiente.

---

### Fase F4 — Historia 4: Catálogo (P2)

**Objetivo**: el Administrador crea, edita, archiva y restaura ítems con unidad y precio de
referencia opcional, sin punto flotante; el Operador consulta el catálogo.

**Prueba independiente**: con el backend (Fase 6): crear "Portón corredizo" (unidad, USD 4.500,00) y
"Soldadura" (Otra: "jornada", ARS 25.000), ver "USD 4.500,00 por unidad" en la lista, borrar el
precio de "Portón corredizo" (pasa a "Sin precio de referencia"), archivarlo y restaurarlo desde la
lista. En un celular con teclado numérico de punto, "4500.50" se guarda como 4.500,50.

**T-002-F401 [T] — Formulario de ítem: esquema, cambios y descripción del precio** · Historia 4.1,
P-9, BR-F9, BR-F10, INV-F5, INV-F9, INV-F10, DD-F8, DD-F11, `ui.md` §14.5, §16.3
- **Red** (unitarios):

  | Función | Caso | Esperado |
  |---|---|---|
  | `catalogItemSchema` | nombre vacío / 121 *code points* / 120 | `required` / `too_long` / ok |
  | | "Otra" sin nombre de unidad / con 21 / "Hora" con nombre de unidad | `unit_label` `required` / `too_long` / ok (se ignora) |
  | | importe `""` / `"4.500,50"` / `"abc"` / `"90.071.992.547.409,92"` | sin precio / 450050 / error de formato / "El importe es demasiado grande." |
  | `toCatalogItemCreate` | importe vacío / con importe y USD / descripción `"  "` / unidad "Hora" con etiqueta escrita | `reference_price: null` / `{ amount_cents: …, currency: 'USD' }` / `description: null` / sin `unit_label` |
  | `catalogItemChanges` | sin cambios | `{}` |
  | | solo el precio, de USD 4.500,00 a USD 5.000,00 | `{ reference_price: { amount_cents: 500000, currency: 'USD' } }` |
  | | importe borrado | `{ reference_price: null }` |
  | | de "Otra" (jornada) a "Unidad" | `{ unit: 'unit' }` (el servidor descarta `unit_label`) |
  | | de "Hora" a "Otra" con "jornada" | `{ unit: 'other', unit_label: 'jornada' }` |
  | | descripción borrada | `{ description: null }`; nunca `name: null` ni `unit: null` (INV-F9) |
  | `catalogItemFormValues` | `null` con moneda base USD / un ítem con precio | unidad "Unidad", moneda USD, importe vacío / importe `"4.500,00"` (con `formatMoneyInput`) |
  | `describeCatalogPrice` | sin precio, `square_meter` / USD 450000, `unit` / ARS 2500000, `other` "jornada" | "Por m² · Sin precio de referencia" / "USD 4.500,00 por unidad" / "ARS 25.000,00 por jornada" (con `formatMoney`) |
  | `catalogUnitLabels` | cada unidad | su texto (`Record`) |
- **Green (implementación)**: `features/configuration/catalog/schemas.ts` y `labels.ts`.

**T-002-F402 [T] — Pantallas del catálogo** · Historia 4 (1, 3), FR-005, FR-008, BR-F1, BR-F8 a
BR-F10, BR-F12, BR-F13, INV-F6, INV-F8, DD-F1, DD-F6, DD-F7, DD-F9, `ui.md` §14.4, §14.5
- **Red**:

  | Interacción | Resultado observable |
  |---|---|
  | Ajustes | enlace "Catálogo" en "Configuración" |
  | Lista vacía como Administrador / como Operador | "Todavía no hay ítems en el catálogo." con "Nuevo ítem" / sin el botón, con el aviso de solo lectura |
  | Lista con 3 ítems (uno archivado) | cada tarjeta con nombre y precio descrito; "Ver archivados (1)"; "+ Nuevo ítem" mide ≥ 44 px (aserción de estilo en E2E, T-002-F602) |
  | Operador | sin enlaces en los nombres; la descripción completa visible; sin "Restaurar" |
  | "+ Nuevo ítem" → completar → "Guardar ítem" → `201` | `POST` con el cuerpo de `toCatalogItemCreate`; `replace` a la lista (Atrás no vuelve al formulario); toast "Agregaste «Portón corredizo» al catálogo." |
  | Elegir "Otra" | aparece "Nombre de la unidad" con su ayuda; al volver a "Unidad", desaparece |
  | Importe "4500.50" | se guarda 450050 |
  | `422` con `reference_price.amount_cents` `invalid_value` | "El importe es demasiado grande." en "Importe"; foco ahí |
  | Abrir un ítem desde la lista | S-17 aparece sin *skeleton* (sembrado desde la lista); `<h1>` "Editar ítem" |
  | Entrar directo a `/settings/catalog/{id}` | *skeleton* "Cargando el ítem" y después el formulario |
  | `GET` → `404` | "No encontramos ese ítem" + "Volver al catálogo" |
  | Cambiar solo el precio → `200` | `PATCH` con solo `reference_price`; toast "Guardamos «Portón corredizo»." |
  | "Guardar ítem" sin cambios | ningún request; toast "No hay cambios para guardar." |
  | Cambiar el nombre y tocar "← Catálogo" | aviso "Tenés cambios sin guardar. ¿Salir igual?" ("Quedarme" con el foco) |
  | "Archivar ítem" → `200` | sin confirmación; toast "Archivaste «…». Lo encontrás en Archivados."; aviso "Este ítem está archivado…" con "Restaurar", que recibe el foco; lo escrito en el formulario se conserva |
  | "Restaurar" en el aviso → `200` | toast "Restauraste «…»."; el aviso desaparece; foco en "Archivar ítem" |
  | "Restaurar" en la lista de archivados → `200` | el ítem vuelve a la lista activa; foco según INV-F6 |
  | Operador en `/settings/catalog/new` | "Sin permiso" del guard de 001 |
  | Guardar → red caída | `FormAlert` "No hay conexión…" con "Reintentar"; lo escrito se conserva |
- **Green (implementación)**: `CatalogPage`, `CatalogItemNewPage`, `CatalogItemEditPage`,
  `CatalogListItem`, `CatalogItemForm`, `features/configuration/catalog/api.ts`; rutas de §6.1
  (las de escritura dentro de `RequirePermission`); enlace en S-08.
- **Refactor**: una sola `CatalogItemForm` para alta y edición.

**Checkpoint Fase F4**: `npm run check` en verde + prueba independiente.

---

### Fase F5 — Historias 2 y 6: Nombres personalizados (terminología en toda la app) y condiciones (P3, P2)

**Objetivo**: la terminología se lee del servidor y se aplica en toda la interfaz con sesión,
incluidos los textos de 001; el Administrador edita los nombres (con género) y las condiciones por
defecto; el Operador los consulta.

**Prueba independiente**: con el backend (Fase 7): cambiar Proyecto por "Obra"/"Obras" femenino y
Presupuesto por "Cotización"/"Cotizaciones" femenino; sin recargar, Ajustes dice "Etapas de las
obras" y "Condiciones de las cotizaciones", y Datos de la empresa dice "Se usan en los PDF de tus
cotizaciones."; recargar `/` no muestra "Proyectos" ni un instante. Definir 30 días y un texto de
condiciones, y verlos como Operador. Con la red cortada solo para `/settings/labels`, la app arranca
con los nombres por defecto.

**T-002-F501 [T] — Terminología conectada: carga, espera y textos de 001** · Historia 2, P-3,
ADR-030, BR-F11, INV-F1, INV-F2, INV-F10, DD-F2, DD-F3, RF-5, `ui.md` §9
- **Red**:

  | Caso | Resultado observable |
  |---|---|
  | `test/msw/handlers.ts` sin handler de `GET /settings/labels` y el shell montado | un test de 001 que monta el shell falla por la petición no interceptada (el rojo de RF-5); con el handler por defecto (nombres de la plantilla), pasa |
  | Sesión presente, `/settings/labels` retenido | encabezado y navegación visibles; en `<main>`, el indicador de carga (`role="status"`); el contenido de la pantalla todavía no está (INV-F2) |
  | `/settings/labels` responde con "Obra" | `<main>` pinta la pantalla con "Obra"; nunca aparece "Proyecto" en el DOM antes |
  | `/settings/labels` → `500` | `<main>` pinta con los nombres por defecto, sin aviso |
  | `/settings/labels` → `401` | salida de sesión de 001 (`reason=session_expired`) |
  | Navegar a la app con la sesión ya en caché | `GET /settings/labels` sale una vez por usuario, en paralelo con el chunk del shell (la precarga del guard; no sale durante una salida de sesión, INV-F22 de 001) |
  | Usuarios (estado vacío) e Invitar (rol Operador) con la variante renombrada | "…para que cargue comitentes y obras." / "Carga comitentes, obras, cotizaciones y cobros. No ve finanzas ni ajustes." |
  | Datos de la empresa con la variante renombrada | "Se usan en los PDF de tus cotizaciones." |
  | Los mismos textos con los nombres por defecto | idénticos a los de 001 (los tests existentes de 001 pasan sin cambios) |
  | `terminologyLiterals.test.ts` (INV-F1) | pasa; con un archivo de *fixture* que tiene "Nuevo proyecto" en un JSX, falla nombrando el archivo |
  | Esquema de nombres: singular o plural vacío / 41 *code points* / emoji cuenta 1 | `required` / `too_long` / ok |
- **Green (implementación)**: `api/labels.ts` con `labelsQuery` y `useTerminology()` sobre la caché;
  precarga en `GuardedSession`; espera de `<main>` en `AppShell`; handler por defecto en
  `test/msw/handlers.ts`; los textos de §9.4 con `useTerminology()`; `terminologyLiterals.test.ts`;
  esquema de `features/configuration/labels/schemas.ts`.
- **Refactor**: que `useTerminology()` memoice por referencia de `Labels`.

**T-002-F502 [T] — Pantallas de nombres y condiciones** · Historias 2 y 6, FR-002, FR-007, FR-008,
P-10, P-F4 (aprobada), BR-F1, BR-F11, BR-F13, INV-F8, `ui.md` §14.6, §14.7, §15.3
- **Red**:

  | Interacción | Resultado observable |
  |---|---|
  | Ajustes | enlaces "Nombres personalizados" y "Condiciones de los presupuestos" |
  | S-18 como Administrador | tres grupos "Proyecto", "Cliente", "Presupuesto" con singular, plural y género; "Así se va a ver: «Nuevo proyecto», «Los proyectos cerrados»."; la aclaración de que los nombres de etapas y categorías ya cargados no cambian (P-F4) |
  | Cambiar Proyecto a "Obra", "Obras", Femenino | la vista previa dice "«Nueva obra», «Las obras cerradas»" sin enviar nada |
  | "Guardar nombres" → `200` | `PUT /settings/labels` con las tres entidades; toast "Guardamos los nombres."; sin recargar, `<h1>` de Etapas dice "Etapas de las obras" al navegar; la etapa sembrada "Presupuesto" conserva su nombre |
  | `422` con `project.singular` `too_long` | error "…(máximo 40 caracteres)." en el singular de Proyecto; foco ahí |
  | Salir con cambios | aviso de cambios sin guardar |
  | S-18 como Operador | "Proyecto: Obra / Obras, femenino" (texto, sin campos) y el aviso de solo lectura |
  | S-19 con 15 días y sin condiciones | "Validez" 15; "Condiciones comerciales" vacío; ayuda "0 de 2.000." |
  | Validez "0" / "366" / "abc" / "30" | error "Tiene que ser un número de días entre 1 y 365." en los tres primeros, sin request / ok |
  | Condiciones con 2.001 *code points* (pegadas) | se ve todo lo pegado; al guardar, error "…(máximo 2.000 caracteres)." sin request |
  | Condiciones "   " → "Guardar condiciones" → `200` | `PUT` con `terms: null`; toast "Guardamos las condiciones." |
  | S-19 como Operador | "Validez: 15 días" y "Sin condiciones por defecto." |
  | Título de S-19 con Presupuesto renombrado a "Cotización" femenino | "Condiciones de las cotizaciones"; bajada "Se proponen al crear una cotización. Cambiarlas no toca las que ya creaste." |
  | Guardar → `503` | `FormAlert` con "Reintentar"; lo escrito se conserva |
- **Green (implementación)**: `LabelsPage`, `EntityLabelFieldset`, `QuoteDefaultsPage`,
  `useReplaceLabels`, `features/configuration/quoteDefaults/api.ts` y su esquema; rutas
  `settings/labels` y `settings/quote-defaults`; enlaces en S-08.
- **Refactor**: que la vista previa use `terminologyFrom` y no arme concordancias propias.

**Checkpoint Fase F5**: `npm run check` en verde + prueba independiente.

---

### Fase F6 — Verificación: end-to-end, accesibilidad y performance

**Objetivo**: los flujos de configuración funcionan de punta a punta contra el binario; la
accesibilidad y los presupuestos de 002 están verificados y reportados; la documentación queda al
día. No se construye nada nuevo: se cierran huecos.

**Prueba independiente**: los E2E de T-002-F601 y T-002-F602 en verde y los reportes de T-002-F603
y T-002-F604 adjuntos.

**T-002-F601 [T] — E2E: una herrería configurada desde la interfaz** · SC-001, FR-001 a FR-008
- **Red** (Playwright, Chromium en cada PR y WebKit antes de liberar, contra el binario; un solo
  flujo en `company-configuration.spec.ts`):

  | Paso | Esperado |
  |---|---|
  | Registro con "Genérico" | panel |
  | Nombres: Proyecto → "Trabajo"/"Trabajos" masculino | Ajustes dice "Etapas de los trabajos" |
  | Etapas: renombrar "En curso" → "En taller"; agregar "Colocación" (abierta) antes de "Finalizado"; subir "Colocación" una vez; intentar eliminar "Finalizado" | orden final esperado; explicación de la acción bloqueada |
  | Categorías: agregar "Hierros", "Pintura" y "Electrodos" de egreso; archivar "Alquiler" | listas esperadas |
  | Catálogo: "Reja de ventana" (m², sin precio), "Portón corredizo" (unidad, USD 4.500,00), "Soldadura" (Otra "jornada", ARS 25.000) | tarjetas con el precio descrito |
  | Condiciones: 30 días y un texto | guardadas |
  | Un Operador invitado (flujo de 001) | ve las cinco pantallas en solo lectura; ninguna muestra "Sin permiso" |
- **Green**: pasa, sin cambios de código de producción (si hace falta uno, va con su test en esta
  rama).

**T-002-F602 [T] — E2E de accesibilidad y tamaños** · NFR-F06 de 001, NFR-F3, NFR-F4, `ui.md` §18
- **Red** (`configuration-accessibility.spec.ts`):

  | Caso | Esperado |
  |---|---|
  | axe en S-13 a S-19 y en los diálogos C-03, C-04 y C-05, como Administrador y como Operador | sin violaciones |
  | 320 y 375 px en cada pantalla, con el menú de acciones abierto en una fila de nombre largo | sin scroll horizontal; el menú dentro del viewport |
  | Botones, enlaces con aspecto de botón, disparadores de menú y etiquetas de opciones de 002 | ≥ 44×44 px |
  | Listas de enlaces y de tarjetas de 002 | objetivos vecinos separados ≥ 8 px (P-F3) |
  | Teclado: mover una etapa con el menú | el foco queda en "Acciones para {nombre}" de la misma etapa |
- **Green**: pasa.

**T-002-F603 — Verificación manual de accesibilidad** · NFR-F3, NFR-F7, `ui.md` §18, §19.2
- Checklist: S-13 y S-14 de punta a punta solo con teclado; VoiceOver (macOS o iOS) y, si hay, NVDA
  en mover (anuncio de posición), eliminar y archivar (destino del foco) y en los diálogos; zoom
  200 %; `prefers-reduced-motion`; contraste de los tokens `--stage-*` con una herramienta; "Moviendo…"
  inmediato al toque en un celular. Resultado en el reporte de la fase, con cada hallazgo y su
  corrección (o su decisión). Lo que no se pueda verificar se dice ("no verificado en NVDA").

**T-002-F604 — Performance** · NFR-F1, NFR-F2, NFR-F6, ADR-030, RF-1, RF-8
- Tamaño gzip de cada chunk de S-13 a S-19 (`vite build`); LCP p75 de `/` con la espera de nombres
  (como T-F704 de 001); render de la lista del catálogo con un *fixture* de 1.000 ítems en un Android
  de gama media (o el perfil de CPU de Lighthouse). Si el LCP de `/` pasa de 2,5 s, se aplica la
  salida de RF-1 y se vuelve al arquitecto.

**T-002-F605 — Documentación viva** (`ui.md` §27): estado de cada fase en esta sección; `ui.md` con
lo que cambió al implementar (enmiendas con su aprobación); en `tasks.md` de 001, el estado de los
hallazgos de T-F705 que corrigió T-002-F103 (1, 2, 3 y 5; P-F2 y P-F3).

**Checkpoint Fase F6**: `npm run check` y `npm run e2e` en verde (Chromium; WebKit reportado);
reportes de T-002-F603 y T-002-F604 adjuntos.

---

### Trazabilidad (frontend)

| Requisito / criterio / decisión | Tareas |
|---|---|
| FR-001 Etapas ordenadas con tipo y color | T-002-F201, T-002-F202, T-002-F601 |
| FR-002 Nombres personalizados | T-002-F501, T-002-F502, T-002-F601 |
| FR-003 / FR-004 Categorías y categorías de sistema | T-002-F301, T-002-F302, T-002-F601 |
| FR-005 Catálogo con precio opcional | T-002-F102, T-002-F401, T-002-F402, T-002-F601 |
| FR-007 Condiciones y validez | T-002-F502, T-002-F601 |
| FR-008 Lectura para todos, escritura solo Administrador (UX) | T-002-F202, F302, F402, F502, T-002-F601, T-002-F602 |
| Historia 1 (1 a 5) | T-002-F201, T-002-F202 |
| Historia 2 / P-3 (toda la interfaz) | T-002-F101, T-002-F501, T-002-F502 |
| Historia 3 (1 a 4) | T-002-F301, T-002-F302 |
| Historia 4 (1, 3) | T-002-F401, T-002-F402 |
| Historia 6 | T-002-F502 |
| SC-001 (desde la interfaz) | T-002-F601 |
| Contrato de 001 v0.6.0 (`name_taken`, `Money`), H-1 (resuelto) | T-002-F001 |
| Contrato de 002 v0.1.0, *bundle* multi-spec | T-002-F101 |
| ADR-023 / ADR-029 dinero, INV-F5, DD-F8 | T-002-F102, T-002-F401 |
| ADR-030 terminología, INV-F1, INV-F2, DD-F2, DD-F3 | T-002-F101, T-002-F501, T-002-F604 |
| INV-F3 / DD-F10 cola de etapas | T-002-F202 |
| INV-F4 / DD-F5 reglas del pipeline en el cliente | T-002-F201, T-002-F202 |
| INV-F6 foco tras quitar o mover | T-002-F101, T-002-F202, T-002-F302, T-002-F402, T-002-F602 |
| INV-F7 sin ramificar por `system_key` | T-002-F301, T-002-F302 |
| INV-F8 / DD-F1 solo lectura para el Operador | T-002-F202, F302, F402, F502, T-002-F601 |
| INV-F9 `PATCH` mínimo | T-002-F201, T-002-F401 |
| INV-F10 / NFR-F5 largos en *code points* | T-002-F101, T-002-F401, T-002-F501 |
| DD-F6 diálogos y páginas | T-002-F103, T-002-F202, T-002-F302, T-002-F402 |
| DD-F7 una lectura con archivados | T-002-F302, T-002-F402 |
| DD-F9 archivar sin confirmación | T-002-F302, T-002-F402 |
| DD-F11 / §13 errores por contexto y *scope* | T-002-F101, T-002-F202, T-002-F302, T-002-F402, T-002-F502 |
| P-F1 / DD-F4 reordenar sin arrastre (aprobada) | T-002-F202, T-002-F602, T-002-F603 |
| P-F2 / DD-F13 correcciones en componentes compartidos (hallazgos 1 a 3 de T-F705 de 001; aprobada) | T-002-F103, T-002-F602, T-002-F605 |
| P-F3 / DD-F12 `SettingsLinkList` y separación ≥ 8 px (hallazgo 5 de T-F705 de 001; aprobada) | T-002-F103, T-002-F602, T-002-F605 |
| P-F4 / BR-F11 datos sembrados sin renombrar (aprobada) | T-002-F502 |
| NFR-F1, NFR-F2, NFR-F6 | T-002-F604 |
| NFR-F3, NFR-F4 | T-002-F103, T-002-F602, T-002-F603 |
| NFR-F7 | T-002-F603 |
| RF-5 (handler por defecto de nombres) | T-002-F501 |
| RF-7 (cambios en pantallas de 001) | T-002-F103 |
