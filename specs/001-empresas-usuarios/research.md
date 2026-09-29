# Research: Empresas, usuarios y roles (backend)

**Spec**: [`spec.md`](spec.md) · **Plan**: [`plan.md`](plan.md)
**Revisión 2026-09-27**: R-06 (duración de sesión, P-5) y R-15 (enumeración en el registro, P-4)
actualizadas con las respuestas del usuario.
**Revisión 2026-09-29**: R-19 a R-23 agregadas por los hallazgos H-1 a H-9 del frontend (plan §18).

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
| **A. Rol por empresa + RLS forzada** (elegida por el usuario) | Cada empresa tiene `crm_t_<hex>`; cada transacción hace `SET LOCAL ROLE`; las políticas comparan `tenant_id` con la empresa derivada de `current_user` | Falla cerrada (el rol de login no tiene privilegios); la identidad de la empresa es la identidad de base (`pg_stat_activity`, logs por usuario); se pueden fijar límites por empresa (`ALTER ROLE ... SET`) | Requiere `CREATEROLE` en el flujo de registro; roles globales al clúster (backups, restores, entornos); `crm_app` miembro de miles de roles (rendimiento a medir); re-análisis de planes al cambiar de rol; bootstrap fuera de las migraciones |
| B. Rol único `crm_tenant` + `SET LOCAL app.tenant_id` | Cada transacción hace `SET LOCAL ROLE crm_tenant` y fija una variable; las políticas usan `current_setting('app.tenant_id')` | Misma falla cerrada si `crm_app` tampoco tiene privilegios; sin `CREATEROLE`; cero operación extra; patrón muy documentado | La empresa es una variable, no una identidad de base; menos trazabilidad en `pg_stat_activity` |
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

Recomendación del arquitecto: B es la opción más simple con la misma protección práctica. Se
diseña sobre A por decisión del usuario, con la reversibilidad garantizada y el riesgo del
hosting planteado en la pregunta P-1 (sigue abierta).

### R-04b Cómo resolver la empresa antes de conocerla (login, tokens, worker)

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Roles de sistema con privilegio por columna + política `USING (true)` solo para esas columnas** | Simple (GRANT de columnas, sin funciones); la fase 2 corre bajo RLS | Un bug en una query de fase 1 puede listar `(id, tenant_id, email)` de todas las empresas | **Elegida**, con el riesgo acotado por el test de privilegios T-B109 y un archivo de queries propio (`auth_lookup.sql`) |
| Funciones `SECURITY DEFINER` por lookup (`lookup_user_by_email(text)`) | Imposible enumerar aun con bug: solo devuelve una fila por email exacto | Un rol dueño por función con su propia política; más objetos que mantener y entender | Descartada por complejidad; es la mejora natural si el riesgo residual se considera inaceptable |
| Tabla global de directorio (`email → tenant_id`) sin RLS | Muy simple | Otra tabla que mantener sincronizada; mismo riesgo de enumeración | Descartada |
| Selector de empresa en el login | Sin lookup global | Contradice "un usuario, una empresa" y agrega un paso al usuario (principio I) | Descartada |

### R-04c Cómo se crea el rol de una empresa

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Función `SECURITY DEFINER` dueña `crm_provisioner` (con `CREATEROLE`), ejecutable solo por `crm_signup`** | `crm_app` no puede crear roles arbitrarios; entrada tipada `uuid`; idempotente; transaccional con el registro | Un objeto más; la función la crea una migración con `SET ROLE crm_provisioner` | **Elegida** |
| `crm_app` con `CREATEROLE` | Simple | Cualquier código (o inyección) bajo `crm_app` crea roles y se otorga membresías | Descartada |
| Crear roles fuera de línea (job/DBA) | Sin `CREATEROLE` en runtime | El registro deja de ser autónomo e inmediato (FR-001, SC-001) | Descartada |

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

## R-12 Almacenamiento de archivos **(usuario: S3 compatible, subida vía backend)** → ADR-011

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`github.com/minio/minio-go/v7`** | API chica y directa; funciona con cualquier S3 compatible (MinIO, AWS, R2, etc.) | Dependencia | **Elegida** |
| `aws-sdk-go-v2` (s3) | Oficial de AWS | Mucho más grande; configuración de *path-style* para MinIO | Alternativa válida |

