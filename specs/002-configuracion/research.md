# Research: Configuración por empresa (backend)

**Spec**: [`spec.md`](spec.md) · **Plan**: [`plan.md`](plan.md) · **Fecha**: 2026-10-23
**Estado**: Aprobado (2026-10-23), junto con el plan.

Alternativas evaluadas por decisión. Criterios que se repiten (los de 001): (1) simplicidad para un
equipo con Go intermedio, (2) seguridad del aislamiento y del dinero, (3) una sola unidad
desplegable, (4) no condicionar a las specs siguientes ni a la UI actual (habrá un rediseño visual).
Cada decisión estructural tiene su ADR; las locales, su `DD-n` en el plan.

---

## R-1 Dónde vive la configuración → DD-1

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Módulo `settings` con cuatro tablas propias** | Frontera clara (ADR-001); 004 a 010 dependen de un módulo chico que no depende de identidad; tablas tipadas con `CHECK` | Un módulo más que cablear en `internal/app` | **Elegida** |
| Dentro del módulo `tenant` | Ya existe y es "de la empresa" | `tenant` creció con registro, logo y roles; mezclaría el ciclo de vida de la empresa con catálogos que consumen otras specs; `tenant` importa `identity`, así que 004 a 010 heredarían esa dependencia | Descartada |
| Columnas nuevas en `tenants` (nombres, condiciones) + tablas para el resto | Una lectura menos | `tenants` es del módulo `tenant`: `settings` no puede escribirla (ningún módulo lee ni escribe tablas ajenas). Sus escrituras toman `FOR NO KEY UPDATE` sobre la fila más referenciada del sistema (DD-40 de 001) | Descartada |
| Tabla genérica clave-valor o `jsonb` de configuración | Flexible | Sin tipos ni `CHECK`; las FK de 004 a 010 no pueden apuntar a un elemento dentro de un `jsonb`; consultas y validaciones a mano | Descartada |

## R-2 Contenido de las plantillas → DD-2

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`catalog.json` embebido, versión 2, con el contenido** | Ya existe (DD-3 de 001); datos versionados con el código; un rubro nuevo es una entrada; principio II | Cambiar una plantilla requiere un release | **Elegida** |
| Plantillas como filas en la base (tablas de plantillas) | Editables sin release | Las mantiene el equipo del producto, no las empresas (supuesto de la spec); habría que proteger tablas globales sin `tenant_id` (excepción al principio III) y escribir una pantalla de administración | Descartada |
| Una función de sembrado en Go por plantilla | Máxima flexibilidad | Código por rubro (prohibido por el principio II) | Descartada |
| Archivos SQL de sembrado por plantilla | Rápidos | SQL dinámico o uno por plantilla; mezcla datos con migraciones | Descartada |

Categorías de sistema **fuera** de la plantilla (en `settings`): son iguales para todo rubro y otras
specs dependen de ellas por clave; si vivieran en la plantilla, un error de datos dejaría a una
empresa sin "Cobros de clientes".

## R-3 Cuándo sembrar → DD-3

Restricción heredada: DD-33 R-c de 001 (el `Seeder` corre dentro de la transacción que retiene el
lock de `GRANT crm_tenant`, solo con SQL por conjuntos y dentro del presupuesto de R-b; si no entra,
002 lo rediseña). T-B910 midió una retención p95 de 96,3 y 133,6 ms contra un target de 140 ms.

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Dentro de la transacción de registro, cuatro sentencias por conjuntos, con compuerta de medición** | Atomicidad (INV-14 de 001) sin estados nuevos: una empresa nunca existe sin configuración; costo constante (no crece con la cantidad de roles) | Suma algunos milisegundos a la retención del lock | **Elegida**, condicionada a T-002-B307 |
| **Respaldo**: estado "configuración pendiente" + sembrado idempotente después del `COMMIT` | No toca la retención del lock | Un estado más: `tenant_settings` puede faltar; toda lectura y escritura tiene que sembrar si falta (bajo `LockTenantSettings`, con `INSERT … ON CONFLICT DO NOTHING` y sin duplicar etapas ni categorías); el primer request de la empresa paga el sembrado; tests de carrera entre el primer `GET` y la primera escritura | Respaldo definido; se aplica solo si la compuerta falla |
| Sembrar en el *worker* (outbox o `PeriodicTask`) | Fuera del request | La empresa queda sin configuración durante segundos; el worker corre como `crm_worker` y tendría que pasar al rol de cada empresa; un fallo deja una empresa a medias sin señal para el usuario | Descartada |
| Una sentencia por fila | Simple de escribir | Viola R-c; el costo crece con el tamaño de la plantilla | Descartada |

