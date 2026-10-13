# Decimosexta revisión: auditoría de seguridad de `b945373`

**Fecha**: 2026-10-12 · **Autor**: `backend-architect` · **Estado**: *Accepted*, aprobada por el usuario el 2026-10-12 con todos los defaults: 3 = B (DD-46), 4 = A (DD-47), 5 = valores propuestos (DD-48), 6 = B (DD-49); contrato v0.5.0. **Enmienda H-16** (*Accepted*, decisión del usuario del 2026-10-13): el reenvío de la verificación sin cupo por usuario responde `429` con `Retry-After` (§3.7); el contrato sigue en v0.5.0.

**Origen**: auditoría de seguridad terminada sobre `main` en `b945373` (skill `security-audit`,
perfil standard, corrida `crm-run-1`). Fue solo lectura de fuente y no se ejecutó nada. Dejó 0
vulnerabilidades confirmadas, 6 candidatos `needs_validation`, 1 rechazado y 1 unidad bloqueada.
Artefactos fuera del repo: `~/Documents/Codex/2026-10-10-podes-pasarme-el-status-del-proyecto/security-audit/crm/run-1/`
(`REPORT.md`, `NEEDS-VALIDATION.md`, `findings.json`, `coverage-ledger.json`).

**Verificado contra** el checkout `test/001-phase-9-followups` (`b945373` + `43eeacc`; la
decimoquinta revisión solo cambia tests y texto, así que el código de producción es el auditado):

- `internal/app/phase6.go`, `internal/app/api.go`, `internal/authz/authz.go`;
- `internal/identity/service.go` (`ResolveSession`, `Me`), `users.go` (`Invite`,
  `withManagedUser`, `ChangeRole`, `Deactivate`, `Reactivate`, `withInvitation`,
  `AcceptInvitation`), `login.go`, `token_flows.go`, `store/queries.sql`, `store/users.sql`;
- `internal/platform/password/password.go`, `internal/platform/ratelimit/ratelimit.go`,
  `internal/platform/outbox/dispatcher.go` (`pollInterval`, `batchSize`, `Run`, `RunOnce`) y
  `store/worker.sql`;
- `internal/tenant/http.go`, `internal/tenant/service.go` (`Register`);
- `internal/identity/emails/templates/*.txt`;
- tests: `internal/identity/users_integration_test.go` (`usersFixture`,
  `TestRoleChangesAndLastAdmin`), `token_flows_integration_test.go` (`gatedLoginHasher`),
  `tenant_lock_integration_test.go` (`gatedResetHasher`), `login_integration_test.go`,
  `internal/testsupport/fixture/company.go`, `internal/testsupport/pgtest/pgtest.go`;
- `plan.md` (DD-1 a DD-43, INV-01 a INV-34, §8 a §16), `contracts/openapi.yaml` v0.4.3,
  `data-model.md` §2.2, ADR-007, ADR-013, `spec.md`.

Yo tampoco ejecuté nada: cada trace se verificó leyendo el código. Los casos Red de la Fase 10
son la validación dinámica que la auditoría no pudo hacer. Corren en CI contra PostgreSQL 18 con
testcontainers, que es justo el entorno que le faltaba a la auditoría. Los `validation_plan.local`
de la auditoría son el punto de partida de cada test; donde los cambio, digo por qué.

**Reabre** (todo *Proposed*): DD-1 (nota), DD-4 y DD-13 (enmiendas, solo si se aprueba la
decisión 3), DD-9 (cupos nuevos y ubicación de las constantes); DD-44 a DD-49 nuevas; INV-35 a
INV-37 nuevas; `plan.md` §4.5, §8, §9.1, §9.3, §10.1, §10.3, §11.1, §12.1, §12.3, §12.4, §14.2
(P-1, P-2), §15 (R-20 a R-23), §16 y §18; contrato v0.5.0; `data-model.md` §2.2; nota en ADR-007;
`tasks.md`, Fase 10 (T-B1001 a T-B1010).

**No contradice ningún ADR aceptado.** ADR-013 ya prevé "reglas más finas que un permiso [que] se
implementan en el servicio del módulo, además del permiso de ruta": la revalidación del actor
(DD-44) y la exigencia de email verificado para invitar (DD-46) son exactamente eso, y las dos
consultan la matriz con `authz.Can` en lugar de comparar roles. ADR-007 recibe una nota fechada
porque su consecuencia aceptada ("bajo ataque, agrega latencia al login") se quedaba corta: hoy
agrega latencia a todos los requests autenticados (§2).

---

## Resumen

| # | Candidato | Trace | Decisión | Diseño | Tareas | Contrato |
|---|---|---|---|---|---|---|
| 1 | `admin-actor-authorized-outside-tenant-lock` | Confirmado. Alcanza también a `Invite`, que le permite al actor crear un admin nuevo que controla | **Arreglo** | DD-44, INV-35 | T-B1001, T-B1002 | No cambia |
| 2 | `login-argon2-inside-db-transaction` | Confirmado. También `AcceptInvitation` y `ConfirmPasswordReset` esperan el semáforo con una conexión tomada | **Arreglo** | DD-45, INV-36, nota en ADR-007 | T-B1003, T-B1004 | No cambia |
| 3 | `signup-unverified-global-email-claim` | Confirmado, con un agravante que la auditoría no ve: el reenvío de la verificación no tiene cupo por usuario | **Decisión del usuario**. Default: invitar exige email verificado (opción B) | DD-46, INV-37, enmiendas a DD-4 y DD-13, R-23 | T-B1009, T-B1010 | **Sí** (v0.5.0) |
| 4 | `invite-reserves-global-email-without-ownership` | Confirmado. Además, el texto de DD-1 dice lo contrario que el contrato y el código | **Decisión del usuario**. Default: riesgo aceptado (opción A) | DD-47, nota en DD-1, R-20 | — | No cambia |
| 5 | `unthrottled-admin-invitations-starve-global-queue` | Confirmado. Según el código, el outbox drena como mucho 10 mensajes por ciclo de 2 s | **Arreglo**; los valores los decide el usuario | DD-48, R-22 | T-B1005, T-B1006 | **Sí** (`429`) |
| 6 | `signup-unbounded-persistent-tenant-roles` | Confirmado. El impacto depende de cuántos prefijos controle el atacante | **Decisión del usuario**. Default: cupo global de registros más alerta (opción B) | DD-49, R-21 | T-B1007, T-B1008 | Solo una descripción |

Con todos los defaults, el contrato pasa a **v0.5.0** (un valor nuevo en `ErrorCode`). Sin la
decisión 3, alcanza con **v0.4.4**. El detalle está en §9.

---

## 1. Un administrador degradado o desactivado con un request en curso

### 1.1 Verificación del trace

El trace es correcto, línea por línea:

- `identity.Authenticate` resuelve la sesión en su propia transacción
  (`service.go:153`, `ResolveSession`) y deja un `authz.Principal` con el rol leído en ese momento
  (`service.go:181`). Esa transacción termina antes de que corra el handler.
- `authz.RequirePermission` decide con ese rol, sin volver a la base (`authz.go:109`).
- `ChangeRole`, `Deactivate` y `Reactivate` entran en `withManagedUser` (`users.go:149`), que
  toma el lock de la empresa (`LockUsersTenant`), bloquea al **objetivo** y valida **su**
  transición. Del actor no vuelve a mirar nada.
- `setUserRole` (`users.go:131`) solo llama a `protectLastAdmin` cuando baja a `operator`. Subir a
  `admin` no tiene ningún control además del permiso de ruta.
- `Reactivate` (`users.go:228`) escribe el estado nuevo del objetivo sin mirar al actor.

Hay dos cosas que la auditoría no dice:

1. **`Invite` tiene el mismo hueco** (`users.go:58`: toma `LockUsersTenant` y nunca mira al actor).
   Es la variante más útil para un atacante. Un admin degradado no necesita restaurar su propio
   rol si su request en curso puede invitar a un email suyo como `admin`: la invitación se acepta
   después, sin ninguna sesión del actor.
