# ADR-024: Clasificación de fallos de entrega del outbox, presupuesto de tiempo del envío SMTP y aislamiento de fallos por mensaje

**Status**: Accepted (aprobado por el usuario 2026-10-01). [ADR-025](025-saneamiento-last-error-y-codigo-extendido.md) (*Accepted*) reemplaza el último párrafo de §2 y partes de §3 (ver nota 2026-10-01 (b)).
**Fecha**: 2026-10-01
**Origen**: revisión del PR fdelillo/crm#8 (Fase 2 de la spec 001): hallazgos I1, I2, M1 y M7
**Relación**: reemplaza **solo** la viñeta "Reintentos" de [ADR-010](010-email-outbox-transaccional.md)
en lo que dice sobre qué error es definitivo. El resto de ADR-010 (outbox transaccional, worker en
el binario, entrega al menos una vez, borrado del payload, puerto `Mailer` con go-mail) sigue
vigente y no cambia.

> **Nota 2026-10-01 (b) (reemplazo parcial, *Accepted*)**, por la tercera revisión del
> PR fdelillo/crm#8: [ADR-025](025-saneamiento-last-error-y-codigo-extendido.md) reemplaza
> (1) el último párrafo de §2 (el código extendido se lee también del comienzo del texto, en todas
> las fases, con control de clase), (2) en §3, la viñeta **Detalle** para las respuestas de la fase
> `data` (detalle fijo `response text omitted`) y (3) el punto (1) del **Saneamiento obligatorio**
> (se redactan también los campos con `://` o con 20 o más caracteres seguidos de
> `[A-Za-z0-9+/=_-]`). En esas tres piezas rige ADR-025. El resto (§1,
> §4, §5, §6 y el formato de §3) no cambia.

## Contexto

ADR-010 y el plan de 001 (§9.4) dicen "respuestas SMTP `5xx` → definitivo", con ejemplos que son
todos del destinatario. La implementación de la Fase 2 aplicó la regla a cualquier `5xx`, incluidos
los de la conexión: un saludo `554 5.7.1 client host blocked` (la IP del servidor está en una lista
de bloqueo) o un `535` de `AUTH` (credenciales SMTP vencidas) pasan **toda la cola** a `failed` y
borran los payloads con los tokens. Es irrecuperable y la causa es nuestra configuración, no los
mensajes. La revisión lo reprodujo con un SMTP falso.

Tres problemas vecinos salieron en la misma revisión:

- **`last_error` no sirve para diagnosticar** (M1): siempre `delivery_failure` o
  `invalid_payload`; el modelo de datos pedía "código SMTP y mensaje del proveedor truncado" sin
  fijar formato. Además, el `Error()` de go-mail (`*mail.SendError`) incluye
  `affected recipient(s): <email>`, así que loguear o guardar el error tal cual filtra el
  destinatario.
- **El envío corre con la transacción del worker abierta** (M7) y `crm_app` tiene
  `idle_in_transaction_session_timeout = 30s` (bootstrap). Verificado en el código de go-mail
  v0.8.1 (`client.go`): el `context` solo acota el *dial* TCP/TLS; el resto de la conversación lo
  acotan *deadlines* de conexión que se renuevan en cada etapa (`WithTimeout`). Con el valor actual
  (10 s) y el `RSET` por defecto hay hasta **5 ventanas** (*dial*, saludo, EHLO+STARTTLS+AUTH,
  MAIL+RCPT+DATA, RSET+QUIT): hasta 50 s. Si PostgreSQL corta la sesión por inactividad, el
  `MarkSent` falla, el mensaje queda `pending` **sin sumar `attempts`** y se reenvía en el ciclo
  siguiente: un SMTP lento genera reenvíos sin fin que nunca consumen intentos.
- **Un mensaje bloquea a todos** (I1): el worker toma siempre el pendiente más viejo; si
  `AsTenant` o `GetMessage` fallan para ese mensaje (p. ej. una empresa restaurada sin su rol,
  plan §12.4), el ciclo corta y el siguiente toma el mismo mensaje. Nadie más recibe emails.

Fuerzas en tensión: no perder mensajes por fallas que no son del mensaje; no reintentar para
siempre algo que nunca va a funcionar (el payload con el token vive en la base mientras está
pendiente); no filtrar datos personales en logs ni en la base; mantener el modelo de ADR-010 (un
mensaje por transacción, `FOR UPDATE SKIP LOCKED`) sin agregar infraestructura.

