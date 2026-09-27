# ADR-006: Autenticación con sesiones en PostgreSQL, token opaco en cookie y protección CSRF

**Status**: Accepted (sesiones en BD y cookie: decisión del usuario; esquema CSRF y detalles: propuesta del arquitecto)
**Fecha**: 2026-09-27
**Origen**: spec 001 (Historias 2 y 3.3)

## Contexto

Los usuarios entran con email y contraseña desde una PWA servida en el mismo origen que la API.
La spec exige que **desactivar a un usuario cierre sus sesiones abiertas** (Historia 3.3) y que
un cambio de rol tenga efecto. Los usuarios operan desde el celular en obra o taller: no deberían
tener que iniciar sesión todos los días.

## Decisión

- **Sesión del lado servidor** en la tabla `sessions`. Al autenticar se genera un token de 32
  bytes de `crypto/rand`; la cookie lleva el token en base64url; la base guarda solo su
  **SHA-256**.
- **Cookie** `__Host-crm_session`: `HttpOnly; Secure; SameSite=Lax; Path=/`, sin `Domain`,
  `Max-Age` = vencimiento absoluto. El prefijo `__Host-` obliga al navegador a exigir `Secure`,
  `Path=/` y ausencia de `Domain` (ningún subdominio puede pisarla).
- **Vencimiento**: 7 días sin uso o 30 días desde el login, lo primero que ocurra.
  `last_seen_at` se actualiza como máximo cada 5 minutos.
- **Resolución en cada request** (`identity.Authenticate`): hash del token → fase 1 como
  `crm_auth` para obtener la empresa → fase 2 como rol de la empresa para validar revocación,
  vencimientos y que el usuario siga `active`, y leer su rol (ADR-005).
- **Revocación**: logout (esa sesión), reset de contraseña (todas las del usuario),
  desactivación (todas, en la misma transacción). Cambiar el rol no revoca: el rol se lee en cada
  request.
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

## Consecuencias

- Ganás: revocación inmediata, rol siempre actualizado, token inaccesible desde JavaScript.
- Aceptás: una consulta de sesión por request; la PWA **debe** servirse desde el mismo origen que
  la API (si se separan, hay que rediseñar CORS y CSRF con un ADR nuevo); navegadores anteriores a
  ~2020 sin `Sec-Fetch-Site` ni `Origin` quedan protegidos solo por `SameSite` y el
  `Content-Type`.
- Desarrollo local: `__Host-` exige `Secure`; los navegadores actuales tratan `http://localhost`
  como contexto seguro, y para tests sin TLS existe `COOKIE_SECURE=false` (nunca en producción;
  la configuración lo valida).
