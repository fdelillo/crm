# Registro de decisiones de arquitectura (ADR)

Decisiones estructurales del proyecto. Formato: Contexto, Decisión, Fundamento, Alternativas
consideradas, Consecuencias.

Vistas relacionadas: [arquitectura general](../diagramas/02-arquitectura.html),
[fronteras entre módulos](../diagramas/04-fronteras.html),
[aislamiento entre empresas](../diagramas/06-aislamiento.html) y
[matriz de permisos](../diagramas/07-permisos.html). Cada vista incluye sus fuentes y revisión.

Flujos implementados: [registro](../diagramas/11-registro.html),
[autenticación](../diagramas/12-autenticacion.html),
[usuarios e invitaciones](../diagramas/13-usuarios.html),
[recuperación](../diagramas/14-recuperacion.html),
[outbox](../diagramas/15-outbox.html) y [logos](../diagramas/16-archivos.html).

Desarrollo y operación: [organización del backend](../diagramas/17-backend.html),
[contratos](../diagramas/18-contratos.html), [integración continua](../diagramas/19-ci.html),
[pruebas](../diagramas/20-pruebas.html), [operación y recuperación](../diagramas/21-operacion.html),
[observabilidad](../diagramas/22-observabilidad.html) y
[trazabilidad](../diagramas/23-trazabilidad.html). El
[frontend previsto](../diagramas/24-frontend.html) se presenta como diseño especificado.

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
| [007](007-hashing-contrasenas-argon2id.md) | Hashing de contraseñas con argon2id | Accepted (nota 2026-10-12 sobre el orden semáforo → conexión: Accepted) | 001 |
| [008](008-identificadores-uuidv7.md) | Identificadores UUIDv7 | Accepted | 001 |
| [009](009-formato-errores-rfc9457.md) | Errores HTTP RFC 9457 con código estable | Accepted | 001 |
| [010](010-email-outbox-transaccional.md) | Emails con outbox transaccional, worker en el binario y puerto `Mailer` SMTP | Accepted (la clasificación de errores de "Reintentos" la reemplaza ADR-024; nota 2026-10-01) | 001 |
| [011](011-almacenamiento-archivos-s3.md) | Almacenamiento compatible con S3, subida vía backend | Accepted (nota 2026-10-01 sobre la imagen de MinIO: Accepted) | 001 |
| [012](012-estrategia-de-tests.md) | Estrategia de tests (TDD, PostgreSQL real, contrato, aislamiento) | Accepted | 001 |
| [013](013-autorizacion-matriz-permisos.md) | Autorización con matriz estática de permisos por rol | Accepted | 001 |
| [014](014-contrato-openapi-canonico.md) | Contrato OpenAPI 3.1 canónico, handlers a mano, validación en tests | Accepted (nota 2026-10-02 sobre cómo se aplica la validación: Accepted, aprobada por el usuario el 2026-10-02) | 001 |
| [015](015-frontend-stack-y-estructura.md) | Frontend: Vite + React + TypeScript en `web/`, carpetas por feature | Proposed | 001 (frontend) |
| [016](016-componentes-shadcn-tailwind.md) | Componentes con shadcn/ui (Radix) y estilos con Tailwind CSS v4 | Accepted (decisión del usuario; detalles Proposed) | 001 (frontend) |
| [017](017-router-react-router-spa.md) | Router con React Router v8 en modo SPA, variante *data* | Accepted (decisión del usuario; detalles Proposed; v8 desde el 2026-10-15, decisión del usuario: nota 2026-10-15) | 001 (frontend) |
| [018](018-server-state-tanstack-query-openapi.md) | Server state con TanStack Query, cliente openapi-fetch y tipos con openapi-typescript | Accepted (decisión del usuario; detalles Proposed) | 001 (frontend) |
| [019](019-distribucion-spa-embebida-go.md) | SPA embebida en el binario Go, mismo origen que la API | Accepted (decisión del usuario; detalles aceptados el 2026-09-29, ver nota) | 001 (frontend) |
| [020](020-pwa-service-worker-minimo.md) | PWA instalable con service worker mínimo, sin offline | Proposed | 001 (frontend) |
| [021](021-formularios-react-hook-form-zod.md) | Formularios con React Hook Form y Zod atado al contrato | Proposed | 001 (frontend) |
| [022](022-estrategia-de-tests-frontend.md) | Estrategia de tests del frontend (Vitest, Testing Library, MSW, Playwright) | Proposed | 001 (frontend) |
| [023](023-idioma-formato-fechas-dinero-frontend.md) | Idioma, formato de fechas y dinero en el frontend (es-AR, `Intl`, sin punto flotante) | Proposed | 001 (frontend) |
| [024](024-clasificacion-fallos-outbox-y-presupuesto-smtp.md) | Clasificación de fallos de entrega del outbox, presupuesto de tiempo del envío SMTP y aislamiento de fallos por mensaje | Accepted (ADR-025, *Accepted*, reemplaza el último párrafo de §2 y partes de §3; nota 2026-10-01 (b)) | 001 (revisión del PR #8) |
| [025](025-saneamiento-last-error-y-codigo-extendido.md) | `last_error` sin texto del proveedor después del contenido, redacción de secuencias tipo secreto y código extendido leído del texto en todas las fases | Accepted | 001 (tercera revisión del PR #8) |
| [026](026-vulnerabilidades-dependencias-govulncheck.md) | Vulnerabilidades conocidas en dependencias Go con govulncheck (bloqueante en PR y `main`, corrida diaria) | Accepted | 001 (auditoría de `b945373`, unidad bloqueada) |
| [027](027-vulnerabilidades-dependencias-npm.md) | Vulnerabilidades conocidas en dependencias npm del frontend con `npm audit` (producción desde `low`, árbol completo solo `critical`) | Accepted (decisión del usuario, 2026-10-16) | 001 (frontend, cierre de F0) |

Los ADR 001–014 quedaron `Accepted` el 2026-09-29, al aprobarse el plan de la spec 001.

Los ADR 015–023 (frontend) se escribieron el 2026-09-29 junto con `specs/001-empresas-usuarios/ui.md`.
Los marcados `Accepted` registran decisiones del usuario; sus detalles y los ADR `Proposed` quedan
pendientes de aprobación.

ADR-024 (2026-10-01) reemplaza solo la clasificación de errores de la viñeta "Reintentos" de
ADR-010; el resto de ADR-010 sigue vigente, por eso ADR-010 no pasa a `Superseded`.

ADR-025 (2026-10-01, `Accepted`) reemplaza solo tres piezas de ADR-024 (de dónde sale el código
extendido, el detalle de las respuestas de la fase `data` y la regla de redacción de `last_error`).
ADR-024 sigue `Accepted` en todo lo demás, con la misma forma de nota que se usó en
ADR-010; no pasa a `Superseded`.

La nota 2026-10-02 de ADR-014 (`Accepted`, aprobada por el usuario el 2026-10-02) precisa cómo se
aplica la verificación de contrato (request validado solo en respuestas `2xx`; `404`/`405` globales
contra `Problem`) sin cambiar la decisión.

ADR-026 (2026-10-14, `Accepted`, aprobada por el usuario el 2026-10-14) cierra la unidad bloqueada
de la auditoría de `b945373`. Aplica a todo el módulo Go; el escaneo de las dependencias del
frontend lo define T-F009.

ADR-027 (2026-10-15; aprobado por el usuario el 2026-10-16) define ese escaneo: `npm audit` en dos
pasadas (producción desde `low`, el árbol completo solo `critical`), en el mismo workflow `vulncheck`
y con el mismo registro de excepciones que ADR-026. ADR-026 no cambia.

La nota 2026-10-15 de ADR-017 (decisión del usuario) pasa React Router de v7 a v8 sin cambiar la
decisión: misma librería, mismo modo *data*, sin loaders, actions ni middleware.
