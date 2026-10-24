# Spec: Configuración por empresa

**Rama**: `002-configuracion`
**Creada**: 2026-09-27
**Estado**: Aprobada (2026-10-23)
**Entrada**: Para servir a distintos rubros sin cambiar código, cada empresa puede
configurar las etapas de sus proyectos, los nombres de algunas entidades, sus
categorías de ingresos/egresos y su catálogo de productos/servicios.

## Escenarios de usuario y pruebas

### Historia 1: Configurar etapas de proyecto (Prioridad: P1)

**Por qué esta prioridad**: los proyectos no existen sin etapas.

**Prueba independiente**: crear, renombrar, reordenar y eliminar etapas, y ver el
pipeline resultante en la configuración. El tablero de proyectos lo muestra desde la
spec 004.

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** agrega, renombra o reordena etapas,
   **entonces** el pipeline de la empresa queda con esos nombres y ese orden (los
   proyectos lo muestran desde 004).
2. **Dado** una etapa, **cuando** se la marca como "de cierre" (tipo *ganado* o
   *perdido*), **entonces** los proyectos en esa etapa se consideran cerrados. Cambiar
   el tipo de una etapa afecta a los proyectos que estén en ella sin modificarlos.
3. **Dado** una etapa, **cuando** el Administrador la elimina, **entonces** sale del
   pipeline y no se puede recuperar. Si tiene proyectos, el sistema pide moverlos a
   otra etapa antes de eliminarla (se verifica en 004).
4. **Dado** cualquier configuración, **entonces** la etapa inicial es la primera y es
   *abierta*, y existe al menos una de cierre *ganado*. Un cambio que lo rompa (crear,
   mover, cambiar el tipo o eliminar) se rechaza con un mensaje claro.
5. **Dado** una etapa activa con un nombre, **cuando** se intenta crear o renombrar
   otra con el mismo nombre (sin distinguir mayúsculas), **entonces** el sistema lo
   rechaza.

### Historia 2: Personalizar nombres (Prioridad: P3)

**Prueba independiente**: cambiar "Proyecto" por "Obra" y ver "Obra" en la interfaz.

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** cambia "Proyecto" por "Obra" (singular,
   plural y género), **entonces** toda la interfaz usa "Obra" con la concordancia
   correcta ("Nueva obra"). Los PDF lo usan desde 005.

### Historia 3: Categorías de ingresos y egresos (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** crea una categoría indicando si es de ingreso
   o egreso, **entonces** aparece entre las categorías activas de ese tipo (los
   movimientos la usan desde 006). El tipo no se puede cambiar después.
2. **Dado** una categoría, **cuando** el Administrador la **archiva**, **entonces**
   deja de ofrecerse pero se conserva en el historial, y se puede restaurar. Las
   categorías no se eliminan.
3. **Dado** las categorías de sistema (ej. "Cobros de clientes"), **entonces** no se
   pueden eliminar ni archivar; sí renombrar.
4. **Dado** una categoría activa, **cuando** se intenta crear, renombrar o restaurar
   otra del mismo tipo con el mismo nombre, **entonces** el sistema lo rechaza.

### Historia 4: Catálogo de productos/servicios (Prioridad: P2)

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** crea un ítem con nombre, descripción, unidad
   (unidad, m², metro lineal, hora, otra) y precio de referencia opcional (importe y
   moneda), **entonces** aparece en el catálogo activo (los presupuestos lo usan desde
   005). Con la unidad "otra", se indica su nombre (p. ej. "jornada").
2. **Dado** un ítem del catálogo, **cuando** se cambia su precio, **entonces** los
   presupuestos ya creados **no** se modifican. (Se verifica en 005: el presupuesto
   copia los datos del ítem.)
3. **Dado** un ítem que ya no se vende, **cuando** se archiva, **entonces** deja de
   ofrecerse en presupuestos nuevos; se puede restaurar.

### Historia 5: Cuentas de dinero iniciales y monedas

*Retirada*: se movió a la [spec 006](../006-cuentas-dinero/spec.md), que crea "Caja
efectivo" en la moneda base de la empresa. No hay configuración de monedas.

