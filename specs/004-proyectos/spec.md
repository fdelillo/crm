# Spec: Proyectos

**Rama**: `004-proyectos`
**Creada**: 2026-09-27
**Estado**: Borrador
**Entrada**: Cada trabajo para un cliente es un proyecto que avanza por las etapas
configuradas por la empresa. Tiene notas, adjuntos (fotos, planos, medidas), cobros,
saldo pendiente y costos imputados para conocer su margen.

## Clarificaciones

### Sesión 2026-09-27

- P: ¿Cuándo nace la deuda del cliente? → R: Al aprobar el presupuesto, por el total.
- P: ¿Se acuerda un plan de pagos? → R: Sí, opcional: cuotas con fecha e importe, usadas para la proyección de cobros y para marcar vencidos. No cambia la deuda total.
- P: ¿El margen separa el IVA? → R: No, se calcula con importes totales.
- P: ¿Cómo se cargan los proyectos que ya están en curso al empezar? → R: Como proyecto en curso con monto acordado y cobrado previo, sin mover la caja.

## Escenarios de usuario y pruebas

### Historia 1: Crear un proyecto y seguir su avance (Prioridad: P1)

**Por qué esta prioridad**: es la unidad central de trabajo del sistema.

**Prueba independiente**: crear un proyecto para un cliente, moverlo por las etapas y
verlo en el tablero.

**Escenarios de aceptación**:

1. **Dado** un cliente, **cuando** se crea un proyecto con nombre, cliente, **moneda
   del proyecto** (ARS o USD) y, opcionalmente, dirección de obra, fecha estimada de
   entrega y descripción, **entonces** queda en la etapa inicial. La moneda indica en
   qué se lleva el monto y el saldo; el cliente igual puede pagar en ambas monedas.
2. **Dado** un proyecto, **cuando** se cambia su etapa, **entonces** el cambio queda
   en su historial con usuario y fecha.
3. **Dado** la lista de proyectos, **cuando** se abre, **entonces** se ven agrupados
   por etapa (tablero en escritorio, lista agrupada en celular), con filtros por
   cliente, etapa y abiertos/cerrados.
4. **Dado** un proyecto con fecha de entrega vencida y no cerrado, **entonces** se
   destaca como atrasado.

### Historia 2: Monto, cobros y saldo (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un proyecto sin presupuesto aprobado (y que no es un proyecto en curso,
   ver Historia 4), **entonces** su monto es cero y no genera deuda del cliente.
2. **Dado** un presupuesto aprobado (ver [005](../005-presupuestos/spec.md)),
   **entonces** el monto del proyecto es el total de ese presupuesto, en la moneda
   del proyecto, y la cuenta corriente del cliente aumenta por ese importe.
3. **Dado** un proyecto con equivalente de referencia (ej. USD 1.000 ≈ ARS 1.200.000),
   **entonces** la ficha lo muestra como dato informativo junto al monto.
4. **Dado** un proyecto con monto, **cuando** se registra un **adicional** o una
   **bonificación** con descripción e importe, **entonces** el monto y la cuenta
   corriente del cliente se ajustan.
5. **Dado** cobros imputados al proyecto, en cualquier moneda (ver
   [007](../007-cuentas-corrientes/spec.md)), **entonces** la ficha muestra, en la
   moneda del proyecto: monto, cobrado, **saldo pendiente** y porcentaje cobrado; y
   el detalle de cada cobro con lo que se recibió realmente (ej. ARS 600.000 →
   cancela USD 500).
6. **Dado** un proyecto que pasa a una etapa de cierre *ganado* con saldo pendiente,
   **entonces** el sistema lo advierte, pero permite el cambio.

### Historia 3: Plan de cobros (Prioridad: P2)

**Escenarios de aceptación**:

