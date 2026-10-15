# ADR-017: Router con React Router v8 en modo SPA (librería), variante *data*

**Status**: Accepted (React Router en modo SPA/librería: decisión del usuario; versión v8 desde el 2026-10-15: decisión del usuario, ver nota) · variante *data* sin loaders ni actions: Proposed
**Fecha**: 2026-09-29
**Origen**: spec 001 (`ui.md` §5, §6)

> **Nota 2026-10-15 (versión mayor; decisión del usuario; no cambia la decisión)**: al arrancar la
> Fase F0, la última estable de React Router es **8.4.0** (v8.0.0 salió el 2026-06-17; v7 sigue
> recibiendo parches) y el usuario decidió usar **v8** en lugar de v7. El título pasó de "v7" a
> "v8"; donde el resto de este ADR dice "v7" (Contexto y primera viñeta de la Decisión) se lee
> "v8", y el texto original se conserva abajo sin editar. Es la misma librería, el mismo paquete
> (`react-router`) y el mismo modo *data*: `createBrowserRouter`, `lazy`, `ErrorBoundary`,
> `useBlocker`, `<ScrollRestoration />` y `createMemoryRouter` siguen en v8 con la misma forma de
> uso. Lo que cambia en v8 y cómo toca a este diseño:
>
> 1. **Pisos**: `react`/`react-dom` ≥ 19.2.7 y Node ≥ 22.22.0 (`engines` del paquete). El stack
>    de F0 (React 19.3.1, Node 24 LTS) cumple.
> 2. **Solo ESM** (`"type": "module"`) y `target`/`lib` ES2022. `web/` ya es un proyecto ESM
>    (Vite); `tsconfig.app.json` usa `lib` ES2022 o posterior. Que Vitest con jsdom navegue con el
>    router lo verifica el test de humo de T-F006.
> 3. **`react-router-dom` ya no existe**: todo se importa de `react-router`, salvo `RouterProvider`,
>    que se importa de `react-router/dom` (conecta `flushSync` de React DOM). Vale para `main.tsx`
>    y para los tests con `createMemoryRouter`.
> 4. **Middleware siempre activo**: el `context` de `loader`/`action`/`middleware` es siempre un
>    `RouterContextProvider` y desaparecen los *future flags* (`v8_middleware` era el único que
>    aplicaba al modo *data*). Sin efecto acá: no hay `loader`, `action` ni `middleware`, y los
>    routers se crean sin `future` ni `getContext`. **Los guards tampoco pasan a `middleware`**
>    (detalle `Proposed`, como el resto de la variante), por la misma razón que no se usan
>    loaders: la sesión vive en la caché de TanStack Query (`['session']`, ADR-018) y los guards de
>    layout la leen con su estado de carga; un middleware de autenticación sería un segundo lugar
>    que decide la redirección. La revisión de código lo señala igual que un loader.
> 5. **Sin efecto en este diseño**: se elimina `hasErrorBoundary` de los objetos de ruta (acá se
>    usa `ErrorBoundary`); el campo `data` de `meta`/`useMatches` y los adaptadores de servidor
>    son de modo framework; 8.3.0 deja de codificar `$ & + , ; = : @` en `href`/`generatePath` (el
>    diseño no los usa); 8.4.0 reorganiza contextos internos `UNSAFE_` (no se usan). 8.3.1 corrige
>    que `<ScrollRestoration>` dejara `history.scrollRestoration` en `auto` tras volver desde el
>    *bfcache*.
>
> Alternativa descartada: quedarse en 7.18.x, que sigue con parches. Contradice la regla de
> versiones de F0 (`tasks.md`, Frontend, convenciones) sin una incompatibilidad que lo justifique,
> y obliga a migrar después con código ya escrito. Trade-off: una versión mayor con cuatro meses
> en el mercado, con menos ejemplos que v7; para este diseño solo cambian los imports y la
> configuración de arriba. Fuentes: <https://reactrouter.com/upgrading/v7>,
> <https://reactrouter.com/changelog> (v8.0.0 a v8.4.0),
> <https://reactrouter.com/api/data-routers/RouterProvider>.

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
