# ADR-018: Server state con TanStack Query, cliente openapi-fetch y tipos generados con openapi-typescript

**Status**: Accepted (TanStack Query + openapi-fetch + openapi-typescript: decisión del usuario) · detalles (generación por spec con intersección, sin store global, defaults de caché, manejo global de 401): Proposed
**Fecha**: 2026-09-29
**Origen**: spec 001 (`ui.md` §10, §11, §12); implementa el paso de *bundle* de DD-17 y ADR-014

## Contexto

Casi todo el estado de la app es del servidor: sesión, empresa, usuarios. Varias pantallas lo
comparten, hay que invalidarlo tras cada cambio y un `401` en cualquier request significa "sesión
vencida". El contrato OpenAPI 3.1 es canónico (ADR-014) y los tipos del cliente deben derivarse de
él. Desde 002, cada spec tendrá su contrato y referenciará componentes de 001 con `$ref` externo
(DD-17), así que hace falta un paso para generar tipos de todas las specs.

## Decisión

- **TanStack Query v5** es el único lugar donde vive el server state. **No hay store global**
  (ni Redux, ni Zustand, ni Context de datos): lo que varias ramas necesitan (sesión, permisos,
  empresa) ya lo comparte la caché. Estado local con `useState`; formularios con React Hook Form.
- **openapi-fetch** como cliente HTTP (`createClient<paths>`, `baseUrl` = origen + `/api/v1`,
  `credentials` del mismo origen). Todas las funciones de query/mutación pasan por `unwrap()`, que
  devuelve los datos o lanza un `ApiError` normalizado (`status`, `code`, `fieldErrors`,
  `suggestedAction`, `retryAfterSeconds`, `requestId`).
- **openapi-typescript** genera los tipos: **un archivo por spec** (`src/api/generated/NNN.ts`)
  configurado en `web/redocly.yaml`, y `src/api/schema.ts` los combina por intersección
  (`paths = Paths001 & Paths002 & …`). Los archivos generados se versionan y `npm run lint` falla
  si están desactualizados. Si `openapi-typescript` no resolviera los `$ref` externos, se agrega un
  `redocly bundle` previo por spec (se valida en T-F004).
- Alias legibles en `src/api/types.ts` (`type User = components['schemas']['User']`); **ningún tipo
  de API se escribe a mano**.
- Defaults: `staleTime` 30 s; reintentos de queries solo ante errores de red y `503` (máx. 2);
  mutaciones sin reintento y con `networkMode: 'always'`; `refetchOnWindowFocus` activo.
- **Manejo global** en `QueryCache.onError`/`MutationCache.onError`: `401 unauthenticated` →
  `queryClient.clear()` y login con `next`; `403 forbidden` → refrescar la sesión. La query de
  sesión mapea su `401` a `null` (anónimo). El callback se inyecta al crear el cliente.
- Claves de caché en un único módulo (`src/api/queryKeys.ts`); cada mutación declara qué
  actualiza o invalida (tabla en `ui.md` §10.3).
- Devtools de TanStack Query solo en desarrollo.

## Fundamento

- La librería resuelve caché compartida, deduplicación, estados de carga, reintentos,
  invalidación y pausa sin conexión; hecho a mano con `useEffect` se reescribe peor en cada
  pantalla.
- openapi-fetch no genera código: usa los tipos de `paths` directamente, así que una ruta o un
  campo inexistente es un error de compilación, con un cliente de ~pocos KB.
- Un archivo por spec + intersección evita el paso de `join` (que tiene que resolver conflictos de
  nombres de componentes entre contratos) y deja diffs de tipos revisables por spec.
- Un único manejador de `401` garantiza el mismo comportamiento en toda la app (ADR-006 pide que
  un `401` en cualquier request vuelva al login).

## Alternativas consideradas

- **`fetch` en `useEffect` + estado propio**: sin dependencias, pero cada pantalla reimplementa
  carga, error, caché e invalidación; fuente clásica de datos desincronizados.
- **SWR**: más simple, pero con menos control de mutaciones e invalidación por clave.
- **RTK Query / Redux Toolkit**: potente, pero trae Redux y un store global que 001 no necesita.
- **`openapi-react-query`** (hooks generados sobre openapi-fetch): menos código repetido, pero las
  claves de caché las arma la librería (menos explícitas para invalidar) y es una abstracción más.
- **Generadores de clientes y hooks (orval, hey-api, openapi-generator)**: mucho código generado
  que el equipo tendría que leer y entender; más lejos de "la firma es el contrato".
- **Redocly `join`** en un único contrato antes de generar: un solo archivo, pero con conflictos
  de nombres de componentes y `tags` entre specs a resolver en cada una.
- **Tipos escritos a mano**: se desincronizan del contrato; violan ADR-014.
- **Zustand para sesión**: la sesión es server state (`/me`); copiarla a un store obliga a
  sincronizarla a mano.

## Consecuencias

- Ganás: tipos del contrato de punta a punta, un solo lugar para cada dato del servidor,
  comportamiento uniforme ante sesión vencida, nada que sincronizar a mano.
- Aceptás: aprender el modelo de TanStack Query (claves, `staleTime`, invalidación); regenerar
  tipos con cada cambio de contrato (verificado por lint); una ruta repetida en dos specs se
  mezclaría en la intersección (lo detecta un test de tipos que exige claves disjuntas, T-F004).
