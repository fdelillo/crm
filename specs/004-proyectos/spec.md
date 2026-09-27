# Spec: Proyectos

**Rama**: `004-proyectos`
**Creada**: 2026-09-27
**Estado**: Borrador
**Entrada**: Cada trabajo para un cliente es un proyecto que avanza por las etapas
configuradas por la empresa. Tiene notas, adjuntos (fotos, planos, medidas), cobros,
saldo pendiente y costos imputados para conocer su margen.

## Escenarios de usuario y pruebas

### Historia 1: Crear un proyecto y seguir su avance (Prioridad: P1)

**Por qué esta prioridad**: es la unidad central de trabajo del sistema.

**Prueba independiente**: crear un proyecto para un cliente, moverlo por las etapas y
verlo en el tablero.

**Escenarios de aceptación**:

1. **Dado** un cliente, **cuando** se crea un proyecto con nombre, cliente y
   (opcionalmente) dirección de obra, fecha estimada de entrega y descripción,
   **entonces** queda en la etapa inicial.
2. **Dado** un proyecto, **cuando** se cambia su etapa, **entonces** el cambio queda
   en su historial con usuario y fecha.
3. **Dado** la lista de proyectos, **cuando** se abre, **entonces** se ven agrupados
   por etapa (tablero en escritorio, lista agrupada en celular), con filtros por
   cliente, etapa y abiertos/cerrados.
4. **Dado** un proyecto con fecha de entrega vencida y no cerrado, **entonces** se
   destaca como atrasado.

### Historia 2: Monto, cobros y saldo (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un proyecto sin presupuesto aprobado, **entonces** su monto es cero y no
   genera deuda del cliente.
2. **Dado** un presupuesto aprobado (ver [005](../005-presupuestos/spec.md)),
   **entonces** el monto del proyecto es el total de ese presupuesto, en su moneda, y
   la cuenta corriente del cliente aumenta por ese importe.
3. **Dado** un proyecto con monto, **cuando** se registra un **adicional** o una
   **bonificación** con descripción e importe, **entonces** el monto y la cuenta
   corriente del cliente se ajustan.
4. **Dado** cobros imputados al proyecto (ver [007](../007-cuentas-corrientes/spec.md)),
   **entonces** la ficha muestra: monto, cobrado, **saldo pendiente** y porcentaje cobrado.
5. **Dado** un proyecto que pasa a una etapa de cierre *ganado* con saldo pendiente,
   **entonces** el sistema lo advierte, pero permite el cambio.

### Historia 3: Costos y margen (Prioridad: P2)

**Escenarios de aceptación**:

1. **Dado** compras, cargos a empleados o gastos directos imputados al proyecto,
   **cuando** el Administrador abre la ficha, **entonces** ve la lista de costos por
   categoría, el total de costos y el **margen** (monto − costos) en importe y porcentaje.
2. **Dado** costos en una moneda distinta de la del proyecto, **entonces** se muestran
   agrupados por moneda y el margen consolidado solicita un tipo de cambio de referencia.
3. **Dado** un Operador, **entonces** no ve costos ni margen.

### Historia 4: Notas y adjuntos (Prioridad: P2)

**Escenarios de aceptación**:

1. **Dado** un proyecto, **cuando** un usuario agrega una nota, **entonces** aparece en
   la bitácora con autor y fecha, de la más reciente a la más antigua.
2. **Dado** un proyecto, **cuando** se sube una foto desde la cámara del celular o un
   archivo (imagen o PDF), **entonces** queda adjunto al proyecto con vista previa.
3. **Dado** un adjunto de más de 15 MB o de un tipo no permitido, **entonces** se rechaza
   con un mensaje claro.
4. **Dado** un adjunto, **cuando** su autor o un Administrador lo elimina, **entonces**
   se quita y queda registrado en la auditoría.

### Casos borde

- Un proyecto con movimientos de cuenta corriente o costos no se puede eliminar; solo
  pasarlo a una etapa de cierre *perdido* o archivarlo.
- Cambiar el cliente de un proyecto con cobros o presupuesto aprobado no está permitido.
- Pasar un proyecto con presupuesto aprobado a *perdido* no elimina la deuda: el
  sistema sugiere registrar una bonificación o anular la aprobación.

## Requisitos

### Requisitos funcionales

- **FR-001**: El sistema DEBE permitir crear proyectos asociados a un cliente.
- **FR-002**: El sistema DEBE mantener la etapa actual y el historial de cambios de etapa.
- **FR-003**: El sistema DEBE calcular el monto del proyecto como total del presupuesto
  aprobado más adicionales menos bonificaciones.
- **FR-004**: El sistema DEBE calcular cobrado y saldo pendiente a partir de la cuenta
  corriente del cliente imputada al proyecto.
- **FR-005**: El sistema DEBE calcular costos (a partir de imputaciones) y margen,
  visibles solo para Administradores.
- **FR-006**: El sistema DEBE permitir notas y adjuntos (imágenes y PDF, hasta 15 MB
  cada uno).
- **FR-007**: El sistema DEBE ofrecer vistas de tablero y de lista con filtros y búsqueda.
- **FR-008**: El sistema DEBE numerar los proyectos correlativamente por empresa
  (ej. P-0001).

### Entidades clave

- **Proyecto (Project)**: número, nombre, cliente, etapa, dirección de obra, fecha
  estimada de entrega, descripción, moneda (la del presupuesto aprobado).
- **Ajuste de proyecto (ProjectAdjustment)**: tipo (adicional/bonificación),
  descripción, importe.
- **Nota (Note)**, **Adjunto (Attachment)**, **Historial de etapas (StageChange)**.

## Criterios de éxito

- **SC-001**: El dueño responde "¿cuánto falta cobrar de la obra X?" en menos de 10
  segundos desde el celular.
- **SC-002**: El margen de cada proyecto se obtiene sin planillas externas.

## Supuestos

- El IVA no se discrimina en costos: el margen se calcula con importes totales.
- Un proyecto tiene una sola moneda, la de su presupuesto aprobado.
