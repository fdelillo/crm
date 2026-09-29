# ADR-019: Distribución de la SPA embebida en el binario Go, mismo origen que la API

**Status**: Accepted (SPA embebida con `go:embed`, mismo origen que `/api/v1`, proxy de Vite en desarrollo: decisión del usuario) · detalles (montaje fuera del router de la API, *fallback*, cabeceras de caché, CSP, compresión): Accepted el 2026-09-29 (ver nota)
**Fecha**: 2026-09-29
**Origen**: spec 001 (`ui.md` §21); relacionado con ADR-001, ADR-006 y S-2 del plan

> **Nota 2026-09-29 (revisión 1 de `ui.md`)**: el usuario aprobó los hallazgos H-1 y H-8 del
> frontend y el backend los incorporó (`plan.md` §18, DD-22, DD-29, §10.7). Con eso, los detalles
> que este ADR tenía como `Proposed` quedan `Accepted`, incluida la dependencia `gzhttp` (aprobada
> por el usuario, solo para la SPA). Los cambios respecto del texto original son de precisión, no
> de decisión: nombres de las firmas del paquete `web` (`DistFS`, `NewHandler`), el punto de
> inyección `RootDeps.SPA` con un stub `503` hasta T-F008, `CrossOriginProtection` entre los
> middlewares comunes, el umbral de gzip y que la **tabla de cabeceras de referencia es
> `plan.md` §10.7** (este ADR y `ui.md` §21.2 la reproducen; si difieren, manda §10.7 y se
> corrigen los otros en el mismo cambio). Queda abierto el hallazgo H-10 (la cookie `__Host-`
> en `http://localhost` con Chrome), que afecta el desarrollo local, no esta decisión.

> **Nota 2026-09-29 (b) (revisión 2 de `ui.md`; corrige la viñeta "Desarrollo" y cierra lo que
> la nota anterior dejaba abierto; no cambia la decisión)**: el usuario resolvió H-10 con **HTTPS
> local con mkcert** y el backend lo incorporó (`plan.md` DD-24 reescrita, INV-23, §10.5.1, §10.7;
> nota (b) en ADR-006). La viñeta "Desarrollo" de la sección Decisión queda así (el texto original
> se conserva abajo sin editar):
>
> 1. Vite corre en **`https://localhost:5173`** con `server.https` y el certificado de
>    `localhost` que genera `make dev-certs` (`.certs/localhost.pem`, `.certs/localhost-key.pem`,
>    el mismo que usa `crm serve`), con `strictPort`.
> 2. El *proxy* de `/api` apunta a **`https://localhost:8443`** (`crm serve` con `TLS_CERT_FILE` y
>    `TLS_KEY_FILE`, solo en modo local), sigue **sin** `changeOrigin` y verifica el certificado:
>    Node confía en la CA con `NODE_EXTRA_CA_CERTS="$(mkcert -CAROOT)/rootCA.pem"`.
> 3. `APP_BASE_URL=https://localhost:5173` en ese modo (los enlaces de email abren Vite). Ya no
>    existe `COOKIE_SECURE` ni se acepta `APP_BASE_URL` con `http://`: la cookie es siempre
>    `__Host-crm_session` con `Secure`, en Chrome, Firefox y Safari.
> 4. Para probar como en producción (y en los E2E): `make build` y `crm serve` en
>    **`https://localhost:8443`**.
> 5. **HSTS**: la fila "toda respuesta" de la tabla de cabeceras (`plan.md` §10.7) lleva
>    `Strict-Transport-Security` **salvo en modo local** (host de `APP_BASE_URL` `localhost` o
>    `127.0.0.1`), para que el navegador del desarrollador no registre HSTS para `localhost`. Lo
>    prueban T-B203/T-B004 y T-F007 en ambos modos.
>
> Detalle operativo (tabla de modos, configuración de Vite, diagnóstico y receta de CI de los E2E)
> en `ui.md` §21.3 y `plan.md` §10.5.1. El trade-off nuevo: un paso de instalación por equipo
> (`mkcert -install`, `make dev-certs`) y la CA local instalada también en el runner de E2E.

## Contexto

La sesión es una cookie `__Host-` `HttpOnly` con protección CSRF basada en mismo origen (ADR-006):
servir la SPA desde otro origen obligaría a rediseñar CORS y CSRF. El backend es un único binario
(ADR-001). Una SPA con rutas de cliente necesita que el servidor devuelva `index.html` para
cualquier ruta de la app, sin tapar los `404` de la API ni servir HTML en lugar de un JS
inexistente. Tras cada deploy cambian los nombres con hash de los assets.

## Decisión

- **Build**: `web/` → `npm run build` → `web/dist/`. Paquete Go `web` en la raíz del repo (fuera de
  `internal/`, porque `go:embed` necesita que `dist` esté bajo el paquete) con `//go:embed all:dist`
  y dos funciones: `DistFS() fs.FS` (el sub-FS `dist`) y `NewHandler(dist fs.FS) http.Handler`
  (recibe un `fs.FS` para poder probarlo con `fstest.MapFS`). `web/dist/` se ignora en git salvo
  `web/dist/.gitkeep`, así el backend compila y testea sin compilar el frontend; sin `index.html` el
  handler responde `503` con un texto que indica correr `make web-build`. `web` solo importa la
  librería estándar y `gzhttp`; solo `internal/app` y `cmd/crm` lo importan.
