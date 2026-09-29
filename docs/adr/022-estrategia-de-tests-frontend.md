# ADR-022: Estrategia de tests del frontend (Vitest + Testing Library + MSW, Playwright para E2E)

**Status**: Proposed
**Fecha**: 2026-09-29
**Origen**: spec 001 (`tasks.md` §Frontend); complementa ADR-012 (backend)

## Contexto

La constitución pide TDD (principio VI) y que los criterios Dado/Cuando/Entonces se traduzcan en
tests. En frontend, el test que vale es el que prueba **lo que el usuario percibe**: textos, roles,
foco, qué pasa al tocar un botón, qué se ve con cada respuesta del servidor. Un test que conoce el
estado interno de un componente se rompe en cada refactor. Además hay cosas que solo se pueden
probar en un navegador real: service worker, CSP, cabeceras, cookie de sesión.

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
- **Funciones puras** (reglas de acciones de usuario, CUIT, `safeNextPath`, formato): tablas de
  casos, igual que en Go.
- **End-to-end**: **Playwright** contra el binario real (SPA embebida) con `docker compose`
  (PostgreSQL, Mailpit, MinIO); los tokens se leen de los emails vía la API HTTP de Mailpit. Pocos
  flujos críticos (registro, login/logout, invitación, reset, sesión revocada) + **axe**
  (`@axe-core/playwright`) en cada pantalla + verificación de PWA, CSP y viewport de 320 px.
  Chromium en cada PR; WebKit antes de liberar.
- **Qué se sustituye y qué no**:

  | Se sustituye | Nunca se sustituye |
  |---|---|
  | La red (MSW) en tests de pantalla | Hooks de datos, `QueryClient`, router, React Hook Form |
  | El reloj (`vi.useFakeTimers`) solo donde hay horas (bloqueo, vencimientos) | El backend en E2E |
  | — | Componentes de shadcn/Radix (se prueban a través de su accesibilidad) |

- **Checkpoint**: `npm run check` (lint + typecheck + tests + build). E2E: `npm run e2e` (necesita
  Docker), en CI en cada PR a `main` (P-F5).

## Fundamento

- Vitest comparte la configuración de Vite (alias, transformaciones): un solo pipeline, arranque
  rápido.
- Testing Library empuja a escribir componentes accesibles: si un test no encuentra un botón por
  su nombre, un lector de pantalla tampoco.
- MSW en el borde prueba la integración completa del cliente (serialización, errores
  problem+json, cabecera `Retry-After`, invalidaciones) con el mismo código que corre en
  producción.
- Playwright cubre lo que jsdom no puede (service worker, CSP, cookies `__Host-`, cabeceras) y
  soporta Chromium y WebKit (Safari de iOS).

## Alternativas consideradas

- **Jest**: el estándar histórico, pero requiere configurar transformaciones de TypeScript/ESM
  que Vitest ya trae con Vite.
- **happy-dom** en vez de jsdom: más rápido, menos fiel; se usa solo si jsdom da problemas con
  MSW/openapi-fetch (supuesto S-F2).
- **Vitest en modo navegador**: más fiel que jsdom, pero más lento y más piezas para arrancar; se
  reevalúa si jsdom se queda corto.
- **Mockear los hooks de datos o el cliente**: tests rápidos que no prueban la integración real
  (el error típico está en el mapeo de la respuesta o en la invalidación).
- **Cypress**: bueno, pero sin WebKit y con un modelo de ejecución propio; Playwright es más
  liviano en CI.
- **Tests de snapshot**: fallan por cambios irrelevantes y no dicen qué comportamiento se rompió.

## Consecuencias

- Ganás: tests que describen comportamiento visible y sobreviven a refactors; accesibilidad
  verificada en cada pantalla; E2E sobre el mismo binario que se despliega.
- Aceptás: mantener handlers de MSW alineados con el contrato (tipados con los tipos generados);
  los E2E necesitan Docker y tardan minutos; axe detecta una parte de los problemas de
  accesibilidad, el resto se verifica a mano (checklist de `tasks.md`).
