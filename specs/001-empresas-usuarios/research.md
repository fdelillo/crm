# Research: Empresas, usuarios y roles (backend)

**Spec**: [`spec.md`](spec.md) · **Plan**: [`plan.md`](plan.md)
**Revisión 2026-09-27**: R-06 (duración de sesión, P-5) y R-15 (enumeración en el registro, P-4)
actualizadas con las respuestas del usuario.
**Revisión 2026-09-29**: R-19 a R-23 agregadas por los hallazgos H-1 a H-9 del frontend (plan §18).
**Segunda revisión 2026-09-29**: R-23 reescrita por el hallazgo H-10 (HTTPS local con mkcert,
decisión del usuario); R-24 agregada por H-11 (límite exacto del logo y del cuerpo multipart).
**Tercera revisión 2026-09-30**: decisiones previas a la Fase 2 aprobadas por el usuario: R-04c
ampliada (lock de `GRANT crm_tenant`, DD-33); R-25 (IP del cliente detrás de proxies, DD-32), R-26
(`405 method_not_allowed`) y R-27 (cancelaciones del cliente) agregadas.
**Cuarta revisión 2026-09-30**: R-28 agregada (primer `SET ROLE` a una empresa recién aprovisionada
desde otra conexión, DD-34); R-04 y R-04c mencionan el hallazgo; supuesto 12 nuevo.
**Quinta revisión 2026-10-05**: nota en R-16 (el lock de la empresa pasa a `FOR NO KEY UPDATE`, DD-40). La evaluación original no se edita.

Alternativas evaluadas por decisión. Las marcadas **(usuario)** las tomó el usuario antes del
plan: acá se documenta por qué son razonables y qué cuestan. Las demás son defaults del
arquitecto. Cada decisión estructural tiene su ADR en `docs/adr/`.

Criterios que se repiten: (1) simplicidad para un equipo con Go intermedio, (2) seguridad del
aislamiento y del dinero, (3) una sola unidad desplegable y barata, (4) dependencias mantenidas y
justificadas.

---

## R-01 Estructura del monolito → ADR-001

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Paquetes por dominio** (`internal/identity`, `internal/tenant`) + `platform/` técnico | Encapsulamiento real de Go (lo no exportado queda dentro del módulo); fronteras visibles; cada spec agrega un módulo | Exige disciplina para no importar el `store` de otro módulo (se automatiza con `depguard`) | **Elegida** |
| Capas técnicas (`handlers/`, `services/`, `repositories/`) | Familiar | Todo se exporta para cruzar capas; los cambios de una feature tocan todas las carpetas; nada impide que un handler lea tablas de otro dominio | Descartada |
| Hexagonal completa (dominio / aplicación / puertos / adaptadores por módulo) | Máxima separación | Tres o cuatro paquetes por módulo para CRUD simple; abruma a un equipo aprendiendo | Descartada: se toma solo la idea de puertos donde hay infraestructura externa (email, S3) |
| Microservicios | — | Prohibido por la constitución; sin requisito que lo pida | Descartada |

## R-02 Router HTTP **(usuario: chi)** → ADR-002

| Opción | A favor | En contra |
|---|---|---|
| **`go-chi/chi/v5`** | Compatible 100 % con `net/http` (handlers y middlewares estándar); subrouters y grupos por permiso (`r.Group` + `RequirePermission`); `chi.Walk` permite listar todas las rutas (lo usa el test de cobertura de aislamiento, T-B801); maduro y sin dependencias | Una dependencia más que el `ServeMux` estándar |
| `net/http.ServeMux` (Go ≥1.22, con métodos y comodines) | Cero dependencias | Sin grupos ni middlewares por subárbol: la autorización por grupo se arma a mano; no tiene `Walk` |
| Gin / Echo / Fiber | Mucho azúcar (binding, validación) | Tipos propios de contexto que no son `net/http` (Fiber ni siquiera usa `net/http`); acoplan todo el código HTTP al framework |

Veredicto: chi. El costo es mínimo y la agrupación de rutas por permiso es exactamente lo que
pide FR-007. (Desde 2026-09-29, chi atiende solo `/api/`; el reparto de primer nivel lo hace un
`ServeMux` de la librería estándar: R-19.)

## R-03 Acceso a datos **(usuario: sqlc + pgx v5)** → ADR-003

| Opción | A favor | En contra |
|---|---|---|
| **sqlc + pgx v5** | SQL explícito y revisable (clave para auditar filtros por `tenant_id`, INV-04); tipos Go generados y verificados contra el esquema; pgx es el driver nativo más completo para PostgreSQL (tipos `inet`, `jsonb`, `uuid`, errores con SQLSTATE) | Queries dinámicas (filtros opcionales) son incómodas; hay que regenerar código |
| GORM / ent / bun (ORM) | Menos SQL | El SQL que corre es implícito: difícil auditar que cada query filtre por empresa; *hooks* y cargas perezosas ocultan queries; `SET LOCAL ROLE` por transacción pelea con el manejo de conexiones del ORM |
| sqlx | SQL explícito, mapeo simple | Sin verificación en compilación; errores de columnas en runtime |
| pgx puro | Control total | Mapeo manual de filas en cada query; mucho código repetido |

## R-04 Aislamiento por empresa **(usuario: rol de PostgreSQL por empresa)** → ADR-005

| Opción | Cómo funciona | A favor | En contra |
|---|---|---|---|
| **A. Rol por empresa + RLS forzada** (elegida por el usuario) | Cada empresa tiene `crm_t_<hex>`; cada transacción hace `SET LOCAL ROLE`; las políticas comparan `tenant_id` con la empresa derivada de `current_user` | Falla cerrada (el rol de login no tiene privilegios); la identidad de la empresa es la identidad de base (`pg_stat_activity`, logs por usuario); se pueden fijar límites por empresa (`ALTER ROLE ... SET`) | Requiere `CREATEROLE` en el flujo de registro; roles globales al clúster (backups, restores, entornos); `crm_app` miembro de miles de roles (rendimiento a medir); re-análisis de planes al cambiar de rol; bootstrap fuera de las migraciones; **registros serializados por el lock de `GRANT crm_tenant`** (R-04c); **el primer `SET ROLE` a una empresa nueva desde otra conexión puede fallar por la caché de membresías** (R-28) |
| B. Rol único `crm_tenant` + `SET LOCAL app.tenant_id` | Cada transacción hace `SET LOCAL ROLE crm_tenant` y fija una variable; las políticas usan `current_setting('app.tenant_id')` | Misma falla cerrada si `crm_app` tampoco tiene privilegios; sin `CREATEROLE`; cero operación extra; patrón muy documentado; sin `GRANT` por registro ni membresías nuevas en runtime | La empresa es una variable, no una identidad de base; menos trazabilidad en `pg_stat_activity` |
| C. Solo filtro en la aplicación | `WHERE tenant_id = $1` en cada query | Simple | Viola la constitución (principio III exige RLS) |
| D. Esquema por empresa | Un esquema por empresa, `search_path` por transacción | Aislamiento físico por esquema | Migraciones × N esquemas; miles de esquemas; `search_path` es otra variable manipulable |
| E. Base por empresa | Una base por empresa | Aislamiento fuerte | Pool por empresa, migraciones × N, costo de hosting; desproporcionado para pymes |

**Análisis honesto de A frente a B**:

- Contra una **omisión** en el código (olvidar el filtro o el cambio de rol), A y B protegen igual
  si en ambas el rol de login no tiene privilegios propios.
- Contra una **inyección SQL** con ejecución arbitraria, ninguna protege: en A, `crm_app` puede
  hacer `SET ROLE` a cualquier empresa; en B, puede fijar cualquier `app.tenant_id`.
- A gana en trazabilidad y en poder aplicar límites por empresa; B gana en operación y en no
  necesitar `CREATEROLE`.
- El costo diferencial de A es **operativo**, no de código: con el diseño de este plan la política
  RLS es idéntica en ambas (usa `app.current_tenant_id()`), y pasar de A a B cambia una función y
  `InTenantTx`.
- La implementación sumó dos costos de A que B no tiene, los dos por cambiar membresías de roles
  en runtime: el lock de `GRANT crm_tenant` (R-04c) y la caché de membresías por backend (R-28).

Recomendación del arquitecto: B es la opción más simple con la misma protección práctica. Se
diseña sobre A por decisión del usuario, con la reversibilidad garantizada y el riesgo del
hosting planteado en la pregunta P-1 (sigue abierta).

### R-04b Cómo resolver la empresa antes de conocerla (login, tokens, worker)

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Roles de sistema con privilegio por columna + política `USING (true)` solo para esas columnas** | Simple (GRANT de columnas, sin funciones); la fase 2 corre bajo RLS | Un bug en una query de fase 1 puede listar `(id, tenant_id, email)` de todas las empresas | **Elegida**, con el riesgo acotado por el test de privilegios T-B109 y archivos de queries propios con ruta exacta (plan §4.4) |
| Funciones `SECURITY DEFINER` por lookup (`lookup_user_by_email(text)`) | Imposible enumerar aun con bug: solo devuelve una fila por email exacto | Un rol dueño por función con su propia política; más objetos que mantener y entender | Descartada por complejidad; es la mejora natural si el riesgo residual se considera inaceptable |
| Tabla global de directorio (`email → tenant_id`) sin RLS | Muy simple | Otra tabla que mantener sincronizada; mismo riesgo de enumeración | Descartada |
| Selector de empresa en el login | Sin lookup global | Contradice "un usuario, una empresa" y agrega un paso al usuario (principio I) | Descartada |

Dónde viven esas queries (2026-09-30): una ruta exacta por módulo dueño de las tablas, para que
`queryrules` (T-B112) pueda eximir archivos y no patrones.

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Un archivo de sistema por módulo dueño de la tabla** (`identity/store/auth_lookup.sql`, `identity/store/cleanup.sql`, `platform/outbox/store/worker.sql`, `tenant/store/provisioning.sql`) | Respeta ADR-001 (un módulo no lee tablas de otro); la lista de excepciones es corta y revisable | La limpieza de tablas de `identity` necesita un gancho en el `Dispatcher` (tareas periódicas registradas por `internal/app`) | **Elegida** |
| Todas las queries de `crm_worker` en `platform/outbox/store/worker.sql` | Un solo archivo | `platform` leería y borraría tablas de `identity` y `tenant`, rompiendo ADR-001 y `depguard` a nivel de SQL | Descartada |
| Eximir por nombre de archivo en cualquier carpeta | Flexible | Cualquiera puede crear un `worker.sql` nuevo y saltearse el filtro sin que nadie lo decida | Descartada (el detector ya marca los homónimos fuera de lugar) |

### R-04c Cómo se crea el rol de una empresa

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Función `SECURITY DEFINER` dueña `crm_provisioner` (con `CREATEROLE`), ejecutable solo por `crm_signup`** | `crm_app` no puede crear roles arbitrarios; entrada tipada `uuid`; idempotente; transaccional con el registro | Un objeto más; la función la crea una migración con `SET ROLE crm_provisioner`; **serializa los registros** (abajo) | **Elegida** |
| `crm_app` con `CREATEROLE` | Simple | Cualquier código (o inyección) bajo `crm_app` crea roles y se otorga membresías | Descartada |
| Crear roles fuera de línea (job/DBA) | Sin `CREATEROLE` en runtime | El registro deja de ser autónomo e inmediato (FR-001, SC-001) | Descartada |

**Lock de `GRANT crm_tenant` (2026-09-30, DD-33, nota en ADR-005).** Desde PostgreSQL 16,
`GRANT <rol> TO x` toma un lock sobre el rol concedido hasta el fin de la transacción. Como la
función hace `GRANT crm_tenant TO crm_t_<hex>`, los registros concurrentes esperan uno detrás de
otro sobre `crm_tenant`. El revisor de la Fase 1 lo verificó con un test: con una transacción de
aprovisionamiento abierta, la segunda cae por el `statement_timeout` de `crm_app` (5 s) dentro del
`GRANT`. Qué hacer:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Aceptar la serialización y acotarla**: `lock_timeout = '2s'` en la transacción de registro (`55P03` → `503` con `Retry-After`), sin E/S de red después de aprovisionar (p95 < 250 ms), sembrado de 002 solo si entra en ese presupuesto, reprovisión con una transacción por empresa | Sin cambios de modelo; a la escala del MVP (registros raros) la espera real es de milisegundos; la falla, si ocurre, es clara y reintentable, nunca un `500` | Un pico de registros simultáneos puede dar algunos `503`; hay que vigilar la duración de la transacción de registro (T-B905) | **Elegida** |
| Hacer el `GRANT` en una transacción previa y separada | Sin lock durante el resto del registro | Rompe INV-14: si el registro falla queda un rol huérfano con membresía; dos pasos que coordinar | Descartada |
| *Pool* de roles creados de antemano (un job crea roles `crm_t_*` con su membresía, uno por transacción; el registro reclama uno con `FOR UPDATE SKIP LOCKED` y usa su UUID como `tenants.id`) | Elimina el lock del camino del registro; **también elimina el problema de R-28** (la membresía a `crm_app` se concede mucho antes del primer uso) | Redefine INV-14 (el rol existe antes que la empresa); una tabla y un job más; roles sin empresa que inventariar | Postergada: mejora natural si T-B905 no cumple o si R-17 del plan se materializa |
| Alternativa B de ADR-005 (sin roles por empresa) | Elimina el problema de raíz | Es cambiar la decisión del usuario | Salida si T-B905 o P-1 lo piden (ADR que reemplace a ADR-005) |
| Subir `statement_timeout` | Cero código | La espera sigue ahí, más larga y con conexiones del pool retenidas | Descartada |

