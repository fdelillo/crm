# ADR-022: Estrategia de tests del frontend (Vitest + Testing Library + MSW, Playwright para E2E)

**Status**: Proposed
**Fecha**: 2026-09-29
**Origen**: spec 001 (`tasks.md` §Frontend); complementa ADR-012 (backend)
**Revisión 2026-09-29**: frecuencia de E2E confirmada por el usuario (P-F5); se agrega el adaptador de
canvas de la preparación del logo a lo que se sustituye en jsdom; los E2E de Chromium dependen del
hallazgo H-10 de `ui.md` (cookie `__Host-` en `http://localhost`).

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
- **End-to-end**: **Playwright** contra el binario real (SPA embebida) con `docker compose`
  (PostgreSQL, Mailpit, MinIO); los tokens se leen de los emails vía la API HTTP de Mailpit. Pocos
  flujos críticos (registro, login/logout, invitación, reset, sesión revocada, logo desde una foto)
  + **axe** (`@axe-core/playwright`) en cada pantalla + verificación de PWA, CSP y viewport de
  320 px. **Chromium en cada PR; WebKit antes de liberar** (confirmado por el usuario, P-F5).
  Requisito: que Chromium pueda guardar la cookie de sesión en el entorno de E2E (hallazgo H-10:
  Chrome rechaza cookies `__Host-` en `http://localhost`; el backend define si el E2E corre con TLS
  local o con otra cookie de desarrollo).
- **Qué se sustituye y qué no**:

  | Se sustituye | Nunca se sustituye |
  |---|---|
  | La red (MSW) en tests de pantalla | Hooks de datos, `QueryClient`, router, React Hook Form |
  | El reloj (`vi.useFakeTimers`) solo donde hay horas (bloqueo, vencimientos) | El backend en E2E |
  | El adaptador de canvas del logo (`decodeImage`, `encodeBitmap`) en jsdom | Las funciones puras de preparación del logo (firma, dimensiones, plan, escalera) |
  | — | Componentes de shadcn/Radix (se prueban a través de su accesibilidad) |

- **Checkpoint**: `npm run check` (lint + typecheck + tests + build). E2E: `npm run e2e` (necesita
  Docker), en CI en cada PR a `main`.

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

## Consecuencias

- Ganás: tests que describen comportamiento visible y sobreviven a refactors; accesibilidad
  verificada en cada pantalla; E2E sobre el mismo binario que se despliega.
- Aceptás: mantener handlers de MSW alineados con el contrato (tipados con los tipos generados);
  los E2E necesitan Docker y tardan minutos; el adaptador de canvas solo se prueba en E2E; axe
  detecta una parte de los problemas de accesibilidad, el resto se verifica a mano (checklist de
  `tasks.md`).
