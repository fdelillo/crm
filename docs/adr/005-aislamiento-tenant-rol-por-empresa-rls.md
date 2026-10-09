# ADR-005: Aislamiento entre empresas con un rol de PostgreSQL por empresa y RLS forzada

**Status**: Accepted (decisión del usuario; el arquitecto recomienda la alternativa B, ver "Recomendación")
**Fecha**: 2026-09-27
**Origen**: spec 001 (`plan.md` §4.2–4.4, `data-model.md` §3)

> **Nota 2026-09-30 (consecuencia descubierta en la implementación; no cambia la decisión)**: desde
> PostgreSQL 16, `GRANT <rol> TO x` toma un lock sobre el rol concedido que dura hasta el fin de la
> transacción. `provision_tenant_role` hace `GRANT crm_tenant TO crm_t_<hex>`, así que **todos los
> registros de empresa se serializan** sobre `crm_tenant` mientras dura su transacción. El revisor
> de la Fase 1 lo verificó: con una transacción de aprovisionamiento abierta, una segunda cae por
> el `statement_timeout` de `crm_app` (5 s) dentro del `GRANT`. No se puede evitar sin romper la
> atomicidad del registro (INV-14): el rol de la empresa necesita esa membresía antes de insertar
> sus filas. Se acepta a la escala del MVP (los registros son raros) con estas restricciones, que
> fija DD-33 del plan de 001:
>
> - **R-a**: la transacción de registro corre con `SET LOCAL lock_timeout = '2s'`; si se vence,
>   `55P03` → `503 service_unavailable` con `Retry-After`, y no se crea nada (ni el rol).
> - **R-b**: desde `provision_tenant_role` hasta el `COMMIT` no hay E/S de red y el objetivo es
>   p95 < 250 ms; se mide con registros concurrentes en T-B905.
> - **R-c**: el sembrado de plantillas de la spec 002 corre dentro de esa transacción solo si entra
>   en el presupuesto de R-b; si no, el plan de 002 lo rediseña (restricción heredada).
> - **R-d**: `crm tenants reprovision-roles` usa una transacción por empresa y no corre en
>   paralelo consigo mismo.
>
> Es además un argumento concreto para la comparación con la **alternativa B** (sin roles por
> empresa no hay `GRANT` por registro): si T-B905 o el hosting (P-1) lo piden, la salida es un ADR
> que reemplace a este por B. Mejora intermedia evaluada y postergada: un *pool* de roles creados
> de antemano (research R-04c del plan de 001).

> **Nota 2026-09-30 (b) (segunda consecuencia descubierta en la implementación; no cambia la
> decisión)**: en PostgreSQL 18.6, después de que una conexión aprovisiona una empresa (crea
> `crm_t_<hex>` y lo concede a `crm_app` con `SET TRUE`) y hace `COMMIT`, el **primer**
> `SET LOCAL ROLE crm_t_<hex>` desde **otra** conexión del pool a veces falla con
> `42501 permission denied to set role`. La hipótesis del desarrollador es que la lista de roles
> "SET-ables" que cada backend guarda en caché (`roles_is_member_of`, `acl.c`) todavía no procesó
> la invalidación de la membresía nueva. La reproducción (test de integración con registros
> concurrentes) daba 64 a 105 fallas en 600 primeros usos. En producción se vería así: "me
> registro, el siguiente request cae en otra conexión y recibo un 500".
>
> Arreglo vigente (DD-34 del plan de 001):
>
> 1. `platform/db` manda en el mismo viaje de red una lectura de catálogo
>    (`SELECT 1 FROM pg_catalog.pg_auth_members LIMIT 0`) antes del `SET LOCAL ROLE`. Leer el
>    catálogo hace que el backend procese las invalidaciones pendientes. Medido: 0 fallas en
>    1800 usos y 0 en 8 corridas de la suite de integración.
> 2. `InTenantTx` hace **un único reintento** de la transacción completa si el `SET ROLE` inicial
>    falla con `42501` antes de ejecutar nada. Nunca reintenta después de que `fn` corrió, ni para
>    roles de sistema, ni en un `AsTenant` a mitad de transacción. Queda registrado con el log y
>    la métrica `set_role_retry`.
>
> El arreglo depende de un **comportamiento interno no documentado** de PostgreSQL: que leer un
> catálogo procese las invalidaciones pendientes antes del chequeo de `SET ROLE`. Una versión
> futura podría cambiarlo (riesgo R-17 del plan de 001). Si pasa, lo detecta el test de
> reproducción y la métrica `set_role_retry_total`.
>
> Es otra consecuencia de tener roles que cambian en runtime, y por lo tanto **un argumento más a
> favor de la alternativa B** en la comparación que sigue abierta con P-1. Con B no hay membresías
> nuevas por registro ni una caché de membresías que se quede atrás. Dentro de A, el *pool* de roles
> creados de antemano (research R-04c) también lo evitaría: la membresía se concede mucho antes
> del primer uso. Alternativas evaluadas en research R-28 del plan de 001.