## R-05 Migraciones **(usuario: goose embebido)** → ADR-004

| Opción | A favor | En contra |
|---|---|---|
| **pressly/goose v3** | SQL plano con anotaciones `Up/Down`; `embed.FS` para llevar las migraciones dentro del binario (`crm migrate up`); API `Provider` para usarlo como librería; permite migraciones sin transacción cuando haga falta (índices concurrentes) | Sin diff automático de esquema |
| golang-migrate | Muy usado | Archivos `up`/`down` separados; manejo de "dirty state" que hay que limpiar a mano |
| Atlas | Diff declarativo, lint de migraciones | Más conceptos (esquema deseado, planes); modelo de licencia con partes comerciales |
| tern (de jackc) | Del mismo autor que pgx | Menos adopción y documentación |

Los roles de clúster **no** van en goose (ADR-004): exigen privilegios que el dueño del esquema no
debe tener y no pertenecen a una base.

## R-06 Autenticación **(usuario: sesiones en PostgreSQL + cookie)** → ADR-006

| Opción | A favor | En contra |
|---|---|---|
| **Sesión en tabla + token opaco en cookie `HttpOnly`** | Revocación inmediata (Historia 3.3); el rol se lee en cada request (cambios de rol inmediatos); el token no es accesible desde JavaScript (XSS no lo roba) | Una consulta por request (índice único, trivial a esta escala); requiere protección CSRF |
| JWT de acceso corto + refresh | Sin consulta por request | La revocación inmediata exige lista negra (vuelve a consultar la base); rol "congelado" en el token hasta que expira; más piezas |
| Token en `Authorization` guardado en `localStorage` | Sin CSRF | Cualquier XSS roba la sesión; peor para una PWA |

Duración de la sesión (**usuario, P-5**):

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **24 h sin uso / 7 días de vida máxima** | Ventana corta si se pierde o comparte un celular en obra/taller; sesiones viejas desaparecen rápido | Quien no usa la app un día vuelve a iniciar sesión; más logins (costo de argon2id acotado por el semáforo) | **Elegida por el usuario** |
| 7 días sin uso / 30 días (default propuesto) | Menos fricción en el celular | Ventana de exposición mayor | Descartada por el usuario |
| Sin expiración por inactividad, solo absoluta | Simple | Una sesión olvidada vale lo mismo que una activa | Descartada |

## R-07 CSRF → ADR-006

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`http.CrossOriginProtection` (stdlib, Go ≥1.25) + `SameSite=Lax` + JSON obligatorio** | Sin tokens en formularios ni cookies extra; librería estándar; tres capas independientes | Navegadores anteriores a ~2020 sin `Sec-Fetch-Site` ni `Origin` quedan cubiertos solo por `SameSite` y el `Content-Type` | **Elegida** |
| Token sincronizador (`gorilla/csrf` o propio) | Clásico y robusto | Endpoint o cookie extra para obtener el token; la SPA debe adjuntarlo en cada request | Descartada (más piezas sin ganancia real con SPA en el mismo origen) |
| Double-submit cookie | Sin estado en servidor | Débil si un subdominio puede escribir cookies (el prefijo `__Host-` lo mitiga, pero agrega complejidad) | Descartada |
| Solo `SameSite=Strict` | Muy simple | Rompe la navegación entrante con sesión (abrir la app desde un link de WhatsApp) | Descartada |

Respuesta del rechazo (H-9, DD-30):

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`SetDenyHandler` con `403 problem+json` `code: forbidden`** | La UI reusa su mapa de errores; queda un evento `csrf_rejected` para operar | Un handler más en `platform/httpx` | **Elegida** |
| Respuesta por defecto de la librería (texto plano) | Cero código | El frontend la trata como "respuesta inesperada"; no es problem+json como el resto de la API | Descartada |
| Código propio `csrf_rejected` en `ErrorCode` | La UI podría distinguirlo | Un código que el usuario nunca debería ver con la SPA en el mismo origen; más superficie en el contrato | Descartada |

## R-08 Hashing de contraseñas → ADR-007

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **argon2id** (`golang.org/x/crypto/argon2`) | Recomendado en primer lugar por OWASP; resistente a GPU por memoria | 19 MiB por hash: hay que limitar la concurrencia (semáforo) | **Elegida** |
| bcrypt | Simple, sin parámetro de memoria | Trunca a 72 bytes; menos resistente a hardware dedicado | Alternativa aceptable (FR-003 la admite) |
| scrypt / PBKDF2 | Estándares | OWASP los ubica después de argon2id | Descartadas |

## R-09 Identificadores → ADR-008

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **UUIDv7** | No secuencial hacia afuera (no enumerable); ordenado por tiempo (índices B-tree compactos); `uuidv7()` nativo en PostgreSQL 18 y `uuid.NewV7()` en Go | 16 bytes; revela el instante de creación | **Elegida** |
| UUIDv4 | Totalmente aleatorio | Inserciones dispersas en el índice | Descartada |
| `bigserial` | Compacto | Enumerable en URLs; revela volumen | Descartada |
| ULID | Ordenado | No es tipo nativo `uuid` de PostgreSQL | Descartada |

## R-10 Formato de errores → ADR-009

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **RFC 9457 + `code` estable (+ `suggested_action` cuando aplica)** | Estándar; herramientas lo reconocen; `code` desacopla el texto de la UI; `suggested_action` le dice a la UI qué salida ofrecer sin parsear textos | Dos campos de extensión propios | **Elegida** |
| Envoltorio propio `{error: {...}}` | Libre | Reinventa un estándar | Descartada |
| Solo status HTTP | Mínimo | No distingue `email_already_registered` de `last_admin` (ambos 409) | Descartada |
| Link absoluto a la pantalla de recuperación en el error | La UI no decide nada | El backend tendría que conocer rutas de la SPA; se acopla a la UI | Descartada: `suggested_action` es un enum estable y la UI decide la ruta |

## R-11 Envío de email **(usuario: outbox + worker en el binario; puerto `Mailer` SMTP)** → ADR-010

| Opción | A favor | En contra |
|---|---|---|
| **Outbox transaccional + worker (goroutine) en el mismo binario** | El email existe si y solo si la operación se confirmó (sin emails de registros que hicieron `ROLLBACK`); reintentos persistentes; sin infraestructura extra | Entrega "al menos una vez" (duplicado posible si se cae entre el envío y el `COMMIT`); *polling* |
| Envío síncrono en el request | Simple | Si el SMTP tarda o falla, el registro falla o se envía un email de algo que después se deshace |
| Goroutine "fire and forget" | Simple | Se pierde en un reinicio; sin reintentos |
| Cola externa (Redis, SQS, NATS) | Escala | Infraestructura extra sin requisito que la justifique |

Librería SMTP:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`github.com/wneessen/go-mail`** | Mantenida; MIME (texto + HTML), STARTTLS/TLS, varios métodos de autenticación, timeouts con `context` | Dependencia externa | **Elegida** |
| `net/smtp` (stdlib) | Sin dependencias | Congelado (no acepta nuevas funciones); armar MIME multiparte a mano | Descartada |
| SDK del proveedor (SES, Postmark…) | Funciones avanzadas | Ata el código a un proveedor antes de elegirlo | Descartada: el puerto `Mailer` permite sumar un adaptador después |

*Polling* vs `LISTEN/NOTIFY`: polling cada 2 s es suficiente para el target (95 % < 30 s) y no
depende de mantener una conexión dedicada. `NOTIFY` queda como optimización futura.

> **Nota 2026-10-01**: la fila de go-mail decía "timeouts con `context`". Verificado en el código de
> v0.8.1: el `context` solo acota el *dial*; el resto de la conversación lo acotan *deadlines* por
> etapa (`WithTimeout`). Consecuencias y clasificación de errores en R-29 y ADR-024.

## R-12 Almacenamiento de archivos **(usuario: S3 compatible, subida vía backend)** → ADR-011

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`github.com/minio/minio-go/v7`** | API chica y directa; funciona con cualquier S3 compatible (MinIO, AWS, R2, etc.) | Dependencia | **Elegida** |
| `aws-sdk-go-v2` (s3) | Oficial de AWS | Mucho más grande; configuración de *path-style* para MinIO | Alternativa válida |

| Opción de subida | A favor | En contra | Veredicto |
|---|---|---|---|
| **Vía backend (multipart), con validación por contenido** | Validación de tipo, tamaño y dimensiones antes de guardar; bucket privado; sin CORS | El backend transfiere los bytes | **Elegida** (logos de ≤ 2 MiB, R-24) |
| URL prefirmada de subida directa | El backend no transfiere bytes | Validación posterior (el archivo ya está en el bucket); CORS en el bucket | Descartada para 001; 004 (adjuntos de 15 MB) puede reevaluarla |

## R-13 Estrategia de tests → ADR-012

| Opción para integración | A favor | En contra | Veredicto |
|---|---|---|---|
| **`testcontainers-go` (módulo postgres) con PostgreSQL 18 real** | Mismo motor, mismas políticas, mismos roles; cada corrida en un clúster nuevo (los roles son de clúster) | Requiere Docker en CI y en la máquina de desarrollo | **Elegida** |
| Base compartida de desarrollo | Rápida | Estado compartido; roles de corridas anteriores | Descartada |
| Mocks del repositorio | Rápidos | No prueban RLS ni constraints: justo lo que más importa | Descartada para la persistencia (sí se usan fakes para email, S3 y reloj) |
| SQLite en memoria | Rápido | No tiene RLS ni roles | Descartada |

Validación de contrato:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`github.com/pb33f/libopenapi-validator`** | Valida requests y responses `net/http` contra OpenAPI 3.0/3.1/3.2 | Dependencia de test | **Elegida** |
| `kin-openapi` (`openapi3filter`) | Muy usada | Soporte 3.1 reciente | Alternativa |
| Generar el servidor con `oapi-codegen` | El compilador obliga a cumplir el contrato | Soporte de 3.1 inicial (v2.8.0); código generado que el equipo debe entender | Descartada por ahora (ADR-014), reevaluable |

## R-14 Autorización (FR-007) → ADR-013

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Matriz estática rol → permisos en código, un permiso por fila de FR-007, middleware por grupo de rutas** | Trazable 1:1 con la spec; un test de tabla fija la matriz completa; sin dependencias | Cambiar la matriz requiere deploy (correcto: roles no configurables en el MVP) | **Elegida** |
| Chequeos `if role == admin` dispersos | Rápido de escribir | Imposible auditar; se olvidan en endpoints nuevos | Descartada |
| Motor de políticas (Casbin, OPA) | Flexible | Complejidad desproporcionada para 2 roles fijos | Descartada |
| Permisos en base de datos | Configurables | Roles configurables están fuera del MVP (visión) | Descartada |

## R-15 Enumeración de cuentas y bloqueo por intentos → DD-7, DD-8, DD-19, DD-20, DD-21

### Registro con email existente (**usuario, P-4**)

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`409 email_already_registered` con mensaje explícito ("Ya existe un usuario con ese email. ¿Querés recuperar la contraseña?") y `suggested_action: password_reset`, sin otros datos** | El dueño entiende qué pasó y tiene una salida en un toque; compatible con entrar al panel al instante (SC-001) | **Confirma que el email tiene cuenta** (enumeración) | **Elegida por el usuario**, con la enumeración aceptada conscientemente |
| `409` con mensaje genérico ("No pudimos crear la cuenta con ese email") | Algo menos explícito | Igual confirma la existencia (el rechazo es el oráculo); peor experiencia | Descartada (era el default; el usuario pidió el mensaje explícito) |
| Responder siempre "te enviamos un email" y notificar al dueño de la cuenta | Sin enumeración | Hay que verificar el email antes de ver el panel; más pasos | Descartada |

Mitigaciones de la enumeración aceptada: rate limit de signup de 5/h por IP **contando rechazos**
(DD-9), costo de argon2id por intento (el hash se calcula antes de saber si el email existe),
el `409` no dice empresa, nombre ni estado (INV-20), y `security_event=signup_email_exists` con su
métrica para detectar barridos (DD-19).

### ¿Siguen siendo no enumerables el login y el reset?

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Mantenerlos no enumerables** (mismas respuestas, hash ficticio, bloqueo por HMAC también para emails inexistentes, `202` constante) | Defensa en profundidad: el registro queda como **único** oráculo y es el más lento (5/h por IP, formulario completo); ya está diseñado y probado, no cuesta nada extra | "Protege" algo que el registro igual revela | **Elegida** (DD-19) |
| Simplificar: login dice "no existe ese usuario", reset responde 404 | Mensajes más claros; `login_throttles` podría indexarse por `user_id` | Crea oráculos 240 veces más rápidos (login 20/min por IP); habilita *credential stuffing* dirigido solo a cuentas existentes; el bloqueo pasaría a revelar existencia | Descartada |

### Coherencia del "recuperar contraseña" sugerido

