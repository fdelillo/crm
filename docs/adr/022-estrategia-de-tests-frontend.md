# ADR-022: Estrategia de tests del frontend (Vitest + Testing Library + MSW, Playwright para E2E)

**Status**: Proposed
**Fecha**: 2026-09-29
**Origen**: spec 001 (`tasks.md` §Frontend); complementa ADR-012 (backend)
**Revisión 2026-09-29**: frecuencia de E2E confirmada por el usuario (P-F5); se agrega el adaptador de
canvas de la preparación del logo a lo que se sustituye en jsdom; los E2E de Chromium dependían del
hallazgo H-10 de `ui.md` (cookie `__Host-` en `http://localhost`).
**Revisión 2 (2026-09-29)**: H-10 resuelto por el usuario con HTTPS local con mkcert (`plan.md`
DD-24, §10.5.1): los E2E corren contra `https://localhost:8443` con la CA de mkcert instalada en el
runner, sin `ignoreHTTPSErrors` salvo como respaldo del supuesto S-12; se agrega la verificación
explícita de la cookie real.

## Contexto

La constitución pide TDD (principio VI) y que los criterios Dado/Cuando/Entonces se traduzcan en
tests. En frontend, el test que vale es el que prueba **lo que el usuario percibe**: textos, roles,
foco, qué pasa al tocar un botón, qué se ve con cada respuesta del servidor. Un test que conoce el
estado interno de un componente se rompe en cada refactor. Además hay cosas que solo se pueden
probar en un navegador real: service worker, CSP, cabeceras, cookie de sesión, y la decodificación
y codificación de imágenes con `createImageBitmap` y `<canvas>` (que jsdom no implementa).

## Decisión

- **Unitarios y de pantalla**: **Vitest** (entorno `jsdom`) + **Testing Library** (`@testing-library/react`,
  `user-event`, `jest-dom`). Consultas por **rol y nombre accesible**, nunca por clase CSS ni
  estructura del DOM.
- **Red interceptada en el borde HTTP con MSW** (`msw/node`): los tests de pantalla usan el
  cliente real (openapi-fetch), la caché real (TanStack Query), el router real (mismo árbol con
  `createMemoryRouter`) y formularios reales. **No se sustituyen hooks de datos.** Las respuestas
  de MSW se tipan con los tipos generados del contrato; los fixtures usan datos ficticios.
- **Tests de tipos** (`*.test-d.ts` con `vitest --typecheck`) para lo que vigila el compilador:
  tipos derivados del contrato, rutas disjuntas entre specs, exhaustividad de mensajes por `code`.
- **Funciones puras** (reglas de acciones de usuario, CUIT, `safeNextPath`, formato, firma y
  dimensiones de imágenes, plan de preparación del logo): tablas de casos, igual que en Go.
- **Código que depende de APIs de navegador que jsdom no tiene** (hoy, solo el adaptador
  `features/tenant/logo/canvas.ts`): se mantiene mínimo; la lógica que lo usa recibe el adaptador
  como parámetro (`prepareLogo(file, deps)`) y se prueba con uno falso; el adaptador real se prueba
  en Playwright con imágenes de verdad.
- **End-to-end**: **Playwright** contra el binario real (SPA embebida) servido por **HTTPS local
  en `https://localhost:8443`**, con `docker compose` (PostgreSQL, Mailpit, MinIO); los tokens se
  leen de los emails vía la API HTTP de Mailpit. Pocos flujos críticos (registro, login/logout,
  invitación, reset, sesión revocada, logo desde una foto) + **axe** (`@axe-core/playwright`) en
  cada pantalla + verificación de PWA, CSP, viewport de 320 px y de la **cookie real**
  (`__Host-crm_session` con `Secure`, `HttpOnly`, `SameSite=Lax`). **Chromium en cada PR; WebKit
  antes de liberar** (confirmado por el usuario, P-F5).
- **Certificado en los E2E**: el job de CI sigue la receta de `plan.md` §10.5.1 (mkcert de versión
  fijada con checksum verificado, `certutil`, `mkcert -install`, `make dev-certs`, `crm serve` en
  modo local con `TLS_CERT_FILE`/`TLS_KEY_FILE`, `NODE_EXTRA_CA_CERTS` para el proceso de
  Playwright). La CA se crea en el runner y muere con él. Playwright corre **sin**
  `ignoreHTTPSErrors`. Que Chromium y WebKit de Playwright confíen en esa CA es el supuesto S-12
  del plan, validado en la primera corrida; respaldo: `ignoreHTTPSErrors` solo para el navegador
  que falle y solo si el service worker se registra igual (T-F703).