> **Nota 2026-10-08 (c) (decimotercera revisión del plan de 001; *Accepted*, aprobada por el usuario el 2026-10-08;
> no cambia la decisión)**: la primera corrida de T-B905 (PostgreSQL 18.6, 10.000
> empresas) no cumplió el target de registros concurrentes: 673 ms de p95 contra < 250 ms con 10
> simultáneos, con 0 `503`. Ese target medía la transacción de registro **incluida la espera por
> el lock** de `crm_tenant`. Con un pool de 8 conexiones, lo medido es ≈ 8 × la retención, así que
> no decía si la retención cumplía (la inferida es de 84–91 ms, sin medir).
>
> El target se redefine (DD-33 R-b y R-e del plan):
>
> - la retención, medida sin contención con 10.000 roles, debe tener p95 < 140 ms;
> - una ráfaga de 10 registros debe terminar con 0 `503`;
> - el pool de producción se fija explícitamente, con (P − 1) × retención ≤ `lock_timeout` / 2.
>
> Lo mide T-B910, con reglas de decisión fijadas antes de medir. El punto (2) de la "Recomendación
> del arquitecto" se evalúa sobre T-B910: si la retención o la ráfaga no cumplen, se escribe el ADR
> que reemplaza a este, con la alternativa B recomendada y el *pool* de roles creados de antemano
> como alternativa.
>
> Hipótesis que confirma o descarta el desglose de T-B910: cada registro cambia `pg_auth_members`,
> y cada conexión recalcula la lista de roles de `crm_app` en su siguiente `SET ROLE`, con un costo
> proporcional a la cantidad de empresas. PostgreSQL 17 eliminó la parte cuadrática de ese cálculo,
> no la lineal. Es la consecuencia 1 de este ADR, ahora con números. Detalle en
> `specs/001-empresas-usuarios/revision-13-t-b905.md`.

> **Nota 2026-10-08 (d) (decimocuarta revisión del plan de 001; *Accepted*, aprobada por el usuario el 2026-10-08; no cambia la
> decisión)**: T-B910 cumplió en las dos corridas (PostgreSQL 18.6, 10.000 empresas, pool de 8):
> retención del lock con p95 de 133,6 y 96,3 ms, contra < 140; 0 `503` en ráfagas de 10 y de 50;
> lecturas, login y registro dentro de sus targets. Es la medición que la consecuencia 1 pedía antes
> de salir a producción (esa consecuencia nombra a T-B905; la que decidió fue T-B910).
>
> La hipótesis de la nota (c) se confirmó en el comportamiento: el costo crece **linealmente** con
> la cantidad de empresas. Cada registro paga dos `GRANT` y dos `SET ROLE`, de ~2,4–3,1 ms por cada
> 1.000 roles, y cada conexión paga una vez, después de un registro, la reconstrucción de su lista
> de membresías. Por eso esta decisión tiene un **techo**: con un pool de 8, la retención deja de
> cumplir su target alrededor de 10.500 empresas (estimación conservadora; rango 10.500–15.000), y
> con dos instancias de 8, alrededor de 3.600–6.600.
>
> El punto (2) de la "Recomendación del arquitecto" pasa a tener disparadores sobre
> `tenant_roles_total` (DD-33 R-f y §12.4 del plan). Al 50 % del techo (hoy 5.000) se escribe el ADR
> que reemplaza a este, con la alternativa B recomendada. Al 75 % (hoy 8.000) ese ADR tiene que
> estar en producción, o el usuario decide seguir con un ADR que lo registre. El techo se recalcula
> con la pendiente medida en la clase de instancia de producción (P-1) y con cada cambio del pool o
> de la cantidad de instancias. Detalle en `specs/001-empresas-usuarios/revision-14-t-b910.md`.

