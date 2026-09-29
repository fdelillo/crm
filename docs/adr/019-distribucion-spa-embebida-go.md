# ADR-019: Distribución de la SPA embebida en el binario Go, mismo origen que la API

**Status**: Accepted (SPA embebida con `go:embed`, mismo origen que `/api/v1`, proxy de Vite en desarrollo: decisión del usuario) · detalles (montaje fuera del router de la API, *fallback*, cabeceras de caché, CSP, compresión): Proposed
**Fecha**: 2026-09-29
**Origen**: spec 001 (`ui.md` §21); relacionado con ADR-001, ADR-006 y S-2 del plan

## Contexto

La sesión es una cookie `__Host-` `HttpOnly` con protección CSRF basada en mismo origen (ADR-006):
servir la SPA desde otro origen obligaría a rediseñar CORS y CSRF. El backend es un único binario
(ADR-001). Una SPA con rutas de cliente necesita que el servidor devuelva `index.html` para
cualquier ruta de la app, sin tapar los `404` de la API ni servir HTML en lugar de un JS
inexistente. Tras cada deploy cambian los nombres con hash de los assets.

## Decisión

- **Build**: `web/` → `npm run build` → `web/dist/`. Paquete Go `web` (`web/embed.go`) con
  `//go:embed all:dist`. `web/dist/` se ignora en git salvo `web/dist/.gitkeep`, así el backend
  compila y testea sin compilar el frontend; sin `index.html` el handler responde `503` con un
  texto que indica correr `make web-build`.
- **Montaje fuera del router de la API**: `internal/app` arma un mux raíz: `/api/` → router chi,
  `/healthz` y `/readyz` → ops, **todo lo demás** → handler de la SPA. Los middlewares de request
  id, recover, logging y cabeceras de seguridad envuelven los tres.
- **Reglas del handler**: solo `GET`/`HEAD` (si no, `405`); archivo existente → se sirve; ruta con
  extensión que no existe → `404` texto plano con `no-store` (nunca `index.html`); cualquier otra
  ruta → `index.html` con `200`.
- **Cabeceras de caché**: `index.html`, `sw.js`, `manifest.webmanifest`, `offline.html` →
  `no-cache`; `/assets/*` (con hash) → `public, max-age=31536000, immutable`; íconos → `public,
  max-age=86400`.
- **CSP** en los documentos HTML: `default-src 'self'; script-src 'self'; style-src 'self'
  'unsafe-inline'; img-src 'self' blob: data:; font-src 'self'; connect-src 'self'; manifest-src
  'self'; worker-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self';
  frame-ancestors 'none'`.
- **Compresión** gzip del handler de la SPA con `github.com/klauspost/compress/gzhttp` (pendiente de
  aprobación del backend-architect: dependencia nueva; se quita si el hosting comprime).
- **Desarrollo**: Vite en `:5173` con *proxy* de `/api` a `http://localhost:8080` (mismo origen
  para el navegador); `APP_BASE_URL=http://localhost:5173` para que los enlaces de email abran
  Vite. Para probar como en producción: `make build` y `crm serve`.
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

## Consecuencias

- Ganás: un binario que contiene todo, sesión y CSRF sin cambios, cabeceras de caché correctas,
  CSP estricta para scripts.
- Aceptás: compilar el frontend antes que el binario de producción (Node en CI); cada deploy
  invalida los chunks viejos (mitigado con la recarga por `vite:preloadError`); la CSP no aplica en
  el servidor de desarrollo de Vite, así que sus violaciones se detectan en el E2E contra el
  binario; cambiar el montaje o las cabeceras exige tocar `internal/app` (código del backend).