## Decisión

### 1. Un solo concepto: la **causa** del fallo

Todo fallo de entrega que no sea una cancelación se describe con un `*outbox.DeliveryError` que
lleva una **causa** (`outbox.Cause`) y la **fase** donde ocurrió (`outbox.Phase`). La causa
determina a la vez si el fallo es definitivo y con qué nivel se loguea. Reemplaza a
`*outbox.PermanentError`.

| Causa | Clase | Log por intento | Cuándo |
|---|---|---|---|
| `network` | Recuperable | `WARN` | Timeout de una etapa o del presupuesto de envío; conexión rechazada, cortada o cerrada (EOF); fallo de DNS; el `NOOP` de control de go-mail falla; cualquier `SendError` de go-mail sin código SMTP (`ErrorCode() == 0`) salvo los casos de §2 |
| `transient` | Recuperable | `WARN` | Cualquier respuesta SMTP `4xx`, en **cualquier** fase (p. ej. `421` en el saludo, `454` en AUTH, `451`/`452` en MAIL, RCPT o DATA) |
| `config` | Recuperable | **`ERROR`** | Problema nuestro o de la cuenta del proveedor, que afecta a todos los mensajes: **cualquier `5xx` en la fase de conexión** (saludo `554`, EHLO, STARTTLS, AUTH `530`/`534`/`535`/`538`); STARTTLS no ofrecido con TLS obligatorio; ningún mecanismo de AUTH compatible; error de certificado o de *handshake* TLS; **cualquier `5xx` en MAIL FROM** (el remitente es configuración); un `5xx` en RCPT TO o DATA que **no** identifica al destinatario (ver regla de abajo); una respuesta con un código que no es `4xx` ni `5xx`; cualquier error que el adaptador o el `Dispatcher` no sepan clasificar |
| `recipient` | **Definitivo** | `WARN` | El destinatario es inválido o no existe (regla de abajo), o su dirección no pasa la validación local antes de conectar |
| `bug` | **Definitivo** | **`ERROR`** | Plantilla inexistente, payload ilegible o incompleto, asunto con salto de línea: un error de programación que reintentar no arregla |

**Regla del destinatario** (la única forma de que una respuesta SMTP sea definitiva): la fase es
RCPT TO o DATA, el código es `5xx`, y además

- el código extendido (RFC 3463) es `5.1.x` (dirección) o `5.2.x` (buzón), **o**
- no hay código extendido, la fase es RCPT TO y el código básico es `550`, `551` o `553`.

Todo otro `5xx` es `config`. Ante la duda se elige recuperable: un falso recuperable cuesta 8
intentos en ~19 h y termina igual en `failed`; un falso definitivo pierde el mensaje (y, si la
causa es común a todos, la cola entera).

Al 8.º intento recuperable fallido el mensaje pasa a `failed` (sin cambios respecto de ADR-010),
con log `ERROR`. Esto vale también para la causa `config` (ver §4).

### 2. Fases

| Fase (`outbox.Phase`) | Qué cubre | Cómo la determina el adaptador SMTP |
|---|---|---|
| `compose` | Antes de conectar: plantilla, payload, validación de direcciones y asunto | La asigna el handler o el adaptador antes de llamar a go-mail |
| `connection` | TCP, TLS (puerto 465), saludo `220`, EHLO/HELO, STARTTLS, AUTH | Todo error de `DialAndSendWithContext` que **no** es `*mail.SendError` (go-mail envuelve todos los errores de esa etapa como `dial failed: %w`) |
| `mail_from` | Comando MAIL FROM | `SendError.Reason == ErrSMTPMailFrom` |
| `rcpt_to` | Comando RCPT TO | `SendError.Reason == ErrSMTPRcptTo` |
| `data` | Comando DATA, envío del contenido y respuesta de fin de datos | `SendError.Reason` ∈ {`ErrSMTPData`, `ErrWriteContent`, `ErrSMTPDataClose`} |
| `unknown` | Errores del handler que no son `DeliveryError` y `SendError` con otro `Reason` | — |

Casos particulares de `SendError`: `ErrConnCheck` → fase `connection`, causa `network`;
`ErrGetRcpts` → `compose`, `recipient`; `ErrGetSender` → `compose`, `config`; `ErrNoUnencoded` y
`ErrAmbiguous` → `unknown`, `config`.