## Contexto

La constitución (principio III) exige que todo dato de negocio pertenezca a una empresa, que toda
consulta filtre por empresa y que se use Row-Level Security como defensa en profundidad, con tests
automáticos. Un fallo de aislamiento es un bug crítico.

RLS necesita saber, dentro de cada sentencia, **qué empresa está operando**. Hay dos formas
habituales de decírselo a PostgreSQL con un pool de conexiones único: una **variable de sesión**
(`SET LOCAL app.tenant_id`) o la **identidad del rol** (`SET LOCAL ROLE`). El usuario eligió la
segunda: un rol de PostgreSQL por empresa.

Además hay flujos que ocurren **antes** de saber la empresa: registro (la empresa aún no existe),
login por email, resolución de la sesión en cada request, reset de contraseña, verificación de
email, aceptación de invitación y el worker de emails. Esos caminos no pueden convertirse en un
bypass del aislamiento.

## Decisión

### Roles

| Rol | Tipo | Propósito |
|---|---|---|
| `crm_owner` | LOGIN | Dueño del esquema; solo migraciones. Sin `BYPASSRLS` |
| `crm_app` | LOGIN | Rol del pool de runtime. **Sin ningún privilegio propio** sobre tablas |
| `crm_tenant` | grupo | Tiene los privilegios DML sobre tablas de empresa |
| `crm_t_<32 hex del UUID>` | por empresa | Miembro de `crm_tenant` con `INHERIT TRUE`; `NOLOGIN`, `NOBYPASSRLS`, no es dueño de nada |
| `crm_auth`, `crm_worker`, `crm_signup` | sistema | Flujos sin empresa conocida; solo columnas de ruteo |
| `crm_provisioner` | sistema | `CREATEROLE`; dueño de la función que crea roles de empresa |

`crm_app` es miembro de los roles de sistema y de cada `crm_t_*` con **`SET TRUE, INHERIT FALSE`**
(opciones por *grant* de PostgreSQL ≥ 16): puede convertirse en ellos con `SET ROLE`, pero no
hereda sus privilegios. Resultado: una query que corre sin cambiar de rol **falla** con
`permission denied` en lugar de filtrar datos (falla cerrada).

### Uso con un pool único

Cada transacción (`platform/db.TxRunner`) hace `BEGIN` → `SET LOCAL ROLE crm_t_<hex>` → queries →
`COMMIT`/`ROLLBACK`. `SET LOCAL` se revierte solo al terminar la transacción, así que ninguna
conexión vuelve al pool con un rol de empresa. `SET ROLE` sin `LOCAL` está prohibido (test de
repositorio T-B011). Una transacción puede pasar de un rol de sistema a **una** empresa, nunca de
una empresa a otra (`ErrTenantAlreadyBound`).

### Políticas

- Todas las tablas de `app`: `ENABLE` y **`FORCE ROW LEVEL SECURITY`**.
- Política `tenant_isolation` (`FOR ALL TO PUBLIC`):
  `USING/WITH CHECK (tenant_id = (SELECT app.current_tenant_id()))`.
- `app.current_tenant_id()` (`STABLE`, `SECURITY INVOKER`) devuelve el UUID codificado en
  `current_user` si el nombre sigue el patrón `crm_t_<32 hex>`; si no, `NULL` (sin filas).
- **Las políticas no nombran roles de empresa.** El mapeo rol → empresa vive en una sola
  función.
- Además, cada query sqlc filtra por `tenant_id` con parámetro explícito (defensa en
  profundidad, INV-04).

### Creación del rol al registrar una empresa