2. **El comportamiento está fijado por un test.** `TestRoleChangesAndLastAdmin`
   (`users_integration_test.go:288–297`) baja a `p` a operador y después, con el mismo `p` (ya
   viejo), espera `ErrLastAdmin` en la línea 292 y un ascenso propio **exitoso** en la 295. Ese
   test afirma el hueco, y DD-44 lo cambia a propósito (T-B1001).

### 1.2 Por qué hay que arreglarlo

Lo que importa no es una carrera de milisegundos entre dos administradores honestos. Importa el
momento en que el diseño más necesita que funcione: **un Administrador desactiva a otro porque
sospecha de él**, o porque su sesión fue robada. Quien controla esa sesión controla también
cuándo termina su request. El body de un `PUT` se lee **después** de autenticar y autorizar
(`phase6.go:102`), así que el atacante puede mandar las cabeceras, retener el body (hasta el
`ReadTimeout` de 15 s, §10.3) y soltarlo cuando vea que lo bajaron. Ese request vuelve a
promoverlo, lo reactiva o invita a un admin nuevo. INV-11 ("pasar un usuario a `disabled`
revoca…; la resolución de sesión verifica en cada request") promete que la desactivación corta
el acceso, y hoy un request retenido a propósito lo esquiva.

### 1.3 Decisión: DD-44 (revalidación del actor bajo el lock de la empresa)

**Toda operación de administración de usuarios** (invitar y reinvitar, cambiar rol, desactivar y
reactivar) **vuelve a leer la sesión y al actor inmediatamente después de tomar el lock de la
empresa** (`LockUsersTenant`), antes de buscar al objetivo, y rechaza si ya no lo autorizan:

| Lo que se lee (una query, `GetSessionUser` por `Principal.SessionID`) | Resultado si falla |
|---|---|
| La sesión existe, no está revocada ni vencida (absoluto e inactividad) y es de `Principal.UserID` | `identity.ErrUnauthenticated` → `401 unauthenticated` (borra la cookie) |
| El usuario está `active` | `identity.ErrUnauthenticated` → `401` |
| `authz.Can(rol leído, authz.SettingsManage)` | `authz.ErrForbidden` → `403 forbidden` |
| `Principal.SessionID == uuid.Nil` (llamador sin sesión) | `identity.ErrUnauthenticated`: falla cerrada |

**Por qué funciona.** Toda baja de rol o de estado de un usuario toma el **mismo** lock de la
empresa (INV-10, DD-40). En `READ COMMITTED`, cada sentencia ve lo confirmado antes de empezar,
así que la lectura posterior al lock ve cualquier degradación o desactivación que se confirmó
antes que esta operación. Una que se confirme después queda ordenada después, y la operación fue
legítima cuando corrió. El lock de la empresa es el **punto de linealización** de la autoridad del
actor. Sin el lock, la relectura solo achica la ventana.

**Firmas** (el cuerpo lo escribe el desarrollador):

```go
// internal/identity (no exportadas)

// sessionValid es la regla única de "sesión vigente" que usan ResolveSession y la
// revalidación del actor: no revocada, no vencida (absoluto e inactividad) y usuario active.
func sessionValid(row store.GetSessionUserRow, now time.Time, idle time.Duration) bool

// requireActor relee la sesión y al actor dentro de tx, después de LockUsersTenant (DD-44).
// Devuelve la fila leída para que Invite la use en DD-46 (email verificado).
func (s *Service) requireActor(ctx context.Context, q *store.Queries, p authz.Principal,
    perm authz.Permission) (store.GetSessionUserRow, error)
```

`withManagedUser` e `Invite` llaman a `requireActor(ctx, q, p, authz.SettingsManage)` en la
sentencia siguiente a `LockUsersTenant`. `writeUserError` (`internal/app/phase6.go`) mapea
`identity.ErrUnauthenticated` → `401` y `authz.ErrForbidden` → `403`. La respuesta `401` borra
la cookie igual que la de `Authenticate`, y las dos ya están en el contrato de las cuatro
operaciones. Cada rechazo deja un log `WARN` con `security_event=actor_revalidation_failed`,
`tenant_id`, `user_id` y `reason` ∈ {`session`, `status`, `permission`}, sin datos personales.

**No agrega locks.** La relectura es un `SELECT` sin `FOR …` sobre `sessions` y `users`, que no
espera locks de fila (MVCC) ni cambia el orden de DD-40. No se bloquea al actor: el lock de la
empresa ya serializa todo lo que podría cambiarle el rol o el estado.

**Alcance deliberado.** Quedan afuera:

- **Las escrituras de la Fase 7** (`PATCH /tenant`, subir y quitar el logo). Ninguna concede ni
  restaura autoridad. Un request en curso de un admin recién degradado produce como mucho **una**
  escritura sobre los datos de la empresa, autorizada al empezar el request y auditada con su
  actor. Revalidar ahí obligaría a `tenant` a leer sesiones de `identity` por una interfaz nueva,
  para un efecto que no escala.
- **`ListUsers`**, que solo lee.
- **Las revocaciones que no toman el lock de la empresa** (cierre de sesión, reset de
  contraseña). Si se confirman después de la relectura, la operación ya estaba linealizada antes.

La regla para las specs siguientes es esta: **toda operación que concede o restaura autoridad**
(roles, estado, invitaciones, y en el futuro claves de API o permisos delegados) revalida al actor
bajo el lock de la empresa.

### 1.4 Alternativas descartadas

- **Bloquear también la fila del actor** (`FOR NO KEY UPDATE`). No gana nada, porque el lock de la
  empresa ya serializa las bajas, y suma un lock más al orden de DD-40, con el caso borde de actor
  igual a objetivo (dos locks de la misma fila en la misma transacción).
- **Revalidar solo el usuario (rol y estado), sin la sesión.** Sirve para no tocar el fixture de
  los tests, que arma un `Principal` sin sesión. Pero deja pasar el caso desactivar y reactivar
  durante el request (sesión revocada, usuario `active` otra vez) y obliga a mantener dos
  definiciones de "autorizado". La sesión ya trae el usuario en la misma query (`GetSessionUser`).
- **Cortar los requests en curso al desactivar** (cancelar contextos por sesión). Necesita un
  registro de requests por sesión en memoria y no sirve con más de una instancia.
- **Aceptar el riesgo** con el argumento de DD-41. DD-41 acepta una sesión que **no autentica**
  mientras el usuario esté `disabled`. Acá el efecto es lo contrario: se recupera autoridad.

### 1.5 Invariante

**INV-35**: toda operación de administración de usuarios (invitar y reinvitar, cambiar rol,
desactivar, reactivar) relee la sesión y al actor **después** de tomar el lock de la empresa de
INV-10 y **antes** de buscar al objetivo. Sesión vigente y usuario `active`, o `401`.
`authz.Can(rol, settings.manage)`, o `403`. Se rompe sin que falle la compilación si alguien mueve
la relectura antes del lock o la saca de una operación nueva.

### 1.6 Tests (T-B1001) y mutaciones

El punto de partida es el `validation_plan.local`: request de A por el router real, con el body
retenido en la primera lectura, mientras B degrada a A. Agrego la variante de servicio, que no
depende de HTTP y usa el mecanismo E de T-B604, ya probado y determinista. Las filas están en
`tasks.md`, T-B1001.

| Mutación | Qué falla |
|---|---|
| Quitar `requireActor` de `withManagedUser` | Ascenso propio, reactivación propia y desactivación por un actor desactivado |
| Llamarlo **antes** de `LockUsersTenant` | Las mismas filas: la lectura ve todavía al actor admin y después se bloquea |
| Revalidar solo el rol (sin sesión ni estado) | Reactivación propia, actor desactivado, sesión cerrada |
| Quitarlo de `Invite` | Invitación de un admin por un actor degradado |
| `writeUserError` sin el mapeo nuevo (cae en `500`) | La fila HTTP (el contrato no admite `500` ahí con ese `code`) |

`TestRoleChangesAndLastAdmin` cambia: las líneas 292 y 295 pasan a esperar `authz.ErrForbidden`,
y para seguir el test se vuelve a ascender a `p` con el `Principal` de `u` (admin activo desde la
aceptación de la línea 285). `usersFixture` arma el `Principal` con
`SessionID = company.Rows["sessions"]` (el fixture ya inserta una sesión). Un test que adelante
el reloj falso más allá de la vida de esa sesión y después administre usuarios crea una sesión
nueva con `CreateSession`.

---

## 2. El login retiene una conexión del pool mientras espera el semáforo del hasher

### 2.1 Verificación del trace

Correcto:

- `Login` (`login.go:104`) abre `InSystemTx`, que toma una conexión del pool (`db.go:161`),
  asegura y bloquea la fila del throttle (`FOR UPDATE`), y recién entonces llama a `VerifyDummy`
  (línea 123) o a `Verify` (línea 139). Con contraseña vieja, además llama a `Hash` (línea 163).
- `hasher.Verify` (`password.go:85`) **espera el semáforo** (`acquire`, línea 63) con la conexión
  ya tomada.
- `cmd/crm/serve.go` arma un solo pool y un solo hasher de 4 cupos para todo el proceso.

Además del login, otros dos flujos derivan dentro de una transacción: `AcceptInvitation`
(`users.go:315`, `Hash` dentro de `withInvitation`) y `ConfirmPasswordReset`
(`token_flows.go:193`, `Hash` dentro de `withToken`). El registro hashea antes de abrir la
transacción (`tenant/service.go:121`) y está bien.

### 2.2 Por qué hay que arreglarlo

Sea P el tamaño del pool (8 en producción, DD-33 R-e) y H el de los cupos del hasher (4). Hoy,
con H + (P − H) = 8 logins simultáneos, el pool queda entero en manos de logins: 4 derivan y 4
esperan el semáforo con su conexión. Desde ese momento, **todo request autenticado de cualquier
empresa** espera el pool, porque `ResolveSession` abre una transacción en cada request. Una sola
IP con la ráfaga de DD-9 (20 de una vez) ya llega: retiene el pool unas ⌈20 / 4⌉ = 5 rondas de
derivación. Con muchas claves de rate limit es sostenible: un /56 de IPv6 tiene 256 prefijos /64,
cada uno con su cupo (DD-32). La duración de una derivación en la clase de instancia de producción
**no está medida** (P-1), así que no doy segundos.

ADR-007 aceptó que "bajo ataque, [el semáforo] agrega latencia al login". El acoplamiento con el
pool convierte eso en latencia para **toda la aplicación**, y eso no se aceptó nunca. Arreglarlo es
barato.

### 2.3 Decisión: DD-45 (cupo del hasher antes que la conexión del pool)

**Orden de recursos: primero el cupo del hasher, después la conexión.** Es la misma regla que el
orden de locks de DD-40, aplicada a dos recursos de Go: quien necesita los dos los toma siempre en
el mismo orden. Así nadie espera uno teniendo el otro.

`password.Hasher` suma `Acquire`, que reserva un cupo y devuelve un `Slot`. Los métodos del `Slot`
derivan sin volver a esperar el semáforo, así que se pueden llamar con una transacción abierta:

```go
// internal/platform/password

// Slot es un cupo reservado del semáforo del Hasher (DD-45). Sus métodos nunca esperan el
// semáforo: quien tiene un Slot puede abrir una transacción y derivar dentro sin retener una
// conexión del pool mientras espera. Release es idempotente.
type Slot interface {
    Hash(plain string) (string, error)
    Verify(plain, encoded string) (ok bool, needsRehash bool, err error)
    VerifyDummy(plain string)
    Release()
}

type Hasher interface {
    // Hash, Verify y VerifyDummy toman y sueltan un cupo cada vez: solo para quien no retiene
    // una conexión (el registro hashea antes de su transacción).
    Hash(ctx context.Context, plain string) (string, error)
    Verify(ctx context.Context, plain, encoded string) (ok bool, needsRehash bool, err error)
    VerifyDummy(ctx context.Context, plain string)
    // Acquire espera un cupo o ctx.Done(). Lo usa todo flujo que deriva dentro de una transacción,
    // antes de abrirla.
    Acquire(ctx context.Context) (Slot, error)
}
```

Los métodos del `Slot` no reciben `context.Context`. Hoy el contexto solo sirve para cortar la
espera del semáforo, porque `argon2.IDKey` no lo acepta. Sin espera, un `ctx` sería un parámetro
que nadie usa, y en Go eso es ruido.

Uso en `identity` (patrón, sin cuerpo):

- `Login`: `slot, err := s.hasher.Acquire(ctx)` **antes** de `InSystemTx`, `defer slot.Release()`,
  y dentro de la transacción `slot.VerifyDummy`, `slot.Verify` y, si hace falta rehash,
  `slot.Hash`. Un cupo alcanza para las dos derivaciones del rehash, que son secuenciales.
- `AcceptInvitation` y `ConfirmPasswordReset`: igual, con `slot.Hash`.
- Si `Acquire` devuelve error (contexto cancelado), el flujo vuelve sin haber tocado la base: sin
  fila de throttle, sin auditoría, sin contador. El handler lo trata como cualquier cancelación
  (§9.2).

**Consecuencia.** Las conexiones retenidas por flujos que derivan son como mucho H, y siempre
quedan P − H para el resto (hoy 4 de 8). Bajo ataque, el login sigue lento (consecuencia aceptada
de ADR-007), pero `/me` y las demás rutas no.

**Lo que no cambia**: DD-7 (el throttle se sigue tomando `FOR UPDATE` y el límite de 5 sigue
siendo exacto), INV-13 (mismo costo de hash en todos los caminos), el orden de locks de la base y
`password.NewHasher(4)`.

**Residual aceptado.** Varios intentos simultáneos contra **el mismo** email toman cada uno un cupo
y después esperan el lock del throttle, así que un atacante concentrado en un email reduce el
paralelismo del hasher para los demás logins. Sigue afectando solo al login (ADR-007), no retiene
más de H conexiones y lo acota el rate limit por IP.

### 2.4 Alternativas descartadas

- **Verificar fuera de la transacción, en dos fases** (fase 1: throttle y hash guardado,
  `COMMIT`; derivar; fase 2: relock y decidir). Saca la derivación de la base por completo, pero
  rompe DD-7: N intentos concurrentes contra un email evalúan N contraseñas antes de que la fase 2
  vea el bloqueo. Se puede tapar descartando en la fase 2 el resultado de los intentos que llegan
  bloqueados, pero el login de la Fase 4 se reescribe entero, con sus tests de concurrencia, para
  ganar H conexiones que ya sobran.
- **Un semáforo propio en `identity`**, del tamaño de H, tomado antes de la transacción. No cambia
  la interfaz de `password`, pero son dos semáforos para un solo recurso, y dentro de la
  transacción el flujo todavía puede esperar el del hasher si el registro lo está usando. El
  invariante no se puede enunciar como "nunca se espera el semáforo con una conexión tomada".
- **Un pool separado para la autenticación.** Aísla, pero DD-33 R-e cuenta P como la suma de los
  pools: cambia la cuenta del techo de ADR-005 y suma configuración.
- **Plazo en la espera del semáforo** (p. ej. 2 s). Acota la retención pero no la elimina: con
  ráfagas sostenidas, el pool sigue tomado.
- **Aceptar.** Ver §2.2.

### 2.5 Invariante y nota en ADR-007

**INV-36**: ningún flujo espera el semáforo del hasher con una conexión del pool tomada. Los que
derivan dentro de una transacción reservan el cupo (`Hasher.Acquire`) **antes** de abrirla y
derivan con el `Slot`. Así, las conexiones retenidas por derivaciones son como mucho H (4) < P (8).
Se rompe sin que falle la compilación si alguien llama a `s.hasher.Verify(ctx, …)` dentro de un
`InSystemTx`, o si sube H o baja P sin mirar esta cuenta (§16).

ADR-007 recibe una nota fechada, porque la decisión (argon2id con semáforo) no cambia. La nota
registra la consecuencia que faltaba y la regla de orden.

### 2.6 Tests (T-B1003) y mutaciones

El punto de partida es el `validation_plan.local`: pool de 2 conexiones, hasher fake de 1 cupo
con una compuerta, dos logins y un `ResolveSession` con plazo de 250 ms. Lo hago determinista sin
plazos como criterio principal: el test afirma `pool.Stat().AcquiredConns() == 1` mientras el
segundo flujo espera el cupo (hoy es 2), y conserva el `ResolveSession` de 250 ms como señal
observable. Filas en `tasks.md`, T-B1003.

| Mutación | Qué falla |
|---|---|
| `Acquire` dentro del callback de `InSystemTx` en `Login` | H1 (`AcquiredConns == 2`, `ResolveSession` vence) |
| Lo mismo en `AcceptInvitation` o en `ConfirmPasswordReset` | H2 o H3 |
| `Login` llama a `s.hasher.VerifyDummy(ctx, …)` (el que espera) dentro de la transacción | H1: el fake falla el test si un flujo de `identity` usa un método que espera |
| `Slot.Verify` vuelve a tomar el semáforo | P1 (se cuelga y vence el plazo del test) |
| `Acquire` no cuenta contra el semáforo | P5 |
| `Release` no idempotente | P3 |

**Tests existentes que hay que adaptar** (sin cambiar lo que prueban): `gatedLoginHasher`
(`token_flows_integration_test.go`), `gatedResetHasher` (`tenant_lock_integration_test.go`, que
usan los mecanismos U y V de T-B604) y los dos `countingHasher` (`identity` y `tenant`). Los gated
tienen que interceptar el `Slot`: sobrescriben `Acquire` y devuelven un `Slot` con la compuerta.
Si no se adaptan, esos tests no fallan en silencio: se cuelgan esperando `entered`, que nunca se
cierra. `phase9bench.prehashed` no cambia, porque el registro sigue usando `Hasher.Hash`.

---

## 3. Registro con un email ajeno: *squatting* y cuenta pre-secuestrada

### 3.1 Verificación del trace

Correcto en cada paso. `Register` solo valida la sintaxis del email (`tenant/service.go:97`),
inserta al admin con la contraseña del atacante (`:146`) y crea una sesión (`:157`). Nada de
`authz`, `identity` ni `tenant` mira `email_verified_at` fuera de los flujos de verificación.
`Invite` no tiene compuerta. El reset llega a la cuenta del atacante (`token_flows.go:41`) y solo
revoca las sesiones de ese usuario (`:212`). Si el segundo admin del atacante desactiva al
usuario con el email de la víctima, el reset no envía nada (`:55`).

**Agravante que la auditoría no menciona en este candidato** (sí en una nota de endurecimiento del
ledger): `POST /auth/email-verification/resend` solo tiene el cupo por IP (20 de ráfaga y 1 cada
3 min), sin cupo por usuario (`token_flows.go:261`). El atacante que registró el email de la
víctima puede mandarle "verificá tu email" una y otra vez. Esto importa para cualquier arreglo
basado en la verificación, porque **el clic de la víctima verifica la cuenta del atacante**.

### 3.2 Choque con DD-4 y DD-2

- **DD-2** (email único global) hace que reclamar un email lo reserve en todo el sistema.
- **DD-4** y **P-2** ("la verificación no bloquea nada"), que eligió el usuario, hacen que el
  reclamo dé una sesión de Administrador sin probar que el email es propio.
- **DD-19** y **DD-21** (el `409` del registro sugiere el reset) guían al dueño real hacia la
  cuenta que armó el atacante.

Las tres son decisiones aceptadas. Cualquier arreglo cambia al menos una, y por eso decide el
usuario.

### 3.3 Modelo de amenaza

- **Pre-secuestro** (lo grave). El atacante registra `dueño@empresa`, invita a un email suyo como
  `admin` y acepta. Meses después, el dueño intenta registrarse, recibe el `409`, sigue el reset y
  queda como Administrador de la empresa del atacante. El atacante conserva su admin con sesión y
  ve todo lo que el dueño cargue: clientes y, con las specs siguientes, dinero. Depende de que el
  dueño no mire `Usuarios`.
- ***Squatting* con bloqueo.** Igual, pero el admin del atacante desactiva al usuario de la
  víctima. El reset no envía nada, el registro da `409` y el dueño no tiene salida por sí mismo.
- ***Squatting* simple.** El email queda reclamado. El dueño se recupera por el reset, pero hereda
  la empresa que armó el atacante: nombre, plantilla y **moneda base** (DD-15: no se edita).

### 3.4 Alternativas

| Opción | Qué es | Qué cierra | Costo | Qué queda |
|---|---|---|---|---|
| **A. Aceptar** | Documentar en DD-4 y R-23, más el runbook | Nada | Solo texto | Las tres variantes |
| **B. Invitar exige email verificado** *(default)* | (1) `Invite` exige `email_verified_at` del actor, leído en la relectura de DD-44, antes de buscar el email: `403 email_not_verified`. (2) Confirmar un reset verifica el email si no lo estaba (el token llegó a esa dirección: el mismo argumento de DD-13 para la invitación). (3) Cupo por usuario del reenvío de la verificación: 3 de ráfaga y 1 cada 20 min (como el del reset por email); sin cupo, `202` sin enviar. (4) El email de verificación avisa que confirmarlo le da acceso a quien creó la cuenta | El pre-secuestro y el bloqueo: el atacante no puede sumar un segundo usuario, así que no le queda nadie dentro y no puede desactivar al dueño. El *squatting* pasa a recuperarse por el reset (que ahora además verifica) | Chico: una condición en `Invite`, una columna más en el `UPDATE` del reset, un limiter en memoria y un texto. Contrato: un valor nuevo en `ErrorCode` (v0.5.0). UI: aviso y botón "Reenviar verificación" en la pantalla de invitar. Spec: una clarificación (la verificación habilita invitar). Fricción: el dueño verifica antes de invitar | (a) Si el dueño **confirma** el email de verificación de una cuenta que no creó, el atacante queda verificado y puede invitar: vuelve el pre-secuestro. Lo acotan (3) y (4). (b) El dueño que recupera por el reset hereda nombre, plantilla y moneda base. (c) El email sigue reclamado hasta que el dueño haga el reset |
| **C. B + verificar exige la sesión del dueño de la cuenta** | `confirmEmailVerification` pasa a requerir la cookie, y el token tiene que ser del usuario de la sesión. Quien no conoce la contraseña no puede verificar | También el residual (a) de B: el clic del dueño ya no verifica la cuenta del atacante | Medio: cambia una operación pública (`security: sessionCookie`, `401`, token de otro usuario → `400 token_invalid`), la UI de `/verify-email` pasa a "ingresá y seguimos", con el token guardado mientras tanto, y un enlace abierto en otro navegador obliga a iniciar sesión | (b) y (c) de B |
| **D. Registro en dos pasos** | El registro no crea nada: manda un enlace; la empresa, el rol y la sesión se crean al confirmarlo | Todo: no hay reclamo sin prueba. Resuelve además el candidato 6 de raíz | Alto: rehace la Fase 3 (`signup` → `202`, endpoint nuevo de confirmación, registro pendiente con email guardado y vencimiento), revierte P-2, cambia SC-001 (el panel recién después de abrir el email) e INV-14 | La UX del registro |

**Recomendación: B.** El riesgo serio es el pre-secuestro, porque da acceso persistente a los
datos de un negocio real. B lo cierra con un cambio chico, salvo que el dueño confirme un email de
una cuenta que no creó. (3) y (4) bajan esa probabilidad sin fricción para el uso normal: quien se
registra recibe un email y lo confirma una vez. C cierra también eso, a cambio de fricción en el
paso de verificar. Lo dejo como salida si aparece el caso (disparador en R-23). D es el diseño
"correcto" de manual, pero cuesta una fase y revierte una decisión del usuario por un ataque
dirigido que B ya vuelve inútil.

**Por qué el gate va en `Invite` y no en `Reactivate`.** `Reactivate` puede reemitir una
invitación (usuario que nunca aceptó), pero un actor sin verificar no puede tener a nadie que
reactivar. Solo el fundador de una empresa llega sin verificar (los invitados se verifican al
aceptar, DD-13), y mientras no verifique no puede invitar. La verificación no se revierte. INV-37
lo enuncia sobre las invitaciones de un Administrador y T-B1009 caracteriza el caso.

**Orden.** El gate va después de la relectura de DD-44 y **antes** de buscar el email. Así, un
admin sin verificar no tiene el oráculo de `409 email_taken` (lo mismo que INV-12 pide para el
permiso).

### 3.5 Decisión propuesta: DD-46 (solo con la opción B)

Los puntos (1) a (4) de la tabla. DD-4 se enmienda ("la verificación solo habilita invitar") y
DD-13 también ("confirmar un reset también verifica"). La relectura de DD-44 ya trae
`email_verified_at` en la misma query, así que el gate no agrega accesos a la base.

**Firmas:**

```go
// internal/identity
var ErrEmailNotVerified = errors.New("identity: actor email not verified") // invitar → 403 email_not_verified
```

**INV-37**: ninguna invitación nueva ni reemitida por un Administrador sale de un actor con el
email sin verificar. La reemisión por un pedido de reset (DD-20) no tiene actor y no cuenta.

### 3.6 Tests (T-B1009) y mutaciones

El punto de partida es el `validation_plan.local` (pasos 1 a 6), invertido: con B, cada paso que
hoy funciona tiene que fallar del modo previsto. Las filas, en `tasks.md`.

| Mutación | Qué falla |
|---|---|
| Quitar el gate | V1, V7 |
| Gate después de `FindInvitedEmail` o del `INSERT` | V2 (devuelve `409 email_taken`) |
| El reset no verifica el email | V5 y el último paso de V7 |
| El reset pisa un `email_verified_at` que ya existía | V6 |
| Sin cupo por usuario en el reenvío, con la clave por IP, o respondiendo `202` sin enviar (enmienda H-16, §3.7) | V8b (los pedidos llegan desde IPs distintas) |
| Cupo por usuario antes del de IP | V8f, V8g |
| `Retry-After` fijo | V8c |

**Fixtures.** El admin de `fixture.NewCompany` y el de un registro nuevo no tienen el email
verificado. Todo test que invita (`usersFixture`, los tests HTTP de la Fase 6, los fixtures de la
Fase 8 y `fixture/e2e`) lo verifica antes: `ConfirmEmailVerification` con el token del outbox, o
un `UPDATE` en el fixture de `identity`. Lo detectan como Red los mismos tests.

### 3.7 Enmienda H-16 (2026-10-13, *Accepted*: decisión del usuario)

**Origen**: hallazgo H-16 del `frontend-architect` (`ui.md` §27.2, revisión 4, aprobada el
2026-10-13). El usuario decidió sumarlo a la Fase 10 como enmienda de esta revisión, sin abrir otra.

**Cambio en DD-46 (3).** Sin cupo por usuario, `POST /auth/email-verification/resend` responde
`429 rate_limited` con `Retry-After` en lugar de `202` sin enviar. El valor son los segundos,
redondeados hacia arriba, hasta el próximo token del cupo por usuario. El cupo no cambia: 3 de
ráfaga y 1 cada 20 min, con la clave `user_id`. Sí cambia dónde vive: deja de estar dentro de
`ResendEmailVerification` y pasa a `ratelimit.MiddlewareBy(limiter, userKey)`, después del cupo por
IP y de `Authenticate`. La clave sale del `Principal`, igual que la de empresa de DD-48. El cupo por
email del reset vive en `identity` porque su clave sale del body.

**No reabre ninguna enumeración:**
- La ruta es autenticada y la clave es el usuario de la sesión. El `429` solo le dice a quien tiene
  la sesión que esa misma cuenta pidió reenvíos hace poco. No dice si otro email existe ni en qué
  estado está otra cuenta.
- El cupo corre antes del handler, así que el `429` no depende de que el email esté verificado.
  Tampoco es un oráculo del estado propio, que además `/me` ya expone.
- INV-13 y DD-39 tratan el reset, que es público y se pide por email. Ahí el `202` uniforme evita
  confirmar que una cuenta existe. El cupo por email del reset sigue respondiendo `202` y no cambia.
- R-23 no cambia, porque el cupo es el mismo. El atacante que le reenvía la verificación a la
  víctima ahora sabe cuánto esperar, pero ya podía calcularlo.

**No contradice DD-46 (3).** Cambia la respuesta y el lugar del cupo, no el propósito (acotar los
reenvíos a la víctima) ni los valores. El texto del 2026-10-12 queda en DD-46 como versión
reemplazada.

**El cupo por IP de DD-9 no cambia.** Sigue en 20 de ráfaga y 1 cada 3 min, con la clave de DD-32,
compartido con `confirm`, antes de `Authenticate`, y cuenta todo pedido que llega a la ruta. Un
pedido que recibe `429` por usuario ya gastó su token de IP, igual que antes con el `202` y como en
DD-48 y DD-49.

**Si se agotan los dos, manda el de IP.** Corre primero, antes de autenticar, y responde con su
`Retry-After` sin llegar al cupo por usuario, que no se gasta. Si se reintenta después de ese plazo,
puede llegar el `429` del cupo por usuario con el suyo. `Retry-After` significa "no antes de", no
promete éxito, y con los dos cupos de DD-48 ya pasa lo mismo.

**Alternativas descartadas:**
- **Mantener `202` sin enviar** (versión del 2026-10-12): protege una enumeración que en una ruta
  autenticada no existe, y obliga a la UI a la línea genérica de DD-F30.
- **Dejar el cupo dentro de `ResendEmailVerification`, con un error tipado**
  (`*identity.RateLimitedError{RetryAfter}`, que el handler mapea a `429`, como `LockedError` en
  el login): suma un tipo de error y un mapeo para escribir la misma respuesta que ya escribe
  `ratelimit`.
- **Devolver como `Retry-After` el mayor de los dos plazos**: exige autenticar antes del cupo por
  IP, y entonces un request sin cupo de IP abriría la transacción de `ResolveSession`, algo que DD-9
  evita. La otra forma es consultar el cupo por usuario sin gastarlo, una operación nueva en
  `Limiter`. Las dos cuestan demasiado para un caso raro: 20 pedidos desde una IP y 3 reenvíos del
  mismo usuario en pocos minutos.

**Trade-offs aceptados:**
- Con los dos cupos agotados, la primera espera puede quedarse corta: como mucho 180 s, y después
  llega la del usuario.
- Un usuario ya verificado que llama a la ruta gasta cupo, aunque la UI no se lo ofrece.
- Un pedido que termina en `503` también gasta, porque el cupo cuenta éxitos y rechazos, como en
  DD-48.

**Contrato: sigue en v0.5.0.** El `429` (`RateLimited`, con `Retry-After`) ya estaba declarado en
`resendEmailVerification` por el cupo por IP, así que cambian la descripción y la entrada del
historial, no un esquema ni un enum. Los tipos generados del cliente no cambian. v0.5.0 no está
publicada (esta revisión no se mergeó en `main` ni se implementó), y una 0.5.1 dejaría en el
historial una 0.5.0 que nunca existió en `main`. Si la rama se hubiera mergeado antes de la
enmienda, sería v0.5.1 *patch*, con el criterio de 0.4.1.

**Invariantes**: no agrega ni altera ninguna.

**Tests**: V8 de T-B1009 pasa de test de servicio a HTTP + contrato y se abre en V8a a V8i. Las
mutaciones están en `tasks.md`. La implementación está en T-B1010.

**Lo que le pide al frontend** (lo decide el `frontend-architect`):
- Retirar DD-F30, es decir, la línea "Si lo pediste varias veces, esperá unos 20 minutos." en el
  `202` del reenvío.
- El `429` del reenvío puede venir del cupo por IP o del cupo por usuario, con la espera exacta.
- El caso de los dos cupos agotados (dos esperas seguidas).
- Cerrar H-16.

---

## 4. Una invitación reserva un email ajeno sin consentimiento

### 4.1 Verificación del trace

Correcto. `Invite` inserta el email en la empresa A antes de que nadie acepte (`users.go:68`),
`Deactivate` conserva el email (`users.go:215`), `users_email_key` es global sin predicado de
estado, el reset de un `disabled` no envía nada, y el registro y la invitación de otra empresa dan
`409`. `users_integration_test.go:142–147` lo fija.

**Hallazgo de documentación.** DD-1 dice en su trade-off: "Un email invitado queda reservado
globalmente **hasta que se acepte o se desactive**". El contrato (`inviteUser`: `email_taken`
también para desactivados) y el código dicen que desactivarlo **no** lo libera. Gana el código
(el contrato es canónico). DD-1 queda desactualizada y se corrige con una nota (§4.4).

