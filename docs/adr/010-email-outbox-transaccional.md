# ADR-010: Envío de emails con outbox transaccional, worker en el binario y puerto `Mailer` SMTP

**Status**: Accepted (decisión del usuario; librería SMTP y políticas de reintento: propuesta del arquitecto). La clasificación de errores de la viñeta "Reintentos" la reemplaza [ADR-024](024-clasificacion-fallos-outbox-y-presupuesto-smtp.md) (ver nota 2026-10-01).
**Fecha**: 2026-09-27
**Origen**: spec 001 (verificación de email, reset de contraseña, invitaciones)

> **Nota 2026-10-01 (reemplazo parcial; el resto de la decisión no cambia)**, por la revisión del
> PR fdelillo/crm#8 (hallazgos I1, I2, M1, M7): la frase "Errores definitivos (`5xx`, plantilla
> inválida) pasan a `failed` de inmediato" de la viñeta **Reintentos** era demasiado amplia: aplicada
> a los `5xx` de la conexión (saludo `554` por IP bloqueada, `535` de AUTH por credenciales
> vencidas) pasa toda la cola a `failed` y borra los payloads. La clasificación de fallos, el formato
> de `last_error`, el presupuesto de tiempo del envío frente a `idle_in_transaction_session_timeout`
> y el aislamiento de fallos por mensaje del `Dispatcher` quedan en
> [ADR-024](024-clasificacion-fallos-outbox-y-presupuesto-smtp.md) (**Accepted** 2026-10-01). En resumen: solo es definitivo un rechazo del **destinatario** (`5xx` en RCPT
> TO o DATA con código extendido `5.1.x`/`5.2.x`, o `550`/`551`/`553` en RCPT TO sin código
> extendido) o un **bug** (plantilla o payload inválidos); todo otro `5xx` es un error de
> configuración recuperable que se loguea en `ERROR`. Siguen vigentes sin cambios: outbox
> transaccional, worker en el binario con *polling* de 2 s y lote de 10, un mensaje por transacción
> con `FOR UPDATE SKIP LOCKED`, backoff 1, 5, 15, 60 min y 6 h con 8 intentos, entrega al menos una
> vez, borrado del payload al terminar, puerto `Mailer` con go-mail, Mailpit en desarrollo. Se
> corrige además un dato de "Puerto `Mailer`": go-mail v0.8.1 respeta el `context` solo durante el
> *dial*; el resto de la conversación lo acotan *deadlines* por etapa (ADR-024 §5).

## Contexto

Varias operaciones de 001 envían un email: registro (verificación), pedido de reset, invitación,
reactivación sin contraseña. Si el email se envía dentro del request y la transacción después
falla, el usuario recibe un enlace a algo que no existe; si se envía después del `COMMIT` desde el
request y el proceso se cae, el email se pierde. El proveedor de email concreto todavía no está
elegido.

## Decisión

- **Outbox transaccional**: la operación inserta una fila en `outbox_messages` en **la misma
  transacción** que la escritura de negocio. El email existe si y solo si la operación se
  confirmó.
- **Worker dentro del mismo binario** (goroutine lanzada por `serve`): cada 2 s toma hasta 10
  mensajes pendientes con `FOR UPDATE SKIP LOCKED`, uno por transacción; lee el contenido bajo el
  rol de la empresa del mensaje (ADR-005), envía y marca `sent`.
- **Reintentos**: errores recuperables (timeout, conexión, respuestas SMTP `4xx`) reprograman con
  backoff 1, 5, 15, 60 min y 6 h; al 8.º intento el mensaje pasa a `failed`. Errores definitivos
  (`5xx`, plantilla inválida) pasan a `failed` de inmediato.
- **Entrega al menos una vez**: si el SMTP aceptó y el `COMMIT` falla, el mensaje se reenvía. Se
  acepta: los enlaces son de un solo uso y el duplicado es inocuo.
- **Datos sensibles**: el payload (que incluye el token en claro del enlace) se borra al pasar a
  `sent` o `failed`; un `CHECK` de la tabla lo obliga.
- **Puerto `Mailer`** (`Send(ctx, Email) error`) con un adaptador **SMTP genérico** implementado
  con `github.com/wneessen/go-mail` (texto + HTML, STARTTLS/TLS, autenticación, timeouts con
  `context`). En desarrollo, **Mailpit**. El proveedor transaccional de producción se elige al
  desplegar y se usa por SMTP; si más adelante conviene su API, se agrega otro adaptador del
  mismo puerto.
- Plantillas en español (Argentina) embebidas en el binario (`text/template` y `html/template`),
  una por tipo de mensaje, en el módulo que las origina.

## Fundamento

- La atomicidad entre la operación y su notificación sale gratis de la transacción de PostgreSQL:
  no hace falta un protocolo distribuido.
- Sin infraestructura adicional (colas, brokers): coherente con el principio de simplicidad y con
  una sola unidad desplegable.
- `SKIP LOCKED` permite, llegado el caso, varias instancias sin enviar dos veces el mismo mensaje.
- SMTP es el denominador común de todos los proveedores transaccionales.
- `net/smtp` de la librería estándar está congelado; go-mail está mantenida y resuelve MIME y TLS.

## Alternativas consideradas

- **Envío síncrono dentro del request**: un SMTP lento o caído hace fallar el registro, y un
  `ROLLBACK` posterior deja un email enviado de algo que no existe.
- **Goroutine *fire and forget* después del `COMMIT`**: se pierde en un reinicio, sin reintentos.
- **Cola externa** (Redis, SQS, NATS): infraestructura extra sin un requisito de volumen que la
  justifique.
- **`LISTEN/NOTIFY` en lugar de *polling***: menor latencia, pero exige una conexión dedicada y
  manejo de reconexión; el *polling* cumple el objetivo (95 % en < 30 s). Queda como optimización.
- **SDK de un proveedor**: ata el código a un proveedor antes de elegirlo.

## Consecuencias

- Ganás: emails consistentes con las operaciones, reintentos persistentes, proveedor
  intercambiable.
- Aceptás: latencia de hasta unos segundos por el *polling*; posibles duplicados; la tabla
  `outbox_messages` necesita limpieza periódica (30 días); el payload con el token vive en la base
  mientras el mensaje está pendiente (minutos).
- Operación: métricas `outbox_pending`, `outbox_oldest_pending_seconds`, `outbox_failed_total`
  y runbook en el plan de 001 (§12.3).
