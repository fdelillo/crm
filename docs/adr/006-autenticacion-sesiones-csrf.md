# ADR-006: Autenticación con sesiones en PostgreSQL, token opaco en cookie y protección CSRF

**Status**: Accepted (sesiones en BD y cookie: decisión del usuario; duración: decisión del usuario, P-5; esquema CSRF y detalles: propuesta del arquitecto)
**Fecha**: 2026-09-27
**Origen**: spec 001 (Historias 2 y 3.3)

> **Revisión 2026-09-27 (antes de aprobar el plan de 001)**: la duración de la sesión pasó de la
> propuesta inicial (7 días sin uso / 30 días) a **24 h sin uso / 7 días**, por respuesta del
> usuario a la pregunta P-5. Se ajustó este mismo ADR en lugar de crear uno nuevo porque el plan
> todavía no estaba aprobado y la decisión estructural (sesión en base + cookie) no cambió. Desde
> la aprobación del plan, cualquier cambio de duración va en un ADR que reemplace a este.

> **Nota 2026-09-29 (detalle; no cambia la decisión)**, por los hallazgos H-3 y H-9 de la
> revisión del frontend-architect:
>
> 1. **Desarrollo local con navegador (H-3, DD-24 del plan de 001)**: `APP_BASE_URL` puede ser
>    `http://` **solo** si el host es `localhost` o `127.0.0.1`, y eso se combina con
>    `COOKIE_SECURE=true` (los navegadores tratan esos orígenes como contexto seguro y aceptan
>    `__Host-` con `Secure`). `COOKIE_SECURE=false` queda **solo para clientes que no son
>    navegador** (tests de integración con `httptest` y similares); la configuración rechaza al
>    arrancar cualquier otra combinación. Esto precisa el último punto de "Consecuencias" sin
>    cambiarlo.
> 2. **Respuesta del rechazo CSRF (H-9, DD-30)**: `http.CrossOriginProtection` se configura con
>    `SetDenyHandler` para que el rechazo sea `403` `application/problem+json` con
>    `code: forbidden` (el mismo formato que el resto de la API, ADR-009) y se registre el evento
>    `csrf_rejected` en el log. `CrossOriginProtection` envuelve el **mux raíz** (DD-22), así que
>    cubre también cualquier método no seguro que llegue fuera de `/api/`.
>
> No se crea un ADR nuevo porque ni el modelo de sesión, ni la cookie, ni las tres capas CSRF
> cambian: se fija una regla de configuración y el formato de un rechazo.

## Contexto

Los usuarios entran con email y contraseña desde una PWA servida en el mismo origen que la API.
La spec exige que **desactivar a un usuario cierre sus sesiones abiertas** (Historia 3.3) y que
un cambio de rol tenga efecto. Los usuarios operan desde el celular en obra o taller, donde un
dispositivo puede perderse o compartirse.

## Decisión

- **Sesión del lado servidor** en la tabla `sessions`. Al autenticar se genera un token de 32
  bytes de `crypto/rand`; la cookie lleva el token en base64url; la base guarda solo su
  **SHA-256**.
- **Cookie** `__Host-crm_session`: `HttpOnly; Secure; SameSite=Lax; Path=/`, sin `Domain`,
  `Max-Age=604800` (el vencimiento absoluto). El prefijo `__Host-` obliga al navegador a exigir
  `Secure`, `Path=/` y ausencia de `Domain` (ningún subdominio puede pisarla).
- **Vencimiento**: **24 h sin uso** o **7 días** desde el login, lo primero que ocurra.
  Configurables con `SESSION_IDLE` (default `24h`) y `SESSION_ABSOLUTE` (default `168h`). El
  absoluto se fija en `sessions.expires_at` al crear la sesión; la inactividad se evalúa contra
  `last_seen_at`, que se actualiza como máximo cada 5 minutos.