### 4.2 Modelo de amenaza

Un Administrador autenticado (con B de §3, además con el email verificado) invita al email de un
tercero y lo desactiva, o solo lo deja invitado. Ese tercero no puede registrar su propia empresa
con ese email: el registro da `409`. Si lo desactivaron, tampoco puede recuperar nada. Si quedó
invitado, la única salida es aceptar y entrar como usuario de la empresa de un desconocido, de la
que no puede irse.

No hay robo de cuenta ni acceso a datos: es una **denegación dirigida** del alta de una dirección
concreta. El que la hace queda identificado en la auditoría (`user.invited` y `user.deactivated`
con su `actor_user_id` y su empresa), y con DD-48 no puede hacerlo en volumen.

### 4.3 Alternativas

| Opción | Qué es | Costo | Qué queda |
|---|---|---|---|
| **A. Aceptar** *(default)* | DD-47: riesgo aceptado. Nota en DD-1. Fila en el runbook. Disparador: el primer caso real o la spec de varias empresas por usuario | Solo texto | La denegación dirigida, sin salida para la víctima |
| **B. Liberar el email al desactivar un invitado que nunca aceptó** | La desactivación de un `invited` sin contraseña borra la fila o libera su email | Medio a alto. Borrar la fila o actualizar `users.email` (columna con índice único) toma `FOR UPDATE` implícito, que DD-40 prohíbe por los deadlocks con el `FOR KEY SHARE` de las FK: hay que reabrir ese análisis y R0 de T-B604. También cambian P-3 (ese usuario ya no se reactiva, se reinvita), el contrato (`email_taken`, `deactivateUser`, `reactivateUser`), privilegios de `crm_tenant` y los tests que lo fijan | Mientras está `invited`, el email sigue bloqueado |
| **C. Separar identidad y membresía** | La invitación es una oferta de membresía que no reserva la identidad de nadie | Alto. Es el cambio que DD-2 ya anuncia para cuando un usuario pueda estar en varias empresas | Nada de este candidato |

