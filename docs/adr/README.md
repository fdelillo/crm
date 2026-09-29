# Registro de decisiones de arquitectura (ADR)

Decisiones estructurales del proyecto. Formato: Contexto, Decisión, Fundamento, Alternativas
consideradas, Consecuencias.

Reglas:

- Numeración secuencial compartida por backend y frontend (`NNN-slug.md`); un número nunca se
  reutiliza.
- Un ADR **aceptado no se edita ni se borra**. Si la decisión cambia, se escribe un ADR nuevo que
  lo reemplaza y el anterior pasa a `Superseded by ADR-NNN`.
- Las decisiones locales a una feature van como `DD-n` en el `plan.md` de su spec.
- Estados: `Proposed` (pendiente de aprobación del usuario), `Accepted`, `Superseded by ADR-NNN`.

## Índice

| ADR | Título | Estado | Origen |
|---|---|---|---|
| [001](001-monolito-modular.md) | Estructura del monolito modular (layout, módulos por dominio, fronteras) | Accepted | 001 |
| [002](002-router-http-chi.md) | Router HTTP con chi | Accepted | 001 |
| [003](003-acceso-a-datos-sqlc-pgx.md) | Acceso a datos con sqlc + pgx v5, sin ORM | Accepted | 001 |
| [004](004-migraciones-goose-y-bootstrap.md) | Migraciones con goose embebidas y bootstrap de roles fuera de goose | Accepted | 001 |
| [005](005-aislamiento-tenant-rol-por-empresa-rls.md) | Aislamiento con un rol de PostgreSQL por empresa y RLS forzada | Accepted | 001 |
| [006](006-autenticacion-sesiones-csrf.md) | Sesiones en PostgreSQL, cookie `__Host-` y protección CSRF | Accepted | 001 |
| [007](007-hashing-contrasenas-argon2id.md) | Hashing de contraseñas con argon2id | Accepted | 001 |
| [008](008-identificadores-uuidv7.md) | Identificadores UUIDv7 | Accepted | 001 |
| [009](009-formato-errores-rfc9457.md) | Errores HTTP RFC 9457 con código estable | Accepted | 001 |
| [010](010-email-outbox-transaccional.md) | Emails con outbox transaccional, worker en el binario y puerto `Mailer` SMTP | Accepted | 001 |
| [011](011-almacenamiento-archivos-s3.md) | Almacenamiento compatible con S3, subida vía backend | Accepted | 001 |
| [012](012-estrategia-de-tests.md) | Estrategia de tests (TDD, PostgreSQL real, contrato, aislamiento) | Accepted | 001 |
| [013](013-autorizacion-matriz-permisos.md) | Autorización con matriz estática de permisos por rol | Accepted | 001 |
| [014](014-contrato-openapi-canonico.md) | Contrato OpenAPI 3.1 canónico, handlers a mano, validación en tests | Accepted | 001 |
| [015](015-frontend-stack-y-estructura.md) | Frontend: Vite + React + TypeScript en `web/`, carpetas por feature | Proposed | 001 (frontend) |
| [016](016-componentes-shadcn-tailwind.md) | Componentes con shadcn/ui (Radix) y estilos con Tailwind CSS v4 | Accepted (decisión del usuario; detalles Proposed) | 001 (frontend) |
| [017](017-router-react-router-spa.md) | Router con React Router v7 en modo SPA, variante *data* | Accepted (decisión del usuario; detalles Proposed) | 001 (frontend) |
| [018](018-server-state-tanstack-query-openapi.md) | Server state con TanStack Query, cliente openapi-fetch y tipos con openapi-typescript | Accepted (decisión del usuario; detalles Proposed) | 001 (frontend) |
| [019](019-distribucion-spa-embebida-go.md) | SPA embebida en el binario Go, mismo origen que la API | Accepted (decisión del usuario; detalles aceptados el 2026-09-29, ver nota) | 001 (frontend) |
| [020](020-pwa-service-worker-minimo.md) | PWA instalable con service worker mínimo, sin offline | Proposed | 001 (frontend) |
| [021](021-formularios-react-hook-form-zod.md) | Formularios con React Hook Form y Zod atado al contrato | Proposed | 001 (frontend) |
| [022](022-estrategia-de-tests-frontend.md) | Estrategia de tests del frontend (Vitest, Testing Library, MSW, Playwright) | Proposed | 001 (frontend) |
| [023](023-idioma-formato-fechas-dinero-frontend.md) | Idioma, formato de fechas y dinero en el frontend (es-AR, `Intl`, sin punto flotante) | Proposed | 001 (frontend) |

Los ADR 001–014 quedaron `Accepted` el 2026-09-29, al aprobarse el plan de la spec 001.

Los ADR 015–023 (frontend) se escribieron el 2026-09-29 junto con `specs/001-empresas-usuarios/ui.md`.
Los marcados `Accepted` registran decisiones del usuario; sus detalles y los ADR `Proposed` quedan
pendientes de aprobación.