Diseño del respaldo (por si la compuerta falla): `TemplateSeeder.Seed` solo registraría
`tenants.industry_template_*` (como en 001); `settings` agregaría `ensureSeeded(ctx, tx, tenantID)`
al comienzo de cada operación, después de `LockTenantSettings`… que no puede tomarse si la fila no
existe: se reemplaza por `INSERT … ON CONFLICT (tenant_id) DO NOTHING` de la fila con los valores de
la plantilla (leída de `tenants` por una interfaz del módulo `tenant`), y si insertó, siembra el resto
en la misma transacción. Exige un ADR o una `DD` nuevos y la aprobación del usuario.

## R-4 Empresas registradas antes de 002 → DD-4

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| ***Backfill* mínimo en la migración** | Las invariantes valen para todas las empresas desde el despliegue; patrón previsto por ADR-004 §6 | Repite textos semilla en SQL; siembra "Genérico" aunque la empresa hubiera elegido otro rubro | **Elegida** (no hay empresas reales, S-2) |
| Sembrado perezoso en el primer acceso | Sin SQL repetido | Es el respaldo de R-3: estado nuevo y carreras, para un caso que hoy no existe en producción | Descartada |
| Comando `crm tenants seed-config` | Explícito | Un paso manual que se puede olvidar al desplegar; mientras tanto, `500` | Descartada |
| *Backfill* por código de plantilla | Fiel a la elección original | Tendría que leer `catalog.json` desde SQL (no puede) o repetir todo el contenido de ambas plantillas | Descartada |

## R-5 Orden de las etapas → DD-5, DD-18

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`position` entero denso, reescrito por el servicio bajo lock** | Simple; el valor es el que ve el usuario (1..n); una sentencia reescribe todo | Sin garantía de la base sobre la densidad (la vigila un test) | **Elegida** |
| Índice único sobre `(tenant_id, position)` | La base impide repetidos | PostgreSQL verifica los índices únicos no diferibles fila por fila durante un `UPDATE`: intercambiar posiciones falla; las constraints diferibles no pueden ser parciales, y las etapas eliminadas conservan su posición | Descartada |
| Orden fraccionario (`float` o cadenas tipo *lexorank*) | Mover toca una fila | Complejidad sin beneficio para decenas de filas; los `float` se agotan con movimientos repetidos | Descartada |
| Lista enlazada (`previous_stage_id`) | Mover toca pocas filas | Lecturas recursivas; difícil de validar | Descartada |

API de reordenamiento:

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`POST /stages/{id}/move {position}`** | Un pedido por arrastre (mobile-first); sin comparar conjuntos; sin listas viejas de otro cliente | Varias etapas = varios pedidos | **Elegida** |
| `PUT /stages/order {stage_ids: […]}` | Un pedido para cualquier reordenamiento | Hay que validar que el conjunto coincida con las activas (una etapa creada por otro usuario invalida la lista); la ruta comparte prefijo con `/stages/{stageId}` | Descartada |

## R-6 Cómo se sostiene la regla del pipeline → DD-5

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Servicio: lock de la fila de `tenant_settings` + función pura sobre la lista resultante** | Una regla, probada una vez; el lock serializa por empresa sin afectar a otras; mismo modo de lock que el resto (`FOR NO KEY UPDATE`) | La base no la impide por sí sola (un `INSERT` a mano podría romperla) | **Elegida** |
| Trigger de constraint (`CONSTRAINT TRIGGER … DEFERRABLE`) | La base la garantiza | La convención de 001 es "sin triggers"; lógica escondida en SQL; costo en cada escritura | Descartada |
| `SERIALIZABLE` | Sin locks explícitos | Reintentos ante `40001` en cada operación; descartado por la misma razón en 001 (R-16 de 001) | Descartada |
| *Advisory lock* por empresa | No toca filas | Descartado en 001 (R-16 de 001); no agrega nada frente a un lock de una fila propia | Descartada |
| Lock de la fila de `tenants` | Ya existe un lock por empresa | `settings` no puede leer ni bloquear tablas de `tenant` (ADR-001), y serializaría con la administración de usuarios | Descartada |

## R-7 Eliminar etapas → DD-6, ADR-028

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Borrado lógico sin restaurar** | El historial de 004 sigue resolviendo la etapa por id; para el usuario es "eliminar" | 004 tiene que mostrar etapas eliminadas en el historial | **Elegida** |
| Borrado físico | Simple en 002 | Rompe las FK del historial de 004 (o fuerza `ON DELETE SET NULL` y se pierde el historial) | Descartada |
| Archivar y restaurar (como categorías) | Un mismo concepto en todo | La spec dice "eliminar"; restaurar una etapa obliga a decidir en qué posición vuelve y si respeta las reglas | Descartada |