**Recomendación: A**, con la vista puesta en C. El daño es acotado y atribuible, no escala con
DD-48 y no da acceso a nada. B cuesta reabrir DD-40, una decisión que costó dos tandas cerrar
(novena y décima), para una variante (la desactivación) de algo que sigue abierto igual
(la invitación pendiente). La solución de fondo es C, y ya está anunciada en DD-2: cuando llegue
la spec de varias empresas por usuario, este candidato desaparece. El disparador está en R-20.

### 4.4 Decisión propuesta: DD-47 y nota en DD-1

**DD-47** registra el riesgo aceptado, por qué y cuándo se revisa. **Nota en DD-1**: el trade-off
correcto es "un email invitado queda reservado globalmente mientras exista la fila: aceptar la
invitación lo convierte en el usuario, y desactivarlo **no** lo libera (contrato `inviteUser`,
DD-47)". El texto aceptado no se reescribe: va como corrección fechada, igual que en DD-40.

**Monitoreo.** No hay una métrica razonable. La señal es un reporte del tipo "me dice que mi email
ya existe y el reset no me llega". El runbook (§12.3) dice qué mirar: un DBA busca el email y, si
es un `disabled` sin `password_hash` de otra empresa, es este caso. **Hoy no hay herramienta para
liberarlo**, y liberarlo a mano sin diseño rompería DD-40 o la auditoría. Si aparece el caso, se
diseña (disparador).