- Función `provisioning.provision_tenant_role(uuid)`: `SECURITY DEFINER`, dueña
  `crm_provisioner`, `search_path` fijo, `EXECUTE` solo para `crm_signup`. Crea el rol si no
  existe (sin `LOGIN`, `BYPASSRLS`, `CREATEROLE`, `SUPERUSER`), lo hace miembro de `crm_tenant` y
  lo concede a `crm_app` (`SET TRUE, INHERIT FALSE`). Idempotente.
- Se llama **dentro de la transacción de registro**, que luego hace `SET LOCAL ROLE` al rol
  recién creado e inserta la empresa y su administrador bajo la RLS de esa empresa. `CREATE ROLE`
  es transaccional: si el registro falla, el rol no queda.
- `crm_app` no tiene `CREATEROLE` ni es miembro de `crm_provisioner`.

### Privilegios sobre tablas futuras

- `ALTER DEFAULT PRIVILEGES FOR ROLE crm_owner IN SCHEMA app GRANT SELECT, INSERT ON TABLES TO
  crm_tenant`: toda tabla nueva queda accesible a todas las empresas (vía el grupo) **sin tocar
  los N roles**.
- `UPDATE` y `DELETE` se conceden explícitamente en cada migración (una tabla inmutable no los
  recibe por accidente).
- `REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC` por defecto.

### Flujos previos a conocer la empresa

Patrón único: **fase 1** con un rol de sistema que solo ve columnas de ruteo (privilegios por
columna + política `USING (true)` limitada a ese rol) para averiguar el `tenant_id`; **fase 2**
con `SET LOCAL ROLE` a la empresa para todo lo demás, bajo RLS.

| Flujo | Rol de fase 1 | Qué puede ver |
|---|---|---|
| Registro | `crm_signup` | Nada; solo ejecutar la función de aprovisionamiento |
| Sesión en cada request | `crm_auth` | `sessions(id, tenant_id, token_hash)` |
| Login, pedido de reset | `crm_auth` | `users(id, tenant_id, email)`, `login_throttles` |
| Tokens (reset, verificación, invitación) | `crm_auth` | `user_tokens(id, tenant_id, token_hash, purpose)` |
| Worker de emails | `crm_worker` | `outbox_messages(id, tenant_id, status, next_attempt_at, created_at)` |
| Limpieza | `crm_worker` | `DELETE` solo de filas vencidas (la política lo limita) |

Hashes de contraseña, nombres, roles, estados, destinatarios y contenidos de emails **solo** se
leen en fase 2. Un test fija el conjunto exacto de privilegios por columna de cada rol de sistema
(T-B109).

## Fundamento

- **Falla cerrada**: con `crm_app` sin privilegios, olvidar el cambio de rol produce un error
  visible, no una fuga silenciosa.
- **Identidad en la base**: cada sentencia corre como el rol de su empresa; `pg_stat_activity`,
  los logs de PostgreSQL (`%u`) y los bloqueos muestran de qué empresa es cada operación.
- **Límites por empresa posibles** en el futuro (`ALTER ROLE crm_t_… SET statement_timeout`).
- **Reversibilidad**: como las políticas solo dependen de `app.current_tenant_id()`, cambiar de
  estrategia afecta a esa función y a `TxRunner`, no a las tablas ni a las queries.
- **`CREATEROLE` acotado**: la aplicación solo puede crear roles a través de una función que
  recibe un `uuid` tipado; desde PostgreSQL 16 un rol con `CREATEROLE` solo administra los roles
  que creó y no puede otorgar atributos que no tiene (como `BYPASSRLS`).

## Alternativas consideradas

- **B. Rol único `crm_tenant` + `SET LOCAL app.tenant_id`** (variable de sesión; la política usa
  `current_setting('app.tenant_id', true)::uuid`). Con `crm_app` igualmente sin privilegios,
  ofrece la **misma** falla cerrada y la **misma** protección ante un filtro olvidado, sin
  `CREATEROLE`, sin roles por empresa y sin operación extra. No fue la elección del usuario; es
  la recomendada por el arquitecto (ver abajo) y es el destino si el hosting no permite A.