- **Resolución en cada request** (`identity.Authenticate`): hash del token → fase 1 como
  `crm_auth` para obtener la empresa → fase 2 como rol de la empresa para validar revocación,
  vencimientos y que el usuario siga `active`, y leer su rol (ADR-005).
- **Revocación**: logout (esa sesión), reset de contraseña (todas las del usuario),
  desactivación (todas, en la misma transacción). Cambiar el rol no revoca: el rol se lee en cada
  request. Reactivar un usuario no reabre sesiones revocadas.
- **Sin fijación de sesión**: el token siempre lo genera el servidor al autenticar.
- **CSRF**, tres capas independientes:
  1. `http.CrossOriginProtection` de la librería estándar (Go ≥ 1.25) sobre todo el router:
     rechaza métodos no seguros de origen cruzado usando `Sec-Fetch-Site` o, si falta, `Origin`
     contra `Host`.
  2. `SameSite=Lax`: el navegador no envía la cookie en `POST` iniciados por otro sitio.
  3. Los endpoints JSON exigen `Content-Type: application/json` (415 si no); la API no habilita
     CORS, así que otro sitio no puede enviar ese tipo sin un *preflight* que falla.

## Fundamento

- Una sesión en la base se revoca con un `UPDATE`: la desactivación tiene efecto en el siguiente
  request, que es lo que pide la spec.
- `HttpOnly` impide que un XSS lea el token; `SameSite` y `CrossOriginProtection` cubren CSRF sin
  tokens en cada formulario.
- 24 h de inactividad acota la exposición de un celular olvidado o compartido; 7 días de vida
  máxima hace que ninguna sesión sobreviva más de una semana aunque se use a diario.
- El costo de una consulta por request es un acceso por índice único; irrelevante a la escala del
  MVP.
- Guardar solo el hash hace que un volcado de la base no entregue sesiones utilizables.

## Alternativas consideradas

- **JWT de acceso corto + refresh token**: la revocación inmediata exige lista negra (consulta a
  la base igual) y el rol queda congelado dentro del token hasta que vence; más piezas para el
  mismo resultado.
- **Token en `Authorization` guardado en `localStorage`**: elimina CSRF pero cualquier XSS roba la
  sesión.
- **Token sincronizador CSRF** (`gorilla/csrf` o propio): robusto pero agrega un endpoint o cookie
  para obtener el token y trabajo en cada request de la SPA, sin ganancia con SPA en el mismo
  origen.
- **`SameSite=Strict`**: rompería la navegación entrante con sesión (abrir la app desde un enlace
  de WhatsApp o del email).
- **Sesiones en memoria o Redis**: se pierden al reiniciar o agregan infraestructura.
- **7 días sin uso / 30 días** (propuesta inicial): menos fricción en el celular, pero ventana de
  exposición mayor; el usuario prefirió 24 h / 7 días.

## Consecuencias

- Ganás: revocación inmediata, rol siempre actualizado, token inaccesible desde JavaScript,
  ventana de exposición corta.
- Aceptás: una consulta de sesión por request; más inicios de sesión (quien no usa la app un día
  vuelve a entrar, lo que agrega carga de argon2id acotada por su semáforo); la PWA **debe**
  servirse desde el mismo origen que la API (si se separan, hay que rediseñar CORS y CSRF con un
  ADR nuevo); navegadores anteriores a ~2020 sin `Sec-Fetch-Site` ni `Origin` quedan protegidos
  solo por `SameSite` y el `Content-Type`.
- La UI tiene que tratar un `401` en cualquier request como "sesión vencida" y volver al login
  sin perder lo que el usuario estaba cargando cuando sea posible (a definir en `ui.md`).
- Desarrollo local: `__Host-` exige `Secure`; los navegadores actuales tratan `http://localhost`
  como contexto seguro, y para tests sin TLS existe `COOKIE_SECURE=false` (nunca en producción;
  la configuración lo valida).
