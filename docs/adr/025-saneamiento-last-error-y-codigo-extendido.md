# ADR-025: `last_error` sin texto del proveedor después del contenido, redacción de secuencias tipo secreto y código extendido leído del texto en todas las fases

**Status**: Accepted
**Fecha**: 2026-10-01
**Origen**: tercera revisión del PR fdelillo/crm#8 (Fase 2 de la spec 001), HEAD 31fb773: hallazgo P2
(un token puede llegar a `last_error` y a los logs) y la observación sobre el código extendido sin
`ENHANCEDSTATUSCODES`.
**Relación**: reemplaza **solo** tres piezas de
[ADR-024](024-clasificacion-fallos-outbox-y-presupuesto-smtp.md): el último párrafo de §2 (de dónde
sale el código extendido), la viñeta **Detalle** de §3 en lo que dice sobre las respuestas de la
fase `data`, y el punto (1) del **Saneamiento obligatorio** de §3. Todo lo demás de ADR-024 (causas,
regla del destinatario, fases, formato `<causa> <fase>[ <código>[ <extendido>]]: <detalle>`, puntos
(2) y (3) del saneamiento, error de configuración persistente, presupuesto de tiempo y aislamiento
por mensaje) sigue vigente sin cambios.

## Contexto

ADR-024 §3 sanea `last_error` y los logs del worker con una sola regla de contenido: todo campo
separado por espacios que contenga `@` se reemplaza por `[redacted]`. Cubre lo que el servidor SMTP
conoce del **sobre** (remitente, destinatario, *message-id*), pero no lo que conoce del **contenido**.
Los emails de 001 llevan un enlace con el token en el fragmento
(`{APP_BASE_URL}/reset-password#token=…`, DD-14). Un proveedor que rechaza el mensaje por su
contenido puede citar la URL o el token en su respuesta al fin de datos (p. ej. un filtro de URLs:
"Message rejected, URL … listed"). Ese texto llega a `DeliveryError.Detail`, pasa el saneamiento
intacto, se guarda en `outbox_messages.last_error` (que sobrevive al borrado del payload) y se
loguea. INV-30 afirma que eso no puede pasar; con la regla actual no se cumple. Verificado en
`internal/platform/outbox/outbox.go` (`sanitizeDetail` solo mira `@`) y en
`internal/platform/mailer/smtp.go` (`sendErrorDetail` pasa el texto del proveedor tal cual).

Segundo problema, vecino: en la fase `connection` el adaptador lee el código extendido (RFC 3463) del
comienzo del texto de la respuesta; en las demás fases usa `SendError.EnhancedStatusCode()`, que
go-mail solo completa si el servidor anuncia `ENHANCEDSTATUSCODES` (verificado en go-mail v0.8.1,
`senderror.go`: `enhancedStatusCode(err, supported)` devuelve `""` si `!supported`). Sin ese anuncio,
una respuesta `550 5.7.1 Relay access denied` en RCPT TO no tiene código extendido y la regla del
destinatario la clasifica como `recipient` (**definitiva**) por el `550`, aunque el texto dice que es
un problema de política o configuración que afecta a todos los mensajes. Es justo el caso que
ADR-024 quiso evitar ("ante la duda, recuperable").

Fuerzas en tensión: que `last_error` siga sirviendo para diagnosticar (código y texto del proveedor);
que nunca guarde ni loguee el token; que la garantía no dependa solo de una heurística; que la
clasificación no dependa de si el servidor anuncia una extensión opcional.

## Decisión

### 1. Sin texto del proveedor en la fase `data`

En `DeliveryError.LastError()` (el único lugar que arma el texto, INV-30): si `Phase == data` y
`SMTPCode != 0`, el detalle es el texto fijo **`response text omitted`**, sea cual sea `Detail`.
Los fallos de red de la fase `data` (`SMTPCode == 0`) conservan su detalle del vocabulario fijo
(`timeout`, `connection closed`, …).

Fundamento: `data` es la **única** fase en la que el servidor ya recibió el contenido. Antes del
comando DATA solo conoce el sobre, que la regla del `@` ya cubre; el token solo viaja en el cuerpo.
Omitir el texto en esa fase es una garantía estructural, no heurística: no depende de cómo el
proveedor cite la URL (entera, cortada, codificada en *quoted-printable*, entre comillas).

