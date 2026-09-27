# Spec: Cuentas corrientes (clientes, proveedores, empleados, socios)

**Rama**: `007-cuentas-corrientes`
**Creada**: 2026-09-27
**Estado**: Borrador
**Entrada**: La empresa necesita saber quién le debe y a quién le debe. Cada tercero
tiene una cuenta corriente por rol y moneda, alimentada por cobros, compras, pagos,
cargos a empleados, aportes y retiros de socios. Ver la
[tabla de efectos](../../docs/modelo-dominio.md#efectos-de-cada-operación).

## Escenarios de usuario y pruebas

### Historia 1: Cobrar a un cliente (Prioridad: P1)

**Por qué esta prioridad**: el cobro de señas y saldos es el ingreso principal.

**Prueba independiente**: registrar una seña para un proyecto y ver que baja su saldo
pendiente y sube la caja.

**Escenarios de aceptación**:

1. **Dado** un cliente con un proyecto aprobado, **cuando** un usuario registra un cobro
   con fecha, cuenta de dinero de destino, importe y proyecto, **entonces** la cuenta
   de dinero aumenta y la cuenta corriente del cliente y el saldo del proyecto disminuyen.
2. **Dado** un proyecto en USD, **cuando** se cobra en una cuenta en ARS, **entonces**
   el sistema exige tipo de cambio, muestra el equivalente en USD que se cancela y
   guarda ambos importes.
3. **Dado** un cobro sin proyecto, **entonces** se registra "a cuenta" del cliente y
   disminuye su saldo general.
4. **Dado** un cobro mayor que el saldo pendiente, **entonces** el sistema lo advierte
   y, si se confirma, el cliente queda con saldo a favor.
5. **Dado** un cobro, **cuando** se desea, **entonces** se puede compartir un
   comprobante simple (no fiscal) por WhatsApp.

### Historia 2: Compras y pagos a proveedores (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un proveedor, **cuando** el Administrador registra una compra con fecha,
   número de comprobante (opcional), importe, moneda, categoría, vencimiento (opcional)
   y descripción, **entonces** la deuda con el proveedor aumenta ("Le debés").
2. **Dado** una compra, **cuando** se imputa total o parcialmente a uno o más
   proyectos, **entonces** suma a sus costos.
3. **Dado** un proveedor con deuda, **cuando** se registra un pago desde una cuenta de
   dinero, **entonces** la caja disminuye y la deuda baja. El pago se aplica a las
   compras pendientes más antiguas, salvo que el usuario elija a cuáles.
4. **Dado** una compra pagada en el momento, **cuando** se usa "Compra de contado",
   **entonces** en un solo paso se registran la compra y el pago.
5. **Dado** una compra en USD pagada desde una cuenta en ARS, **entonces** se exige
   tipo de cambio.

### Historia 3: Empleados: cargos, pagos y adelantos (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un empleado, **cuando** se registra un cargo (ej. "Jornales semana 12",
   importe, categoría), **entonces** aumenta lo que la empresa le debe.
2. **Dado** un cargo, **cuando** se lo imputa a uno o más proyectos, **entonces** suma
   a sus costos de mano de obra.
3. **Dado** un empleado, **cuando** se le paga, **entonces** baja lo que se le debe. Si
   el pago supera la deuda, el excedente queda como **adelanto** ("Te debe").
4. **Dado** un pago sin cargo previo, **cuando** se usa "Pago directo", **entonces** se
   registran el cargo y el pago en un paso, con imputación opcional a proyectos.

### Historia 4: Socios: aportes y retiros (Prioridad: P2)

**Escenarios de aceptación**:

1. **Dado** un socio, **cuando** aporta dinero a una cuenta de la empresa, **entonces**
   la caja aumenta y crece su saldo aportado.
2. **Dado** un socio, **cuando** retira dinero, **entonces** la caja disminuye y
   disminuye su saldo aportado neto.
3. **Dado** la ficha del socio, **entonces** se muestran total aportado, total retirado
   y neto, por moneda.