---

## 5. Invitaciones sin límite: el outbox global y el remitente

### 5.1 Verificación del trace

Correcto. `POST /users/invitations` no tiene limiter (`phase6.go:74`; el limiter de invitaciones
solo cubre `preview` y `accept`, `phase6.go:21` y `:34`). Cada reinvitación encola otro mensaje
(`token_flows.go:108`) sin deduplicar. `LockDueMessage` ordena globalmente por
`next_attempt_at, id` (`worker.sql:6`). `RunOnce` toma como mucho `batchSize = 10` por ciclo y
`Run` espera `pollInterval = 2 s` entre ciclos (`dispatcher.go:18–19`, `:146`). **Del código sale
un drenaje máximo de unos 5 mensajes por segundo por proceso.** Es una cota del código, no una
medición: el SMTP real puede bajarlo. Un Administrador produce invitaciones al ritmo de una
transacción corta, así que el productor le gana al consumidor por mucho.

`POST /users/{id}/reactivate` también encola una invitación (usuario que nunca aceptó): un ciclo
desactivar → reactivar es otro productor sin límite.

### 5.2 Por qué hay que arreglarlo

Hay dos efectos, y el segundo es el más caro:

1. **Desplazamiento entre empresas.** Los mensajes de seguridad de otras empresas (reset,
   verificación, invitaciones legítimas) esperan detrás. Con la cota de 5 msg/s, cada 1.000
   invitaciones encoladas son unos 200 s de demora para todos. El token de reset vence a la hora.
