# Spec: Cuentas de dinero y movimientos de caja

**Rama**: `006-cuentas-dinero`
**Creada**: 2026-09-27
**Estado**: Borrador
**Entrada**: La empresa necesita saber cuánto dinero tiene y dónde (efectivo, bancos,
billeteras virtuales), en pesos y en dólares, y registrar ingresos, gastos y
transferencias entre sus cuentas.

## Clarificaciones

### Sesión 2026-09-27

- P: ¿Hay fecha de cierre que bloquee cargas o anulaciones? → R: No. Solo el Administrador anula, sin límite de antigüedad.
- P: ¿Cómo se manejan los cheques? → R: Con una cartera de cheques; se modela con tipos especiales de cuenta de dinero (spec 010).

## Escenarios de usuario y pruebas

### Historia 1: Crear cuentas de dinero (Prioridad: P1)

**Por qué esta prioridad**: todo movimiento de dinero ocurre en una cuenta.

**Prueba independiente**: crear "Caja efectivo USD" con saldo inicial y verlo en el
listado de cuentas.

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** crea una cuenta con nombre, tipo (efectivo,
   banco, billetera virtual, otra), moneda y saldo inicial con fecha, **entonces** la
   cuenta aparece con ese saldo.
2. **Dado** una cuenta con movimientos, **entonces** su moneda no se puede cambiar.
3. **Dado** una cuenta que ya no se usa, **cuando** tiene saldo cero, **entonces** se
   puede archivar.
4. **Dado** el uso de cheques, **entonces** existen además dos tipos especiales de
   cuenta que crea el sistema: "Cartera de cheques" y "Cheques propios a debitar"
   (ver [010](../010-cheques/spec.md)). No se crean a mano.

### Historia 2: Registrar gastos e ingresos directos (Prioridad: P1)

Egresos o ingresos que no afectan la cuenta corriente de un tercero (combustible,
impuestos, alquiler pagado en el momento, una venta de rezago).

**Escenarios de aceptación**:

1. **Dado** un Administrador, **cuando** registra un gasto con fecha, cuenta de dinero,
   importe, categoría y descripción, **entonces** el saldo de la cuenta disminuye.
2. **Dado** un gasto, **cuando** se lo imputa a uno o más proyectos (por importe o
   porcentaje), **entonces** suma a los costos de esos proyectos. La suma de lo
   imputado no puede superar el importe del gasto.
3. **Dado** un Administrador, **cuando** registra un ingreso directo con categoría,
   **entonces** el saldo de la cuenta aumenta.
4. **Dado** un gasto, **cuando** se adjunta una foto del comprobante, **entonces** queda
   asociada al movimiento.

### Historia 3: Transferencias y cambio de moneda (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** dos cuentas de la misma moneda, **cuando** se transfiere un importe,
   **entonces** baja el saldo del origen y sube el del destino en el mismo importe.
2. **Dado** una cuenta en ARS y otra en USD, **cuando** se registra una compra o venta
   de dólares indicando el **importe que sale** y el **importe que entra** (ej. salen
   ARS 1.200.000, entran USD 1.000), **entonces** se registran ambos movimientos con
   esos importes. Mientras se carga, el sistema muestra el tipo de cambio implícito
   (≈ 1.200 ARS/USD) como control, sin guardarlo.
3. **Dado** una transferencia, **entonces** no aparece como ingreso ni gasto en los
   reportes de flujo de caja.

### Historia 4: Consultar movimientos y saldos (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** una cuenta, **cuando** se abre, **entonces** se ve su saldo actual y sus
   movimientos con saldo acumulado, filtrables por fecha, categoría, tercero y proyecto.
2. **Dado** el panel de cuentas, **entonces** se ve el saldo de cada cuenta y el total
   por moneda.

### Historia 5: Anular movimientos (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un movimiento, **cuando** un Administrador lo anula indicando un motivo,
   **entonces** deja de computar en saldos y reportes, se muestra tachado con el motivo
   y se revierten sus efectos asociados (cuenta corriente, imputaciones de costo).
2. **Dado** un movimiento que es parte de una operación compuesta (transferencia, cobro,
   pago), **cuando** se anula, **entonces** se anula la operación completa.
3. **Dado** un movimiento anulado, **entonces** no puede "desanularse": se registra uno nuevo.

### Historia 6: Arqueo (Prioridad: P3)

**Escenarios de aceptación**:

1. **Dado** una cuenta, **cuando** el Administrador ingresa el saldo real contado,
   **entonces** el sistema muestra la diferencia y, si se confirma, registra un
   movimiento de ajuste con categoría "Ajuste de arqueo".

### Casos borde

- Un movimiento que deja una cuenta en saldo negativo se permite (ej. descubierto
  bancario), pero se advierte.
- Fechas futuras: se permiten hasta 1 día después de hoy para contemplar zonas horarias.
- Importes cero o negativos no se permiten; el sentido lo da el tipo de operación.

## Requisitos

### Requisitos funcionales

- **FR-001**: El sistema DEBE permitir cuentas de dinero con una única moneda (ARS o USD).
- **FR-002**: El sistema DEBE registrar movimientos con: fecha, cuenta, sentido
  (entrada/salida), importe, moneda, categoría, descripción, tercero y proyecto
  opcionales, usuario y adjunto opcional.
- **FR-003**: El sistema DEBE soportar transferencias entre cuentas propias. Si las
  monedas difieren, el usuario informa el importe de salida y el de entrada; no se
  registra tipo de cambio (Constitución, principio IV).
- **FR-004**: El sistema DEBE permitir imputar gastos directos a uno o más proyectos.
- **FR-005**: El sistema DEBE derivar los saldos de los movimientos no anulados
  (Constitución, principio IV).
- **FR-006**: El sistema DEBE implementar la anulación de operaciones completas, con
  motivo y auditoría.
- **FR-007**: Toda operación DEBE ejecutarse en una única transacción.
- **FR-008**: Los movimientos de caja solo son visibles para Administradores. Un
  Operador que registra un cobro elige la cuenta de destino, pero no ve su saldo.

### Entidades clave

- **Cuenta de dinero (MoneyAccount)**: nombre, tipo, moneda, saldo inicial, fecha de
  saldo inicial, archivada.
- **Movimiento de caja (CashMovement)**: ver FR-002; referencia a la operación que lo
  generó; estado (vigente/anulado), motivo de anulación.
- **Operación (Operation)**: agrupa los registros que genera una acción del usuario
  (ej. transferencia = 2 movimientos) para anularlos juntos.
- **Imputación de costo (CostAllocation)**: proyecto, importe, moneda.

## Criterios de éxito

- **SC-001**: Registrar un gasto desde el celular lleva menos de 20 segundos.
- **SC-002**: El saldo de cada cuenta coincide siempre con la suma de sus movimientos
  vigentes (verificado por tests y por una tarea de consistencia).

## Supuestos

- Monedas del MVP: ARS y USD. El modelo admite agregar más monedas sin cambios de diseño.
- No hay cierre de períodos contables en el MVP.