## R-8 Color de etapa → DD-7

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Token de una paleta cerrada (8)** | El rediseño visual reasigna colores sin migrar datos; contraste y modo oscuro los decide la UI | Sin colores arbitrarios | **Elegida** |
| Hex libre (`#RRGGBB`) | Libertad total | Ata los datos al estilo actual; colores ilegibles posibles | Descartada |
| Sin color | Menos | FR-001 lo pide | Descartada |

## R-9 Categorías de sistema → DD-8, ADR-028

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Columna `system_key` con `CHECK` e índice único parcial** | Las specs siguientes buscan por clave estable, aunque el usuario la renombre; la base impide archivarla y fija su tipo | Agregar una clave es una migración (alterar el `CHECK` y sembrar) | **Elegida** |
| `is_system boolean` + buscar por nombre | Menos columnas | Renombrar (que la spec permite) rompe la búsqueda | Descartada |
| Tabla separada de categorías de sistema | Separación | Los movimientos tendrían dos FK posibles; los reportes por categoría unirían dos tablas | Descartada |
| Ids fijos conocidos | Sin columna | Los ids son UUIDv7 por empresa (ADR-008) | Descartada |

## R-10 Inmutabilidad del tipo de una categoría → DD-8

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Privilegio `UPDATE` por columna (sin `type` ni `system_key`)** | La base lo garantiza sin triggers; mismo mecanismo que usarán las tablas financieras (checklist de 001 §6) | Cambiar el tipo, si algún día se quiere, exige una migración | **Elegida** |
| Solo en el servicio | Simple | Una query nueva podría cambiarlo sin que falle nada | Descartada |
| Trigger | Garantía | Convención "sin triggers" | Descartada |

## R-11 Nombres únicos → INV-6

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Índice único parcial con `lower(name)` entre activas** | La base lo garantiza también ante concurrencia; los archivados/eliminados liberan el nombre | No unifica acentos (S-5) | **Elegida** |
| `citext` | Comparación sin mayúsculas nativa | Extensión adicional; mismo problema con acentos | Descartada |
| Verificación en el servicio | Mensajes a medida | Carrera entre dos creaciones concurrentes | Descartada |
| Sin unicidad | Nada que validar | Dos "Vidrios" en un desplegable confunden y ensucian los reportes | Descartada (para catálogo sí se elige: ver DD-9) |

## R-12 Precio de referencia → DD-9, ADR-029

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`bigint` de centavos + moneda, opcional como par** | Constitución IV; exacto; "a cotizar" sin valor mágico | El cliente formatea por texto (ADR-023) | **Elegida** |
| `numeric(18,2)` | Decimal nativo | La constitución pide enteros en la unidad mínima; pgx lo mapea a un tipo que invita a convertir a `float` | Descartada |
| Obligatorio, con 0 como "sin precio" | Una columna siempre presente | 0 es un precio válido (bonificado); significado oculto | Descartada |
| Precio en la moneda base de la empresa (sin columna de moneda) | Más simple | La carpintería cotiza en ARS y en USD; la moneda base no es la del producto | Descartada |

## R-13 Semántica de `PATCH` del catálogo → DD-9

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **`Patch[T]` genérico (presente, valor) + lock-mezcla-escritura** | Distingue ausente de `null` para cualquier tipo, incluido `Money`; mismo patrón de lock que DD-43 de 001 | Un tipo genérico nuevo | **Elegida** |
| `map[string]*string` (como `tenant.Update`) | Ya existe | Solo sirve para textos | Descartada |
| `PUT` completo | Sin ambigüedad | El formulario tendría que reenviar todo; concurrencia de última escritura gana sobre campos que el usuario no tocó | Descartada |

## R-14 Nombres personalizados → DD-10

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Columnas en `tenant_settings` con singular, plural y género** | Tipado; un `PUT` completo; concordancia gramatical | Agregar una entidad nombrable es una migración | **Elegida** |
| `jsonb` de etiquetas | Agregar entidades sin migrar | Sin `CHECK`; validación a mano | Descartada |
| Tabla `labels(entity, singular, plural)` | Extensible | Tres filas fijas como tabla; más queries | Descartada |
| Sin género | Menos campos | "Nuevo Obra", "Obras abiertos" | Descartada |