- **Montaje fuera del router de la API** (plan DD-22, INV-22): `internal/app` arma un
  `http.ServeMux` raíz: `/api/` → router chi, `GET /healthz` y `GET /readyz` → ops, **todo lo
  demás** → `RootDeps.SPA`. Hasta T-F008, `RootDeps.SPA` es un stub que responde `503` con el mismo
  texto; T-F008 lo reemplaza por `web.NewHandler(web.DistFS())`. Los middlewares comunes (request
  id, recover, logging, cabeceras de seguridad y `CrossOriginProtection`) envuelven al mux raíz; los
  propios de la API (`no-store`, rate limit, sesión, permisos) quedan dentro de chi; los de la SPA
  (CSP, caché por tipo de archivo, gzip) dentro de `web`.
- **Reglas del handler**: solo `GET`/`HEAD` (si no, `405`); archivo existente → se sirve; ruta con
  extensión que no existe → `404` texto plano con `no-store` (nunca `index.html`); cualquier otra
  ruta → `index.html` con `200`.
- **Cabeceras de caché**: `index.html`, `sw.js`, `manifest.webmanifest`, `offline.html` →
  `no-cache`; `/assets/*` (con hash) → `public, max-age=31536000, immutable`; íconos → `public,
  max-age=86400`. Tabla completa (API, logo, ops y SPA) en `plan.md` §10.7.
- **CSP** en los documentos HTML: `default-src 'self'; script-src 'self'; style-src 'self'
  'unsafe-inline'; img-src 'self' blob: data:; font-src 'self'; connect-src 'self'; manifest-src
  'self'; worker-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self';
  frame-ancestors 'none'`.
- **Compresión** gzip **solo** del handler de la SPA (HTML, JS, CSS, manifest, service worker) con
  `github.com/klauspost/compress/gzhttp` y su umbral por defecto (1 KB); los íconos PNG y la API no
  se comprimen. Dependencia aprobada por el usuario (plan DD-29). Si el hosting pone un proxy que
  comprime, se quita.
- **Desarrollo**: Vite en `:5173` con *proxy* de `/api` a `http://localhost:8080`, sin
  `changeOrigin` (mismo origen para el navegador); `APP_BASE_URL=http://localhost:5173` para que
  los enlaces de email abran Vite (el backend lo acepta con `COOKIE_SECURE=true`, DD-24). Para
  probar como en producción: `make build` y `crm serve`.
- **Makefile**: `web-build` (`npm ci && npm run build` en `web/`), `web-check` (`npm run check`),
  `build` (`web-build` y después `go build`), `check-all` (`check` del backend + `web-check`).
- **Errores de carga tras un deploy**: la SPA escucha `vite:preloadError` y recarga una vez.

## Fundamento

- Mismo origen = la cookie viaja sola, `SameSite=Lax` y `Sec-Fetch-Site: same-origin` funcionan sin
  CORS; se mantiene todo ADR-006 sin cambios.
- Un solo artefacto desplegable (constitución: monolito), versión de frontend y backend siempre
  alineadas: no hay un frontend nuevo hablando con una API vieja.
- Montar la SPA fuera de chi deja intactos los tests que recorren las rutas de la API (cobertura de
  aislamiento con `chi.Walk`, rutas contra el contrato) y el `404 problem+json` de la API.
- `no-cache` en `index.html` + `immutable` en assets con hash es la combinación estándar: cada
  carga ve el `index.html` nuevo, y los assets se descargan una sola vez por versión.
- No servir `index.html` para archivos con extensión evita el error "MIME type text/html" y que un
  navegador cachee HTML como JS.
- `script-src 'self'` sin inline es la parte de la CSP que realmente frena un XSS.
- Comprimir solo estáticos públicos evita analizar ataques tipo BREACH sobre respuestas con datos
  personales (plan DD-29).

## Alternativas consideradas

- **SPA en un CDN u otro dominio** (Vercel, Netlify, S3+CloudFront): despliegues separados y
  caché en el borde, pero exige CORS con credenciales y rediseñar CSRF (ADR-006 lo prohíbe sin un
  ADR nuevo); dos artefactos con versiones que pueden desalinearse.
- **nginx/Caddy delante sirviendo `dist` y haciendo proxy a Go**: estándar, pero agrega un proceso
  y configuración que el hosting (P-1 del plan) todavía no define. Queda como opción si el hosting
  ya trae un proxy.
- **Servir `dist` desde disco** (`http.Dir`) en vez de embebido: permite cambiar el frontend sin
  recompilar, pero el binario deja de ser autocontenido y la versión puede desalinearse.
- **Registrar la SPA como ruta `/*` en chi**: más directo, pero rompe los tests de rutas del
  backend (T-B004, T-B801, ADR-014).
- **Proxy de desarrollo del backend hacia Vite** (Go sirve todo y reenvía a `:5173`): un solo
  puerto, pero más código Go solo para desarrollo; el proxy de Vite viene incluido.
- **CSP con *nonce*** para permitir estilos inline de forma estricta: requiere generar HTML por
  request (el `index.html` es estático); desproporcionado para el riesgo de estilos inline.
- **gzip a mano con `compress/gzip`** o **archivos precomprimidos**: sin dependencia o sin CPU por
  request, pero con más lógica propia (`Accept-Encoding`, `Vary`, `ETag`, umbral) fácil de hacer
  mal (plan R-21).

## Consecuencias

- Ganás: un binario que contiene todo, sesión y CSRF sin cambios, cabeceras de caché correctas,
  CSP estricta para scripts, estáticos comprimidos en cualquier hosting.
- Aceptás: compilar el frontend antes que el binario de producción (Node en CI); cada deploy
  invalida los chunks viejos (mitigado con la recarga por `vite:preloadError`); la CSP no aplica en
  el servidor de desarrollo de Vite, así que sus violaciones se detectan en el E2E contra el
  binario; cambiar el montaje o las cabeceras exige tocar `internal/app` o `web` (código Go que
  implementa `backend-developer`); una dependencia Go más (`gzhttp`).