El `409` del registro sugiere recuperar la contraseña también a quien existe como **invitado** (sin
contraseña todavía) o **desactivado**. Opciones para el pedido de reset en esos estados:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Invitado: reemitir la invitación (7 días) y enviar el email de invitación. Desactivado: nada** | El invitado recibe lo que necesita sin plantilla nueva (reusa DD-5); el desactivado no puede entrar igual | Un invitado puede renovar su invitación por su cuenta (solo llega a su propio email; rate limit 3/h por email) | **Elegida** (DD-20) |
| No enviar nada a invitados | Sin cambios | El invitado que sigue la sugerencia del `409` no recibe nada y queda trabado | Descartada |
| Enviar un enlace de reset al invitado para que defina contraseña | Un solo flujo | Saltearía la aceptación de la invitación (nombre, auditoría `invitation_accepted`) y mezclaría dos estados | Descartada |
| Email explicativo al desactivado ("tu usuario está desactivado") | Mejor experiencia | Plantilla nueva; el desactivado igual debe hablar con su Administrador | Descartada por ahora (reevaluable) |

### Bloqueo por intentos

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Contador por HMAC del email, exista o no la cuenta** | Bloqueo idéntico para emails inexistentes; no guarda emails de no-usuarios en claro | Cualquiera puede bloquear una cuenta ajena 15 min | **Elegida** (se mantiene tras P-4) |
| Contador en `users` | Simple | El bloqueo solo ocurre en cuentas existentes: el `429` revela que la cuenta existe | Descartada |
| Solo rate limit por IP | Sin bloqueo de cuentas | No cumple Historia 2.2 y no frena ataques distribuidos | Complemento, no reemplazo |

### Invitación con email existente

Responde `409 email_taken`, sin `suggested_action` (DD-21): el Administrador no puede recuperar la
cuenta de otra persona. Como el registro ya confirma la existencia de un email, ocultarlo en la
invitación no agregaría protección.

## R-16 Último Administrador bajo concurrencia → INV-10

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`SELECT ... FROM tenants WHERE id = $1 FOR UPDATE` al inicio de cada cambio de rol/estado** | Simple, explícito, serializa solo dentro de la empresa | Hay que recordarlo en cada operación (tests concurrentes lo vigilan) | **Elegida** |
| Aislamiento `SERIALIZABLE` | Correcto sin locks explícitos | Reintentos por errores de serialización que hay que manejar | Descartada |
| Advisory lock por empresa | Sin tocar filas | Otra primitiva que aprender; claves numéricas a derivar del UUID | Descartada |
| Trigger que verifica la regla | En la base | Lógica de negocio escondida en la base; mismo problema de concurrencia sin lock | Descartada |

> **Nota 2026-10-05** (DD-40, plan §18 novena tanda): la opción elegida se mantiene, pero con
> `FOR NO KEY UPDATE` en lugar de `FOR UPDATE`. La evaluación no consideró que PostgreSQL verifica
> cada FK hacia `tenants` con `SELECT … FOR KEY SHARE` sobre la fila de la empresa, incompatible con
> `FOR UPDATE`. Un reset o un login que ya tenía bloqueado al usuario y después insertaba su token,
> sesión o auditoría quedaba en ciclo con una operación de administración que tenía la empresa y
> esperaba al usuario (`40P01`, reproducido en CI con PostgreSQL 18). `FOR NO KEY UPDATE` sigue
> serializando entre sí las operaciones de administración de una empresa y no conflictúa con
> `FOR KEY SHARE`. La desventaja "hay que recordarlo en cada operación" ahora incluye el modo; lo
> vigilan la regresión y la regla estática de T-B604. El advisory lock tampoco chocaría con las FK,
> pero sigue sin agregar nada frente a `FOR NO KEY UPDATE`.

## R-17 Rate limiting → DD-9

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **En memoria (`golang.org/x/time/rate`), por IP y por email** | Sin infraestructura; paquete del proyecto Go | Por instancia | **Elegida** (S-1) |
| En PostgreSQL | Compartido entre instancias | Escritura por request anónimo | Descartada hasta tener varias instancias |
| En el proxy (nginx, Cloudflare) | Fuera de la app | Depende del hosting aún no elegido | Complemento posible (también contra enumeración distribuida por el registro) |

La IP que usa el rate limit sale de `httpx.ClientIP` y se agrupa por /64 en IPv6 (R-25).

## R-18 Logs y métricas → DD-12

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`log/slog` + `expvar` (stdlib)** | Cero dependencias; JSON estructurado | `expvar` es básico (sin histogramas) | **Elegida** para el MVP |
| zap / zerolog | Rápidos | Innecesarios a esta escala | Descartada |
| Prometheus `client_golang` | Estándar de métricas | Dependencia y servidor de métricas que el hosting aún no define | Reevaluar al elegir hosting |

## R-19 Montaje de la SPA junto a la API (H-1) → DD-22, ADR-019

Complementa R-F08 (vista del frontend) con lo que le importa al backend: cómo se reparte el
tráfico sin romper los tests de rutas.

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`http.ServeMux` raíz en `internal/app`: `/api/` → chi, `GET /healthz`/`GET /readyz` → ops, `/` → `web`; middlewares comunes envolviendo al mux raíz** | `chi.Walk` sobre el router de la API ve solo la API (T-B801 y el test de rutas contra el contrato no cambian); `/api/…` inexistente sigue siendo `404 problem+json`; cabeceras de seguridad y `CrossOriginProtection` iguales para todo | Dos formas de declarar rutas (tres destinos fijos en stdlib, la API en chi) | **Elegida** |
| `r.Mount("/", web)` o `r.NotFound(web)` en chi | Un solo router | `NotFound` de la API pasaría a devolver `index.html`; `Mount` en `/` aparece en `chi.Walk` y obliga a excluirlo a mano en los tests | Descartada |
| Router chi raíz con un subrouter `/api` y la SPA en otro subrouter | Todo chi | Mismo problema de `Walk` (hay que filtrar); más difícil de explicar que tres líneas de `ServeMux` | Descartada |
| Proxy delante (nginx/Caddy) que separa `/api` de los estáticos | Separación en infraestructura | Otro proceso y hosting sin definir (P-1); contradice el binario único de ADR-019 | Descartada |

Dónde van los middlewares: los que valen para cualquier respuesta (IP del cliente, request id,
recover, logging, cabeceras de seguridad, `CrossOriginProtection`) envuelven al mux raíz; los que
son de la API (`no-store`, rate limit, sesión, permisos) quedan dentro de chi; los de la SPA (CSP,
caché por tipo de archivo, gzip) dentro de `web`.

## R-20 Caché del logo por empresa (H-2) → DD-23

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`private, no-cache` + `ETag` = UUID del objeto; `304` sin leer S3; parámetro `v` declarado e ignorado** | Nunca se muestra un logo sin preguntarle al servidor con la sesión actual (el `ETag` de otra empresa no coincide); revalidar cuesta un `304`; la UI puede cambiar la URL para refrescar el `<img>` al instante | Una revalidación por uso del logo | **Elegida** |
| `private, max-age=300` (diseño anterior) | Menos requests | En un celular compartido se ve hasta 5 min el logo de la empresa anterior, y el viejo tras reemplazarlo | Descartada (hallazgo H-2) |
| `no-store` | Lo más simple | Descarga el logo completo en cada uso (hasta 2 MiB por pantalla con encabezado) | Descartada |
| URL por empresa y versión (`/tenant/logo/{object_id}`) con `immutable` | Caché perfecta | Otro endpoint o un `logo_url` en `Tenant`; si la URL se filtra, el logo queda cacheado aunque la sesión cambie (sigue exigiendo sesión para descargarlo, pero el navegador no revalida) | Descartada: más contrato para un beneficio chico |
| Exponer `logo_version` en `Tenant` en vez del parámetro `v` | La UI no inventa la versión | Cambia `Tenant` y `TenantSummary`; la UI ya resuelve con `?v={id}-{updated_at}` | Descartada por ahora (se puede agregar sin romper) |
| Agregar `Vary: Cookie` | Separa entradas de caché por sesión | Con `no-cache` la revalidación ya consulta al servidor con la cookie; `Vary: Cookie` suele desactivar la caché en algunos navegadores | Descartada (innecesaria) |

## R-21 Compresión HTTP (H-8) → DD-29, ADR-019

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`github.com/klauspost/compress/gzhttp` envolviendo solo el handler de la SPA** | Mantenida; umbral por defecto de 1 KB; excluye tipos ya comprimidos; opciones para `ETag` (`SuffixETag`/`DropETag`); funciona en cualquier hosting | Dependencia nueva (aprobada por el usuario) | **Elegida** |
| Comprimir también la API | Menos bytes en listas grandes | Respuestas de 001 chicas; comprimir respuestas con datos personales y entrada reflejada obliga a analizar ataques tipo BREACH (la librería trae `RandomJitter`, pero es otra pieza a entender) | Descartada para 001; reevaluar con listados grandes (003, 009) |
| Implementar gzip a mano con `compress/gzip` | Sin dependencia | Manejo correcto de `Accept-Encoding`, `Vary`, `ETag`, `Content-Length`, tipos y umbral: fácil de hacer mal | Descartada |
| Precomprimir en el build (`.gz`/`.br`) | Cero CPU por request | Más lógica en el handler de la SPA para elegir la variante | Descartada por ahora |
| Delegar en el proxy del hosting | Cero código | Hosting sin definir (P-1) | Si el hosting comprime, se quita `gzhttp` (S-11 del plan) |

Supuesto a validar en T-F007: que `gzhttp` agregue `Vary: Accept-Encoding` (no quedó explícito en
la documentación consultada; si no lo hace, se agrega en el handler).

## R-22 Zona horaria en el registro (H-6) → DD-27

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Zona desconocida → default + log; `time/tzdata` embebido** | El usuario nunca ve un error por un campo invisible; con la base IANA embebida, toda zona real del navegador se reconoce aunque el contenedor sea mínimo | El binario crece unos cientos de KB; un navegador mal configurado deja la empresa en Buenos Aires (se corrige en Datos de la empresa) | **Elegida** |
| `422 invalid_timezone` y reintento del cliente sin la zona (DD-F10) | El servidor es estricto | Un request extra que consume cupo de rate limit de signup; lógica de reintento en la UI | Descartada (la UI puede quitar su reintento) |
| Solo embeber `tzdata`, sin default | Acepta todas las zonas reales | Un valor basura sigue dando `422` en un campo que no se ve | Descartada |
| Depender del `zoneinfo` del sistema | Binario más chico | En imágenes `distroless`/`scratch` falla toda zona salvo UTC | Descartada |

## R-23 HTTPS en desarrollo y cookie `__Host-` (H-3, H-10) → DD-24, nota en ADR-006

**Reescrita el 2026-09-29 (H-10).** La versión anterior elegía "`http://` solo para `localhost` y
`127.0.0.1`, con `COOKIE_SECURE=true`" y afirmaba que se podía desarrollar con Chrome o Firefox.
Es incorrecto: **Chrome/Chromium acepta cookies `Secure` en `http://localhost` pero rechaza el
prefijo `__Host-`**; Firefox la acepta; Safari no acepta ni `Secure` en `http://localhost` (fuentes
abajo y R-F15). Con esa regla no había sesión en Chrome ni en los E2E de Chromium.

### Cómo se obtiene la misma cookie que en producción en desarrollo y E2E (**usuario: HTTPS local**)

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **HTTPS local con un certificado de `mkcert`** (CA local instalada en el equipo; `crm serve` con TLS solo en modo local; Vite con `server.https` y el mismo certificado) | La cookie es **la misma** que en producción (`__Host-crm_session`, siempre `Secure`) en Chrome, Firefox y Safari; los E2E prueban lo real; no hay ninguna configuración que permita emitir la cookie sin `Secure` | Un paso de instalación por equipo (`mkcert -install` + `make dev-certs`), dos variables (`TLS_CERT_FILE`/`TLS_KEY_FILE`) y la CA a instalar también en CI | **Elegida por el usuario** (era el respaldo previsto en la versión anterior de esta sección) |
| `http://localhost` con `COOKIE_SECURE=true` (regla anterior, H-3) | Cero instalación | Sin sesión en Chrome/Chromium ni Safari; los E2E en Chromium (P-F5) no funcionan | Descartada (H-10) |
| Cookie sin prefijo (`crm_session`) cuando `APP_BASE_URL` es `http://localhost` | Cero instalación | Dos comportamientos de cookie; los tests dejan de probar la cookie real; una regla de configuración más que puede filtrarse a otro entorno | Descartada |
| Desarrollar y correr E2E solo con Firefox | Cero cambios | Contradice P-F5; el navegador más usado queda sin E2E | Descartada |

### ¿Se sigue aceptando `APP_BASE_URL=http://localhost`?

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **No: `APP_BASE_URL` siempre `https://`; se elimina `COOKIE_SECURE`** | Una regla de una línea; ninguna combinación de variables emite la cookie sin `Secure` (la amenaza A05 "cookie sin `Secure` en producción" desaparece por construcción); nada que la necesite (ver tests Go abajo) | Quien corra `crm serve` sin TLS en su equipo no puede abrirlo con el navegador (igual no tendría sesión) | **Elegida** |
| Mantener la excepción `http://localhost` para quien no instale mkcert | Arranque sin certificados | Configuración que "funciona a medias" (sin sesión en Chrome y Safari); dos caminos que documentar y probar | Descartada |
| Mantener `COOKIE_SECURE=false` para clientes no navegador | Tests sin TLS | Innecesario: los tests usan `httptest.NewTLSServer` o `httptest.NewRecorder`, y `curl` también puede usar HTTPS | Descartada |