| Opción de subida | A favor | En contra | Veredicto |
|---|---|---|---|
| **Vía backend (multipart), con validación por contenido** | Validación de tipo, tamaño y dimensiones antes de guardar; bucket privado; sin CORS | El backend transfiere los bytes | **Elegida** (logos de ≤ 2 MB) |
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

## R-17 Rate limiting → DD-9

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **En memoria (`golang.org/x/time/rate`), por IP y por email** | Sin infraestructura; paquete del proyecto Go | Por instancia | **Elegida** (S-1) |
| En PostgreSQL | Compartido entre instancias | Escritura por request anónimo | Descartada hasta tener varias instancias |
| En el proxy (nginx, Cloudflare) | Fuera de la app | Depende del hosting aún no elegido | Complemento posible (también contra enumeración distribuida por el registro) |

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

Dónde van los middlewares: los que valen para cualquier respuesta (request id, recover, logging,
cabeceras de seguridad, `CrossOriginProtection`) envuelven al mux raíz; los que son de la API
(`no-store`, rate limit, sesión, permisos) quedan dentro de chi; los de la SPA (CSP, caché por
tipo de archivo, gzip) dentro de `web`.

## R-20 Caché del logo por empresa (H-2) → DD-23

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`private, no-cache` + `ETag` = UUID del objeto; `304` sin leer S3; parámetro `v` declarado e ignorado** | Nunca se muestra un logo sin preguntarle al servidor con la sesión actual (el `ETag` de otra empresa no coincide); revalidar cuesta un `304`; la UI puede cambiar la URL para refrescar el `<img>` al instante | Una revalidación por uso del logo | **Elegida** |
| `private, max-age=300` (diseño anterior) | Menos requests | En un celular compartido se ve hasta 5 min el logo de la empresa anterior, y el viejo tras reemplazarlo | Descartada (hallazgo H-2) |
| `no-store` | Lo más simple | Descarga el logo completo en cada uso (hasta 2 MB por pantalla con encabezado) | Descartada |
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

## R-23 `APP_BASE_URL` y cookie `Secure` en desarrollo (H-3) → DD-24

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`http://` solo para `localhost` y `127.0.0.1`, con `COOKIE_SECURE=true`; `false` solo con esos orígenes y para clientes no navegador** | Desarrollo con navegador real (Vite o el binario) con la misma cookie que producción; imposible configurar `http://` o cookie sin `Secure` para un dominio real | Safari de escritorio puede rechazar la cookie `Secure` en `localhost` (se desarrolla con Chrome o Firefox) | **Elegida** |
| Regla anterior (`http://` solo con `COOKIE_SECURE=false`) | Estricta | Un navegador rechaza la cookie `__Host-` sin `Secure`: el desarrollo local no funcionaba | Descartada (hallazgo H-3) |
| TLS local obligatorio (certificado de desarrollo) | Idéntico a producción | Paso extra de instalación para un equipo que aprende | Respaldo si falla S-F3 |
| Cambiar el nombre de la cookie en desarrollo (sin `__Host-`) | Funciona sin `Secure` | Dos comportamientos de cookie; los tests dejan de probar el real | Descartada |

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
7. Con el *proxy* de Vite, el navegador envía `Sec-Fetch-Site: same-origin` en los `POST` a
   `/api` y `CrossOriginProtection` los acepta sin `AddTrustedOrigin` (spike S-F3 de T-F006).

---

## Frontend

Autor: `frontend-architect` (2026-09-29). Diseño en [`ui.md`](ui.md); decisiones estructurales en
ADR-015 a ADR-023. Las marcadas **(usuario)** las tomó el usuario; el resto son defaults del
arquitecto pendientes de aprobación.

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
| **Mux raíz: `/api/` → chi; `/healthz`, `/readyz`; resto → SPA** | Los tests de rutas del backend (T-B004, T-B801, rutas vs contrato) no cambian; un `/api/…` inexistente sigue en `404 problem+json` | Un nivel más de ruteo en `internal/app` | **Elegida** (hallazgo H-1 de `ui.md`) |
| Ruta `/*` dentro de chi | Directo | Rompe `chi.Walk` y los tests de cobertura | Descartada |

*Fallback* y cabeceras:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`index.html` para rutas sin extensión; `404` para archivos inexistentes; `no-cache` en HTML, `immutable` en assets con hash** | No sirve HTML como JS; cada carga ve la versión nueva; assets descargados una vez | — | **Elegida** |
| `index.html` para todo lo que no exista | Simple | Chunks viejos reciben HTML (error de MIME) y pueden cachearse | Descartada |
| HTML con `max-age` | Menos requests | Pestañas nuevas con `index.html` viejo apuntando a chunks inexistentes | Descartada |