La regla vive en `LastError()` y no en el adaptador: vale para cualquier implementación futura de
`Mailer`. El código básico y el extendido se conservan, y son los que dicen la causa del rechazo
(`5.7.1` política, `5.6.x` contenido, `5.3.4` tamaño, `5.1.x` dirección).

Ejemplos (ilustrativos de la forma):

```
config data 554 5.7.1: response text omitted
recipient data 550 5.1.1: response text omitted
network data: timeout
```

### 2. Redacción ampliada, en todas las fases

El punto (1) del saneamiento de ADR-024 §3 pasa a ser: después de convertir los caracteres de
control en espacios, cada campo separado por espacios se reemplaza por `[redacted]` si cumple
**cualquiera** de estas condiciones:

| Condición | Qué cubre |
|---|---|
| Contiene `@` | Sin cambios respecto de ADR-024: destinatario, remitente, *message-id* |
| Contiene `://` | Toda URL con esquema (enlaces de ayuda del proveedor, enlaces del mensaje) |
| Contiene una secuencia de **20 o más** caracteres seguidos del conjunto `[A-Za-z0-9+/=_-]` | Tokens de `securetoken` (43 caracteres base64url), fragmentos `#token=…` sin esquema, JWT, claves y otros identificadores largos en base64, base64url o hexadecimal |

Los puntos (2) y (3) de ADR-024 §3 (una línea, espacios simples, ≤ 1000 runas con `…`) no cambian.

Este punto es **defensa en profundidad** para las fases anteriores a `data` y para los textos de
go-mail: no se espera que el token aparezca ahí (el servidor no lo conoce), pero el costo de
redactar un campo de más es nulo frente al de guardar un secreto en una columna que sobrevive al
borrado del payload.

### 3. Código extendido: del servidor o del comienzo del texto, en todas las fases

Reemplaza el último párrafo de ADR-024 §2. Para toda respuesta SMTP con código básico `c`:

1. Si go-mail informa un código extendido (`SendError.EnhancedStatusCode() != ""`), se usa ese.
2. Si no (fase `connection`, o servidor sin `ENHANCEDSTATUSCODES`), se lee del **comienzo** del
   texto de la respuesta, ya sin el código básico: debe tener la forma `^[245]\.\d{1,3}\.\d{1,3}`
   seguida de un espacio o del final del texto.
3. El código leído del texto se usa **solo si su primer dígito coincide con el primero de `c`**
   (RFC 3463: la clase del código extendido corresponde a la del básico). Si no coincide, se ignora
   y queda como parte del texto.
4. Cuando se usa, se quita del comienzo del detalle (ya está en el prefijo de `last_error`).

La regla del destinatario de ADR-024 §1 no cambia: ahora simplemente ve el código extendido en más
casos. Efecto sobre respuestas sin `ENHANCEDSTATUSCODES`:

| Respuesta | Fase | Antes (ADR-024) | Ahora |
|---|---|---|---|
| `550 5.7.1 Relay access denied` | `rcpt_to` | `recipient` (definitiva, por el `550`) | `config` (recuperable) |
| `550 5.1.1 User unknown` | `rcpt_to` | `recipient` | `recipient` (sin cambios) |
| `550 User unknown` | `rcpt_to` | `recipient` | `recipient` (sin cambios: sin extendido, decide el básico) |
| `554 5.1.1 Unknown user` | `rcpt_to` | `config` | `recipient` |
| `550 5.1.1 Recipient rejected` | `data` | `config` | `recipient` |
| `550 4.2.2 Mailbox full` (clase incoherente) | `rcpt_to` | `recipient` | `recipient` (el extendido se ignora; decide el básico) |
| `554 4.7.1 blocked` (clase incoherente) | `connection` | `config 554 4.7.1` | `config 554` con `4.7.1 blocked` en el detalle |

## Fundamento