### Tests de integración de la API que encadenan requests con la cookie

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`httptest.NewTLSServer(root)` + `srv.Client()` con un `cookiejar`** | El cliente ya confía en el certificado del servidor de test (sin mkcert en CI para `make check`); la cookie viaja por HTTPS como en producción; el jar aplica `Secure` y `Path` | No verifica el prefijo `__Host-` (el `cookiejar` de Go no implementa prefijos): se cubre con una aserción explícita sobre `Set-Cookie` | **Elegida** |
| `httptest.NewServer` (HTTP) + `cookiejar` | Igual de simple | Funciona solo porque el `cookiejar` de Go trata `localhost`/loopback como seguro (detalle de la librería, verificado en su código fuente actual); el test dependería de esa excepción | Descartada |
| `httptest.NewRecorder` copiando `Set-Cookie` a `Cookie` a mano | Sin servidor | Helper propio; no ejercita `Secure` ni `Path` | Se mantiene para tests de un solo request (handlers y middlewares) |
| `COOKIE_SECURE=false` | — | Variable eliminada | Descartada |

### Certificado en CI para los E2E

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`mkcert` en el job de E2E** (binario de una versión fijada con checksum verificado, `certutil` instalado, `mkcert -install`, `mkcert -cert-file … -key-file … localhost 127.0.0.1`), CA nueva por corrida | Mismo mecanismo que en desarrollo; los navegadores confían en el certificado (sin errores de certificado que puedan afectar al service worker); la CA muere con el runner | Pasos extra en el job; que Chromium y WebKit de Playwright usen los almacenes donde instala mkcert es un supuesto (8) | **Elegida** |
| Playwright con `ignoreHTTPSErrors` y un certificado sin CA de confianza | Sin instalar CA | Con errores de certificado el service worker podría no registrarse (supuesto sin verificar); no es lo que ve un usuario | Respaldo si falla el supuesto 8, solo si T-F703 (PWA) pasa igual |
| Certificado y clave versionados en el repo | Cero pasos | Una clave privada en el repositorio, aunque sea de desarrollo; la CA igual hay que instalarla | Descartada |
| Generar CA y certificado con `openssl` | Sin herramienta extra | Más pasos y más fácil equivocarse (SAN, extensiones) | Descartada |

### HSTS en modo local

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **No enviar `Strict-Transport-Security` cuando el host de `APP_BASE_URL` es `localhost`/`127.0.0.1`** | Evita que el navegador del desarrollador registre HSTS para `localhost` y fuerce HTTPS en otros proyectos locales (supuesto 9) | Una rama por modo en el middleware de cabeceras | **Elegida** |
| Enviarlo siempre | Sin ramas | Riesgo de "romper" `http://localhost` para otros proyectos del desarrollador | Descartada |

## R-24 Límite exacto del logo y del cuerpo multipart (H-11) → DD-31, DD-11, nota en ADR-011

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **2 MiB = 2 097 152 bytes, medidos sobre el contenido del archivo (la parte `file`)** | Decisión del usuario; el cliente puede apuntar a un número exacto sin adivinar el costo de los encabezados multipart | "2 MB" en textos de la UI es una aproximación (aceptable para el usuario final) | **Elegida por el usuario** |
| 2 000 000 bytes | Coincide con el "MB" decimal | Otro número; el usuario eligió MiB | Descartada |
| Limitar solo el cuerpo multipart completo | Una sola medida | El límite del archivo pasaría a depender del largo del nombre de archivo y del *boundary* que elige el navegador | Descartada |

Límite del cuerpo:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Cuerpo ≤ 2 097 152 + 65 536 = 2 162 688 bytes (`http.MaxBytesReader`), aparte del límite del archivo** | Acota lo que se lee de la red antes de mirar las partes; 64 KiB alcanzan para *boundary*, encabezados de la parte y un nombre de archivo largo | Dos constantes | **Elegida** |
| Sin límite de cuerpo, solo del archivo | Una constante | Un cliente puede enviar partes extra o encabezados enormes antes de la parte `file` | Descartada |

Cómo se lee:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`r.MultipartReader()` + exactamente una parte `file`, leída con un tope de `LogoMaxBytes + 1` bytes en memoria** | Sin archivos temporales; control exacto de qué partes se aceptan; saber que se superó el límite cuesta leer un byte de más; los bytes quedan en memoria para validar (firma, `image.DecodeConfig`) y subir a S3 con tamaño conocido | Hasta ~2 MiB por subida concurrente en memoria (irrelevante a la escala S-3) | **Elegida** |
| `r.ParseMultipartForm(maxMemory)` | Una llamada | Guarda en disco temporal lo que supera `maxMemory`; acepta partes extra sin avisar; más difícil de acotar con precisión | Descartada |
| *Streaming* directo a S3 mientras se valida | Sin buffer | La validación de tipo y dimensiones necesita el principio del archivo antes de decidir; complica el orden "validar antes de guardar" (ADR-011) | Descartada |

Metadatos EXIF en el servidor:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Guardar los bytes validados sin modificarlos; no depender de que el cliente quite EXIF** | El backend no cambia (el usuario lo pidió); no se decodifica la imagen completa en el servidor | Un cliente que no sea la SPA podría subir un JPEG con EXIF (ubicación, orientación) | **Elegida** (riesgo aceptado R-13 del plan; la SPA siempre vuelve a codificar, DD-F21) |
| Quitar los segmentos EXIF en Go sin decodificar | Sin GPS en el bucket | Se pierde la orientación sin aplicarla (la foto queda de costado); código de parseo de JPEG propio | Descartada |
| Decodificar y volver a codificar en Go | Limpieza total | Decodificar imágenes en el servidor es superficie de DoS; pérdida de calidad; cambia el comportamiento del backend | Descartada |

## R-25 IP del cliente detrás de proxies (`X-Forwarded-For`) → DD-32

Contexto: hoy la IP sale de `RemoteAddr`. Detrás del proxy del hosting (P-1 abierta) sería la IP
del proxy: el rate limit por IP (DD-9) pasaría a ser **global** (un atacante agota el cupo de
todos) y los logs, las sesiones y la auditoría quedarían con una IP inútil. La política tiene que
poder configurarse sin conocer todavía el hosting.

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`TRUSTED_PROXIES` (lista de CIDR, vacía por defecto) + `X-Forwarded-For` leído de derecha a izquierda salteando los proxies de confianza** | Sin configurar no se confía en nadie (seguro por defecto); la IP que se toma es la que agregó un proxy propio, no la que escribió el cliente; funciona con uno o varios proxies; se configura en el despliegue | Una variable más; si se olvida detrás de un proxy, el rate limit por IP vuelve a ser global (runbook) | **Elegida** |
| Tomar la IP de más a la izquierda de XFF | Trivial | La escribe el cliente: esquiva el rate limit y ensucia la auditoría con IPs falsas | Descartada |
| `TRUSTED_HOPS=n` (tomar la n-ésima desde la derecha) | Una sola cifra | Se rompe en silencio si cambia la topología o si alguien llega directo sin pasar por el proxy (tomaría una IP que puso el cliente) | Descartada |
| Leer también `Forwarded` (RFC 7239) y `X-Real-IP` | Más proxies soportados | Tres cabeceras que el cliente puede enviar y hay que reconciliar; los proxies comunes agregan XFF | Descartada por ahora (se agrega si el hosting elegido lo exige) |
| Esperar a elegir el hosting (P-1) | Sin trabajo ahora | Bloquea la Fase 2 (rate limit y logs) sin necesidad | Descartada |

Detalles:

| Tema | Opción elegida | Alternativa descartada y por qué |
|---|---|---|
| Valores peligrosos | `0.0.0.0/0` y `::/0` rechazados al arrancar | Aceptarlos: equivale a confiar en cualquier cliente (IP falsificable) |
| Entrada mal formada en XFF antes de encontrar la IP del cliente | Usar `RemoteAddr` y registrar `WARN` | Ignorarla y seguir: a la izquierda solo hay valores del cliente |
| Normalización | `netip.Addr.Unmap()` (una IPv4 escrita como IPv6 cuenta como IPv4), sin puerto | Comparar texto crudo: la misma IP podría tener dos claves de rate limit |
| Clave del rate limit en IPv6 | Prefijo /64 (IPv4: la dirección) | Dirección completa: un cliente IPv6 controla al menos un /64 y rota direcciones para esquivar el límite |
| Dónde se calcula | Un middleware (`httpx.ClientIP`) al principio de la cadena común; el resto lee el valor del contexto | Cada consumidor lee cabeceras: se desalinean y es fácil que uno confíe en XFF |

## R-26 Código de error para `405` → contrato v0.4.0

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`method_not_allowed` en `ErrorCode`, con cabecera `Allow`, como regla global del contrato** | El `code` coincide con el status; la UI lo muestra como error de programación; el mapa exhaustivo de mensajes del frontend obliga a darle texto | Un valor más en el enum (versión *minor*) | **Elegida** |
| Seguir usando `malformed_request` | Sin cambios de contrato | Es el código de los `400`: la UI mostraría "revisá los datos" ante un error de programación | Descartada |
| `not_found` | Sin cambios de contrato | Oculta un error distinto (la ruta existe) y rompe la cabecera `Allow` | Descartada |

`Allow` con chi v5.3.2 (verificado en el código fuente, `mux.go` líneas 411–418 y 521–532): el
manejador de `405` **por defecto** de chi agrega `Allow` con los métodos de la ruta, pero un
manejador propio registrado con `MethodNotAllowed` (el nuestro, para responder problem+json) se
llama **sin** esa lista, y el campo que la guarda (`methodsAllowed`) no es exportado. Por eso el
manejador propio arma `Allow` preguntándole al router por cada método con `Mux.Match(
chi.NewRouteContext(), método, ruta)`, que es lo que ya hace `internal/app/root.go`.

## R-27 Cancelaciones del cliente (`context.Canceled`) → plan §9.2

Contexto: con pgx v5.11, cuando el contexto del request se cancela (el cliente cortó la
conexión) la query devuelve `context.Canceled` sin envolver en un error de PostgreSQL. Hoy queda
sin clasificar y termina en `500 internal` con log `ERROR`.

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`db.ErrCanceled`; la capa HTTP no responde si el cliente se fue (log `INFO`, `status=499` solo en el log) y responde `503` si la cancelación vino de otro lado (apagado)** | Los `ERROR` quedan para bugs reales; no se escribe a una conexión cerrada; el apagado sigue dando una respuesta reintentable | Un centinela y una rama más | **Elegida** |
| Mapear a `ErrUnavailable` (`503`) | Sin centinela nuevo | Cuenta como caída de la base y dispara alertas por culpa de clientes que se fueron | Descartada |
| Dejarlo como `500` | Cero trabajo | `ERROR` falsos que tapan los bugs reales | Descartada |

En el worker, una cancelación por apagado deja el mensaje como estaba (`pending`, sin sumar
`attempts`): no es error recuperable ni definitivo.

## R-28 Primer `SET ROLE` a una empresa recién aprovisionada desde otra conexión → DD-34, nota (b) en ADR-005

Contexto (hallazgo de la Fase 1, 2026-09-30): en PostgreSQL 18.6, después de que una conexión
aprovisiona una empresa y hace `COMMIT`, el primer `SET LOCAL ROLE crm_t_<hex>` desde **otra**
conexión del pool de `crm_app` a veces falla con `42501 permission denied to set role`. Hipótesis
del desarrollador: la lista de roles "SET-ables" que cada backend guarda en caché
(`roles_is_member_of`, `acl.c`) todavía no procesó la invalidación de la membresía recién
concedida. La reproducción es el test
`TestTxRunner_NewCompanyIsUsableOnAnyConnectionRightAfterProvisioning`
(`internal/platform/db/txrunner_integration_test.go`), con registros concurrentes: 64 a 105 fallas
en 600 primeros usos. Efecto en producción: el primer request después de registrarse puede caer en
otra conexión y dar `500`, con una falsa alerta de `rls_violation`.

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Lectura de catálogo en el mismo viaje que el `SET LOCAL ROLE` + un único reintento en `InTenantTx`** (antes de `fn`; nunca después, ni en roles de sistema, ni en `AsTenant`) | La lectura cubre **todos** los caminos que cambian de rol (medido: 0/1800 y 0 fallas en 8 corridas de la suite); el reintento es una red si la lectura deja de alcanzar en otra versión; cambio local a `platform/db`, sin tocar servicios ni contrato; el log y la métrica dejan ver si la red se usa | Depende de un comportamiento interno no documentado (R-17 del plan); el reintento no cubre `InSystemTx` → `AsTenant` (resolución de sesión, justo el primer `GET /me` después de registrarse) | **Elegida por el usuario** (DD-34) |
| Solo el reintento | No depende de detalles internos para el caso normal: si falla, se reintenta | Cada primer uso de una empresa nueva paga un viaje fallido y un `WARN`; no cubre `AsTenant` (el camino más común después de registrarse), así que ese `500` seguiría pasando; que la **segunda** vez funcione tampoco está garantizado | Descartada como defensa única |
| Solo la lectura de catálogo | Una sentencia más en el mismo viaje; cubre todos los caminos; medido 0/1800 | Si una versión futura cambia cuándo se procesan las invalidaciones, vuelve el `500` sin red | Descartada como defensa única (se combina con el reintento) |
| `DISCARD ALL` o reconectar las conexiones del pool después de cada registro | Conexiones "limpias" | El registro no controla las demás conexiones del pool (cada backend tiene su caché); `DISCARD ALL` borra las sentencias preparadas que usa pgx; no está verificado que limpie esta caché; reconectar todo el pool por registro es desproporcionado | Descartada |
| Esperar o dormir un rato antes del primer uso | Trivial | No hay un tiempo seguro; agrega latencia al registro; enmascara el problema | Descartada |
| *Pool* de roles creados de antemano (R-04c) | La membresía a `crm_app` existe mucho antes del primer uso: el problema desaparece sin depender de detalles internos; resuelve también el lock de DD-33 | Redefine INV-14; una tabla y un job más | Postergada: salida estructural dentro de A si R-17 se materializa |
| Alternativa B de ADR-005 (rol único + variable de sesión) | Sin membresías nuevas en runtime: el problema no existe | Es cambiar la decisión del usuario (ADR que reemplace a ADR-005) | Argumento para la comparación abierta en P-1 |

