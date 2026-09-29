# ADR-012: Estrategia de tests (TDD, PostgreSQL real, contrato y aislamiento)

**Status**: Accepted (aprobado por el usuario con el plan de 001, 2026-09-29)
**Fecha**: 2026-09-27
**Origen**: spec 001; aplica a todas las specs

> **Nota 2026-09-29 (detalle; no cambia la decisión)**, por el hallazgo H-10: la cookie de sesión
> es siempre `__Host-crm_session` con `Secure` y ya no existe `COOKIE_SECURE` (nota (b) en
> ADR-006). Por eso los tests **HTTP** que encadenan requests con la cookie (registro → `/me`,
> login → logout) corren sobre `httptest.NewTLSServer` con el handler raíz real, usando
> `srv.Client()` (ya confía en el certificado de prueba) con un `cookiejar`; el helper vive en
> `internal/testsupport/apitest`. Los tests de un solo request siguen con `httptest.NewRecorder`.
> Ninguno necesita mkcert, así que `make check` no cambia. Como el `cookiejar` de Go no verifica
> el prefijo `__Host-`, los atributos de la cookie se afirman explícitamente sobre `Set-Cookie`.
> El certificado de mkcert solo lo usan el desarrollo con navegador y el job de E2E (DD-24 del
> plan de 001). No se crea un ADR nuevo: la pirámide, lo que se sustituye y el checkpoint no
> cambian; se fija cómo se levanta el servidor en los tests HTTP.

## Contexto

La constitución pide tests antes que código en el núcleo financiero (principio VI), tests de
contrato para OpenAPI y tests automáticos de aislamiento entre empresas (principio III). Buena
parte de las garantías vive **en la base** (RLS, roles, constraints, FKs compuestas): un test que
sustituye la base no prueba lo que más importa.

## Decisión

- **Framework**: paquete `testing` nativo, tests *table-driven*, subtests con `t.Run`. Sin
  testify. `github.com/google/go-cmp` permitido para comparar estructuras.
- **Pirámide**:
  - **Unitarios** (mayoría): lógica pura — política de bloqueo, transiciones de estado, backoff,
    validación de CUIT, matriz de permisos, mapeo de errores. Sin Docker. `make test`.
  - **Integración** (bordes reales): servicios contra **PostgreSQL 18 real** levantado con
    `testcontainers-go` (módulo postgres); adaptadores contra Mailpit y MinIO en contenedores.
    Build tag `integration`. `make test-int`.
  - **HTTP**: `httptest` sobre el router real con la base real; `Mailer` y `ObjectStorage`
    sustituidos por fakes. Cada request y response se valida contra el contrato con
    `github.com/pb33f/libopenapi-validator`.
  - **End-to-end**: pocos, manuales o scriptados contra `docker compose` (prueba independiente de
    cada fase).
- **Qué se sustituye**: email, almacenamiento de objetos y reloj (`clock.Clock`). **Qué no**:
  PostgreSQL, el router, los middlewares, y cada adaptador en su propio test.
- **Harness** (`internal/testsupport/pgtest`): un contenedor por paquete de tests, bootstrap de
  roles, migraciones como `crm_owner` y pool como **`crm_app`**. Nunca se prueba comportamiento
  con un superusuario (ignora RLS).
- **Datos**: cada test crea sus empresas con UUID y emails aleatorios; los tests corren en
  paralelo sin limpiar tablas.
- **Tests de invariantes que protegen a las specs futuras** (se escriben en 001, cubren todo lo
  que venga):
  - catálogo: RLS forzada, políticas, dueños, privilegios, `SECURITY DEFINER`, vistas;
  - FKs compuestas con `tenant_id`;
  - privilegios por columna de los roles de sistema;
  - aislamiento en base por tabla (descubiertas del catálogo);
  - aislamiento HTTP con **cobertura de rutas**: el test falla si existe una ruta sin caso;
  - reglas de repositorio (sin `SET ROLE` ni SQL armado fuera de `platform/db`).
- **Tiempo**: las queries que comparan vencimientos reciben `now` desde `clock.Clock`, para
  controlar el tiempo sin dormir.
- **Checkpoint** de cada fase: `make check` (lint + unitarios + integración), en CI con Docker.
  Un test de integración nunca se saltea en silencio si falta Docker: falla.

## Fundamento

- Solo una base real prueba RLS, roles, constraints y concurrencia (`FOR UPDATE`, `SKIP LOCKED`).
- Un contenedor por corrida aísla los roles, que son objetos del clúster.
- Los tests genéricos sobre el catálogo y sobre `chi.Walk` escalan solos: una tabla o una ruta
  nueva queda cubierta o hace fallar el test.
- Validar contra el contrato en cada test HTTP convierte el OpenAPI en una especificación
  ejecutable (principio VI).

## Alternativas consideradas

- **Mocks del repositorio**: rápidos, pero no prueban RLS ni constraints; se usan solo para
  dependencias externas.
- **Base compartida de desarrollo**: estado compartido y roles de corridas anteriores.
- **SQLite en memoria**: no tiene RLS ni roles.
- **testify**: no aporta lo suficiente frente a `testing` + `go-cmp` y agrega un estilo que el
  equipo tendría que aprender.
- **kin-openapi** para validar el contrato: alternativa válida; se prefirió libopenapi-validator
  por su soporte declarado de OpenAPI 3.1.

## Consecuencias

- Ganás: tests que prueban lo que importa (aislamiento, atomicidad) y que protegen a las specs
  futuras.
- Aceptás: Docker obligatorio para `make check`; la suite de integración tarda más que una de
  mocks (se paraleliza por paquete y por test).