1. **Dado** un proyecto con monto, **cuando** se carga un plan de cobros con cuotas
   (descripción, fecha e importe o porcentaje; ej. "Seña 50 % hoy", "Saldo contra
   entrega 15/11"), **entonces** queda asociado al proyecto. El plan es opcional y no
   cambia la deuda del cliente.
2. **Dado** un plan de cobros, **entonces** los cobros del proyecto se aplican a las
   cuotas en orden de fecha, y cada cuota se ve como *pagada*, *parcial*, *pendiente*
   o *vencida*.
3. **Dado** una cuota vencida sin pagar, **entonces** el proyecto se destaca y la cuota
   aparece en el reporte de deudores (ver [009](../009-reportes/spec.md)).
4. **Dado** un plan cuya suma no coincide con el monto (ej. después de un adicional),
   **entonces** el sistema lo advierte y proyecta la diferencia en la fecha estimada
   de entrega.

### Historia 4: Proyectos en curso al empezar a usar el sistema (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un proyecto que ya estaba en marcha, **cuando** el Administrador lo da de
   alta como **proyecto en curso** con su monto acordado y lo **cobrado antes del
   sistema**, **entonces** el proyecto queda con ese monto y ese saldo pendiente, sin
   necesidad de un presupuesto detallado.
2. **Dado** ese alta, **entonces** se generan en la cuenta corriente del cliente un
   movimiento "Monto inicial" y otro "Cobrado antes del sistema", **sin** movimiento
   de caja.
3. **Dado** un proyecto en curso, **entonces** a partir de ahí funciona igual que
   cualquier otro: cobros, ajustes, plan de cobros, costos.

### Historia 5: Costos y margen (Prioridad: P2)

**Escenarios de aceptación**:

1. **Dado** compras, cargos a empleados o gastos directos imputados al proyecto,
   **cuando** el Administrador abre la ficha, **entonces** ve la lista de costos por
   categoría, el total de costos y el **margen** (monto − costos) en importe y porcentaje.
2. **Dado** costos en una moneda distinta de la del proyecto, **entonces** se muestran
   agrupados por moneda y el margen consolidado solicita un tipo de cambio de
   referencia, que se usa solo para esa consulta y no se guarda.
3. **Dado** un Operador, **entonces** no ve costos ni margen.

### Historia 6: Notas y adjuntos (Prioridad: P2)

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
- La moneda del proyecto solo puede cambiarse mientras no tenga presupuesto aprobado
  ni cobros; al cambiarla, los presupuestos en borrador pasan a la nueva moneda sin
  convertir sus precios, y el sistema lo advierte.
- Pasar un proyecto con presupuesto aprobado a *perdido* no elimina la deuda: el
  sistema sugiere registrar una bonificación o anular la aprobación.

## Requisitos

### Requisitos funcionales

- **FR-001**: El sistema DEBE permitir crear proyectos asociados a un cliente,
  indicando obligatoriamente la moneda del proyecto (ARS o USD).
- **FR-002**: El sistema DEBE mantener la etapa actual y el historial de cambios de etapa.
- **FR-003**: El sistema DEBE calcular el monto del proyecto como total del presupuesto
  aprobado (o monto acordado, en un proyecto en curso) más adicionales menos
  bonificaciones.
- **FR-004**: El sistema DEBE calcular cobrado y saldo pendiente a partir de la cuenta
  corriente del cliente imputada al proyecto.
- **FR-005**: El sistema DEBE calcular costos (a partir de imputaciones) y margen,
  visibles solo para Administradores.
- **FR-006**: El sistema DEBE permitir notas y adjuntos (imágenes y PDF, hasta 15 MB
  cada uno).
- **FR-007**: El sistema DEBE ofrecer vistas de tablero y de lista con filtros y búsqueda.
- **FR-008**: El sistema DEBE numerar los proyectos correlativamente por empresa
  (ej. P-0001).
- **FR-009**: El sistema DEBE permitir un plan de cobros opcional por proyecto y
  aplicar los cobros a sus cuotas por orden de fecha.
- **FR-010**: El sistema DEBE permitir dar de alta proyectos en curso con monto
  acordado y cobrado previo, sin afectar cuentas de dinero.

### Entidades clave

- **Proyecto (Project)**: número, nombre, cliente, etapa, dirección de obra, fecha
  estimada de entrega, descripción, moneda del proyecto, equivalente de referencia
  (opcional, informativo: importe en la otra moneda).
- **Ajuste de proyecto (ProjectAdjustment)**: tipo (adicional/bonificación),
  descripción, importe.
- **Cuota del plan de cobros (PaymentScheduleItem)**: descripción, fecha, importe.
- **Nota (Note)**, **Adjunto (Attachment)**, **Historial de etapas (StageChange)**.

## Criterios de éxito

- **SC-001**: El dueño responde "¿cuánto falta cobrar de la obra X?" en menos de 10
  segundos desde el celular.
- **SC-002**: El margen de cada proyecto se obtiene sin planillas externas.

## Supuestos

- El IVA no se discrimina en costos: el margen se calcula con importes totales.
- Un proyecto tiene una sola moneda para su monto y su saldo, aunque se cobre en
  ambas monedas.