### Historia 5: Consultar una cuenta corriente (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un tercero, **cuando** se abre su cuenta corriente, **entonces** se ven los
   movimientos con fecha, concepto, importe, saldo acumulado y proyecto, por moneda.
2. **Dado** una cuenta corriente, **cuando** se pide un resumen, **entonces** se genera
   un PDF compartible por WhatsApp (ej. estado de cuenta para el cliente).

### Historia 6: Anulaciones (Prioridad: P1)

**Escenarios de aceptación**:

1. **Dado** un cobro, pago, compra, cargo, aporte o retiro, **cuando** un Administrador
   lo anula con motivo, **entonces** se revierten todos sus efectos (caja, cuenta
   corriente, imputaciones) en una sola transacción.
2. **Dado** una compra con pagos aplicados, **cuando** se anula, **entonces** los pagos
   quedan como saldo a favor de la empresa con ese proveedor.

### Casos borde

- Un cobro no puede imputarse a un proyecto de otro cliente.
- Un cobro en la misma moneda que el proyecto no pide tipo de cambio.
- Una compra imputada a un proyecto de otra moneda: la imputación queda en la moneda
  de la compra (ver margen en [004](../004-proyectos/spec.md)).
- Los saldos se calculan por moneda: nunca se suman ARS y USD sin tipo de cambio explícito.

## Requisitos

### Requisitos funcionales

- **FR-001**: El sistema DEBE mantener una cuenta corriente por tercero, rol y moneda,
  cuyo saldo se deriva de sus movimientos vigentes.
- **FR-002**: El sistema DEBE soportar las operaciones: aprobación de presupuesto,
  ajuste de proyecto, cobro, compra, pago a proveedor, compra de contado, cargo a
  empleado, pago a empleado, pago directo a empleado, aporte y retiro.
- **FR-003**: Cada operación DEBE generar, de forma atómica, los registros indicados en
  la [tabla de efectos](../../docs/modelo-dominio.md#efectos-de-cada-operación).
- **FR-004**: Toda operación que vincule importes de distinta moneda DEBE registrar el
  tipo de cambio y ambos importes.
- **FR-005**: El sistema DEBE permitir imputar compras y cargos a uno o más proyectos
  sin superar su importe.
- **FR-006**: El sistema DEBE aplicar los pagos a compras pendientes por antigüedad
  (FIFO) o según la selección del usuario.
- **FR-007**: El sistema DEBE sugerir como categoría del pago la de la compra que
  cancela si es una sola; si no, la categoría de sistema correspondiente.
- **FR-008**: El Operador solo puede registrar cobros de clientes y ver saldos de
  clientes y proyectos (ver [001](../001-empresas-usuarios/spec.md)).
- **FR-009**: El sistema DEBE mostrar los saldos con lenguaje llano: "Te debe" / "Le debés".

### Entidades clave

- **Cuenta corriente (PartyLedger)**: tercero, rol, moneda.
- **Movimiento de cuenta corriente (LedgerEntry)**: cuenta corriente, fecha, tipo de
  operación, importe con signo, proyecto (opcional), operación de origen, estado.
- **Compra (Purchase)**: proveedor, fecha, comprobante, importe, moneda, categoría,
  vencimiento, saldo pendiente derivado.
- **Cargo a empleado (EmployeeCharge)**: empleado, fecha, concepto, importe, categoría.
- **Aplicación de pago (PaymentApplication)**: pago, compra, importe.

## Criterios de éxito

- **SC-001**: Los saldos de clientes y proveedores coinciden con los que la empresa
  llevaba en planillas al migrar (validado con el primer cliente).
- **SC-002**: Registrar un cobro desde el celular lleva menos de 20 segundos.

## Supuestos

- Los saldos iniciales de terceros al comenzar a usar el sistema se cargan como un
  movimiento de tipo "Saldo inicial".
- No se calculan intereses por mora.
- No se reparten utilidades entre socios en el MVP.