2. **El remitente del producto como cañón de correo.** Cualquiera se registra (DD-4), invita a
   miles de direcciones, y cada email lleva el nombre de empresa que eligió el atacante (una nota
   del ledger muestra además que ese nombre admite CR/LF en la parte de texto). La consecuencia
   real es que el proveedor SMTP suspende la cuenta, y entonces no sale **ningún** email de
   ninguna empresa (runbook: "Credenciales SMTP vencidas, IP bloqueada…").

### 5.3 Decisión: DD-48 (cupos de invitación por empresa y por IP)

- **Por empresa**: un limiter con clave `tenant_id` cubre `POST /users/invitations` y
  `POST /users/{id}/reactivate` (un solo cupo para el flujo "emitir invitaciones", con el mismo
  criterio de DD-9: un cupo por flujo, no por ruta).
- **Por IP**: otro limiter, con la clave de DD-32, en las mismas dos rutas.
- **Dónde**: después de `Authenticate` y `RequirePermission` (un operador recibe `403` sin
  gastar cupo, y un `401` tampoco gasta) y **antes** de decodificar el body. Cuenta éxitos y
  rechazos, igual que el registro (DD-9). Excedido: `429 rate_limited` con `Retry-After`.
- **Efecto secundario útil.** El `409 email_taken` de la invitación funciona como oráculo de
  existencia de emails para un Administrador (el candidato rechazado de la auditoría, que DD-21
  permite). Hoy no tiene límite; con DD-48 queda acotado por el mismo cupo.

**Valores (los decide el usuario).** Default:

| Cupo | Ráfaga | Reposición | Sostenido | Máximo en 1 h | Máximo por día |
|---|---:|---|---|---:|---:|
| Por empresa (invitar + reactivar) | 10 | 1 cada 6 min | ≈10/h | 20 | ≈250 |
| Por IP (invitar + reactivar) | 20 | 1 cada 3 min | ≈20/h | 40 | ≈500 |

Las cotas salen de B + ⌊T/Δ⌋ (DD-9). Por qué estos valores: S-3 supone unos 5 usuarios por
empresa, así que 10 de ráfaga cubren invitar al equipo completo y reenviar un par de veces. Con
estos valores, una empresa desplaza como mucho 10 mensajes de una vez (unos 2 s de demora para el
resto, según la cota del §5.1), y sostenido no forma cola: ≈10/h contra una capacidad del orden de
18.000/h. Un atacante con K claves de IP (y K empresas, cada una con su email verificado si se
aprueba DD-46) puede encolar de golpe unos 20·K: con K = 256 (un /56), unos 17 minutos de demora
una vez, y sostenido 5.120/h, todavía por debajo de la capacidad. Eso pasa de "sin límite" a
"finito y visible" (runbook).

**Firmas** (`internal/platform/ratelimit`, que no puede importar `authz`; la clave por empresa la
arma `internal/app`):