Por qué el reintento es seguro **solo** en el `SET ROLE` inicial de `InTenantTx`:

| Punto | ¿Reintentar? | Razón |
|---|---|---|
| `SET LOCAL ROLE` inicial de `InTenantTx`, antes de `fn` | Sí, una vez | La transacción no ejecutó nada: el `ROLLBACK` no deshace trabajo y `fn` corre como mucho una vez |
| Un `42501` dentro de `fn` | No | Puede ser una violación real de RLS (INV-19); reintentar repetiría efectos y taparía un bug de aislamiento |
| `InSystemTx` (roles de sistema) | No | Sus membresías vienen del bootstrap y no cambian en runtime: un `42501` es un bug o un problema de despliegue |
| `Tx.AsTenant` / `Tx.AsSystem` a mitad de transacción | No | La transacción ya ejecutó la fase 1: reintentar exigiría repetir `fn` completa, y eso ya no es un detalle de `platform/db` |
| Otro SQLSTATE (p. ej. `22023`, el rol no existe; verificado en PostgreSQL 18) | No | No es este problema: es el caso de restore sin roles (plan §12.4) |

### ¿Conviene reportarlo a PostgreSQL?

**Sí, conviene**, con una reproducción mínima fuera de la aplicación. Si es un bug, un arreglo
upstream permitiría quitar la dependencia de un detalle interno (R-17). Si no lo es, la respuesta
aclara la causa real (S-16) y si la lectura de catálogo es un arreglo legítimo o una casualidad.
Qué debería tener el reporte (lo arma quien el usuario decida; no es código del repo):

- **Sin pgx**: dos o más sesiones de `psql` (o un script con `libpq`) conectadas como un rol
  equivalente a `crm_app`. En la sesión A: `CREATE ROLE r NOLOGIN; GRANT r TO app WITH INHERIT
  FALSE, SET TRUE; COMMIT`. En la sesión B, ya conectada de antes: `BEGIN; SET LOCAL ROLE r;`.
  Repetirlo en bucle y con varias sesiones B para medir la frecuencia de `42501`.
- **Registrar el orden exacto** de `BEGIN` de B frente al `COMMIT` de A. Es el dato que decide si
  es un bug. Si solo falla cuando B abrió su transacción **antes** del `COMMIT` de A, puede ser un
  comportamiento esperado de la caché dentro de una transacción ya iniciada, y eso diría además
  que el caso "me registro y el siguiente request falla" (secuencial) es menos probable que lo que
  muestra el test concurrente. Si falla también cuando el `BEGIN` de B es posterior, es más
  claramente un bug.
- Versión exacta (18.6) y, si es posible, la misma prueba en el último *minor* de 17 y de 16, para
  saber si es una regresión.
- El efecto de la lectura de catálogo previa como dato, no como pedido.
- Canal: la lista `pgsql-bugs` o el formulario de reporte de bugs del proyecto, siguiendo sus guías
  de reporte.

Hasta tener respuesta, DD-34 se mantiene tal cual; el test de reproducción queda en la suite para
detectar cualquier cambio de comportamiento al subir de versión.

## R-29 Clasificación de fallos SMTP, `last_error` y presupuesto del envío → ADR-024, DD-35