- La omisión por fase convierte INV-30 en una propiedad que se puede demostrar ("el servidor no
  conoce el token antes de DATA, y después de DATA no se guarda su texto"), en vez de una que
  depende de adivinar el formato con el que un proveedor cita una URL.
- Se pierde poco diagnóstico: el texto de la fase `data` es justo el más propenso a citar el
  contenido, y los códigos que se conservan dicen la causa. En las fases donde el texto es más útil
  para el operador (credenciales, IP bloqueada, remitente no verificado) se conserva.
- La redacción ampliada es barata y uniforme, y cubre lo que la omisión no alcanza (un texto de
  go-mail o de una fase previa con una URL o un identificador largo).
- Leer el código extendido del texto en todas las fases hace que la clasificación no dependa de una
  extensión opcional del servidor, y aplica la misma regla que ya se usaba en la fase `connection`.
  Los casos que pasan a definitivos (`5.1.x`/`5.2.x` explícitos en el texto) son exactamente la
  evidencia que ADR-024 ya acepta como concluyente cuando el servidor anuncia la extensión. El
  control de clase evita decidir con un código que contradice al básico.

## Alternativas consideradas

- **Solo redacción por patrones, sin omisión por fase** (lo mínimo que pedía la revisión): es una
  heurística. Un token cortado por un salto de línea *quoted-printable* en trozos de menos de 20
  caracteres, o citado con otra codificación, pasaría. Se adopta como segunda capa, no como única.
- **Nunca guardar texto del proveedor (solo causa, fase y códigos)**: la garantía más fuerte, pero
  pierde el texto de las fases `connection`, `mail_from` y `rcpt_to`, que es el que distingue, por
  ejemplo, "client host blocked" de otro `554` en el saludo, y que nunca puede contener el token.
- **Redactar por coincidencia exacta con los valores del payload** (el `Dispatcher` conoce el token
  y el enlace): exacta para el valor tal cual, pero no para formas transformadas (*quoted-printable*
  con `=\r\n`, codificación de URL, truncado), y acopla `LastError()` con el payload de cada
  plantilla. La omisión por fase cubre lo mismo sin ese acoplamiento.
- **Guardar el texto completo cifrado o en otra tabla con acceso restringido**: infraestructura y una
  clave nueva para un dato de diagnóstico; desproporcionado a la escala de 001.
- **Mantener el código extendido solo cuando lo informa go-mail**: deja `550 5.7.1` sin
  `ENHANCEDSTATUSCODES` como definitivo; si la causa es de configuración, se pierde un mensaje por
  intento (y la cola entera si es común a todos), contra el criterio de ADR-024.
- **Buscar el código extendido en cualquier parte del texto** (como hace go-mail con `FindString`):
  podría tomar un número con forma `d.d.d` del medio del texto (una versión, una dirección); se lee
  solo al comienzo, donde lo pone RFC 3463.
- **Leerlo del texto sin controlar la clase**: un `550 4.2.2` decidiría con un código que contradice
  al básico; ante la contradicción, se usa la regla del código básico de ADR-024.

## Consecuencias

- Ganás: INV-30 se cumple por construcción para el token; los textos de ayuda del proveedor con URL
  y los identificadores largos no llegan a la base; la clasificación de un rechazo no depende de que
  el servidor anuncie `ENHANCEDSTATUSCODES` (el `5.7.1` sin anuncio deja de ser definitivo).
- Aceptás: en la fase `data` el operador ve solo los códigos y, para el detalle, tiene que mirar el
  panel o los logs del proveedor por fecha y hora (fila nueva del runbook, plan §12.3); algunos
  campos inocuos de 20 o más caracteres (nombres de host con guiones, identificadores de cola) se
  redactan; unos pocos rechazos sin `ENHANCEDSTATUSCODES` pasan a definitivos (`554 5.1.1` en RCPT
  TO, `550 5.1.1` en DATA), con la misma evidencia que ya se aceptaba con el anuncio.
- Operación: sin métricas nuevas. El runbook suma la lectura de `response text omitted`.
- Mantenimiento: cambiar el conjunto o el umbral de la redacción, o la regla de la fase `data`,
  obliga a actualizar este ADR (o uno nuevo si cambia la decisión), INV-30, plan §9.4/§12.1,
  `data-model.md` §2.5 y los casos de T-B211/T-B213 (matriz del plan §16).