Exposición: endpoint propio `GET /settings/labels` en lugar de sumarlos a `GET /me`, porque
`SessionInfo` de 001 tiene `additionalProperties: false` y cambiarlo acopla la sesión a la
configuración; la SPA puede pedir los dos al arrancar (TanStack Query los cachea).

## R-15 Códigos de error → DD-12

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Un código nuevo (`name_taken`) y reuso de `invalid_state` con significado por endpoint** | Cambio mínimo del enum compartido; `name_taken` aparece en seis endpoints y se muestra junto al campo | La UI distingue el texto de `invalid_state` por endpoint | **Elegida** |
| Códigos nuevos por regla (`last_won_stage`, `closing_stage_first`, `system_category`) | Mensajes precisos sin contexto | Cuatro valores más en el enum compartido y en el mapa exhaustivo del frontend | Descartada |
| Todo como `422` con códigos de campo | Sin `409` | Las reglas del pipeline no son de un campo; el enum de `FieldError` también es compartido | Descartada |

## R-16 Contratos de varias specs en los tests → DD-13

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Lista explícita de contratos en el validador + `$ref` externos resueltos** | Agregar una spec es una línea; un contrato en diseño no rompe `make check`; un solo lugar para los componentes compartidos | Depende de que libopenapi resuelva archivos locales (S-4; respaldo: fusión en memoria) | **Elegida** |
| Cargar `specs/*/contracts/openapi.yaml` por patrón | Automático | El contrato de una spec en diseño (sin código) haría fallar el test de rutas contra el contrato | Descartada |
| Un único archivo de contrato para todo el sistema | Un solo documento | Revisión por spec más difícil; cambia la convención de `ui.md` §11.4 de 001 | Descartada |
| Copiar los componentes compartidos en cada contrato | Sin referencias entre archivos | Copias que divergen (el enum de errores, sobre todo) | Descartada |

Rutas implementadas por fases: `pendingOperations` (lista de `operationId` de 002 todavía sin
handler) en el test de rutas contra el contrato. Alternativa descartada: implementar todos los
handlers como esqueletos en la Fase 2 (código que responde algo falso durante varias fases).

## R-17 Permisos de lectura → DD-11

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Lectura para cualquier rol, escritura `settings.manage`** | El Operador necesita leer para trabajar en 004 y 005; la matriz no cambia | El Operador ve nombres de categorías de egreso (sin importes) | **Elegida** |
| Todo con `settings.manage` | Más restrictivo | Las pantallas del Operador (tablero, presupuestos) no podrían cargar etapas ni catálogo | Descartada |
| Permiso nuevo `settings.view` | Explícito | Cambia FR-007 y ADR-013 para un permiso que tendrían los dos roles | Descartada |

## R-18 Auditoría de la configuración → DD-14

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Auditar toda escritura, con `data` sin texto libre** | Trazabilidad entre varios administradores; renombrar una categoría cambia cómo se leen los reportes | Un `INSERT` por operación | **Elegida** |
| No auditar | Menos código | Ninguna traza de quién cambió el pipeline o una categoría de sistema | Descartada |
| Auditar con nombres viejos y nuevos en `data` | Más información | Texto libre del usuario en una tabla append-only (DD-37 (5) de 001) | Descartada |

## R-19 Condiciones comerciales: texto libre o estructuradas → DD-10 **(usuario, 2026-10-23)**

| Opción | A favor | En contra | Veredicto |
|---|---|---|---|
| **Texto libre en 002; las condiciones de pago estructuradas las define 005** | 002 no anticipa un modelo que todavía no se diseñó; la plantilla trae un texto legible; 005 decide la forma junto con el plan de cobros que las consume | 005 Historia 4.3 no puede precargar el plan de cobros desde el texto de 002 | **Elegida** |
| Porcentajes estructurados (seña, saldo) en `tenant_settings` desde 002 | La precarga de 005 funcionaría sin más diseño | Adelanta el modelo del plan de cobros (004/005) sin su spec; columnas que nadie usa hasta 005 | Descartada |

---

## Fuentes consultadas

- Documentación de *rolodex* de libopenapi (resolución de referencias a archivos locales y remotos,
  `BasePath` y `AllowFileReferences`): <https://pb33f.io/libopenapi/rolodex/>. Base de S-4; no
  confirma por sí sola el comportamiento de libopenapi-validator con esas referencias.
- Patrón de inserción por conjuntos con `unnest` de arrays paralelos (general, no específico de
  sqlc): <https://forum.djangoproject.com/t/speeding-up-postgres-bulk-create-by-using-unnest/36508>.
  Base de DD-3; el soporte de sqlc queda como supuesto S-3.
