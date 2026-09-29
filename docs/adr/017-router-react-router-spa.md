# ADR-017: Router con React Router v7 en modo SPA (librería), variante *data*

**Status**: Accepted (React Router en modo SPA/librería: decisión del usuario) · variante *data* sin loaders ni actions: Proposed
**Fecha**: 2026-09-29
**Origen**: spec 001 (`ui.md` §5, §6)

## Contexto

La SPA necesita rutas públicas (registro, login, enlaces de email), rutas con sesión y rutas que
requieren un permiso, redirección al login recordando el destino, confirmación al salir de un
formulario con cambios, *code splitting* por ruta y un límite de errores. React Router v7 tiene
tres modos: **declarativo** (`<BrowserRouter>`), **data** (`createBrowserRouter` +
`RouterProvider`, suma loaders, actions, `useBlocker`, `ErrorBoundary` y `lazy` por ruta) y
**framework** (compilador, SSR, rutas por archivo). El usuario eligió el uso como librería en una
SPA (sin modo framework).

## Decisión

- **React Router v7, paquete `react-router`, variante *data***: `createBrowserRouter` con un árbol
  de rutas en objetos (`src/app/router.tsx`) y `RouterProvider`.
- **No** se usan `loader` ni `action` para datos del servidor: el server state es de TanStack
  Query (ADR-018). Del modo data se usan solo: `lazy` (división de código por ruta),
  `ErrorBoundary` en la raíz, `useBlocker` (cambios sin guardar) y `<ScrollRestoration />`.
- **Guards como rutas de layout** (`RequireSession`, `PublicOnly`, `RequirePermission`) que leen la
  sesión de la caché y renderizan `<Outlet/>`, `<Navigate>` o el estado "Sin permiso".
- Parámetros de redirección: `/login?next=<ruta interna>&reason=<motivo>`; `next` se valida para
  aceptar solo rutas internas (evita *open redirect*).
- Las pruebas usan el mismo árbol de rutas con `createMemoryRouter`.

## Fundamento

- La variante data trae, sin librerías extra, tres cosas que 001 necesita: `useBlocker`
  (solo existe en data/framework), límites de error por ruta y carga diferida por ruta.
- Dejar los datos en TanStack Query evita tener **dos** cachés (la del router y la de Query) con
  reglas de invalidación distintas; los loaders quedan disponibles si en el futuro hace falta
  precargar.
- Guards como componentes de layout son el patrón más documentado y se prueban renderizando.

## Alternativas consideradas

- **Modo declarativo** (`<BrowserRouter>`): más simple aún, pero sin `useBlocker`, sin
  `ErrorBoundary` por ruta y con *code splitting* manual (`React.lazy` + `Suspense`).
- **Loaders de React Router como capa de datos**: evitaría una librería, pero no trae caché
  compartida entre pantallas, deduplicación, invalidación por clave ni reintentos (lo que resuelve
  TanStack Query); combinarlos duplica cachés.
- **Modo framework**: SSR y compilador; descartado por ADR-015 (SPA estática embebida).
- **TanStack Router**: rutas tipadas de punta a punta y buena integración con Query, pero menos
  difundido y con más conceptos (árbol de rutas generado o tipado a mano); el usuario eligió React
  Router.

## Consecuencias

- Ganás: el router más usado del ecosistema, confirmaciones de salida y *code splitting* sin
  dependencias extra, guards fáciles de probar.
- Aceptás: la configuración vive fuera de JSX (objetos de ruta), algo menos intuitivo al
  principio; hay que recordar **no** usar loaders para datos del servidor (lo señala la revisión
  de código).
