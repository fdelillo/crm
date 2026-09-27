# Spec: Configuración por empresa

**Rama**: `002-configuracion`
**Creada**: 2026-09-27
**Estado**: Borrador
**Entrada**: Para servir a distintos rubros sin cambiar código, cada empresa puede
configurar las etapas de sus proyectos, los nombres de algunas entidades, sus
categorías de ingresos/egresos y su catálogo de productos/servicios.

## Escenarios de usuario y pruebas

### Historia 1: Configurar etapas de proyecto (Prioridad: P1)

**Por qué esta prioridad**: los proyectos no existen sin etapas.

**Prueba independiente**: crear, renombrar y reordenar etapas, y verlas reflejadas
en el tablero de proyectos.

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** agrega, renombra o reordena etapas,
   **entonces** los proyectos muestran el nuevo pipeline.
2. **Dado** una etapa, **cuando** se la marca como "de cierre" (tipo *ganado* o
   *perdido*), **entonces** los proyectos en esa etapa se consideran cerrados.
3. **Dado** una etapa con proyectos, **cuando** se intenta eliminarla, **entonces** el
   sistema pide moverlos a otra etapa antes de eliminarla.
4. **Dado** cualquier configuración, **entonces** siempre existe al menos una etapa
   inicial y una de cierre *ganado*.

### Historia 2: Personalizar nombres (Prioridad: P3)

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** cambia "Proyecto" por "Obra" (singular y
   plural), **entonces** toda la interfaz y los PDF usan "Obra".

### Historia 3: Categorías de ingresos y egresos (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** crea una categoría indicando si es de ingreso
   o egreso, **entonces** queda disponible al registrar movimientos.
2. **Dado** una categoría con movimientos, **cuando** se intenta eliminarla, **entonces**
   solo se puede **archivar** (deja de ofrecerse, pero se conserva en el historial).
3. **Dado** las categorías de sistema (ej. "Cobros de clientes"), **entonces** no se
   pueden eliminar ni archivar; sí renombrar.

### Historia 4: Catálogo de productos/servicios (Prioridad: P2)

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** crea un ítem con nombre, descripción, unidad
   (unidad, m², metro lineal, hora, otra), precio de referencia y moneda, **entonces**
   puede usarlo al armar presupuestos.
2. **Dado** un ítem del catálogo, **cuando** se cambia su precio, **entonces** los
   presupuestos ya creados **no** se modifican.
3. **Dado** un ítem que ya no se vende, **cuando** se archiva, **entonces** deja de
   ofrecerse en presupuestos nuevos.

### Historia 5: Cuentas de dinero iniciales y monedas (Prioridad: P1)

Ver [006](../006-cuentas-dinero/spec.md). La plantilla crea por defecto "Caja efectivo ARS".

### Casos borde

- Renombrar una etapa o categoría no altera los datos históricos, que la referencian
  por identificador.
- Una plantilla aplicada solo precarga datos: luego la empresa los modifica libremente.

## Requisitos

### Requisitos funcionales

- **FR-001**: El sistema DEBE permitir a cada empresa definir etapas ordenadas, cada
  una de tipo *abierta*, *ganado* o *perdido*, con un color.
- **FR-002**: El sistema DEBE permitir personalizar los nombres singular y plural de:
  Proyecto, Cliente y Presupuesto.
- **FR-003**: El sistema DEBE permitir crear, renombrar y archivar categorías de
  ingreso y de egreso.
- **FR-004**: El sistema DEBE incluir categorías de sistema no eliminables para las
  operaciones automáticas: Cobros de clientes, Pagos a proveedores, Pagos a empleados,
  Aportes de socios, Retiros de socios.
- **FR-005**: El sistema DEBE permitir gestionar un catálogo de ítems con unidad,
  precio de referencia y moneda.
- **FR-006**: El sistema DEBE definir plantillas de rubro que precargan etapas,
  categorías, nombres y catálogo de ejemplo (ver
  [modelo de dominio](../../docs/modelo-dominio.md#configuración-por-empresa)).
- **FR-007**: El sistema DEBE permitir configurar el texto por defecto de condiciones
  comerciales y la validez (en días) de los presupuestos.

### Entidades clave

- **Etapa (Stage)**: nombre, orden, tipo (abierta/ganado/perdido), color.
- **Categoría (Category)**: nombre, tipo (ingreso/egreso), sistema (sí/no), archivada.
- **Ítem de catálogo (CatalogItem)**: nombre, descripción, unidad, precio de
  referencia, moneda, archivado.
- **Etiquetas (Labels)**: nombres personalizados por entidad.

## Criterios de éxito

- **SC-001**: Una empresa de otro rubro (ej. una herrería) queda operativa solo
  configurando estas opciones, sin cambios de código.

## Supuestos

- Las plantillas de rubro las mantiene el equipo del producto; las empresas no crean
  plantillas propias en el MVP.
- No hay campos personalizados en el MVP.
