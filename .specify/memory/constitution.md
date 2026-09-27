# Constitución del proyecto CRM

Principios no negociables que rigen todas las specs, planes y código del proyecto.
Cualquier `plan.md` debe verificar su cumplimiento en la sección *Constitution Check*.

## Principios

### I. Simplicidad primero

El sistema es para pequeñas empresas sin formación contable. Cada funcionalidad
debe poder usarse sin capacitación.

- Se construye solo lo que una spec aprobada pide (YAGNI).
- Las operaciones frecuentes (cargar un cliente, un cobro o un gasto) se completan
  en **una sola pantalla**.
- Se usa lenguaje del negocio ("me debe", "le debo") y no jerga contable
  ("debe/haber", "asiento").
- Ante dos diseños válidos, se elige el que tiene menos conceptos para el usuario.

### II. Genérico por configuración, no por código

El primer cliente es una carpintería de aluminio, pero el producto sirve para
cualquier rubro que gestione clientes, proyectos y flujo de caja.

- No existe código específico de un rubro. Las diferencias entre rubros se resuelven
  con **configuración por empresa**: etapas de proyecto, nombres de entidades,
  categorías de ingresos/egresos y catálogo.
- Los rubros se modelan como **plantillas** de configuración inicial
  (ej. "Carpintería de aluminio", "Genérico").

### III. Aislamiento entre empresas (tenants)

- Todo dato de negocio pertenece a exactamente una empresa (`tenant_id`).
- Toda consulta está filtrada por empresa. Como defensa en profundidad se usa
  Row-Level Security de PostgreSQL.
- Existen tests automáticos que verifican que un usuario no puede leer ni modificar
  datos de otra empresa. Un fallo de aislamiento es un bug crítico.

### IV. Integridad del dinero

- Los importes se guardan como **enteros en la unidad mínima** (centavos) junto con
  su **moneda** (ISO 4217). Está prohibido usar punto flotante para dinero.
- Toda operación entre monedas distintas registra el **tipo de cambio** usado.
- Los movimientos de dinero y de cuentas corrientes son **inmutables**: no se editan
  ni se borran. Se corrigen **anulando** (con motivo) y registrando uno nuevo.
- Los saldos se **derivan** de los movimientos. Si se cachean, deben poder
  recalcularse y coincidir siempre.
- Cada operación que afecta varios registros (ej. un cobro que mueve caja y cuenta
  corriente) es **atómica** (una transacción de base de datos).
- Toda operación financiera deja traza de auditoría: quién, cuándo, qué.

### V. Spec Driven Development

- No se escribe código de una funcionalidad sin `spec.md` aprobada.
- Flujo: `spec.md` (qué y por qué) → `plan.md` (cómo) → `tasks.md` → implementación.
- Las specs describen comportamiento observable y son independientes del stack.
- Los criterios de aceptación (Dado/Cuando/Entonces) se traducen en tests.
- Las dudas se marcan `[NEEDS CLARIFICATION]` y se resuelven antes de planificar.

### VI. Tests primero en el núcleo financiero

- El dominio financiero (saldos, cuentas corrientes, tipo de cambio, anulaciones,
  imputación de costos) se desarrolla con tests escritos antes que el código.
- Los contratos de API (OpenAPI) se validan con tests de contrato.

### VII. Mobile-first

- La interfaz se diseña primero para celular (uso en obra o taller) y luego se
  amplía a escritorio.
- Se distribuye como PWA instalable. No se requiere funcionamiento offline en el MVP.

## Stack tecnológico

| Capa | Decisión | Motivo |
|------|----------|--------|
| Backend | **Go**, API REST | Lenguaje que domina el equipo: permite auditar y depurar el código generado con IA justo donde un error cuesta caro (dinero, aislamiento). Binario único y barato de hostear. |
| Base de datos | **PostgreSQL** | Transacciones, integridad referencial y Row-Level Security para multiempresa. |
| Frontend | **React + TypeScript** (PWA) | Buena experiencia móvil y ecosistema maduro. |
| Contrato | **OpenAPI** | Fuente de verdad entre frontend y backend; permite generar clientes y tests. |
| Arquitectura | **Monolito modular** | Un solo servicio desplegable, módulos separados por dominio. Sin microservicios. |
| Archivos | Almacenamiento compatible con S3 | Adjuntos, logos y PDFs. |

Las librerías concretas (router, acceso a datos, migraciones, UI) se eligen en el
`plan.md` de la primera funcionalidad y se registran como ADR en `docs/adr/`.

## Convenciones

- **Documentación en español; código en inglés** (identificadores, tablas, endpoints,
  commits). La equivalencia de términos está en [`docs/glosario.md`](../../docs/glosario.md).
- Fechas en UTC en la base de datos; se muestran en la zona horaria de la empresa.
- Cada funcionalidad vive en `specs/NNN-nombre/` y se desarrolla en una rama del
  mismo nombre.

## Gobierno

- Esta constitución prevalece sobre cualquier otra práctica o documento.
- Se modifica por pull request, explicando el motivo y el impacto en specs existentes.
- Versionado semántico: MAJOR si se elimina o redefine un principio, MINOR si se
  agrega uno, PATCH para aclaraciones.

**Versión**: 1.0.0 | **Ratificada**: 2026-09-27 | **Última modificación**: 2026-09-27
