# ADR-002: Router HTTP con chi

**Status**: Accepted (decisión del usuario)
**Fecha**: 2026-09-27
**Origen**: spec 001

## Contexto

La API REST necesita rutas con parámetros, middlewares globales (logging, recover, CSRF, headers
de seguridad) y, sobre todo, **middlewares por grupo de rutas** para aplicar la matriz de
permisos de FR-007 (`settings.manage`, luego `money_accounts.manage`, etc.). También hace falta
poder listar todas las rutas registradas para que el test de aislamiento verifique que ningún
endpoint queda sin cubrir (SC-002).

## Decisión

Usar `github.com/go-chi/chi/v5` como router. Handlers y middlewares con las firmas estándar de
`net/http` (`http.Handler`, `func(http.Handler) http.Handler`). La API se monta bajo `/api/v1`;
cada módulo expone una función que registra sus rutas en un `chi.Router`, y los permisos se
aplican con `r.Group` + `authz.RequirePermission(...)`.

## Fundamento

- chi es 100 % compatible con `net/http`: cualquier middleware estándar funciona, incluido
  `http.CrossOriginProtection` de la librería estándar (ADR-006).
- Grupos y subrouters con middlewares propios: la autorización queda declarada en un solo lugar
  por grupo de rutas, no repetida en cada handler.
- `chi.Walk` recorre todas las rutas registradas: base del test de cobertura de aislamiento
  (tarea T-B801).
- El patrón de ruta (`/users/{userId}/role`) está disponible para loguear sin la URL cruda (sin
  ids en las métricas).
- Librería chica, madura y sin dependencias transitivas.

## Alternativas consideradas

- **`net/http.ServeMux` (Go ≥ 1.22)**: soporta métodos y comodines, pero no tiene grupos con
  middlewares ni un mecanismo para listar rutas; habría que construir ambos. Descartada por
  eso, no por calidad.
- **Gin / Echo**: tipos de contexto propios que acoplan todos los handlers al framework;
  validación y *binding* mágicos que ocultan comportamiento a un equipo que está aprendiendo.
- **Fiber**: no usa `net/http` (usa fasthttp); incompatible con middlewares estándar.

## Consecuencias

- Ganás: autorización por grupo de rutas, listado de rutas para tests, cero acoplamiento más
  allá de `chi.URLParam` y el armado del router.
- Aceptás: una dependencia externa que la librería estándar casi cubre. Si en el futuro
  `ServeMux` suma grupos y listado de rutas, migrar es mecánico porque los handlers ya son
  `net/http` puro.