```go
// KeyFunc extrae la clave del cupo de un request. ok == false es un bug de cableado (la ruta no
// tiene lo que la clave necesita): responde 500.
type KeyFunc func(r *http.Request) (key string, ok bool)

// MiddlewareBy es Middleware con otra clave. Middleware(l) sigue siendo MiddlewareBy(l, clave por IP).
func MiddlewareBy(limiter *Limiter, key KeyFunc) func(http.Handler) http.Handler

// internal/app (no exportada): clave por empresa del Principal que dejó Authenticate.
func tenantKey(r *http.Request) (string, bool)
```

Las constantes viven en `internal/app/api.go` (`BuildAPIRouter`), donde están hoy todas las de
DD-9. DD-9 todavía dice `cmd/crm/serve.go`, y eso es otro hallazgo de documentación (§8).

**Alternativas descartadas o diferidas:**

- **Solo por empresa.** No acota a quien crea muchas empresas, cada una con su cupo (el registro
  admite unas 125 por día por IP).
- **Prioridad en el outbox** (los mensajes de seguridad antes que las invitaciones). Ataca el
  desplazamiento, no el volumen del remitente. Exige que `crm_worker` vea una columna nueva
  (INV-05, `data-model.md` §3.4, T-B109) y un índice. **Diferida**: R-22 la deja como salida si
  `outbox_oldest_pending_seconds` muestra demora con invitaciones de por medio.
- **Cupo diario por empresa en la base** (contar las filas del outbox). Es más exacto, pero suma
  una query por invitación y un índice para un problema que el token bucket ya acota.
- **Deduplicar mensajes pendientes por destinatario.** Solo cubre la reinvitación al mismo email;
  no cubre muchos destinatarios.

### 5.4 Tests (T-B1005) y mutaciones

Parto del `validation_plan.local` (11 invitaciones de A y el reset de B), pero el test de diseño
no es el desplazamiento: es que la 11.ª invitación ya no entra. El desplazamiento finito queda
como consecuencia de los valores, y no se fija con un test, porque depende del reloj del
`Dispatcher`.

| Mutación | Qué falla |
|---|---|
| Clave por IP en lugar de por empresa | Q1 (dos IPs de la misma empresa) |
| Clave por usuario | Q1b (dos admins de la misma empresa) |
| Limiter después de decodificar y solo en éxitos | Q2 (el `409` y el `422` cuentan) |
| `reactivate` sin limiter, o con un cupo propio | Q4 |
| Limiter antes de `RequirePermission` | Q7 |
| Sin el cupo por IP | Q6 |

---

## 6. Registros sin verificar que dejan roles de PostgreSQL permanentes

### 6.1 Verificación del trace

Correcto. Un registro confirmado crea `crm_t_<hex>` y sus membresías (`00002_tenant_functions.sql:37`).
No hay borrado de empresas ni `DROP ROLE` en runtime, y la limpieza no toca empresas
(`cleanup.go:36`). El único freno es el cupo por IP (5 de ráfaga y 1 cada 12 min, `api.go:22`).
Las cifras de la auditoría sobre la medición (10.000 roles medidos; 10.500 y 25.000–32.000
extrapolados) coinciden con `revision-14-t-b910.md`.

### 6.2 Por qué importa aunque la auditoría no lo pruebe

Con el cupo por IP, cada clave admite como mucho 5 + 24 h / 12 min = **125 registros por día**.
El primer disparador de DD-33 R-f (5.000 roles) está a **40 claves-día** de distancia, y el techo
conservador (10.500), a 84. Un /56 de IPv6 tiene 256 claves /64 (DD-32). Son cotas de admisión
del código, no un ataque observado: alcanzan para ver que el techo de ADR-005, pensado para
crecimiento orgánico (S-3: unas 1.000 empresas), se puede cruzar **en días**, y que el daño es
**permanente**. No hay herramienta para borrar las empresas ni los roles, y cada registro
encarece todos los registros siguientes y el primer `SET ROLE` de cada conexión.

Los disparadores de §12.4 son sobre el **total** de roles, no sobre el **ritmo**: un ataque los
cruza antes de que alguien los mire.

### 6.3 Alternativas

| Opción | Qué es | Costo | Qué queda |
|---|---|---|---|
| **A. Alerta de ritmo** | Alerta si `tenants_total` sube más de 20 en una hora (lo orgánico de S-3 son unas pocas por día), más una fila del runbook: bloquear prefijos en el proxy | Texto y la configuración de la alerta al desplegar (P-1) | Sin respuesta humana, nada frena el crecimiento: 125/día por clave |
| **B. A + cupo global de registros** *(default)* | Un limiter con clave constante, después del de IP, en `POST /auth/signup`. Default: 30 de ráfaga y 1 por minuto (≈60/h). Cuenta como el de IP: éxitos y rechazos que pasaron el cupo por IP. Excedido: `429 rate_limited` con `Retry-After`, más la métrica `signup_global_limited_total` y un log `WARN` `event=signup_global_limited` | Un limiter y su test. Contrato: una descripción | **Palanca de denegación.** Quien sostenga 60/h desde 60 / 5 = **12 claves** deja sin registro a los usuarios legítimos mientras dure el ataque (R-21). A cambio, el crecimiento queda acotado a unos 1.470 por día, y la alerta salta en la primera hora |
| **C. B + clave /56 para el registro en IPv6** | Solo para el registro, la clave IPv6 pasa de /64 a /56 | Una línea y su test | Usuarios que comparten un /56 comparten el cupo. Supuesto a validar: los proveedores residenciales asignan un /56 o un /48 por cliente, así que un /56 es más o menos un cliente |
| **D. Registro en dos pasos** | La opción D del §3: no hay rol sin email verificado | Alto (§3.4) | — |
| **E. Purga de empresas abandonadas sin verificar** | CLI `crm tenants purge-unverified` con credenciales de migración | Medio a alto: borrar empresas y roles (`DROP ROLE`), privilegios, auditoría y la interacción con la alternativa B de ADR-005 | Hasta que se corre, los roles están |

**Recomendación: B.** El trade-off es concreto. Sin cupo global, el daño es permanente y crece con
la cantidad de prefijos del atacante. Con cupo global, el daño pasa a ser temporal (los registros
legítimos dan `429` mientras dure el ataque) y queda acotado (unos 1.470 por día como máximo), y la
señal del ataque aparece sola. La cuenta que manda es la misma para cualquier valor: **claves
necesarias para bloquear = ritmo global / ritmo por clave**, y **crecimiento máximo por día ≈ ritmo
global × 24 h**. Con 60/h, el primer disparador de R-f queda a más de 3 días de un ataque sostenido
desde cero, y la alerta salta en una hora. C es complementaria y barata, pero depende de un
supuesto que no verifiqué: la dejo como opción. La salida estructural ya está planificada: la
alternativa B de ADR-005 (sin un rol por empresa), en el disparador del 50 %. Con ella, este
candidato deja de afectar la disponibilidad y quedan filas, que se purgan cuando haga falta (E).

### 6.4 Decisión propuesta: DD-49

Cupo global (B) + alerta de ritmo (A) + runbook. **Una sola instancia** (S-1): con varias, el cupo
global es por instancia, igual que DD-9. Se recalcula si cambian los cupos por IP del registro.

### 6.5 Tests (T-B1007) y mutaciones

El `validation_plan.local` (5 + 1 desde un /64, otro /64, duplicado) verifica el cupo por IP, que
ya existe y tiene test (T-B305). El test nuevo es el del cupo global, con bodies inválidos
(`422`), que gastan cupo sin crear empresas: el test no crea 30 roles.

| Mutación | Qué falla |
|---|---|
| Clave global por IP | G2 |
| Global antes que el de IP | G4 (un `429` por IP gasta cupo global) |
| Global después de validar el body | G2 (los `422` no gastan) |
| Sin métrica ni log | G2 (afirma `signup_global_limited_total` y el evento) |

---

## 7. Candidato descartado y unidad bloqueada

- **`invite-email-taken-global-existence-oracle`**: de acuerdo con el rechazo, porque DD-21 lo
  permite a propósito. DD-48 lo acota igual, como efecto secundario (§5.3).