### Historia 6: Condiciones comerciales por defecto (Prioridad: P2)

**Prueba independiente**: definir 30 días y un texto, y verlos como valores por
defecto.

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** define la validez (1 a 365 días) y el texto de
   las condiciones comerciales, **entonces** quedan como valores por defecto de los
   presupuestos nuevos (005).
2. **Dado** un cambio de esos valores, **entonces** los presupuestos ya creados no se
   modifican.

Las condiciones son **texto libre**. Las condiciones de pago estructuradas (porcentajes,
p. ej. para precargar el plan de cobros) las define la [spec 005](../005-presupuestos/spec.md).

### Casos borde

- Renombrar una etapa o categoría no altera los datos históricos, que la referencian
  por identificador.
- Una plantilla aplicada solo precarga datos: luego la empresa los modifica libremente.
- La plantilla se aplica solo al registrarse; no se reaplica ni se cambia de rubro.
- Cualquier usuario de la empresa puede leer la configuración; solo el Administrador
  la modifica.

## Requisitos

### Requisitos funcionales

- **FR-001**: El sistema DEBE permitir a cada empresa definir etapas ordenadas, cada
  una de tipo *abierta*, *ganado* o *perdido*, con un color de una paleta cerrada. La
  primera etapa es la inicial y es *abierta*; siempre hay al menos una *ganado*.
- **FR-002**: El sistema DEBE permitir personalizar los nombres singular, plural y
  género de: Proyecto, Cliente y Presupuesto.
- **FR-003**: El sistema DEBE permitir crear, renombrar, archivar y restaurar
  categorías de ingreso y de egreso. Las categorías no se eliminan y su tipo no cambia.
- **FR-004**: El sistema DEBE incluir categorías de sistema no eliminables para las
  operaciones automáticas: Cobros de clientes, Pagos a proveedores, Pagos a empleados,
  Aportes de socios, Retiros de socios. Se pueden renombrar, no archivar. Las specs
  siguientes pueden agregar categorías de sistema (p. ej. "Ajuste de arqueo", 006).
- **FR-005**: El sistema DEBE permitir gestionar un catálogo de ítems con unidad y
  precio de referencia opcional (importe en centavos y moneda), que se pueden archivar
  y restaurar.