- **C. Solo filtro en la aplicación**: viola el principio III.
- **D. Esquema por empresa**: migraciones multiplicadas por la cantidad de empresas; `search_path`
  es otra variable manipulable.
- **E. Base por empresa**: pool y migraciones por empresa; costo desproporcionado para pymes.
- **Conectarse directamente como el rol de cada empresa (pool por empresa)**: miles de pools;
  inviable.
- **Funciones `SECURITY DEFINER` por cada lookup de fase 1**: elimina hasta la posibilidad de
  enumerar emails con un bug, a costa de un rol dueño y una política por función; descartada por
  complejidad, queda como endurecimiento posible.

## Consecuencias

### Lo que se gana

- Falla cerrada, trazabilidad por empresa en la base, reversibilidad a la alternativa B.

### Lo que se acepta (costos reales)

1. **Cantidad de roles**: un rol por empresa, más ocho fijos. Con N empresas, `crm_app` es
   miembro de N roles. En PostgreSQL 16 se reportaron `GRANT` y `SET ROLE` muy lentos con miles
   de roles (cálculo de membresías de costo cuadrático), luego corregido en el código de
   catálogo; además, cada registro modifica `pg_auth_members` e invalida la caché de membresías de
   todos los backends. **Hay que medirlo** en la versión exacta de producción con 10.000 roles
   (T-B905) antes de salir.
2. **Planes**: PostgreSQL re-analiza las sentencias preparadas con RLS cuando cambia el rol actual;
   con muchas empresas alternando en las mismas conexiones se reutilizan menos planes. A la escala
   del MVP es despreciable, pero es un costo que la alternativa B no tiene.
3. **Seguridad de la versión**: CVE-2026-14666 (planes cacheados que ignoraban cambios de membresía
   de roles en políticas RLS) exige el último *minor* de PostgreSQL 18.
4. **Hosting**: requiere un PostgreSQL donde un rol no superusuario tenga `CREATEROLE`, se puedan
   crear funciones `SECURITY DEFINER` y se pueda restringir qué roles se conectan. Algunos
   servicios gestionados limitan la creación de roles por SQL: hay que verificarlo con el
   proveedor concreto (pregunta abierta P-1 del plan de 001).
5. **Backups y restores**: los roles son del clúster; `pg_dump` de la base **no** los incluye.
   Respaldo completo = `pg_dump` + `pg_dumpall --roles-only`, o backup físico del clúster.
   Restaurar en otro clúster = bootstrap + restore + `crm tenants reprovision-roles`
   (idempotente). Las políticas no nombran roles, así que no hay que editarlas.
6. **Entornos**: dos entornos en el mismo clúster compartirían los roles `crm_t_*`; se exige un
   clúster por entorno.
7. **Tests**: los roles creados en tests persisten en el clúster del contenedor; cada corrida usa
   un contenedor nuevo y cada test crea empresas con UUID aleatorio. Los tests **deben** conectarse
   como `crm_app` (un superusuario ignora RLS y haría pasar los tests en falso).
8. **Operación**: inventario de roles vs. empresas como métrica (`tenant_roles_total`); un rol
   faltante produce 500 con un log que lo nombra (runbook en el plan de 001, §12.3).
9. **Complejidad para el equipo**: ocho roles fijos, un bootstrap fuera de goose, una función
   `SECURITY DEFINER`. Mitigado con tests de catálogo que detectan cualquier desvío.
10. **Límite que no cambia**: una inyección SQL con ejecución arbitraria bajo `crm_app` puede
    hacer `SET ROLE` a cualquier empresa. Es el mismo límite que tiene B; la defensa es no tener
    SQL dinámico (ADR-003).

### Recomendación del arquitecto

La alternativa B da la misma protección práctica con menos operación. Se implementa A por decisión
del usuario, con dos salvaguardas: (1) la pregunta P-1 (hosting) debe resolverse antes de elegir
proveedor, y el bootstrap se prueba contra ese proveedor en la Fase 0 (T-B012); (2) si el
benchmark T-B905 no cumple los targets, se escribe un ADR que reemplace a este por la alternativa
B, que solo cambia `app.current_tenant_id()` y `TxRunner`.