CSP:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`script-src 'self'` estricto + `style-src 'self' 'unsafe-inline'`** | Frena XSS por scripts; compatible con Radix/sonner | Estilos inline permitidos (riesgo bajo) | **Elegida** |
| CSP estricta con *nonces* | Máxima | Requiere HTML generado por request | Descartada |
| Sin CSP | Nada que mantener | Sin defensa en profundidad ante XSS | Descartada |

Desarrollo local:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Proxy de Vite (`/api` → `:8080`)** | Incluido en Vite; mismo origen para el navegador | Depende de que el navegador acepte la cookie `Secure` en `http://localhost` (S-F3) | **Elegida por el usuario** |
| Go reenvía a Vite | Un solo puerto | Código Go solo para desarrollo | Descartada |

Compresión:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`gzhttp` en Go envolviendo el handler de la SPA** | Funciona en cualquier hosting | Dependencia Go nueva (aprobación del backend, H-8) | **Propuesta** |
| Archivos precomprimidos (`.br`/`.gz`) en el build | Sin costo de CPU por request | Más lógica en el handler | Descartada por ahora |
| Delegar en el proxy del hosting | Cero código | Hosting sin definir (P-1) | Si el hosting lo trae, reemplaza a la propuesta |

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
| **Vitest + Testing Library + MSW (jsdom)** | Misma configuración que Vite; consultas por rol; red interceptada en el borde con el cliente real | Mantener handlers alineados al contrato (tipados) | **Elegida** |
| Jest | Estándar histórico | Configuración de TS/ESM extra | Descartada |
| happy-dom | Más rápido | Menos fiel | Respaldo (S-F2) |
| Vitest modo navegador | Más fiel | Más lento y más piezas | Reevaluable |
| Mockear hooks de datos | Rápido | No prueba la integración real | Descartada |

End-to-end:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Playwright contra el binario + `docker compose` + axe** | Prueba SW, CSP, cookies y cabeceras reales; Chromium y WebKit | Necesita Docker; minutos de CI | **Elegida** (frecuencia en P-F5) |
| Cypress | Buena experiencia | Sin WebKit | Descartada |
| Solo pruebas manuales | Nada que mantener | Sin regresión automática en flujos críticos | Descartada |

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
| Idioma de las rutas (DD-F1) | Inglés (`/reset-password`) | Español: URLs más amigables, pero sin coherencia con endpoints en inglés y con caracteres especiales (P-F3) |
| Token del enlace (DD-F2) | Del fragmento a `history.state` al montar | Dejarlo en el fragmento (visible, copiable al compartir la URL); `sessionStorage` (sobrevive a la pestaña más de lo necesario) |
| Confirmar email (DD-F3) | Botón explícito | Automático al abrir: el doble montaje de React en desarrollo o un escáner de enlaces pueden consumir el token |
| Invitar (DD-F6) | Página propia | Modal: peor con el botón atrás en el celular y más manejo de foco |
| Cambio de rol (DD-F7) | Acción con confirmación | Selector en la fila: cambios por un toque accidental |
| Email entre pantallas (DD-F9) | `location.state` | Query string: el email quedaría en logs de acceso del servidor |
| Zona horaria en el registro (DD-F10) | La del navegador con reintento sin ella ante `422` | No enviarla (la empresa arranca en Buenos Aires aunque esté en otra zona) |
| Logo grande (DD-F11) | Rechazar con indicación | Redimensionar en el navegador (P-F2): más código y difícil de probar en jsdom |
| Tema (DD-F13) | Solo claro | Claro + oscuro: el doble de verificación de contraste sin pedido de la spec |
| Navegación (DD-F15) | Barra inferior (celular) + lateral (escritorio) | Menú hamburguesa: oculta la navegación y queda lejos del pulgar |
| Repetir contraseña (DD-F16) | No, con mostrar/ocultar | Campo de confirmación: un campo más sin beneficio con el control de visibilidad |
| Bloqueo (DD-F17) | Estado de `/login` | Ruta propia: pierde el email y no hay nada que enlazar |

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

Supuestos a validar durante la implementación (detalle en `ui.md` §25): S-F1 (`$ref` externos en
openapi-typescript, T-F004), S-F2 (openapi-fetch + MSW en jsdom, T-F003), S-F3 (cookie `Secure` en
`http://localhost`, T-F006).