- **Qué se sustituye y qué no**:

  | Se sustituye | Nunca se sustituye |
  |---|---|
  | La red (MSW) en tests de pantalla | Hooks de datos, `QueryClient`, router, React Hook Form |
  | El reloj (`vi.useFakeTimers`) solo donde hay horas (bloqueo, vencimientos) | El backend en E2E |
  | El adaptador de canvas del logo (`decodeImage`, `encodeBitmap`) en jsdom | Las funciones puras de preparación del logo (firma, dimensiones, plan, escalera) |
  | — | Componentes de shadcn/Radix (se prueban a través de su accesibilidad) |
  | — | El certificado y la cookie en E2E (HTTPS real con la CA de mkcert) |

- **Checkpoint**: `npm run check` (lint + typecheck + tests + build; no necesita certificados).
  E2E: `npm run e2e` (necesita Docker y los certificados de `make dev-certs`), en CI en cada PR a
  `main`.

## Fundamento

- Vitest comparte la configuración de Vite (alias, transformaciones): un solo pipeline, arranque
  rápido.
- Testing Library empuja a escribir componentes accesibles: si un test no encuentra un botón por
  su nombre, un lector de pantalla tampoco.
- MSW en el borde prueba la integración completa del cliente (serialización, errores
  problem+json, cabecera `Retry-After`, invalidaciones) con el mismo código que corre en
  producción.
- Separar las decisiones sobre imágenes (puras) del uso del canvas (adaptador) deja la mayor parte
  de la lógica del logo cubierta por tests rápidos, y lo que depende del navegador probado donde
  existe.
- Playwright cubre lo que jsdom no puede (service worker, CSP, cookies `__Host-`, cabeceras,
  canvas) y soporta Chromium y WebKit (Safari de iOS).
- Correr los E2E sobre HTTPS con un certificado de confianza es la única forma de que la cookie
  `__Host-crm_session` exista en Chromium (H-10) y de que el E2E pruebe exactamente la cookie de
  producción.

## Alternativas consideradas

- **Jest**: el estándar histórico, pero requiere configurar transformaciones de TypeScript/ESM
  que Vitest ya trae con Vite.
- **happy-dom** en vez de jsdom: más rápido, menos fiel; se usa solo si jsdom da problemas con
  MSW/openapi-fetch (supuesto S-F2).
- **Vitest en modo navegador**: más fiel que jsdom (tendría canvas real), pero más lento y más
  piezas para arrancar; se reevalúa si jsdom se queda corto o si crece el código de imágenes.
- **Paquete `canvas` de Node para jsdom**: daría canvas en Vitest, pero es una dependencia nativa
  (compilación en CI) y no reproduce el comportamiento de cada navegador (orientación EXIF,
  codificadores), que es justo lo que importa.
- **Mockear los hooks de datos o el cliente**: tests rápidos que no prueban la integración real
  (el error típico está en el mapeo de la respuesta o en la invalidación).
- **Cypress**: bueno, pero sin WebKit y con un modelo de ejecución propio; Playwright es más
  liviano en CI.
- **Tests de snapshot**: fallan por cambios irrelevantes y no dicen qué comportamiento se rompió.
- **E2E por HTTP plano**: sin certificados, pero Chromium no guarda la cookie `__Host-` en
  `http://localhost` (H-10) y el backend ya no acepta `APP_BASE_URL` con `http://`.
- **`ignoreHTTPSErrors` siempre** en lugar de instalar la CA: menos pasos, pero con errores de
  certificado el service worker podría no registrarse y el E2E dejaría de ver lo que ve un usuario;
  queda solo como respaldo de S-12.

## Consecuencias

- Ganás: tests que describen comportamiento visible y sobreviven a refactors; accesibilidad
  verificada en cada pantalla; E2E sobre el mismo binario que se despliega, con la misma cookie que
  producción.
- Aceptás: mantener handlers de MSW alineados con el contrato (tipados con los tipos generados);
  los E2E necesitan Docker, la CA de mkcert en el runner y tardan minutos; el adaptador de canvas
  solo se prueba en E2E; axe detecta una parte de los problemas de accesibilidad, el resto se
  verifica a mano (checklist de `tasks.md`).