- **FR-006**: El sistema DEBE definir plantillas de rubro que precargan, al registrarse
  la empresa, etapas, categorías, nombres, catálogo de ejemplo y condiciones
  comerciales (contenido en "Plantillas de rubro", abajo; ver
  [modelo de dominio](../../docs/modelo-dominio.md#configuración-por-empresa)).
- **FR-007**: El sistema DEBE permitir configurar el texto por defecto de condiciones
  comerciales y la validez (en días) de los presupuestos.
- **FR-008**: El sistema DEBE permitir a cualquier usuario de la empresa leer la
  configuración y solo al Administrador modificarla.

### Plantillas de rubro (FR-006)

Comunes a las dos plantillas:

| Elemento | Valor |
|---|---|
| Nombres | Proyecto / Proyectos (masculino) · Cliente / Clientes (masculino) · Presupuesto / Presupuestos (masculino) |
| Categorías de sistema (FR-004) | Ingreso: Cobros de clientes, Aportes de socios · Egreso: Pagos a proveedores, Pagos a empleados, Retiros de socios |
| Validez de presupuestos | 15 días |

**Genérico** (`generic`):

| Elemento | Contenido |
|---|---|
| Etapas (en orden) | Presupuesto (abierta, azul) → Aprobado (abierta, verde azulado) → En curso (abierta, amarillo) → Finalizado (ganado, verde) · Perdido (perdido, gris) |
| Categorías de egreso | Materiales, Mano de obra, Flete, Alquiler, Servicios, Impuestos, Otros gastos |
| Categorías de ingreso | Otros ingresos |
| Catálogo de ejemplo | (ninguno) |
| Condiciones comerciales | (ninguna) |

**Carpintería de aluminio** (`aluminum_carpentry`):

| Elemento | Contenido |
|---|---|
| Etapas (en orden) | Presupuesto (abierta, azul) → Aprobado (abierta, verde azulado) → En producción (abierta, naranja) → Instalación (abierta, violeta) → Finalizado (ganado, verde) · Perdido (perdido, gris) |
| Categorías de egreso | Perfiles de aluminio, Vidrios, Accesorios, Flete, Mano de obra, Alquiler, Servicios, Impuestos |
| Categorías de ingreso | Otros ingresos |
| Catálogo de ejemplo (sin precio) | Ventana corrediza (m²), Puerta de abrir (unidad), Mampara de baño (unidad), Colocación (unidad) |
| Condiciones comerciales | "Seña del 50 % al aprobar el presupuesto y saldo contra entrega." |

Las empresas registradas antes de esta spec reciben la configuración mínima: los
nombres por defecto, las categorías de sistema y las etapas de "Genérico" (sin
catálogo ni condiciones).

### Entidades clave

- **Etapa (Stage)**: nombre, orden, tipo (abierta/ganado/perdido), color (de paleta);
  eliminada.
- **Categoría (Category)**: nombre, tipo (ingreso/egreso), clave de sistema (si es de
  sistema), archivada.
- **Ítem de catálogo (CatalogItem)**: nombre, descripción, unidad (y su nombre si es
  "otra"), precio de referencia opcional con moneda, archivado.
- **Nombres personalizados (Labels)**: singular, plural y género por entidad.
- **Condiciones por defecto (QuoteDefaults)**: validez en días y texto.

## Criterios de éxito

- **SC-001**: Una empresa de otro rubro (ej. una herrería) queda operativa solo
  configurando estas opciones, sin cambios de código. En 002 se verifica con un
  escenario automatizado que configura una herrería solo por la API; de punta a punta,
  cuando existan 004 a 007.

## Supuestos

- Las plantillas de rubro las mantiene el equipo del producto; las empresas no crean
  plantillas propias en el MVP.
- No hay campos personalizados en el MVP.
- No hay configuración de monedas: ARS y USD están siempre disponibles y la moneda base
  se fija al registrarse (001).

## Clarificaciones aprobadas

### Sesión 2026-10-23

Aprobadas por el usuario con las opciones recomendadas del plan
([`plan.md`](plan.md) §14.2). Ya están incorporadas al cuerpo de la spec.

- **P-1**: La Historia 5 (cuenta de dinero inicial) se mueve a la 006, que crea "Caja
  efectivo" en la moneda base. No hay configuración de monedas.
- **P-2**: Cambiar el tipo de una etapa se permite si respeta las reglas del pipeline;
  "cerrado" se deriva del tipo de la etapa actual. Eliminar una etapa es definitivo;
  mover sus proyectos antes de eliminarla se implementa y se verifica en 004.
- **P-3**: Los nombres personalizados llevan singular, plural y género. La interfaz
  los usa desde 002; los PDF, desde 005; los emails del sistema no los usan.
- **P-4**: Contenido de las plantillas según "Plantillas de rubro". Ítems de ejemplo sin
  precio. La plantilla se aplica solo al registrarse.
- **P-5**: Cualquier usuario lee la configuración; solo el Administrador la modifica.
- **P-6**: La etapa inicial es la primera y es abierta; al menos una *ganado*; *perdido*
  opcional; nombres de etapas activas únicos.
- **P-7**: Color de etapa de una paleta cerrada de 8 colores.
- **P-8**: Las categorías se archivan y restauran, nunca se eliminan; el tipo no cambia;
  nombres únicos por tipo entre las activas; las specs siguientes agregan sus
  categorías de sistema (p. ej. "Ajuste de arqueo" en 006).
- **P-9**: Precio de referencia opcional (importe y moneda juntos); la unidad "otra"
  lleva nombre; archivar y restaurar; nombres de ítems repetidos permitidos.
- **P-10**: FR-007 tiene su historia (Historia 6, P2): texto de hasta 2.000 caracteres y
  validez de 1 a 365 días.
- Las condiciones comerciales de 002 quedan como texto libre; las condiciones de pago
  estructuradas (porcentajes) las define la 005.