Contexto (revisión del PR fdelillo/crm#8, hallazgos I1, I2, M1 y M7): la Fase 2 trató todo `5xx`
como definitivo, incluidos los de la conexión (saludo `554` por IP bloqueada, `535` de AUTH), lo
que pasa toda la cola a `failed`; `last_error` no servía para diagnosticar; el envío corre con la
transacción del worker abierta, y un mensaje que falla antes de enviarse frenaba a todos.

Verificado en el código de go-mail v0.8.1 (*module cache*: `client.go`, `senderror.go`,
`smtp/smtp.go`):

- `DialAndSendWithContext` llama a `DialToSMTPClientWithContext` (TCP o TLS en 465, saludo en
  `smtp.NewClient`, `Hello`, STARTTLS, `auth`) y envuelve **cualquier** error de esa etapa como
  `dial failed: %w`; los errores de MAIL, RCPT y DATA son `*SendError` envueltos como
  `send failed: %w`.
- `*SendError` **no** implementa `Unwrap`; expone `Reason`, `ErrorCode()`, `EnhancedStatusCode()`
  (solo si el servidor anuncia `ENHANCEDSTATUSCODES`) e `IsTemp()` (si el texto empieza con `4`). Su
  `Error()` agrega `affected recipient(s): <direcciones>` y `affected message ID: …`.
- El `context` solo se usa en el *dial* (`context.WithDeadline(ctx, now + connTimeout)`). Después:
  `conn.SetDeadline` (saludo), `UpdateDeadline` (EHLO, STARTTLS, AUTH) y `checkConn` renueva el
  *deadline* antes del envío y antes del `RSET` (salvo `WithoutRset`). Ventanas: 5 con `RSET`, 4 sin
  él. `DefaultTimeout` = 15 s.
- `Msg.IsDelivered()` es `true` desde la respuesta al fin de datos, aunque después fallen el `RSET`
  o el `QUIT`.
- Con un `DialContextFunc` propio, go-mail no hace TLS implícito y considera la conexión no cifrada:
  el autodescubrimiento de AUTH deja de elegir `PLAIN`/`LOGIN`.

Qué fallos son definitivos:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| Todo `5xx` definitivo (estado anterior) | Simple | Un problema propio de configuración destruye la cola | Descartada |
| Definitivo = `5xx` de un `SendError` (sugerencia de la revisión) | Usa la separación natural de go-mail | Remitente no verificado (MAIL FROM), *relay* denegado `5.7.1` (RCPT TO) o rechazo de contenido (DATA) son de configuración y seguirían siendo definitivos para todos | Descartada como regla final; se toma la idea |
| `SendError.IsTemp()` | Ya viene en la librería | No distingue destinatario de configuración; no existe para el *dial* | Descartada |
| **Causa + fase, con la regla del destinatario** (`5.1.x`/`5.2.x` en RCPT TO o DATA; `550`/`551`/`553` en RCPT TO sin código extendido) | Solo es definitivo lo que es del mensaje; el sesgo a recuperable queda acotado por los 8 intentos | Algunos rechazos definitivos reales se reintentan 8 veces | **Elegida** |

Error de configuración persistente (`535`):

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Recuperable que consume intentos + `ERROR` por intento + corte del ciclo + métrica por causa** | Final acotado (~19 h) del payload con el token; visible; sin estado nuevo | Una caída de más de ~19 h hace fallar los mensajes de ese período | **Elegida** |
| No consumir intentos ante `config` | No se pierde nada mientras dure la caída | Mensajes y payloads pendientes sin plazo; los tokens de reset y verificación vencen igual | Descartada |
| *Circuit breaker* en memoria que pausa el worker | Menos intentos por segundo | Más estado; no reduce el total de intentos | Descartada |
| Alerta propia (email o *pager*) | Aviso activo | Sin infraestructura de alertas en 001; avisar por email cuando el SMTP no anda no sirve | Descartada |

Presupuesto del envío frente a `idle_in_transaction_session_timeout` (30 s):

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Presupuesto fijo de 20 s + 5 s por etapa + `WithoutRset` + tests que fijan la relación** | Cambio local al adaptador y al `Dispatcher`; verificable | Un proveedor lento en una etapa produce reintentos | **Elegida** (DD-35) |
| Subir `idle_in_transaction_session_timeout` de `crm_app` | Cero código | Debilita la protección contra transacciones olvidadas en toda la aplicación | Descartada |
| Enviar fuera de la transacción (*lease* sobre `next_attempt_at`) | Sin acoplamiento; libera la conexión durante el envío | Cambia el modelo de reclamo de ADR-010 y la semántica de cancelación ya probada | Postergada (salida si S-17 falla) |
| `DialContextFunc` propio que cierra la conexión al cancelar | `context` respetado en toda la conversación | Rompe TLS implícito y la elección de AUTH | Descartada |

Aislamiento por mensaje:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Aplazar en otra transacción (`next_attempt_at`, demora por la edad del mensaje)** | Privilegio y política existentes; *backoff* exponencial sin estado; el ciclo sigue | Una escritura nueva como `crm_worker` | **Elegida** |
| Excluir los ids fallidos dentro del ciclo | Sin escrituras | El mensaje vuelve a ocupar el primer lugar en cada ciclo; un `ERROR` cada 2 s | Descartada |
| `SAVEPOINT` en la misma transacción | Sin carrera entre transacciones | Amplía la interfaz `db.Tx` | Descartada |
| Contador de aplazamientos en una columna nueva | *Backoff* exacto | Migración y privilegio nuevos; la edad da lo mismo | Descartada |

## R-30 Imagen de MinIO para desarrollo y tests → DD-36, nota en ADR-011

Contexto (hallazgo M10): el test de integración usa `bitnamilegacy/minio@sha256:…` porque desde el
entorno de Codex no se pudo descargar `quay.io/minio/minio`; `compose.yaml` usa
`quay.io/minio/minio:latest` sin fijar. Situación verificada con búsqueda web el 2026-10-01:

- Octubre de 2025: MinIO deja de publicar binarios e imágenes de la edición comunitaria (solo
  código fuente). Última versión: `RELEASE.2025-10-15T17-29-55Z`, que corrige CVE-2025-62506.
- Abril de 2026: el repositorio `minio/minio` queda archivado.
- Septiembre de 2026: se borran `minio/minio` y `minio/mc` de Docker Hub, y desde el 2026-09-24/25
  `quay.io/minio/minio` responde `401` a todo pull anónimo (todos los tags y digests). **El fallo
  de descarga no fue del entorno.**

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| `quay.io/minio/minio` (`latest` o un `RELEASE.*` fijo) | Oficial | `401` sin login desde 2026-09-24/25; sin versiones comunitarias nuevas | Descartada |
| `minio/minio` en Docker Hub | — | Borrado | Descartada |
| `bitnamilegacy/minio@sha256:…` (lo que usa hoy el test) | Se descarga | Congelada en 2025-05-24, antes del arreglo de CVE-2025-62506; arranque y rutas de Bitnami distintos de los de MinIO, así que `compose.yaml` necesitaría otra configuración que los tests | Descartada |
| `cgr.dev/chainguard/minio` | Mantenida y parcheada (Chainguard compila un *fork* con los arreglos), procedencia SLSA, gratis, mismo entrypoint y comando | El nivel gratuito solo publica `latest`; no promete que un digest viejo siga disponible (otros proyectos terminaron copiándola a su propio registro por eso) | **Respaldo** |
| **`ghcr.io/coollabsio/minio:RELEASE.2025-10-15T17-29-55Z` + digest** | Última versión oficial compilada desde el código fuente con el `docker-entrypoint.sh` de MinIO; tag de versión reproducible; pública | Tercero; congelada, sin parches futuros | **Elegida** |
| Copia propia en GHCR (`ghcr.io/fdelillo/…`) | Control total | Un paso operativo y un paquete que mantener | Postergada (salida si la elegida desaparece) |
| Otro servidor S3 (SeaweedFS, Garage, RustFS) | Mantenidos | Más configuración (credenciales, *layout*); RustFS anterior a 1.0; ADR-011 fija MinIO en desarrollo | Postergada |

Por qué los parches no pesan para elegir: el contenedor solo escucha en `127.0.0.1` en desarrollo
y es efímero en tests; producción usa el servicio S3 que se elija con el hosting (P-1).

---

## Fuentes consultadas

- Go 1.27 (versión estable actual): <https://go.dev/doc/devel/release>
- `net/http.CrossOriginProtection` (Go 1.25), incluido `SetDenyHandler`: <https://pkg.go.dev/net/http#CrossOriginProtection>, <https://go.dev/src/net/http/csrf.go>, <https://www.alexedwards.net/blog/preventing-csrf-in-go>
- `gzhttp` (opciones de `ETag`, umbral, filtro de tipos, `RandomJitter`): <https://pkg.go.dev/github.com/klauspost/compress/gzhttp>
- `time/tzdata` (base IANA embebida; importarlo desde `main`): <https://pkg.go.dev/time/tzdata>
- PostgreSQL 18 `uuidv7()`: <https://www.thenile.dev/blog/uuidv7>
- PostgreSQL 16, cambios en `CREATEROLE` y opciones `INHERIT`/`SET` por *grant*: <https://www.percona.com/blog/improved-createrole-attribute-for-user-management-in-postgresql-16/>, <https://thebuild.com/blog/all-your-gucs-in-a-row-createroleselfgrant/>
- Rendimiento con miles de roles en PostgreSQL 16: <https://postgrespro.com/list/thread-id/2694306>
- CVE-2026-14666 (RLS y cambios de roles): <https://www.postgresql.org/support/security/CVE-2026-14666/>
- OWASP Password Storage (argon2id m=19 MiB, t=2, p=1): <https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html>
- goose Provider con `embed.FS`: <https://pressly.github.io/goose/documentation/provider/>
- libopenapi-validator: <https://github.com/pb33f/libopenapi-validator>
- oapi-codegen v2.8.0 (OpenAPI 3.1 inicial): <https://github.com/oapi-codegen/oapi-codegen/releases/tag/v2.8.0>
- go-mail: <https://github.com/wneessen/go-mail>
- Cookies `Secure` y con prefijo en `http://localhost` por navegador (H-10, aportadas por el frontend): <https://github.com/httpwg/http-extensions/issues/2605>, <https://issues.chromium.org/issues/40202941>, <https://bugzilla.mozilla.org/show_bug.cgi?id=1618113>
- `net/http/cookiejar` (trata `localhost`/loopback como seguro; no implementa prefijos `__Host-`/`__Secure-`): <https://raw.githubusercontent.com/golang/go/master/src/net/http/cookiejar/jar.go>
- mkcert (almacenes de confianza en Linux con `certutil`, `-cert-file`/`-key-file`, `-CAROOT`, `NODE_EXTRA_CA_CERTS` para Node, advertencia sobre `rootCA-key.pem`): <https://github.com/FiloSottile/mkcert>
- chi v5.3.2, manejador de `405` y cabecera `Allow` (código fuente en el *module cache* del proyecto, `github.com/go-chi/chi/v5@v5.3.2/mux.go`, funciones `MethodNotAllowedHandler`, `routeHTTP`, `methodNotAllowedHandler` y `Match`; y `context.go`, campo `methodsAllowed` no exportado)
- Lock de `GRANT crm_tenant` en PostgreSQL 16+: verificado por el revisor de la Fase 1 con un test de integración contra PostgreSQL 18 (sin fuente documental citada)
- `42501` en el primer `SET ROLE` desde otra conexión (R-28): observado y medido por el desarrollador en la Fase 1 contra PostgreSQL 18.6 (test `TestTxRunner_NewCompanyIsUsableOnAnyConnectionRightAfterProvisioning`, commit e3d990b). La causa (`roles_is_member_of` en `acl.c`) es su hipótesis; el arquitecto **no** la verificó en el código fuente de PostgreSQL y no hay fuente documental (supuesto 12)
- go-mail v0.8.1, comportamiento de errores, *deadlines* y `context` (R-29): código fuente en el *module cache* del proyecto (`github.com/wneessen/go-mail@v0.8.1/client.go`: `DialAndSendWithContext`, `DialToSMTPClientWithContext`, `auth`, `sendSingleMsg`, `checkConn`, `ResetWithSMTPClient`, `WithDialContextFunc`; `senderror.go`; `smtp/smtp.go`: `NewClient`, `Hello`, `StartTLS`); `db/bootstrap/001_roles_and_database.sql` (`idle_in_transaction_session_timeout`)
- Códigos de estado extendidos de SMTP (RFC 3463): <https://www.rfc-editor.org/rfc/rfc3463>
- Imágenes de MinIO (R-30): <https://www.chainguard.dev/unchained/secure-and-free-minio-chainguard-containers>, <https://github.com/gofr-dev/gofr/pull/4378>, <https://github.com/HeliosSoftware/hfs/issues/1520>, <https://github.com/ponack/crucible-iap/issues/387>, <https://bex.co/blog/2026/09/25/minio-docker-hub-removal-quay-repoint>, <https://github.com/enorm-labs/event-junkie/issues/1859>, <https://github.com/coollabsio/minio>, CVE-2025-62506: <https://asec.ahnlab.com/en/91238/>

Supuestos a validar durante la implementación (no verificados con documentación primaria):

1. `SET LOCAL ROLE` a un rol cuya membresía se otorgó **en la misma transacción** funciona (el
   registro atómico lo necesita). Spike T-B012 en la Fase 0.
2. sqlc parsea sin errores las migraciones con `CREATE POLICY`, `GRANT` por columna y `ALTER
   DEFAULT PRIVILEGES` (ignorando lo que no modela). Spike T-B008 en la Fase 0.
3. La versión exacta de PostgreSQL 18 que corrige CVE-2026-14666 (el aviso menciona 18.5; las
   notas de la versión, 18.6): usar el último *minor* disponible.
4. El arreglo del cálculo de membresías con miles de roles está incluido en PostgreSQL 18 (se
   mide en T-B905).
5. Las políticas `TO crm_auth`/`TO crm_worker` y el comportamiento de `SELECT ... FOR UPDATE`
   con RLS (aplica también las políticas de `UPDATE`) se comportan como describe
   `data-model.md` §3.4: los tests T-B109, T-B110 y T-B211 lo confirman.
6. `gzhttp` agrega `Vary: Accept-Encoding` a las respuestas comprimibles (T-F007).
7. Con el *proxy* de Vite (ahora en `https://localhost:5173`), el navegador envía
   `Sec-Fetch-Site: same-origin` en los `POST` a `/api` y `CrossOriginProtection` los acepta sin
   `AddTrustedOrigin` (spike de T-F006).
8. En CI, tras `mkcert -install` con `certutil` disponible, Chromium y WebKit de Playwright confían
   en el certificado de `localhost` (mkcert documenta el almacén del sistema y NSS; que los
   navegadores de Playwright los usen no está verificado). Se valida en la primera corrida de
   T-F701; respaldo: `ignoreHTTPSErrors` si T-F703 pasa igual.
9. Un navegador que recibe `Strict-Transport-Security` por HTTPS en `localhost` puede registrarlo
   y forzar HTTPS en otros puertos de `localhost` (no verificado con fuente primaria; omitirlo en
   modo local no tiene costo, así que se omite igual).
10. pgx v5.11 devuelve `context.Canceled` (o un error que lo envuelve, comprobable con
    `errors.Is`) cuando se cancela el contexto de una query. Observado por el desarrollador en la
    Fase 1; lo fijan los casos nuevos de T-B105.
11. El proxy del hosting que se elija **agrega** su entrada a `X-Forwarded-For` (no la reemplaza)
    y sale desde rangos conocidos (S-14 del plan). Se verifica al cerrar P-1.
12. La causa del `42501` de R-28 es la caché de membresías por backend, y leer un catálogo antes
    del `SET ROLE` hace que el backend procese las invalidaciones pendientes (S-16 del plan). Es
    consistente con la medición, pero no está verificado en el código de PostgreSQL ni
    documentado. Lo vigila el test de reproducción de T-B103; el reporte upstream sugerido lo
    confirmaría.
13. El proveedor SMTP transaccional de producción responde cada etapa en menos de 5 s (S-17 del
    plan, DD-35). Se mide al elegir el proveedor (P-1).
14. `ghcr.io/coollabsio/minio:RELEASE.2025-10-15T17-29-55Z` sigue descargable sin login y arranca
    como la imagen oficial (`server /data`, `MINIO_ROOT_*`, `/minio/health/ready`) (S-18 del plan,
    DD-36). Lo confirma el test de T-B215; si falla, respaldo de R-30.

---

## Frontend

Autor: `frontend-architect` (2026-09-29). Diseño en [`ui.md`](ui.md); decisiones estructurales en
ADR-015 a ADR-023. Las marcadas **(usuario)** las tomó el usuario; el resto son defaults del
arquitecto pendientes de aprobación.
**Revisión 2026-09-29**: respuestas del usuario a P-F1 a P-F5; R-F08 (compresión aprobada,
desarrollo local y H-10), R-F13 (DD-F10, DD-F11, DD-F21) actualizadas; R-F14 (preparación del logo
en el navegador) y R-F15 (cookie de sesión en `http://localhost`, H-10) agregadas.
**Segunda revisión 2026-09-29**: decisiones del usuario sobre H-10 (HTTPS local con mkcert), H-11
(archivo ≤ 2 097 152 bytes) y P-F6 (todo JPEG se re-codifica). R-F08 (desarrollo local sobre
HTTPS), R-F11 (E2E con la CA de mkcert), R-F13 (DD-F21 confirmada, DD-F22 nueva), R-F14 (tope de
bytes del archivo preparado) y R-F15 (resuelta) actualizadas; R-F16 (prueba en un celular real,
H-12) agregada.

Criterios que se repiten: (1) simplicidad para un usuario con nivel básico en React/TypeScript
(pocas abstracciones propias, patrones de la documentación oficial); (2) accesibilidad y uso desde
el celular; (3) el contrato OpenAPI como fuente de verdad; (4) mismo origen que la API (ADR-006);
(5) dependencias mantenidas y justificadas contra la alternativa nativa.

## R-F01 Herramienta de build y tipo de aplicación → ADR-015

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Vite + React + TypeScript, SPA** | Estándar actual; *proxy* de desarrollo incluido; build estático que Go puede embeber; Vitest comparte configuración; documentación de shadcn/Tailwind v4 lo asume | Sin SSR (no hace falta: todo está detrás del login) | **Elegida** |
| Next.js / React Router *framework* / Remix | SSR, rutas por archivo | Runtime de servidor Node que no se embebe como estáticos; conceptos de servidor sin requisito; rompe el "un binario" | Descartada |
| Rsbuild / Parcel / webpack | Funcionan | Menos ejemplos con el stack elegido; sin integración directa con Vitest | Descartada |
| JavaScript sin TypeScript | Menos para aprender al principio | Se pierden los tipos derivados del contrato | Descartada (la constitución fija TypeScript) |

## R-F02 Ubicación del frontend en el repositorio → ADR-015

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`web/` en el mismo repositorio** (con `web/embed.go`) | Un PR cambia contrato, backend y frontend juntos; el contrato está a un `../`; `go:embed` necesita que `dist` esté bajo el paquete que embebe | Node y Go en la misma CI | **Elegida** |
| Repositorio separado | Ciclos independientes | Versiones cruzadas del contrato; dos CI; sin beneficio para un equipo chico con un binario | Descartada |
| Dentro de `internal/` | Junto al código Go | Mezcla un proyecto Node con paquetes internos; nombre poco reconocible | Descartada |

## R-F03 Organización de carpetas → ADR-015

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Por feature con *colocation*** (`features/<dominio>/` con pantallas, hooks, esquemas y tests) + `components/`, `lib/`, `api/` compartidos | Lo que cambia junto vive junto; una spec = una carpeta; mismo criterio que ADR-001 en Go | Hay que decidir qué es compartido (regla: lo usan dos features) | **Elegida** |
| Por tipo técnico (`components/`, `hooks/`, `services/` globales) | Familiar | Cada feature dispersa en cinco carpetas que crecen sin límite | Descartada |
| *Feature-Sliced Design* u otras metodologías con capas | Reglas explícitas | Muchos conceptos (capas, slices, segmentos) para un usuario que aprende | Descartada |

## R-F04 Componentes y estilos **(usuario: shadcn/ui + Tailwind)** → ADR-016

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **shadcn/ui + Tailwind CSS v4** | Accesibilidad de Radix en diálogos, menús y radios sin escribirla; componentes copiados al proyecto (código propio, legible); un solo sistema de estilos | Tailwind v4 exige Safari ≥ 16.4, Chrome ≥ 111, Firefox ≥ 128; actualizar componentes es manual | **Elegida por el usuario** |
| MUI / Mantine / Chakra | Completas | Sistema de temas propio, bundle más grande, estilo difícil de cambiar | Descartada |
| React Aria + estilos propios | Accesibilidad excelente | Más bajo nivel; todo el estilo a mano | Descartada |
| Componentes a mano + CSS Modules | Control total | Reescribir diálogos, menús y selects accesibles | Descartada |

Base de shadcn:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Radix UI** | Madura, la más documentada; la mayoría de los ejemplos y respuestas la usan | Algunas piezas insertan `<style>` en runtime (CSP `style-src 'unsafe-inline'`) | **Elegida** |
| Base UI | Más nueva, API moderna | Menos material disponible hoy | Descartada por ahora |

Íconos: **lucide-react** (el set de shadcn) frente a Phosphor (sugerido por el catálogo de diseño):
se elige lucide para no mezclar estilos con los componentes de shadcn.

Tipografía: **fuente del sistema** (0 KB, LCP más rápido) frente a Inter autoalojada (aspecto
uniforme entre plataformas, ~20–40 KB por peso, estimación no medida). Se elige la del sistema
(DD-F14).

## R-F05 Router **(usuario: React Router en modo SPA/librería)** → ADR-017

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **React Router v7, variante *data* (`createBrowserRouter`), sin loaders para datos** | `useBlocker`, `ErrorBoundary` y `lazy` por ruta incluidos; el server state queda en TanStack Query | Configuración en objetos (menos intuitiva que JSX); hay que evitar loaders por convención | **Elegida** (modo SPA: usuario; variante: arquitecto) |
| React Router declarativo (`<BrowserRouter>`) | Lo más simple | Sin `useBlocker` ni `ErrorBoundary` por ruta; *code splitting* manual | Descartada |
| React Router con loaders como capa de datos | Una librería menos | Sin caché compartida ni invalidación por clave; dos cachés si se combina con Query | Descartada |
| TanStack Router | Rutas tipadas | Menos difundido; el usuario eligió React Router | Descartada |

## R-F06 Server state y cliente HTTP **(usuario: TanStack Query + openapi-fetch)** → ADR-018

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **TanStack Query + openapi-fetch, hooks escritos por feature** | Caché, deduplicación, reintentos, invalidación, pausa sin conexión; cliente tipado sin código generado; claves de caché explícitas | Dos librerías a aprender | **Elegida por el usuario** |
| `fetch` en `useEffect` | Sin dependencias | Cada pantalla reimplementa carga, error e invalidación | Descartada |
| SWR | Más simple | Menos control de mutaciones e invalidación | Descartada |
| RTK Query | Potente | Trae Redux y un store global | Descartada |
| `openapi-react-query` (hooks sobre openapi-fetch) | Menos código | Claves de caché generadas (menos explícitas); otra abstracción | Descartada |
| Generadores de hooks (orval, hey-api) | Todo generado | Mucho código generado que leer | Descartada |

Estado global del cliente:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Sin store global**: server state en Query, local con `useState`, formularios en React Hook Form, toasts en sonner | Nada que sincronizar; menos conceptos | — | **Elegida** |
| Context propio para la sesión | Familiar | La sesión es server state (`/me`): copiarla obliga a sincronizarla | Descartada |
| Zustand | Simple | Sin estado global que cambie seguido en 001 | Descartada |

Manejo del `401` (sesión vencida):

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Global en `QueryCache`/`MutationCache.onError`, callback inyectado al crear el `QueryClient`** | Un solo lugar; se prueba aislado; distingue `unauthenticated` de `invalid_credentials` por `code` | Hay que excluir la query de sesión (mapea `401` a `null`) | **Elegida** |
| Middleware de openapi-fetch (`onResponse`) | Antes de llegar a Query | No tiene acceso natural a la caché ni al router; mezcla transporte con navegación | Descartada |
| Por pantalla | Control fino | Se olvida en alguna; comportamiento inconsistente | Descartada |
| Reautenticación en un diálogo sin salir de la pantalla | No pierde lo cargado | Más estados, reintento de la mutación original, foco; desproporcionado para formularios de 001 | Descartada para 001 (DD-F5) |

## R-F07 Tipos del contrato y *bundle* multi-spec **(usuario: openapi-typescript)** → ADR-018

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Un archivo de tipos por spec (`redocly.yaml` → `generated/NNN.ts`) combinados por intersección de `paths`** | Sin conflictos de nombres de componentes; diffs por spec; agregar una spec = una línea | Una ruta repetida en dos specs se mezclaría (lo detecta un test de tipos) | **Elegida** |
| `redocly join` en un contrato único y un solo archivo de tipos | Un archivo | Conflictos de componentes y `tags` a resolver en cada spec | Descartada |
| `redocly bundle` por spec antes de generar | Resuelve `$ref` externos seguro | Paso extra si `openapi-typescript` ya los resuelve | **Respaldo** si falla S-F1 |
| Tipos escritos a mano | Sin herramienta | Se desincronizan; viola ADR-014 | Descartada |

## R-F08 Distribución **(usuario: SPA embebida con `go:embed`, mismo origen, proxy de Vite)** → ADR-019

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Embebida en el binario, mismo origen** | Cookie y CSRF de ADR-006 sin cambios; un artefacto; versiones alineadas | Compilar el frontend antes; chunks viejos desaparecen en cada deploy | **Elegida por el usuario** |
| CDN / otro dominio | Caché en el borde | CORS con credenciales y rediseño de CSRF | Descartada |
| nginx/Caddy delante | Estándar | Otro proceso; hosting sin definir (P-1) | Opción futura |
| Archivos desde disco | Cambiar la UI sin recompilar | Binario no autocontenido | Descartada |

Montaje del handler de la SPA:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Mux raíz: `/api/` → chi; `/healthz`, `/readyz`; resto → SPA** | Los tests de rutas del backend (T-B004, T-B801, rutas vs contrato) no cambian; un `/api/…` inexistente sigue en `404 problem+json` | Un nivel más de ruteo en `internal/app` | **Elegida** (H-1; adoptada por el backend, DD-22 y R-19) |
| Ruta `/*` dentro de chi | Directo | Rompe `chi.Walk` y los tests de cobertura | Descartada |

*Fallback* y cabeceras:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`index.html` para rutas sin extensión; `404` para archivos inexistentes; `no-cache` en HTML, `immutable` en assets con hash** | No sirve HTML como JS; cada carga ve la versión nueva; assets descargados una vez | — | **Elegida** (tabla de referencia: `plan.md` §10.7) |
| `index.html` para todo lo que no exista | Simple | Chunks viejos reciben HTML (error de MIME) y pueden cachearse | Descartada |
| HTML con `max-age` | Menos requests | Pestañas nuevas con `index.html` viejo apuntando a chunks inexistentes | Descartada |

CSP:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`script-src 'self'` estricto + `style-src 'self' 'unsafe-inline'`** | Frena XSS por scripts; compatible con Radix/sonner | Estilos inline permitidos (riesgo bajo) | **Elegida** |
| CSP estricta con *nonces* | Máxima | Requiere HTML generado por request | Descartada |
| Sin CSP | Nada que mantener | Sin defensa en profundidad ante XSS | Descartada |

Desarrollo local (**revisado en la segunda revisión**: HTTPS local, decisión del usuario sobre
H-10):

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Proxy de Vite sobre HTTPS**: Vite en `https://localhost:5173` con `server.https` y el certificado de mkcert de `.certs/` (el mismo de `crm serve`); `/api` → `https://localhost:8443` sin `changeOrigin` y con verificación de certificado; Node confía en la CA con `NODE_EXTRA_CA_CERTS`; `strictPort` | Incluido en Vite; mismo origen para el navegador; la cookie `__Host-` real en cualquier navegador; una configuración rota se ve (error de certificado) en vez de quedar escondida | Exportar una variable de entorno por shell; un paso de instalación por equipo (lo pide igual el backend) | **Elegida** (proxy: usuario; HTTPS: usuario, H-10; detalle: DD-F23) |
| Proxy de Vite por HTTP a `http://localhost:8080` (versión anterior) | Cero instalación | Chrome rechaza la cookie `__Host-` en `http://localhost` y Safari rechaza `Secure` (H-10); el backend ya no acepta `APP_BASE_URL` con `http://` | Descartada |
| Proxy con `secure: false` (sin verificar el certificado del backend) | No hace falta `NODE_EXTRA_CA_CERTS` | Acostumbra a ignorar errores de certificado; un certificado vencido o equivocado pasa inadvertido; diferente de lo que hace el navegador | Descartada |
| `@vitejs/plugin-basic-ssl` (certificado autofirmado propio de Vite) | Sin mkcert para el frontend | Otro certificado distinto del de `crm serve`; el navegador muestra un aviso cada vez; una dependencia más | Descartada |
| Go reenvía a Vite | Un solo puerto | Código Go solo para desarrollo | Descartada |

Compresión:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`gzhttp` en Go envolviendo solo el handler de la SPA** | Funciona en cualquier hosting | Dependencia Go nueva | **Elegida** (aprobada por el usuario, DD-29 y R-21 del backend) |
| Archivos precomprimidos (`.br`/`.gz`) en el build | Sin costo de CPU por request | Más lógica en el handler | Descartada por ahora |
| Delegar en el proxy del hosting | Cero código | Hosting sin definir (P-1) | Si el hosting lo trae, se quita `gzhttp` |

## R-F09 PWA y service worker → ADR-020

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Manifest + SW mínimo a mano (solo navegaciones → `offline.html`)** | Instalable; página propia sin conexión; no guarda nada de la app ni de la API | Sin precache (la caché HTTP cubre visitas repetidas) | **Elegida** |
| vite-plugin-pwa / Workbox con precache | Arranque más rápido; base para offline | Precache de la app: aviso de versión nueva, recargas; más conceptos | Descartada hasta que haya offline |
| Sin SW | Nada que mantener | Sin sugerencia automática de instalación; error genérico sin conexión | Descartada |
| Cachear `/api` | Algo de lectura sin conexión | Datos viejos y de otra sesión en dispositivos compartidos | Descartada (NFR-F09) |

## R-F10 Formularios y validación → ADR-021

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **React Hook Form + Zod (esquemas atados al tipo del contrato)** | Errores por campo, foco, `dirtyFields`; pocos re-renders; los `422` se aplican por nombre de campo | Dos librerías; los valores de las reglas se copian del contrato | **Elegida** |
| Nativo controlado | Sin dependencias | Cada formulario reimplementa errores y foco | Descartada |
| TanStack Form | Muy tipado | Más nuevo, menos ejemplos con shadcn | Descartada |
| React 19 `useActionState` | Nativo | Pensado para SSR/acciones de servidor | Descartada |
| Zod generado del OpenAPI | Sin duplicar reglas | Otra herramienta de generación | Descartada |

Cuándo validar:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Al enviar y, tras el primer intento, al cambiar** | No molesta mientras se escribe; el error desaparece apenas se corrige | El usuario ve los errores recién al enviar | **Elegida** |
| Al salir de cada campo | Aviso temprano | Errores sobre campos que el usuario todavía no terminó | Descartada |
| En cada tecla | Inmediato | Ruido y errores prematuros | Descartada |

## R-F11 Tests del frontend → ADR-022

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Vitest + Testing Library + MSW (jsdom)** | Misma configuración que Vite; consultas por rol; red interceptada en el borde con el cliente real | Mantener handlers alineados al contrato (tipados); jsdom no tiene canvas (el adaptador del logo se sustituye y se prueba en E2E) | **Elegida** |
| Jest | Estándar histórico | Configuración de TS/ESM extra | Descartada |
| happy-dom | Más rápido | Menos fiel | Respaldo (S-F2) |
| Vitest modo navegador | Más fiel (canvas real) | Más lento y más piezas | Reevaluable |
| Paquete `canvas` de Node para jsdom | Canvas en Vitest | Dependencia nativa; no reproduce los codificadores ni la orientación EXIF de cada navegador | Descartada |
| Mockear hooks de datos | Rápido | No prueba la integración real | Descartada |

End-to-end:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Playwright contra el binario en `https://localhost:8443` + `docker compose` + axe** | Prueba SW, CSP, la cookie `__Host-` real, cabeceras y canvas reales; Chromium y WebKit | Necesita Docker, minutos de CI y la CA de mkcert en el runner (receta de plan §10.5.1; supuesto S-12) | **Elegida**; Chromium en cada PR, WebKit antes de liberar (**usuario, P-F5**) |
| Cypress | Buena experiencia | Sin WebKit | Descartada |
| Solo pruebas manuales | Nada que mantener | Sin regresión automática en flujos críticos | Descartada |

Certificado en los E2E (complementa R-23 del backend):

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **CA de mkcert instalada en el runner; Playwright sin `ignoreHTTPSErrors`** | Lo mismo que ve un usuario; el service worker se registra sin dudas; el E2E detecta un certificado mal armado | Depende de que los navegadores de Playwright usen los almacenes donde instala mkcert (S-12) | **Elegida** |
| `ignoreHTTPSErrors: true` | Sin depender de S-12 | Con errores de certificado el service worker podría no registrarse; esconde problemas reales | **Respaldo** solo para el navegador que falle S-12 y solo si T-F703 pasa |
| E2E por HTTP plano | Sin certificados | Sin sesión en Chromium (H-10); el backend ya no lo permite | Descartada |

## R-F12 Idioma y formato de fechas y dinero → ADR-023

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **es-AR sin librería de i18n; mensajes de error centralizados por `code`** | Textos directos; nada que traducir | Cambiar un texto es buscarlo en el componente | **Elegida** |
| react-intl / i18next | Estándar multilenguaje | Innecesario con un idioma | Descartada |

Dinero (se implementa en la primera spec con importes):

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Centavos → string decimal por manipulación de texto → `Intl.NumberFormat.format(string)`** | Exacto por construcción; nativo | Requiere navegadores con `Intl.NumberFormat` v3 (cubiertos por los mínimos) | **Elegida** |
| `cents / 100` + `Intl` | Trivial | Aritmética de punto flotante (prohibida) | Descartada |
| dinero.js / big.js | Exactas | Dependencia para algo que resuelve `Intl` | Descartada |
| `bigint` | Sin límite | `JSON.parse` entrega `number`; habría que interceptar el parseo | Descartada |

Fechas: zona horaria **de la empresa** (constitución) frente a la del navegador (más simple, pero
un usuario de viaje vería horas distintas). Se elige la de la empresa; la del navegador solo en
pantallas sin sesión.

## R-F13 Decisiones locales de pantalla → `DD-F` de `ui.md`

| Decisión | Opción elegida | Alternativa descartada y por qué |
|---|---|---|
| Idioma de las rutas (DD-F1) | Inglés (`/reset-password`) (**usuario, P-F3**) | Español: URLs más amigables, pero sin coherencia con endpoints en inglés y con caracteres especiales |
| Token del enlace (DD-F2) | Del fragmento a `history.state` al montar | Dejarlo en el fragmento (visible, copiable al compartir la URL); `sessionStorage` (sobrevive a la pestaña más de lo necesario) |
| Confirmar email (DD-F3) | Botón explícito | Automático al abrir: el doble montaje de React en desarrollo o un escáner de enlaces pueden consumir el token |
| Invitar (DD-F6) | Página propia | Modal: peor con el botón atrás en el celular y más manejo de foco |
| Cambio de rol (DD-F7) | Acción con confirmación | Selector en la fila: cambios por un toque accidental |
| Email entre pantallas (DD-F9) | `location.state` | Query string: el email quedaría en logs de acceso del servidor |
| Zona horaria en el registro (DD-F10, revisada) | La del navegador, sin validar ni reintentar (el backend usa la default si no la conoce, DD-27) | Reintento sin `timezone` ante `422` (versión anterior): innecesario desde DD-27; no enviarla: la empresa arrancaría en Buenos Aires aunque esté en otra zona |
| Logo grande (DD-F11, revisada) | Achicarlo en el navegador (**usuario, P-F2**; detalle en R-F14) | Rechazarlo con una indicación (default anterior): el usuario pidió que las fotos del celular se puedan subir |
| JPEG dentro de los límites (DD-F21) | Volver a codificarlo siempre (**usuario, P-F6**) | Subirlo tal cual: conserva la ubicación GPS de la foto y puede quedar de costado en el PDF |
| Tope de bytes del archivo preparado (DD-F22, nueva) | 2 097 152 bytes, el límite exacto del servidor (detalle en R-F14) | Dejar un margen (1 900 000): solo tenía sentido mientras no se sabía qué medía el servidor (H-11) |
| Desarrollo con Vite (DD-F23, nueva) | HTTPS con el certificado de mkcert y *proxy* verificado (detalle en R-F08) | *Proxy* HTTP o `secure: false` (R-F08) |
| Tema (DD-F13) | Solo claro (**usuario, P-F4**) | Claro + oscuro: el doble de verificación de contraste sin pedido de la spec |
| Navegación (DD-F15) | Barra inferior (celular) + lateral (escritorio) | Menú hamburguesa: oculta la navegación y queda lejos del pulgar |
| Repetir contraseña (DD-F16) | No, con mostrar/ocultar | Campo de confirmación: un campo más sin beneficio con el control de visibilidad |
| Bloqueo (DD-F17) | Estado de `/login` | Ruta propia: pierde el email y no hay nada que enlazar |

## R-F14 Preparación del logo en el navegador **(usuario: achicar, P-F2)** → DD-F11, DD-F20, DD-F21, DD-F22

Dónde se achica:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **En el navegador, con `createImageBitmap` + `<canvas>` + `toBlob` (APIs nativas)** | Sin dependencias; sube menos bytes por datos móviles; el backend no cambia (el usuario pidió mantener sus límites) | Código propio que depende de cada navegador (orientación, codificadores); no se prueba en jsdom | **Elegida** |
| Librería de compresión en el navegador (browser-image-compression, compressorjs, pica) | Resuelve escalado, EXIF y calidad | Una dependencia más para ~100 líneas propias; su API y su comportamiento por navegador igual hay que probarlos en E2E | Descartada |
| Achicar en el servidor (aceptar archivos grandes y reescalar en Go) | Un solo lugar, sin diferencias de navegador | Cambia los límites del backend (el usuario pidió no cambiarlos); sube 5 MB por datos móviles; decodificar imágenes grandes en el servidor es superficie de DoS | Descartada |
| Rechazar y pedir otra imagen (default anterior) | Cero código | El usuario pidió lo contrario | Descartada |

Cómo se detecta el formato y el tamaño:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Firma de los primeros bytes y dimensiones del encabezado (PNG `IHDR`, JPEG `SOF`), funciones puras** | No se decodifica una imagen enorme para descubrir que es enorme; se prueba con tablas de bytes en Vitest; no depende de `file.type` (que puede venir vacío o mal) | Parsers chicos propios | **Elegida** |
| `file.type` y extensión | Trivial | Se puede equivocar (fotos sin tipo, extensiones cambiadas) | Descartada (solo se usa `file.type` como pista para SVG) |
| Decodificar siempre y leer `bitmap.width` | Sin parsers | Una foto de 48 MP puede agotar la memoria de un celular antes de saber que había que rechazarla | Descartada como primer paso |

Tope de bytes del archivo preparado (**segunda revisión**: el usuario fijó el límite en 2 MiB del
archivo, H-11, DD-31):

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **2 097 152 bytes, el límite exacto del servidor, comparado con `≤`** | El servidor mide el contenido de la parte `file`, que es exactamente el `size` del archivo preparado; los encabezados multipart tienen su propio margen (cuerpo ≤ 2 162 688); un PNG que el servidor acepta se sube sin tocar; la mejor calidad posible dentro del límite | Si el backend baja el límite y el cliente no se actualiza, aparece un `413` (se detecta; la matriz de mantenimiento lo cubre) | **Elegida** (DD-F22) |
| 1 900 000 bytes (margen de la revisión anterior) | Tolera diferencias de medición | Ya no hay diferencia que tolerar (H-11 resuelto); re-codifica PNG de 1,9–2 MiB que el servidor aceptaría y baja la calidad de los JPEG sin motivo | Descartada |
| 2 000 000 bytes | "2 MB" decimal | No coincide con el límite elegido por el usuario; mismo problema que el margen | Descartada |

Formato de salida:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **PNG → PNG, JPEG → JPEG** | PNG conserva la transparencia; JPEG no se vuelve más pesado | — | **Elegida** |
| Todo a JPEG | Archivos chicos | Pierde la transparencia de un logo PNG (queda un fondo negro o blanco) | Descartada |
| Todo a PNG | Sin pérdida | Una foto en PNG pesa mucho más; más pasos para entrar en el límite | Descartada |
| WebP | Mejor compresión | El backend acepta solo PNG/JPEG (DD-11) y los PDF de 005 también | Descartada |

Formatos de entrada:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Aceptar PNG y JPEG; rechazar SVG, GIF, WebP, HEIC y otros con un mensaje que dice qué hacer** | Lo mismo que acepta el servidor; sin sorpresas (un GIF animado no queda "congelado" en silencio; un SVG no se rasteriza a un tamaño arbitrario) | Un logo en WebP o SVG requiere exportarlo | **Elegida** |
| Convertir cualquier formato que el navegador pueda decodificar | Más permisivo | Resultados distintos por navegador (HEIC solo en Safari), pérdida silenciosa de animación o de calidad vectorial | Descartada por ahora |

Metadatos y orientación de los JPEG:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Volver a codificar todo JPEG (aplicando la orientación con `imageOrientation: 'from-image'`)** | Quita GPS y datos del dispositivo; el logo queda derecho en cualquier visor y en el PDF; el servidor no lo hace (guarda los bytes tal cual, INV-24 del plan) | Una re-codificación con calidad 0,90 aunque no hiciera falta achicar | **Elegida** (DD-F21, **confirmada por el usuario, P-F6**) |
| Solo volver a codificar cuando supera los límites | Sin re-codificación innecesaria | Un JPEG chico de celular sube con GPS y puede quedar de costado en el PDF | Descartada |
| Borrar los segmentos EXIF por bytes, sin re-codificar | Sin pérdida | Se pierde la orientación sin aplicarla: la foto queda de costado | Descartada |

Dónde se procesa:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Hilo principal con `<canvas>`; `toBlob` asíncrono; estado "Preparando la imagen…"** | Lo más simple; funciona en todos los navegadores del piso | La decodificación puede trabar la UI unos cientos de ms en celulares lentos | **Elegida** |
| Web Worker con `OffscreenCanvas` | UI siempre fluida | Más piezas (worker, mensajes) para una acción que se hace una vez | Descartada por ahora (se reevalúa si NFR-F13 no se cumple) |

## R-F15 Cookie de sesión en `http://localhost` (H-10) → **resuelta por el usuario: HTTPS local con mkcert** (DD-24 del plan)

Vista del frontend; la decisión la tomó el usuario y la incorporó el backend (`platform/config`,
nota (b) en ADR-006, DD-24, R-23).

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **TLS local para desarrollo y E2E** (certificado de `localhost` de una CA local de `mkcert`, `crm serve` con TLS solo en modo local, Vite con `server.https` y el mismo certificado, la CA instalada en el runner de CI) | La misma cookie `__Host-` que producción en todos los navegadores; los E2E prueban lo real | Un paso de instalación por equipo y dos variables de configuración | **Elegida por el usuario** |
| Cookie sin prefijo solo con `APP_BASE_URL` `http://localhost` | Cero instalación | Dos comportamientos de cookie; los E2E dejan de probar la real | Descartada |
| Desarrollar y correr los E2E solo con Firefox | Cero cambios | Contradice P-F5 (Chromium en cada PR); el navegador más usado queda sin E2E | Descartada |
| Esperar a que Chrome acepte `__Host-` en `localhost` | Cero cambios | Sin fecha conocida | Descartada |

## R-F16 Prueba en un celular real con HTTPS (H-12) → hallazgo hacia el backend

El checkpoint F6 (foto de la galería, S-F8, NFR-F13), NFR-F11 (instalar la PWA) y T-F705
(TalkBack/VoiceOver) necesitan un origen HTTPS en el que confíe el teléfono y al que el teléfono
llegue. `localhost` no sirve desde el celular y DD-24 solo permite TLS propio con
`localhost`/`127.0.0.1`.

| Opción | A favor | En contra | Veredicto del frontend |
|---|---|---|---|
| **Entorno de *staging* con dominio y HTTPS del hosting** | Es el modo normal de producción (sin TLS propio); sirve para Android e iPhone; sin tocar DD-24 | Depende de elegir hosting (P-1) | **Recomendada** |
| Android: reenvío de puertos de Chrome (`chrome://inspect` → *Port forwarding*) a `localhost:8443` + la CA de mkcert instalada en el teléfono | Sin tocar DD-24: el teléfono abre `https://localhost:8443`, que es modo local; documentado por Chrome y mkcert | Solo Android y con cable USB; que Chrome en Android confíe en una CA instalada por el usuario para navegar no está verificado (S-F10) | **Respaldo** hasta tener *staging* |
| Ampliar el modo local de DD-24 a IPs de la red local | Android e iPhone por Wi-Fi | Más reglas en `platform/config`; certificados por IP; el binario de desarrollo queda expuesto a la red; decisión del backend | No recomendada |
| Túnel público (servicios que publican el puerto local en un dominio con HTTPS) | Rápido, cualquier teléfono | Expone el entorno de desarrollo a internet; dependencia externa; `APP_BASE_URL` con un dominio del túnel deja de ser modo local | Descartada |

## Fuentes consultadas (frontend)

- Tailwind CSS v4, navegadores soportados: <https://tailwindcss.com/docs/compatibility>
- Vite 8 y `build.target` por defecto (`baseline-widely-available`): <https://vite.dev/blog/announcing-vite8>, <https://vite.dev/config/build-options>
- Vite, `vite:preloadError`: <https://vite.dev/guide/build#load-error-handling>
- React Router, modos: <https://reactrouter.com/start/modes>
- shadcn/ui con React Hook Form y `Field`: <https://ui.shadcn.com/docs/forms/react-hook-form>
- `@hookform/resolvers` 5.1 con Zod 4: <https://github.com/react-hook-form/resolvers/releases>
- openapi-typescript, varias APIs con `redocly.yaml`: <https://openapi-ts.dev/cli>
- Redocly `bundle` / `join`: <https://redocly.com/docs/cli/commands/bundle>, <https://redocly.com/docs/cli/commands/join>
- Chrome, criterios de instalación: <https://developer.chrome.com/blog/update-install-criteria>
- `Intl.NumberFormat.prototype.format` con strings decimales exactos: <https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/NumberFormat/format>
- Cookies `Secure` y con prefijo en `http://localhost` por navegador: <https://github.com/httpwg/http-extensions/issues/2605>, <https://issues.chromium.org/issues/40202941>, <https://bugzilla.mozilla.org/show_bug.cgi?id=1618113>
- mkcert (almacenes soportados, Firefox solo en macOS y Linux, `certutil` en Linux, `NODE_EXTRA_CA_CERTS` porque Node no usa el almacén del sistema, CA en iOS y Android, advertencia sobre `rootCA-key.pem`): <https://github.com/FiloSottile/mkcert>
- Chrome DevTools, reenvío de puertos a un Android para abrir un servidor local: <https://developer.chrome.com/docs/devtools/remote-debugging/local-server>
- `createImageBitmap` y `imageOrientation: 'from-image'`: <https://developer.mozilla.org/en-US/docs/Web/API/Window/createImageBitmap>, <https://caniuse.com/mdn-api_createimagebitmap_options_imageorientation_parameter_from-image>, <https://html.spec.whatwg.org/multipage/imagebitmap-and-animations.html>

Supuestos a validar durante la implementación (detalle en `ui.md` §25): S-F1 (`$ref` externos en
openapi-typescript, T-F004), S-F2 (openapi-fetch + MSW en jsdom, T-F003), S-F7 (orientación EXIF
en `createImageBitmap`, T-F706), S-F8 (iOS convierte HEIC a JPEG al elegir, checkpoint F6, depende
de H-12), S-F9 (codificación PNG/JPEG del canvas, T-F706), S-F10 (reenvío de puertos en Android con
la CA de mkcert, respaldo de H-12) y S-12 del plan (Chromium y WebKit de Playwright confían en la CA
de mkcert, primera corrida de T-F701). S-F3 quedó refutado y ya no aplica: lo resolvió H-10 (HTTPS
local).
