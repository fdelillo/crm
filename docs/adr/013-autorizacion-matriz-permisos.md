# ADR-013: Autorización con una matriz estática de permisos por rol

**Status**: Accepted (aprobado por el usuario con el plan de 001, 2026-09-29)
**Fecha**: 2026-09-27
**Origen**: spec 001 (FR-007); lo usan todas las specs

## Contexto

FR-007 define qué puede hacer cada rol (Administrador, Operador) en 15 áreas funcionales. En 001
solo se aplica "Configuración y usuarios", pero cada spec siguiente va a proteger sus endpoints
con esta matriz. Los roles no son configurables en el MVP (visión, "Fuera del MVP"). La spec pide
`403` para un Operador en un recurso restringido y `404` (no `403`) para un recurso de otra
empresa.

## Decisión

- Paquete `internal/authz` con:
  - `Role` (`admin`, `operator`) y `Permission`: **un permiso por fila de FR-007**
    (`customers.manage`, `projects.manage`, `quotes.manage`, `customer_receipts.create`,
    `balances.view`, `project_costs.view`, `suppliers.view_contact`, `party_finances.manage`,
    `money_accounts.manage`, `financial_operations.manage`, `checks.manage`,
    `bank_statements.import`, `reports.view`, `settings.manage`, `movements.void`).
  - Una tabla estática `rol → conjunto de permisos` y `Can(role, permission) bool`.
  - `Principal{TenantID, UserID, SessionID, Role}` en el `context`, puesto por el middleware de
    sesión.
  - Middleware `RequirePermission(p)` que se aplica **por grupo de rutas** en chi.
- La decisión de permiso se toma **antes** de buscar el recurso y no depende de su existencia:
  un Operador recibe `403` en `/users/{id}` sea cual sea el id. Un Administrador que pide un id de
  otra empresa recibe `404` (la RLS no encuentra la fila).
- Reglas más finas que un permiso (p. ej. en 004 "el autor o un Administrador borra un adjunto")
  se implementan en el servicio del módulo, **además** del permiso de ruta, y se documentan en su
  plan.
- `/me` devuelve la lista de permisos del rol para que la UI oculte lo que no corresponde; eso es
  UX: la autorización real siempre la hace el servidor.

## Fundamento

- Trazabilidad 1:1 con la spec: un test de tabla compara la matriz del código con FR-007 y con el
  enum del contrato.
- Declarar el permiso en el grupo de rutas lo hace visible y verificable (el test de permisos por
  endpoint recorre todas las rutas).
- Decidir antes de buscar evita que el status revele si un recurso existe.

## Alternativas consideradas

- **`if role == admin` dentro de cada handler**: rápido de escribir, imposible de auditar, fácil
  de olvidar en un endpoint nuevo.
- **Motor de políticas (Casbin, OPA)**: flexible, pero desproporcionado para dos roles fijos y
  15 permisos.
- **Permisos en la base, editables**: los roles configurables están fuera del MVP.
- **Permisos por endpoint en lugar de por área funcional**: más granular de lo que pide la spec;
  la matriz dejaría de leerse como FR-007.

## Consecuencias

- Ganás: una matriz auditable, un único middleware, tests que cubren todas las rutas.
- Aceptás: cambiar la matriz exige cambiar código, contrato y spec (FR-007) juntos; roles
  configurables en el futuro requerirán un ADR nuevo que reemplace a este.
