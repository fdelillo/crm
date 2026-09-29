# ADR-015: Stack base y estructura del frontend (Vite + React + TypeScript, `web/`, carpetas por feature)

**Status**: Proposed
**Fecha**: 2026-09-29
**Origen**: spec 001 (`specs/001-empresas-usuarios/ui.md` §0, §9, §22)

## Contexto

La constitución fija React + TypeScript como PWA mobile-first. El backend es un monolito Go que
sirve la API en `/api/v1` con sesión por cookie en el mismo origen (ADR-006) y va a servir también
la SPA (ADR-019). No hay requisitos de SEO (todo está detrás de un login) ni de renderizado en el
servidor. El usuario tiene nivel básico en React/TypeScript: cada herramienta y cada capa propia es
algo más que aprender. Hay 10 specs en el MVP: la estructura tiene que aguantar decenas de
pantallas sin volverse una pila de carpetas `components/`, `hooks/` y `utils/` globales.

## Decisión

1. **Build con Vite** (última estable, ≥ 8), plantilla React + TypeScript, **SPA sin SSR**. `npm`
   como gestor de paquetes; Node LTS fijado en `engines` y `.nvmrc`.
2. **TypeScript estricto**: `strict`, `noUncheckedIndexedAccess`, alias `@/*` → `src/*` (lo exige
   shadcn). React 19.
3. **Ubicación**: carpeta `web/` en la raíz del mismo repositorio. `web/dist/` es el resultado del
   build; el paquete Go `web` (`web/embed.go`) lo embebe (ADR-019).
4. **Organización por feature con *colocation*** dentro de `web/src/`:
   - `app/`: router, `QueryClient`, layout raíz, shell, guards, error boundary.
   - `api/`: cliente HTTP, errores, mensajes por `code`, claves de caché y tipos **generados**.
   - `features/<dominio>/`: pantallas, componentes, hooks, esquemas y tests de cada dominio, con
     los nombres del glosario (`auth`, `tenant`, `users`, luego `parties`, `projects`…).
   - `components/`: presentación compartida; `components/ui/`: componentes generados por shadcn.
   - `lib/`: funciones puras transversales (formato, CUIT, rutas seguras).
   - Tests junto al archivo que prueban (`LoginPage.test.tsx`).
5. **Fronteras verificadas por lint** (`no-restricted-imports` de ESLint): `components/` no
   importa `api/`, `features/` ni `app/`; una feature no importa otra salvo `features/auth/session`;
   solo `api/` hace requests.
6. Herramientas de calidad: ESLint (typescript-eslint, `react-hooks`, `jsx-a11y`, `no-console`),
   Prettier, `tsc -b` como *typecheck*. Script único de checkpoint: `npm run check`.

## Fundamento

- Sin SEO ni SSR, una SPA es lo más simple de construir, probar y desplegar: el binario Go solo
  tiene que servir archivos estáticos.
- Vite es el estándar actual para SPAs React: servidor de desarrollo rápido, build de producción
  optimizado, *proxy* de desarrollo incorporado (lo necesita ADR-019) y Vitest comparte su
  configuración (ADR-022).
- TypeScript estricto convierte los tipos generados del contrato en documentación ejecutable: el
  error más común del frontend (suponer que un campo llegó) lo marca el editor. Para quien está
  aprendiendo, el editor enseña.
- Mismo repositorio: un cambio de contrato, backend y frontend va en un solo PR y la CI los
  verifica juntos; el contrato vive en `specs/`, a un `../` de distancia.
- Por feature: lo que cambia junto vive junto; agregar una spec es agregar una carpeta. Es el
  mismo criterio que ADR-001 aplica al backend (paquetes por dominio).

## Alternativas consideradas

- **Next.js / React Router en modo framework / Remix**: SSR, rutas por archivos y servidor Node.
  Sin requisito que lo justifique; agregan un runtime de servidor que el binario Go no puede
  embeber como archivos estáticos, y muchos conceptos más (componentes de servidor, loaders).
- **Rsbuild / Parcel / webpack**: funcionan, pero con menos documentación y ejemplos para
  shadcn/Tailwind v4 y sin la integración directa con Vitest.
- **pnpm o yarn**: más rápidos o estrictos, pero `npm` viene con Node y es lo que asume casi toda
  la documentación; la diferencia no importa a esta escala.
- **Repositorio separado para el frontend**: contrato y cambios coordinados en dos repos, dos CI,
  versiones cruzadas; sin beneficio para un equipo chico con un único binario.
- **Frontend dentro de `internal/`**: mezcla un proyecto Node con paquetes Go internos; `web/` es
  la convención más reconocible.
- **Carpetas por tipo técnico** (`components/`, `hooks/`, `services/` globales): cada feature se
  dispersa en cinco carpetas y crecen sin límite; escala mal pasada una decena de pantallas.
- **JavaScript sin tipos**: menos que aprender al principio, pero se pierde la derivación de tipos
  del contrato, que es la garantía principal entre frontend y backend.

## Consecuencias

- Ganás: un proyecto convencional (lo que muestran los tutoriales de Vite + React + shadcn),
  fronteras verificadas por lint, tipos del contrato de punta a punta.
- Aceptás: Node en la máquina de desarrollo y en CI además de Go; el binario de producción
  necesita que el frontend se compile antes (`make build`, ADR-019); TypeScript estricto exige
  manejar `undefined` explícitamente (más código, menos errores).