- **Unidad bloqueada (CVE de dependencias)**: `govulncheck` no se pudo correr sin red.
  Recomiendo sumarlo a CI en un PR aparte (es configuración de CI, no diseño). No lo agrego a la
  Fase 10 para no mezclar.

---

## 8. Hallazgos de documentación

1. **DD-1** dice que desactivar un invitado libera su email. El contrato y el código dicen que no
   (§4.1). Se corrige con una nota.
2. **DD-9** dice que los cupos por IP son constantes en `cmd/crm/serve.go`. Viven en
   `internal/app/api.go` (`BuildAPIRouter`, que lo aclara en su comentario). Se corrige.
3. **§8 del plan** dice "versión 0.4.0" del contrato, y el contrato está en 0.4.3. Se corrige a la
   versión nueva.
4. **ADR-007** no registra que esperar el semáforo con una conexión tomada acopla el login con
   todo el pool (§2.2). Nota fechada.
5. **`TestRoleChangesAndLastAdmin`** afirma el comportamiento del candidato 1 (§1.1). No es
   documentación, pero es un test que fija una propiedad insegura: cambia en T-B1001.

---

## 9. Cambios de contrato

**Versión propuesta: v0.5.0** si se aprueba la decisión 3 (opción B), porque agrega un valor a
`ErrorCode` (*minor*, como v0.4.0). Sin la decisión 3, **v0.4.4**: se declaran respuestas `429`
cuyo `code` ya existe y cambian descripciones, el mismo criterio de *patch* que v0.4.1.

| # | Operación | Cambio exacto | Por | Solo con la decisión 3 |
|---|---|---|---|:---:|
| C1 | `components/schemas/ErrorCode` | Nuevo valor `email_not_verified` (después de `email_taken`) | DD-46 | Sí |
| C2 | `inviteUser` (`POST /users/invitations`) | `403` deja de ser `$ref: Forbidden` y pasa a una respuesta propia: `code` ∈ {`forbidden`, `email_not_verified`}, schema `Problem` | DD-46 | Sí |
| C3 | `inviteUser` | Nueva respuesta `429` → `$ref: RateLimited` (`code` = `rate_limited`, cabecera `Retry-After`) | DD-48 | No |
| C4 | `reactivateUser` (`POST /users/{userId}/reactivate`) | Nueva respuesta `429` → `$ref: RateLimited` | DD-48 | No |
| C5 | `inviteUser`, `reactivateUser` | Descripción: el cupo compartido por empresa y por IP; en `inviteUser`, además, que exige el email verificado | DD-48, DD-46 | En parte |
| C6 | `signup` (`POST /auth/signup`) | Descripción: además del cupo por IP, un cupo global; el `429` ya estaba declarado | DD-49 | No |
| C7 | `confirmPasswordReset` | Descripción: si el email no estaba verificado, queda verificado | DD-46 | Sí |
| C8 | `confirmEmailVerification` | Descripción: la verificación habilita invitar (antes: "no bloquea ninguna funcionalidad") | DD-46 | Sí |
| C9 | `resendEmailVerification` | Descripción: cupo por usuario de 3 de ráfaga y 1 cada 20 min, después del cupo por IP; sin cupo, `429 rate_limited` con `Retry-After` (el `429` ya estaba declarado por el cupo por IP); con los dos cupos agotados manda el de IP. *Versión del 2026-10-12, reemplazada por la enmienda H-16 (§3.7)*: sin cupo, `202` sin enviar | DD-46 | Sí |
| C10 | `getMe` y `CurrentUser.email_verified` | Descripción: `email_verified = false` también significa que no puede invitar | DD-46 | Sí |
| C11 | `info.version` e historial | `0.5.0` (o `0.4.4`) con su entrada | — | — |

**Sin cambios de contrato**: DD-44 (`401` y `403` ya están declarados en las cuatro operaciones
con la semántica correcta: "sesión inválida, vencida o revocada" y "el rol no tiene el permiso"),
DD-45 y DD-47.

**Lo que esto le pide al frontend** (lo decide el `frontend-architect` en `ui.md`):

- Un mensaje para `email_not_verified` en el mapa exhaustivo de `ErrorCode`. T-F101 deja de
  compilar hasta tenerlo, a propósito.
- En la pantalla de usuarios: con `email_verified = false`, el botón "Invitar" deshabilitado con
  el aviso "Verificá tu email para invitar" y la acción "Reenviar verificación" (ya existe el
  endpoint).
- `429 rate_limited` con `Retry-After` en "Invitar", "Reenviar invitación" (la misma ruta, DD-5)
  y "Reactivar".
- Un `401` en una acción de administración (el actor fue desactivado mientras tanto) se trata
  como cualquier `401`: sesión terminada.
- *(Enmienda H-16, §3.7)* El `429 rate_limited` con `Retry-After` del reenvío de la verificación
  también sale del cupo por usuario, así que la línea genérica de DD-F30 sobra.

---

## 10. Lo que tiene que aprobar el usuario

| # | Pregunta | Alternativas y costo | Default |
|---|---|---|---|
| 1 | ¿Se revalidan la sesión y el actor bajo el lock de la empresa en invitar, cambiar rol, desactivar y reactivar (DD-44, INV-35), cambiando el test que hoy afirma el ascenso con un `Principal` viejo? | Bloquear también al actor (más locks, sin ganancia); revalidar solo el usuario sin la sesión (deja pasar desactivar y reactivar durante el request); aceptar | **Sí** |
| 2 | ¿Se reserva el cupo del hasher antes de abrir la transacción en login, aceptación de invitación y confirmación de reset (DD-45, INV-36), sumando `Acquire` y `Slot` a `password.Hasher`? | Semáforo propio en `identity` (dos semáforos para un recurso); login en dos fases (rompe la exactitud de DD-7); pool separado (cambia DD-33 R-e); aceptar | **Sí** |
| 3 | Registro con un email ajeno (producto): ¿qué se hace? | **A** aceptar (sin costo; pre-secuestro posible); **B** invitar exige el email verificado, el reset verifica, cupo por usuario del reenvío y texto del email (chico; contrato v0.5.0; una clarificación en la spec; el dueño verifica antes de invitar); **C** B + verificar exige la sesión del dueño (medio; fricción al verificar); **D** registro en dos pasos (alto; revierte P-2 y cambia SC-001) | **B** |
| 4 | Una invitación reserva un email ajeno (producto): ¿qué se hace? | **A** aceptar, con DD-1 corregida, runbook y disparador (solo texto); **B** liberar el email al desactivar un invitado que nunca aceptó (reabre DD-40 y P-3; contrato); **C** separar identidad y membresía (la spec de varias empresas por usuario) | **A** |
| 5 | Valores de los cupos de invitación (producto) | Por empresa 10 + 1 cada 6 min y por IP 20 + 1 cada 3 min (default); solo por empresa; valores más altos (p. ej. 20 + 1 cada 3 min por empresa); prioridad en el outbox (diferida, R-22) | **Los del default** |
| 6 | Crecimiento de roles por registros sin verificar (producto) | **A** solo alerta de ritmo; **B** cupo global de 30 + 1 por minuto, más la alerta (quien sostenga 12 claves bloquea los registros mientras dure el ataque); **C** B + clave /56 para el registro en IPv6; **D** registro en dos pasos; **E** purga de abandonadas | **B** |
| 7 | ¿Contrato v0.5.0 (con la decisión 3 = B) o v0.4.4 (sin ella)? | — | **v0.5.0** |
| 8 | ¿Fase 10 en una rama nueva desde `main` después del merge de la decimoquinta (`fix/001-phase-10-security`), en el orden de `tasks.md`, con la decisión 3 como último paso para poder sacarla sin rehacer nada? | Mezclarla con la rama de la decimoquinta (no conviene: esa rama es solo de tests y texto) | **Sí** |

Si la decisión 3 no es B: DD-46 pasa a registrar la opción elegida (A → riesgo aceptado con R-23;
C → B más el cambio de `confirmEmailVerification`), T-B1009 y T-B1010 salen o se amplían, y el
contrato queda en v0.4.4 (A) o suma la operación de C.