Errores de la fase `connection` que no son respuestas SMTP: timeout (`net.Error` con `Timeout()`,
`context.DeadlineExceeded`, `os.ErrDeadlineExceeded`) → `network`; `*net.DNSError` → `network`;
conexión rechazada, cortada o cerrada (`ECONNREFUSED`, `ECONNRESET`, `io.EOF`,
`io.ErrUnexpectedEOF`, `net.ErrClosed`) → `network`; otro `*net.OpError` → `network`; errores de
certificado o de TLS (`*tls.CertificateVerificationError`, errores de `crypto/x509`,
`tls.AlertError`, `tls.RecordHeaderError`) → `config`; cualquier otro (p. ej. "STARTTLS no
soportado", "server does not support SMTP AUTH", ningún mecanismo de AUTH compatible) → `config`.

Código SMTP y código extendido: en la fase `connection` salen del `*textproto.Error` de la cadena
(`errors.As`; el código extendido, del comienzo del mensaje si tiene la forma `d.d.d`); en las
demás, de `SendError.ErrorCode()` y `SendError.EnhancedStatusCode()` (go-mail solo lo informa si el
servidor anuncia `ENHANCEDSTATUSCODES`; si no, la regla del destinatario usa el código básico).

### 3. `last_error` y logs: formato único, saneado, sin destinatario

```
<causa> <fase>[ <código SMTP>[ <código extendido>]]: <detalle>
```

Ejemplos (los valores son ilustrativos de la forma, no datos reales):

```
config connection 535 5.7.8: Authentication credentials invalid
config connection 554 5.7.1: client host blocked
transient rcpt_to 452 4.2.2: Mailbox full
recipient rcpt_to 550 5.1.1: [redacted] User unknown
network connection: timeout
network data: timeout
config connection: server does not support SMTP AUTH
bug compose: unknown email template
bug compose: invalid payload
```

- **Detalle**: para una respuesta SMTP, el texto del proveedor sin el código básico ni el extendido
  (que ya están en el prefijo). Para `SendError`, como no expone la respuesta original (no tiene
  `Unwrap`), se toma su `Error()` y se le quitan el prefijo de la razón (hasta el primer `": "`), el
  código y el código extendido del comienzo, y los sufijos `, affected recipient(s): …` y
  `, affected message ID: …`. Para errores de red, un vocabulario fijo: `timeout`,
  `connection refused`, `connection closed`, `dns lookup failed`, `network error`; para TLS,
  `tls handshake failed`. Para los demás errores de conexión no clasificados, el texto del error
  (go-mail usa textos fijos sin datos del mensaje). Para `bug`, textos fijos del handler (nunca
  valores del payload).
- **Saneamiento obligatorio**, en un único lugar (`DeliveryError.LastError()`), aplicado también a
  lo que va al log: (1) todo token separado por espacios que contenga `@` se reemplaza por
  `[redacted]` (cubre al destinatario aunque el proveedor lo repita, al remitente y a los
  *message-id*); (2) caracteres de control (incluidos `\r`, `\n`, `\t`) → espacio, espacios
  repetidos → uno, sin espacios en los extremos; (3) se trunca a **1000 caracteres** (runas, no
  bytes: es el `CHECK (char_length(last_error) <= 1000)` de la migración 00006), terminando en `…`
  si se cortó.
- **Nunca** se loguea ni se guarda el `error` original de una entrega (`DeliveryError.Err`): puede
  contener el destinatario. Los logs del worker llevan `error_cause`, `smtp_phase`, `smtp_code` y
  `last_error` (el texto saneado).
- `last_error` se escribe al reprogramar (`retry`) y al pasar a `failed`; `MarkSent` lo deja en
  `NULL` (como hoy). No cambia al aplazar por un fallo del `Dispatcher` (§6; `crm_worker` no tiene
  ese privilegio y no hubo intento de entrega) ni al cancelar.

### 4. Error de configuración persistente (p. ej. `535` por credenciales vencidas)

- Es **recuperable** y **consume intentos** como cualquier otro: con el backoff vigente (1, 5, 15,
  60 min y 6 h), el 8.º intento ocurre unas **19 h 21 min** después del primero y el mensaje pasa a
  `failed`.
- Cada intento se loguea en **`ERROR`** (`error_cause=config`), y la métrica
  `outbox_delivery_errors_total{cause}` (Fase 9) permite alertar sobre `cause="config"`. La señal de
  "nadie recibe emails" (`outbox_oldest_pending_seconds > 600`) ya existe en el runbook.
- **Corte del ciclo**: si un fallo ocurre en la fase `connection` (cualquier causa), el
  `Dispatcher` registra ese mensaje y termina el ciclo sin intentar el resto del lote. Los demás
  fallarían igual, y cada `AUTH` con credenciales inválidas cuenta para el bloqueo de la cuenta en el
  proveedor. El ciclo siguiente (2 s después) sigue con el próximo mensaje vencido.
- No se agrega un *circuit breaker* en memoria ni un canal de alerta propio (ver Alternativas).

### 5. Presupuesto de tiempo del envío (acoplado a `idle_in_transaction_session_timeout`)

| Elemento | Valor | Dónde |
|---|---|---|
| `idle_in_transaction_session_timeout` de `crm_app` | 30 s | `db/bootstrap/001_roles_and_database.sql` |
| Presupuesto total de `Handle` (`outbox.SendBudget`) | **20 s** | `context.WithTimeout` que el `Dispatcher` pone alrededor de `Handle` |
| Timeout por etapa de go-mail (`WithTimeout`) | **5 s** | Adaptador SMTP |
| Ventanas de *deadline* de una conversación, con `WithoutRset()` | **4**: *dial*; saludo; EHLO + STARTTLS + AUTH + NOOP; MAIL + RCPT + DATA + fin de datos + QUIT | go-mail v0.8.1 (`DialToSMTPClientWithContext`, `checkConn`) |
| Peor caso de la conversación | 4 × 5 s = 20 s ≤ `SendBudget` | — |
| Margen para las sentencias de la transacción | 10 s | — |

Reglas:

- El adaptador usa `gomail.WithTimeout(5 * time.Second)` y `gomail.WithoutRset()` (el `RSET` después
  de entregar el único mensaje de la conexión no aporta nada y agrega una ventana).
- `SendBudget + 10 s ≤ idle_in_transaction_session_timeout`. Un test de integración lo verifica
  leyendo el valor real como `crm_app`; otro test unitario verifica `4 × timeout por etapa ≤
  SendBudget`. Cambiar cualquiera de los tres valores, o subir de versión go-mail, obliga a revisar
  esta tabla (matriz del plan §16).
- Un `Handle` que termina por el presupuesto (`context.DeadlineExceeded` con el contexto del ciclo
  todavía vivo) es causa `network`, detalle `timeout`.
- **Entrega ya hecha**: si `DialAndSendWithContext` devuelve error pero `Msg.IsDelivered()` es
  `true` (el servidor aceptó el fin de datos y falló el `QUIT`), el adaptador devuelve `nil`.
- **Apagado**: go-mail no corta la conversación cuando se cancela el contexto (solo el *dial*), así
  que `Handle` puede tardar hasta `SendBudget` en volver. La transacción del mensaje usa
  `context.WithoutCancel` del contexto del ciclo (sus sentencias ya están acotadas por
  `statement_timeout` e `idle_in_transaction_session_timeout`) y solo `Handle` recibe el contexto
  cancelable. Así: si `Handle` vuelve con `context.Canceled`, el mensaje queda exactamente como
  estaba (como hasta ahora); si vuelve con éxito durante el apagado, se marca `sent` y no se
  reenvía. El timeout de apagado del proceso debe ser **≥ 25 s** (`SendBudget` + 5 s).

### 6. Aislamiento de fallos por mensaje en el `Dispatcher`

Un fallo atribuible a **un** mensaje nunca frena a los demás. Paso por paso (cada mensaje sigue
en su propia transacción `InSystemTx(crm_worker)` → `AsTenant`, como en ADR-010):

| Paso que falla | Error | Qué hace el `Dispatcher` | Log |
|---|---|---|---|
| `LockDueMessage` | cualquiera salvo cancelación | Termina el ciclo (no es de un mensaje: la base no responde) | `ERROR` `event=outbox_cycle_failed` con `err` |
| `AsTenant`, `GetMessage` | `db.ErrUnavailable` | Termina el ciclo, sin aplazar | `ERROR` `event=outbox_cycle_failed` con `err` |
| `AsTenant`, `GetMessage` | cualquier otro (`db.ErrPrivilege`, rol inexistente `22023`, `db.ErrNotFound`, …) | `ROLLBACK`; **aplaza** el mensaje en una transacción aparte y **sigue con el lote** | `ERROR` `outcome=deferred`, `step`, `message_id`, `tenant_id`, `defer_seconds`, `err`; si es `db.ErrPrivilege`, además `security_event=rls_violation` (INV-19) |
| Payload ilegible (JSON) | — | `failed` con `bug compose: invalid payload` | `ERROR` `outcome=failed` |
| `Handle` | `DeliveryError` / otro | Según la causa (§1); si la fase es `connection`, termina el ciclo después de registrar el fallo (§4) | Según la causa |
| `MarkSent`, `MarkRecoverable`, `MarkFailed` | `db.ErrUnavailable` | Termina el ciclo (el mensaje queda `pending`; si ya se entregó, se reenvía: al menos una vez) | `ERROR` `event=outbox_cycle_failed` con `err` |
| `MarkSent`, `MarkRecoverable`, `MarkFailed` | cualquier otro | `ROLLBACK`; aplaza en una transacción aparte y sigue con el lote. Evita reenviar cada 2 s un mensaje que no se puede marcar | `ERROR` `outcome=deferred`, `step=mark`, `err` |
| El aplazamiento mismo | cualquiera | Termina el ciclo (evita volver a tomar el mismo mensaje dentro del lote) | `ERROR` `event=outbox_cycle_failed` con `err` |
| Cualquiera | cancelación (`context.Canceled`, `db.ErrCanceled`) | Termina sin tocar el mensaje | `INFO` `outcome=canceled` |

- **Aplazar** es una actualización como `crm_worker` que cambia **solo** `next_attempt_at` del
  mensaje, con tres condiciones: el `id`, `status = 'pending'` y que `next_attempt_at` siga siendo
  el valor leído por `LockDueMessage`. Usa el privilegio `UPDATE (next_attempt_at)` y la política
  `worker_lock` que **ya existen** (no hay privilegios nuevos). Si otro worker ya lo procesó,
  afecta 0 filas y no es error. No suma `attempts` ni escribe `last_error`: no hubo intento de
  entrega.
- **Demora del aplazamiento**: `clamp(edad del mensaje, 10 s, 15 min)`, con `edad = now −
  created_at`. Sin columnas nuevas ni estado en memoria, la espera se duplica en cada aplazamiento
  (10 s, 20 s, 40 s, …) hasta 15 min. El caso transitorio de DD-34 (primer `SET ROLE` a una empresa
  recién registrada) se reintenta a los 10 s; una empresa sin rol se reintenta cada 15 min hasta que
  se corre `crm tenants reprovision-roles` (plan §12.4).
- Un mensaje aplazado indefinidamente sigue `pending` (y su payload, en la base) hasta que el
  problema se resuelve; lo hacen visible `outbox_oldest_pending_seconds` y el runbook.

## Fundamento

- Separar "¿es culpa del mensaje?" de "¿es culpa nuestra o del proveedor?" es lo único que decide
  bien qué es definitivo: un fallo común a todos los mensajes nunca puede ser definitivo para cada
  uno. Las fases de go-mail permiten esa separación sin analizar textos: lo que ocurre dentro del
  *dial* (conexión, TLS, saludo, AUTH) nunca es del mensaje.
- Una sola causa determina clase y nivel de log: no hay combinaciones inconsistentes posibles, y el
  operador ve en `ERROR` exactamente lo que requiere su intervención (configuración y bugs).
- El sesgo hacia "recuperable" está acotado por los 8 intentos: el peor caso de clasificar mal un
  definitivo es un mensaje que falla 19 h después, no una cola perdida.
- Un formato fijo y saneado en un solo lugar hace que `last_error` y los logs sean útiles para el
  diagnóstico (código, fase, texto del proveedor) sin depender de que cada llamador se acuerde de
  quitar el destinatario.
- El presupuesto de tiempo se apoya en lo que go-mail realmente hace (verificado en su código), no
  en lo que se esperaba (que respetara el `context` en toda la conversación). Con `WithoutRset` y
  5 s por etapa, el peor caso entra en el presupuesto con margen.
- El aplazamiento reusa un privilegio y una política que ya existen; la demora derivada de la edad
  evita una columna nueva y da un *backoff* exponencial sin estado.

## Alternativas consideradas

- **Mantener "todo `5xx` es definitivo"**: es la causa del hallazgo; un problema propio de
  configuración destruye la cola.
- **Definitivo solo con `SendError` (MAIL FROM, RCPT TO, DATA), como sugería la revisión**: mejor,
  pero sigue tratando como definitivos rechazos que son de configuración y afectan a todos (remitente
  no verificado en MAIL FROM, *relay* denegado `5.7.1` en RCPT TO, rechazo de contenido en DATA).
  Se toma la idea y se acota a la regla del destinatario.
- **Usar `SendError.IsTemp()` de go-mail como criterio**: solo mira si el código empieza con `4`; no
  distingue destinatario de configuración y no existe para los errores del *dial*.
- **Que un error `config` no consuma intentos** (reintentar sin límite hasta que se arregle): el
  payload con el token quedaría pendiente sin plazo y los mensajes nunca terminarían; los tokens de
  reset (1 h) y de verificación (48 h) vencen igual, y una invitación se puede reemitir (DD-5). Se
  prefiere un final acotado y visible.
- ***Circuit breaker* en memoria que pausa todo el worker ante un error de conexión**: más estado y
  más casos de test; no reduce el total de intentos (cada mensaje igual hace los suyos). El corte del
  ciclo da el alivio inmediato sin estado.
- **Alerta activa propia (email o *pager*) ante `config`**: no hay infraestructura de alertas en el
  alcance de 001, y avisar por email justo cuando el SMTP no funciona no sirve. Queda en logs
  `ERROR`, métrica y runbook.
- **Subir `idle_in_transaction_session_timeout` de `crm_app`**: protege a toda la aplicación de
  transacciones olvidadas abiertas; subirlo para el worker debilita esa protección en todos los
  caminos.
- **Sacar el envío de la transacción** (reservar el mensaje moviendo `next_attempt_at` como
  *lease*, enviar sin transacción abierta y marcar en otra): elimina el acoplamiento con
  `idle_in_transaction_session_timeout` y libera la conexión del *pool* durante el envío, pero
  cambia el modelo de reclamo de ADR-010 y la semántica de cancelación ya diseñada y probada
  (T-B211); un apagado dejaría el mensaje reservado hasta que venza el *lease*. A esta escala, un
  presupuesto fijo vigilado por tests resuelve el problema con menos cambio. Queda como salida si el
  presupuesto resultara corto para el proveedor elegido.
- **Hacer que go-mail respete el `context` con un `DialContextFunc` propio que cierre la conexión al
  cancelarse**: con un *dialer* propio go-mail deja de hacer TLS implícito (puerto 465) y considera
  la conexión no cifrada, con lo que el autodescubrimiento de AUTH descarta `PLAIN`/`LOGIN`
  (verificado en `client.go` v0.8.1). Demasiado frágil.
- **Excluir los ids fallidos dentro del ciclo en vez de aplazarlos** (alternativa de la revisión):
  arregla el ciclo actual pero no el siguiente; el mensaje vuelve a ocupar el primer lugar del lote
  en cada ciclo y genera un `ERROR` cada 2 s.
- **Aplazar con un `SAVEPOINT` en la misma transacción**: evita la segunda transacción, pero exige
  ampliar la interfaz `db.Tx` con *savepoints*. La segunda transacción usa la API existente y su
  condición sobre `next_attempt_at` cubre la carrera con otro worker.
- **Contar los aplazamientos en una columna nueva**: daría un *backoff* exacto, pero exige una
  migración y un privilegio nuevo para `crm_worker`; la edad del mensaje da el mismo efecto sin
  tocar el esquema.

## Consecuencias

- Ganás: un problema de configuración o de reputación del servidor (credenciales, IP bloqueada,
  remitente no verificado) ya no destruye la cola: los mensajes esperan y salen solos al
  arreglarlo, si es dentro de ~19 h. Un mensaje roto o de una empresa sin rol ya no bloquea a los
  demás. `last_error` y los logs dicen fase, código y texto del proveedor sin datos personales. El
  envío no puede exceder el tiempo que PostgreSQL tolera con la transacción abierta.
- Aceptás: algunos rechazos definitivos reales (p. ej. un `5.7.1` de política del destinatario) se
  reintentan 8 veces antes de fallar; una caída de configuración de más de ~19 h hace fallar los
  mensajes de ese período (los usuarios vuelven a pedir el enlace o el Administrador reinvita); un
  proveedor que tarde más de 5 s en una etapa produce un reintento (y, si ya había aceptado el
  mensaje, un duplicado, aceptado por ADR-010); el apagado puede esperar hasta 20 s un envío en
  curso; el worker ahora sí escribe como `crm_worker` (solo `next_attempt_at`, con el privilegio que
  ya tenía).
- Operación: métricas `outbox_delivery_errors_total{cause}` y `outbox_deferred_total`, y filas
  nuevas del runbook (plan §12.3).
